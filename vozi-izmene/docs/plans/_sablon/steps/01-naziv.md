# Korak 01: NAZIV

Status: TODO
(TODO | U TOKU | GOTOVO | GOTOVO-DRUGAČIJE | BLOKIRAN)

## Šta
Jedna rečenica, rezultat koji se vidi.

## Zašto
Jedna do dve rečenice. Dovoljno da izvršilac razume nameru i razumno odstupi ako mora.

## Gde
- Čitaj: `app/modules/x/CLAUDE.md`, `app/modules/x/service.py`
- Menjaj: `app/modules/x/service.py`, `app/modules/x/tests/test_x.py`
- Ne diraj: `app/modules/y/` (razlog)
- Menja testove: (samo ako korak namerno menja ponašanje: imena postojećih testova koje sme da promeni ili obriše; inače izbriši ovaj red)

## Kako
Konkretno, po tačkama. Nazivi funkcija, potpisi, ponašanje na greške. Skica koda ako je dvosmisleno, ili: "Polazna tačka: `proto/x.py` (prošao test Y). Prilagodi konvencijama modula."
1. ...
2. ...

## Testovi
Izvršilac ih piše PRE koda, tačno pod ovim imenima. Jedan red: ime, pa šta tvrdi (ulaz → očekivani ishod).
- `test_x_vraca_y`: ... → ...
- `test_x_nepostojeci_404`: ... → 404

## Provera
```
pytest app/modules/x -q -k "test_x_vraca_y or test_x_nepostojeci_404"
```
Očekivano: navedeni testovi prolaze.

## Napomene od prethodnih koraka
(upisuju prethodni koraci pri zatvaranju; jedan red, "od koraka NN: ..."; prazno ako nema)

## Izveštaj (popunjava izvršilac)
- Urađeno:
- Odstupanja od plana (i zašto):
- Otvoreno (samo ono što je važno jedino čoveku; ostalo je prosleđeno, vidi ispod):
- Prosleđeno: (npr. "napomena u korak 03", "Poznati problemi modula x", "IDEJE.md")
- Commit:
