"""Vozi aktivni plan bez coveka: /korak, /provera, /korak ... svaki u novoj sesiji.

    python .claude/scripts/vozi.py                 dok plan ne bude gotov ili dok nesto ne zapne
    python .claude/scripts/vozi.py --max-koraka 3  najvise 3 koraka pa stani
    python .claude/scripts/vozi.py --suvo          samo kazi sta bi uradio

Svaki poziv je `claude -p "/korak"`: nov proces = cist kontekst, isto kao /clear.
Stanje se cita iz fajlova plana (isto sto cita hook status.py), ne iz razgovora.
Staje kad: plan zavrsen, korak BLOKIRAN i posle /provera, korak nije commit-ovan,
radni direktorijum nije cist, ili istekne --max-koraka. Ctrl+C prekida u svakom trenutku."""

from __future__ import annotations

import argparse
import re
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PLANS = ROOT / "docs" / "plans"


def git(*args: str) -> str:
    r = subprocess.run(
        ["git", *args], cwd=ROOT, capture_output=True, text=True, encoding="utf-8", errors="replace"
    )
    return r.stdout.strip()


def aktivan_plan() -> Path | None:
    readme = PLANS / "README.md"
    if not readme.exists():
        return None
    m = re.search(r"^Aktivan:\s*(.+)$", readme.read_text(encoding="utf-8"), re.MULTILINE)
    ime = m.group(1).strip().strip("*` ") if m else ""
    plan = PLANS / ime / "PLAN.md"
    return plan if ime and plan.exists() else None


def koraci(plan: Path) -> list[tuple[bool, str]]:
    """[(gotov, 'NN'), ...] iz liste koraka u PLAN.md."""
    text = plan.read_text(encoding="utf-8")
    return [(s == "x", nn) for s, nn in re.findall(r"^- \[( |x)\] (\d+)", text, re.MULTILINE)]


def provereno_do(plan: Path) -> int:
    m = re.search(r"^Provereno do:\s*(\d+)", plan.read_text(encoding="utf-8"), re.MULTILINE)
    return int(m.group(1)) if m else 0


def status_koraka(plan: Path, nn: str) -> str:
    fajlovi = sorted((plan.parent / "steps").glob(f"{nn}-*.md"))
    if not fajlovi:
        return "?"
    m = re.search(r"^Status:\s*(\S+)", fajlovi[0].read_text(encoding="utf-8"), re.MULTILINE)
    return m.group(1) if m else "?"


def treba_provera(plan: Path) -> bool:
    """Ima nepregledanih koraka koji nisu cist GOTOVO (isto pravilo kao u rad.md)."""
    od = provereno_do(plan)
    for gotov, nn in koraci(plan):
        if int(nn) <= od:
            continue
        status = status_koraka(plan, nn)
        if status == "BLOKIRAN" or (gotov and status != "GOTOVO"):
            return True
    return False


def claude(komanda: str, args: argparse.Namespace) -> int:
    exe = shutil.which("claude")
    if not exe:
        sys.exit("vozi: nema `claude` u PATH-u")
    cmd = [exe, "-p", komanda, "--permission-mode", "acceptEdits"]
    if args.budzet:
        cmd += ["--max-budget-usd", str(args.budzet)]
    print(f"\n=== {komanda} ===", flush=True)
    if args.suvo:
        print("(suvo) " + " ".join(cmd))
        return 0
    proc = subprocess.Popen(
        cmd,
        cwd=ROOT,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    nepoveren = False
    for red in proc.stdout:
        print(red, end="", flush=True)
        nepoveren |= "has not been trusted" in red
    proc.wait()
    if nepoveren:
        # bez trust-a claude -p ignorise `allow` iz settings.json, pa pytest i git commit ne prolaze
        stani(
            "folder nije oznacen kao poveren. Pokreni `claude` ovde jednom interaktivno, "
            "prihvati pitanje o poverenju, pa ponovo."
        )
    return proc.returncode


def stani(poruka: str) -> None:
    print(f"\nvozi: STOP. {poruka}")
    sys.exit(1)


def main() -> None:
    p = argparse.ArgumentParser(description="Vozi aktivni plan: /korak i /provera bez rucnog /clear.")
    p.add_argument("--max-koraka", type=int, default=50)
    p.add_argument("--budzet", type=float, help="najvise USD po jednom pozivu claude-a")
    p.add_argument("--suvo", action="store_true", help="samo ispisi sta bi pokrenuo")
    args = p.parse_args()
    sys.stdout.reconfigure(encoding="utf-8")

    uradjeno = 0
    blokiran_jednom: set[str] = set()
    while True:
        plan = aktivan_plan()
        if plan is None:
            stani("nema aktivnog plana (docs/plans/README.md, red `Aktivan:`). Napravi ga sa /plan.")
        if git("status", "--porcelain"):
            stani("radni direktorijum nije cist. Commit-uj ili vrati izmene pa pokreni ponovo.")

        if treba_provera(plan):
            claude("/provera", args)
            if args.suvo:
                return
            if treba_provera(plan):
                stani("/provera nije pomerila `Provereno do:`. Pogledaj izlaz iznad.")

        sledeci = next((nn for gotov, nn in koraci(plan) if not gotov), None)
        if sledeci is None:
            print(f"\nvozi: plan {plan.parent.name} je zavrsen.")
            return
        if uradjeno >= args.max_koraka:
            print(f"\nvozi: uradjeno {uradjeno} koraka (--max-koraka). Sledeci je {sledeci}.")
            return

        pre = git("rev-parse", "HEAD")
        claude(f"/korak {sledeci}", args)
        if args.suvo:
            return
        uradjeno += 1
        status = status_koraka(plan, sledeci)
        print(f"\nvozi: korak {sledeci} -> {status}", flush=True)

        if status == "BLOKIRAN":
            if sledeci in blokiran_jednom:
                korak = f"{plan.parent.name}/steps/{sledeci}-*.md"
                stani(f"korak {sledeci} je BLOKIRAN i posle /provera. Treba covek: {korak}")
            blokiran_jednom.add(sledeci)
            if git("status", "--porcelain"):
                git("add", "-A")
                git("commit", "-q", "-m", f"plan {plan.parent.name[:3]} korak {sledeci}: BLOKIRAN")
            continue  # sledeci krug pokrece /provera, pa ponovo ovaj korak
        if status not in ("GOTOVO", "GOTOVO-DRUGAČIJE"):
            stani(f"korak {sledeci} nije zatvoren (Status: {status}). Pogledaj izlaz iznad.")
        if git("rev-parse", "HEAD") == pre:
            stani(f"korak {sledeci} je {status}, ali nema novog commit-a.")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\nvozi: prekinuto (Ctrl+C). Stanje je u fajlovima plana; nastavi istom komandom.")
