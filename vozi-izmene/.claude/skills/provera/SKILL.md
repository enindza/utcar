---
name: provera
description: Pregledaj izveštaje koraka koji još nisu pregledani i uskladi naredne korake plana sa onim što je stvarno urađeno. Pokreće samo korisnik, jačim modelom.
disable-model-invocation: true
model: opus
---
Napomene korisnika: $ARGUMENTS

Ti si arhitekta koji je napisao plan. Neko drugi ga izvršava i ponekad odstupi. Tvoj posao je da naredni koraci i dalje vode ka Cilju, bez da plan raste.

## Šta čitaš
1. `docs/plans/README.md`, red `Aktivan:`.
2. `PLAN.md` aktivnog plana: Cilj, Obim, `Provereno do: NN`, lista koraka.
3. Fajlove koraka od `NN+1` do poslednjeg označenog `[x]` ili `BLOKIRAN`: samo sekcije Status, Izveštaj i Napomene. To su nepregledani koraci.
4. Fajlove narednih (neoznačenih) koraka, cele.
Ne čitaj: već pregledane korake, `docs/LOG.md`, kod, osim ciljane provere ako izveštaj tvrdi nešto što menja interfejs. Za tu proveru najviše 2 poziva `istrazivac` subagenta.

## Odluči za svaku stavku iz Odstupanja i Otvoreno
- **Ništa**: odstupanje ne utiče na naredne korake. Zabeleži jednom rečju.
- **Ispravi naredni korak na licu mesta**: promeni "Kako", "Gde" ili "Provera" tog koraka. Ovo je podrazumevano.
- **Nov korak**: samo ako je nužan da bi se Cilj ostvario, i tada u PLAN.md pod koracima jedan red zašto je dodat. Ne dodaj korak za "lepo bi bilo".
- **Ukloni korak**: ako je odstupanje već obavilo njegov posao, označi ga `[x]` sa napomenom "obavljeno u koraku NN".
- **Preusmeri stavku** koju `/korak` nije prosledio: modul (`Poznati problemi`), `docs/plans/IDEJE.md`, skill ili pravilo.
- **BLOKIRAN korak**: reši uzrok (prepravi korak, ili ga podeli) i vrati mu `Status: TODO` i `- [ ]`.
- **Korak koji je oborio `vozi.py` jer je dirao postojeći test**: ako je promena ponašanja namerna i u skladu sa Ciljem, dodaj taj test u red `- Menja testove:` pod "Gde" tog koraka; ako nije, pojasni u "Kako" da kod mora da zadovolji postojeći test. Test nikad ne menjaš ti.

## Zatvaranje
1. U `PLAN.md` postavi `Provereno do: <poslednji pregledani korak>`.
2. Ako se Obim ili Cilj promenio, ažuriraj ih u `PLAN.md` u 1-2 reda. Ne piši istoriju.
3. `git add docs/plans app && git commit -m "plan NNN: provera do koraka NN"`
4. Ispiši: šta si promenio (jedan red po koraku), šta si ostavio kako jeste, i "Sledeće: /clear, /korak".
