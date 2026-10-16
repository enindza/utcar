---
name: plan
description: Napravi plan za veću izmenu, podeljen na male samostalne korake koje kasnije izvršava /korak (i slabiji model). Pokreće samo korisnik.
disable-model-invocation: true
model: opus
---
Cilj plana: $ARGUMENTS

Ti si arhitekta. Praviš plan koji će izvršiti drugi model, korak po korak, u odvojenim sesijama bez ikakvog sećanja na ovu.

## 1. Razumi obim (čitaj samo ovo)
- `CLAUDE.md` (koren), `docs/ARHITEKTURA.md`, `docs/plans/IDEJE.md` (stavke koje se tiču ovog cilja uključi u plan i obriši ih iz IDEJE.md).
- Ako stavka iz IDEJE.md u koloni "odakle" pokazuje na `istrazivanja/<ime>/`, pročitaj `docs/plans/istrazivanja/<ime>/BELESKA.md` celu. Skripte iz tog foldera samo ako ti treba potpis.
- `CLAUDE.md` samo onih modula koje izmena dira.
- Konkretne fajlove samo ako ti treba potpis funkcije. Za pretragu koristi `istrazivac` subagenta, najviše 3 poziva u ovoj sesiji, samo za konkretna pitanja ("gde je `Item` model", "ko zove `service.create_item`").
- Ako obim nije jasan (koji moduli, koji interfejsi), pitaj korisnika pre pisanja. Ne pretpostavljaj. Pitanja završi redom: "Odgovori sa `/plan <odgovori>`" (tako nastavak ostaje na Opusu).

## 2. Ako rešenje treba prvo isprobati
- Ne piši kod koji nisi pokrenuo. Ali kod koji si pokrenuo i testirao ide u plan kao činjenica.
- Istraživanje radi u `docs/plans/NNN-kratko-ime/proto/`, nikad u `app/`. Tamo smeju skripte, testovi, beleške o odbačenim varijantama (jedan red po varijanti: šta i zašto ne).
- Kad rešenje radi: proto ostaje u folderu plana i commit-uje se sa planom. Korak koji ga koristi pod "Kako" kaže: "Polazna tačka: `proto/x.py` (prošao test Y). Prilagodi konvencijama modula, ne kopiraj slepo." Provera koraka uključuje test koji je proto već prošao.
- Ako je istraživanje već urađeno van plana (`docs/plans/istrazivanja/<ime>/`, sačuvano sa `/zabelezi`): `git mv docs/plans/istrazivanja/<ime> docs/plans/NNN-kratko-ime/proto/<ime>` i dalje ga tretiraj kao svoj proto. Ono što `BELESKA.md` navodi pod "Radi" ne testiraj ponovo, nego prenesi u "Kako" i "Provera" koraka; ono pod "Odbačeno" ne probaj ponovo. "Nije provereno" i "Zavisnosti" idu u korak koji ih se tiče.

## 3. Napravi plan
- Folder `docs/plans/NNN-kratko-ime/` (NNN = sledeći broj po `docs/plans/README.md`). Kopiraj `docs/plans/_sablon/` i popuni.
- `PLAN.md`: cilj, obim (dira / ne dira), interfejsi koji se menjaju, `Provereno do: 00`, lista koraka sa `- [ ]`.
- Za svaki korak jedan fajl `steps/NN-ime.md` po šablonu.

## 4. Pravila za korak (najvažnije)
- Jedan korak = jedna sesija = jedan commit. Ako ne staje u oko 30 min rada ili dira više od 3-4 fajla, podeli ga.
- Korak mora biti izvršiv bez čitanja drugih koraka. Navedi tačno: koje fajlove čitati, koje menjati, koje ne dirati.
- "Zašto" u 1-2 rečenice. Izvršilac mora da razume nameru da bi smeo razumno da odstupi.
- "Kako" konkretno: nazivi funkcija, potpisi, ponašanje na greške. Skica koda gde je dvosmisleno, ili upućivanje na proto.
- Kod koji već postoji (proto, stariji modul, uzorak iz drugog projekta) prosledi u korak: putanju ili ceo isečak, uz napomenu odakle je i da li je proveren. Ne prepričavaj ga; izvršilac ga bez izvora ne može otkriti.
- Kod kog još nema ne piši umesto izvršioca.
- "Testovi": ti odlučuješ šta se proverava, ne izvršilac. Za svaki korak koji menja kod navedi nove testove po imenu i šta svaki tvrdi (ulaz → očekivani ishod), uključujući bar jedan slučaj greške. Ponašanje opiši tako da test ne može da prođe a da posao ne bude urađen.
- "Provera" je komanda koja se pokreće i očekivan ishod. Bez provere korak ne postoji. Komanda pokreće testove iz "Testovi" po imenu (`pytest putanja -q -k "ime_a or ime_b"`), ne goli `pytest -q` (osim u završnom integracionom koraku), jer `vozi.py` po njoj proverava korak bez čitanja izveštaja.
- Postojeći testovi su ugovor: korak ne sme da ih menja ni briše. Ako plan namerno menja ponašanje koje neki test čuva, navedi taj test u redu `- Menja testove:` pod "Gde". `vozi.py` obara korak koji dira test koji tu nije naveden.
- Prvi korak koji pravi novi modul: uključi i pisanje njegovog `CLAUDE.md` (po `app/modules/_sablon/CLAUDE.md`).
- Svaki korak koji menja interfejs između modula: uključi ažuriranje `docs/ARHITEKTURA.md` u tom koraku.
- Poslednji korak: integraciona provera (`pytest -q`) i čišćenje.
- Redosled: prvo ono što drugi koraci koriste (modeli, sheme, service), pa ulazni sloj (ruteri ili CLI komande), pa integracija.

## 5. Završi
- U `docs/plans/README.md` postavi `Aktivan: NNN-kratko-ime` i dodaj red u tabelu.
- `git add docs/plans && git commit -m "plan NNN: kratko ime"`
- Ispiši listu koraka, jedan red po koraku, i završi tačno ovim: "Sledeće: /clear, pa /korak".
