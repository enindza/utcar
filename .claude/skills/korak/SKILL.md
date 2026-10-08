---
name: korak
description: Izvrši sledeći korak plana realizacije iz plan/STANJE.md (jedan korak po pozivu), ažuriraj stanje, commit i push. Koristi se posle /clear kada korisnik napiše /korak.
---

# /korak – izvrši sledeći korak plana

Radiš **tačno jedan** korak plana i staješ. Kontekst je verovatno očišćen
(`/clear`), pa sve što znaš o prethodnim koracima dolazi iz fajlova – ne
pretpostavljaj ništa što tamo ne piše.

Argument (opciono): broj koraka, npr. `/korak 4` – tada radi taj korak (samo ako
su svi prethodni ✅ ili ako korisnik eksplicitno traži), inače uzmi „Sledeći korak“.

## 1. Učitaj stanje

1. `git status` i `git log --oneline -5`. Ako postoje necommit-ovane izmene,
   prvo pogledaj `STANJE.md` – ako je neki korak 🔧 (prekinut), nastavi njega.
2. Povuci najnovije: `git pull origin <grana iz STANJE.md>` (retry pri mrežnoj
   grešci 2s/4s/8s/16s).
3. Pročitaj **ceo** `plan/STANJE.md` (sledeći korak, promene ugovora, otvorena
   pitanja, dnevnik – posebno unos prethodnog koraka).
4. Pročitaj u `plan/PLAN.md`: sekcije „Odluke“, „Tehničke odluke“, „Ugovori“ i
   **samo sekciju svog koraka**.
5. Ako u „Otvorena pitanja“ postoji nerešeno pitanje koje blokira ovaj korak –
   postavi ga korisniku (AskUserQuestion) i stani.

## 2. Označi početak

U `STANJE.md` postavi status koraka na 🔧 (u toku). Ne commit-uj samo zbog ovoga.

## 3. Implementiraj

- Uvek `export GOTOOLCHAIN=local` pre `go` komandi.
- Drži se ugovora iz `PLAN.md` + „Promene ugovora“ iz `STANJE.md`.
- Radi **samo** ono što je u sekciji tvog koraka. Ne radi unapred sledeće korake,
  ne diraj `pusher.go`. Ako primetiš problem van koraka – upiši ga u
  „Poznati problemi“.
- Ako moraš da odstupiš od ugovora – odstupi minimalno i upiši to u
  „Promene ugovora“ (sa brojem koraka i razlogom).
- Prati stil postojećeg koda; `gofmt` na fajlove koje menjaš.

## 4. Proveri

```
export GOTOOLCHAIN=local
gofmt -l .        # ne sme da prijavi fajlove koje si ti menjao
go vet ./...
go test ./...
```

Sve mora da prođe. Ako ne prolazi, popravi pre nego što nastaviš. Pre commit-a
pročitaj svoj diff (`git diff`) kritički.

## 5. Ažuriraj `plan/STANJE.md`

- Status koraka → ✅, kolona Commit → kratak opis (hash se ne zna pre commit-a;
  može ostati prazno ili se popuniti u sledećem koraku).
- „Sledeći korak“ → naziv sledećeg koraka; njegov status → ⏳.
- Dodaj unos **na vrh** dnevnika po šablonu: šta je urađeno, fajlovi,
  odluke/odstupanja, provera, i **„Za sledeći korak“** – sve što sledeći korak
  treba da zna (imena funkcija, zamke, primeri test podataka). Ovo je jedini
  način da sledeći korak sazna nešto što nije u planu.
- Ako je ovo bio poslednji korak: „Sledeći korak“ → „Nema – plan završen“.

## 6. Commit i push

```
git add -A
git commit -m "Korak N: <naziv>" -m "<kratak opis>"   # + attribution linije iz sistema
git push -u origin <grana>
```

Retry push-a samo pri mrežnoj grešci (2s/4s/8s/16s). Ne pravi PR (osim ako
korisnik traži; korak 10 ga samo predlaže).

## 7. Javi korisniku i stani

Kratko, na srpskom:
- šta je urađeno u koraku (2–4 stavke),
- rezultat testova,
- šta je sledeći korak,
- poruka: **„Sada uradi `/clear` pa `/korak` za sledeći korak.“**

Ne počinji sledeći korak.
