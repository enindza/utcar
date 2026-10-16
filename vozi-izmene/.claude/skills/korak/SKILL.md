---
name: korak
description: Izvrši sledeći nezavršeni korak aktivnog plana, proveri ga, upiši izveštaj, ažuriraj navigaciju, commit-uj. Pokreće samo korisnik.
disable-model-invocation: true
model: sonnet
---
Argument (opciono): broj koraka. $ARGUMENTS
Bez argumenta = prvi neoznačen `- [ ]` korak.

## Šta čitaš (i ništa više)
1. `docs/plans/README.md`, red `Aktivan:`.
2. `docs/plans/<aktivan>/PLAN.md`: Cilj, Obim, lista koraka. Uzmi traženi korak.
3. `docs/plans/<aktivan>/steps/NN-*.md` za taj korak, uključujući sekciju "Napomene od prethodnih koraka".
4. Fajlove navedene pod "Gde / Čitaj" i `proto/` fajlove ako ih korak navodi. `CLAUDE.md` modula se učitava sam kad otvoriš fajl u njemu.

Ne čitaj: druge korake, `docs/LOG.md`, stare planove, fajlove van "Gde". Ako ti nešto nedostaje, probaj `Grep`/`Glob` sam; ako ne uspe, najviše jedan poziv `istrazivac` subagenta, pa ciljano čitanje. Zabeleži to u Odstupanja.

## Izvršenje
- U fajlu koraka postavi `Status: U TOKU`.
- Prvo napiši testove iz sekcije "Testovi", tačno pod tim imenima, i pokreni ih: moraju da padnu (funkcija još ne postoji). Tek onda piši kod.
- Postojeće testove i fixture ne menjaj, ne briši i ne preskači (`skip`, `xfail`), osim onih navedenih u redu `Menja testove:`. Ako postojeći test pada zbog tvoje izmene, greška je u kodu ili u planu, ne u testu: popravi kod, ili `Status: BLOKIRAN` i opiši u Otvoreno.
- Radi tačno ono što korak kaže, u navedenim fajlovima.
- Van obima smeš samo ako je nužno da Provera prođe. Zabeleži u Odstupanja.
- Ako nešto nije moguće ili nije jasno: ne izmišljaj i ne zaobilazi. Uradi što možeš, postavi `Status: BLOKIRAN`, opiši u Otvoreno, i stani.
- Pokreni Proveru. Korak nije gotov dok Provera ne prođe. Ako ne prolazi posle 3 pokušaja, BLOKIRAN.

## Zatvaranje (obavezno, ovim redom)
1. Popuni "Izveštaj" u fajlu koraka. `Status: GOTOVO`, ili `GOTOVO-DRUGAČIJE` ako si odstupio (i napiši kako).
2. U `PLAN.md` promeni `- [ ] NN` u `- [x] NN` samo za taj korak (jedan Edit).
3. Ako si dodao, obrisao ili preimenovao fajl, modul, rutu, tabelu, env promenljivu ili javnu funkciju: ažuriraj `CLAUDE.md` tog modula, a po potrebi mapu u korenskom `CLAUDE.md` i `docs/ARHITEKTURA.md`. Tu živi znanje o projektu, ne u logu.
4. Isprazni "Otvoreno": svaka stavka ide na mesto gde će je pročitati onaj koga se tiče. U Izveštaju ostaje samo ono što je važno jedino čoveku.
   | Stavka se tiče | Upiši u |
   |---|---|
   | narednog koraka ovog plana (promenjeno ime, potpis, putanja, preduslov) | sekcija "Napomene od prethodnih koraka" u fajlu **tog** koraka, jedan red sa "od koraka NN:" |
   | koda jednog modula (bug, dug, nedoslednost) | "Poznati problemi / TODO" u `CLAUDE.md` tog modula |
   | budućeg posla van ovog plana | `docs/plans/IDEJE.md`, jedan red, dopisivanjem (`echo >>`) |
   | alata ili konvencije (skill, pravilo, mapa) | odgovarajući `SKILL.md`, `.claude/rules` ili korenski `CLAUDE.md` |
   Ako odstupanje **obesmišljava** neki naredni korak (ne samo menja detalj), ne prepravljaj ga sam: napiši to u napomenu tog koraka i na kraju preporuči `/provera`.
5. Dopiši jedan red u log bez čitanja fajla:
   echo "YYYY-MM-DD | plan NNN | korak NN | GOTOVO | jedna rečenica" >> docs/LOG.md
6. `ruff check . && ruff format .` pa `git add -A && git commit -m "plan NNN korak NN: naziv"`
7. Ispiši korisniku: šta je urađeno (3 reda max), odstupanja, gde si prosledio otvorene stavke. Završi tačno sa:
   - `Status: GOTOVO`: "Sledeće: /clear pa /korak" (ili "Plan završen. Sledeće: /clear").
   - `GOTOVO-DRUGAČIJE` ili `BLOKIRAN`: "Sledeće: /clear, /provera".
