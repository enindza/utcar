# UPUTSTVO: kako se radi sa ovim šablonom

Ovaj fajl je za tebe (čoveka). Claude ga ne učitava. Sve što Claude treba da zna je u `CLAUDE.md`, `.claude/` i `docs/`.

## 1. Problem i rešenje u tri rečenice

Claude radi loše na složenim projektima kad mora da pogađa gde je šta i kad mu se u kontekst sruči sve odjednom (veliki `CLAUDE.md`, `progress.md` od 800 linija, ceo projekat "za svaki slučaj"). Rešenje nije jedan veliki fajl nego **hijerarhija malih fajlova** koja se učitava po potrebi: koren kaže "gde je šta", svaki modul kaže "šta sam ja i šta drugi smeju od mene", a plan kaže "šta tačno sad radiš i ništa drugo". Znanje o projektu živi na jednom mestu (navigacioni fajlovi), a istorija ide u log koji niko ne čita.

## 2. Šta Claude Code učitava i kada (ovo je ključno)

| Fajl | Kada se učita | Šta zato ide u njega |
|---|---|---|
| `CLAUDE.md` u korenu | uvek, na početku svake sesije | mapa, pravila, spisak modula, komande. Kratko. Ispod 80 linija. |
| `CLAUDE.md` u podfolderu | tek kad Claude otvori fajl iz tog foldera | opis tog modula: svrha, javni interfejs, zavisnosti, fajlovi |
| `.claude/rules/*.md` bez `paths:` | uvek | konvencije koje važe svuda (python.md) |
| `.claude/rules/*.md` sa `paths:` | tek kad radi sa fajlom koji odgovara šablonu | konvencije za rutere ili CLI komande, testove, migracije... |
| `.claude/skills/<ime>/SKILL.md` | samo opis (1 red) uvek; ceo sadržaj tek kad ukucaš `/ime` | procedure: `/plan`, `/korak`, `/sredi`, `/modul`, `/sablon` |
| `.claude/hooks/status.py` | pokreće se na startu sesije, izlaz ide u kontekst | 1 red: aktivan plan, sledeći korak, poslednji commit; na svežem klonu šablona kaže `Nov projekat iz sablona: /sablon` |
| `docs/ARHITEKTURA.md` | nikad sam; Claude ga čita kad ga pravilo ili korak uputi | granice modula i veze između njih |
| `docs/plans/...` | samo aktivan plan i samo jedan korak | šta tačno sad raditi |
| `docs/LOG.md` | nikad se ne čita, samo se dopisuje | hronologija |
| `CLAUDE.local.md` | uvek, ali nije u git-u | tvoje lične napomene (putanje na tvom računaru i slično) |

Posledica: ono što Claude "uvek zna" mora da bude malo, a ono što treba retko mora da bude na predvidivom mestu do koga vodi putokaz iz korena.

## 3. Struktura šablona

```
CLAUDE.md                  mapa projekta (uvek u kontekstu)
UPUTSTVO.md                ovaj fajl, za tebe
conftest.py                test fixture-i za ceo projekat (client, db)
.claude/
  settings.json            dozvole + hook
  sablon.json              URL šablona, verzija na kojoj je projekat, šta šablon održava
  hooks/status.py          "gde smo" posle /clear; javi kad šablon ima noviju verziju
  scripts/sablon.py        preuzima novu verziju šablona (poziva ga /sablon)
  scripts/vozi.py          izvršava aktivni plan bez ručnog /clear: /korak, /provera, /korak ...
  rules/rad.md             uvek: pravila rada (obim, šta čitati, tok rada)
  rules/python.md          uvek
  rules/api.md             samo za router.py i main.py (profil fastapi)
  rules/cli.md             samo za cli.py, main.py i __main__.py (profil cli)
  rules/testovi.md         samo za testove
  skills/plan/             /plan  -> pravi plan (jači model)
  skills/korak/            /korak -> izvršava sledeći korak (Sonnet)
  skills/sredi/            /sredi -> sređuje stari projekat
  skills/modul/            /modul -> nov modul iz šablona
  skills/provera/          /provera -> uskladi plan sa izveštajima koraka (jači model)
  skills/sablon/           /sablon -> povuci najnoviju verziju šablona
  skills/zabelezi/         /zabelezi -> sačuva zaključak istraživanja van plana, za kasniji /plan
docs/
  ARHITEKTURA.md           moduli i veze
  LOG.md                   samo dopisivanje
  plans/README.md          koji je plan aktivan
  plans/IDEJE.md           stavke za buduće planove, čita ih /plan
  plans/_sablon/           šablon plana i koraka
  plans/istrazivanja/      istraživanja van plana (BELESKA.md + skripte), čekaju /plan; _sablon/ je šablon beleške
  plans/000-primer-plana/  popunjen primer, obriši kad ti ne treba
app/
  main.py                  registruje rutere (fastapi) ili komande (cli), jedan red po modulu
  core/                    config, db, errors + CLAUDE.md
  modules/_sablon/CLAUDE.md  šablon opisa modula
  modules/primer/          pokazni modul: router ili cli, service, schemas, models, tests, CLAUDE.md
tests/                     integracioni testovi preko više modula
profili/                   samo u šablonu: CLI razlike i proba.py; instalacija ga briše
```

Jedan modul = jedan folder = jedna celina. Moduli razgovaraju samo preko `service.py` i `schemas.py`, i samo onako kako piše u `ARHITEKTURA.md`. To je ono što si opisao kao "ručno bih rekao kako komuniciraju": ovde je to zapisano jednom, na jednom mestu, i Claude ga čita kad dira vezu.

### Profili: fastapi i cli

Šablon pravi dve vrste aplikacija. Razlikuje se samo ulazni sloj:

| | fastapi | cli |
|---|---|---|
| ulazni sloj | `router.py` | `cli.py` |
| registracija | `include_router` | `add_typer` |
| pravilo | `api.md` | `cli.md` |
| test fixture | `client` | `runner` |
| pokretanje | `uvicorn` | `python -m app` |

Zajedničko je sve ostalo: `service.py`, `schemas.py`, `models.py`, `app/core/config.py`, ceo `.claude/` i tok rada (plan, koraci, `/modul`, `/sablon`). Oba profila imaju bazu (SQLite + SQLAlchemy) i fixture `db`.

Profil se bira jednom, pri instalaciji (odeljak 4), i pamti se u `.claude/sablon.json`. Promenu profila postojećeg projekta skripta ne radi.

Održavanje šablona:
- koren šablona je profil fastapi, a CLI razlike su u `profili/cli/`, na putanjama koje imaju u projektu (`profili/cli/app/main.py` je u projektu `app/main.py`);
- zajedničko se menja u korenu; za fajl koji ima parnjaka u `profili/cli/` proveri da li i parnjaku treba ista izmena (detalji u `profili/CLAUDE.md`);
- liste fajlova koje šablon održava (zajedničke i po profilu) su u `.claude/sablon.json`;
- provera: `.venv/Scripts/python profili/proba.py`, uz `pip install -r profili/cli/requirements.txt` u `.venv` šablona;
- repo šablona ima lokalnu oznaku `git config sablon.izvor true` (ne putuje kloniranjem); posle kloniranja samog šablona radi održavanja postavi je ponovo, inače ga hook i `sablon.py` vide kao nov projekat.

## 4. Novi projekat, korak po korak

1. (opciono) Napravi prazan repo na Forgejo-u, da ga `/sablon` odmah poveže. Zatim `git clone http://192.168.31.10:3053/enindza/blank-start.git ime-projekta`, `cd ime-projekta`, `claude` (hook kaže `/sablon`).
2. `/sablon`: pita profil (vidi "Profili" u odeljku 3), URL repo-a, naziv i opis. Zatim: `origin` šablona postaje `sablon`, tvoj URL postaje `origin`; postavi kostur profila i obriše `profili/`; upiše naziv i opis u `CLAUDE.md`, `app/core/config.py` i `docs/ARHITEKTURA.md` (bez koda); napravi `.venv` (`py -3.11`) i pusti pytest (fastapi 4 testa, cli 5); dva commit-a. Istorija šablona ostaje u projektu; to je namerno, tako `/sablon` kasnije zna odakle je krenuo.
3. `git push -u origin main`.
4. Za svaki modul: `/modul ime - svrha`. Commit.
5. Za sve što je veće od jednog fajla: `/plan ...` pa `/korak` (odeljak 5).
6. Modul `primer` obriši kad ti više ne treba kao uzor: folder, red u `main.py` (ruter ili komanda, po profilu), red u `CLAUDE.md`, red u `ARHITEKTURA.md`, import u `core/db.py` i u `conftest.py`.

Bez Claude-a: `python .claude/scripts/sablon.py --profil cli --origin <URL>` (ili `--profil fastapi`), pa ručno naziv, venv i pytest.

## 5. Tok rada: plan pa koraci

**Model se bira sam:** `/plan`, `/zabelezi`, `/provera` i `/sredi` rade na Opusu, `/korak` na Sonnetu (polje `model:` u skill-u). To važi samo za tu poruku: sledeća poruka koju napišeš ide na model sesije. Zato, kad skill nešto pita, odgovaraš ponovnim pozivom (`/plan <odgovor>`, `/zabelezi <odgovor>`, `/sredi faza 2 <ispravke>`), ne običnom porukom. `/model` ti treba samo za rad van ovih skill-ova (npr. Fable umesto Opusa, ili debug blokiranog koraka). Najjeftinije je da sesija stoji na Sonnetu.

**Pravljenje plana (Opus):**
```
/clear
/plan Dodaj modul auth sa JWT loginom; korisnici u bazi; primer modul traži login za POST.
```
Claude pročita samo koren, arhitekturu i module koje dira, napravi `docs/plans/001-auth/` sa `PLAN.md` i po jednim fajlom za svaki korak, upiše plan kao aktivan i commit-uje. Pročitaj korake. Ako ti nešto ne valja, reci mu sad, dok je jeftino.

**Izvršavanje (Sonnet):**
```
/clear
/korak
```
Claude pročita samo `README` planova, `PLAN.md`, jedan fajl koraka i fajlove koje korak navodi. Uradi, pusti proveru, upiše izveštaj u fajl koraka, štiklira korak, ažurira `CLAUDE.md` modula ako je nešto dodao, dopiše red u log, commit-uje, i kaže "Sledeće: /clear pa /korak".

Ti: `/clear`, `/korak`, i tako do kraja. Svaki korak počinje sa čistim kontekstom, a hook mu na startu kaže gde je stao.

**Bez ručnog `/clear`:** kad pročitaš plan i on ti valja, iz terminala (ne iz Claude-a):
```
python .claude/scripts/vozi.py
```
Skripta sama pokreće `claude -p "/korak NN"` za svaki korak. Svaki poziv je nov proces, dakle čist kontekst, isto kao `/clear`. Posle koraka GOTOVO-DRUGAČIJE ili BLOKIRAN sama pokrene `/provera` pa nastavlja. Staje kad je plan gotov, kad je korak BLOKIRAN i posle `/provera`, kad korak nije commit-ovan ili kad radni direktorijum nije čist. Opcije: `--max-koraka N` (stani posle N koraka), `--budzet 2` (najviše 2 USD po pozivu), `--suvo` (samo kaži šta bi pokrenuo). Ctrl+C prekida, a ista komanda nastavlja od mesta gde je stala. Izmene fajlova se prihvataju same (`--permission-mode acceptEdits`); komande van liste u `.claude/settings.json` se odbijaju, ne pitaju, pa ako korak zapne na tome, dodaj komandu u `allow`.

**Kad korak ne prođe:** Claude postavi `Status: BLOKIRAN` i opiše problem u sekciji "Otvoreno". Ti onda ili prepraviš fajl koraka rukom, ili se vratiš na jači model: `/model opus`, `/clear`, pa "korak 03 je blokiran, pročitaj docs/plans/001-auth/steps/03-*.md i reši".

**Zašto ovako radi sa Sonnet-om:** svaki korak ima "Šta / Zašto / Gde / Kako / Provera". Sonnet ne mora ništa da zaključuje o celini, samo da izvrši mali, jasno omeđen zadatak i dokaže testom da radi. "Zašto" mu daje dovoljno da razumno odstupi, a "Izveštaj" te tera da vidiš gde je odstupio. Primer popunjenog koraka sa izveštajem: `docs/plans/000-primer-plana/steps/01-service-list.md`.

**Kad ste rešenje već isprobali tokom planiranja:** istraživanje ide u `docs/plans/NNN-ime/proto/`, nikad u `app/`. Kod koji je prošao test ostaje tamo i commit-uje se sa planom, a korak kaže "polazna tačka: proto/x.py, prilagodi konvencijama modula". Planer ne piše kod koji nije pokrenuo, ali isprobani kod je činjenica i putuje sa planom.

**Kad istražuješ u slobodnoj sesiji, bez plana:** probaš dve-tri varijante, doneseš zaključak, i pre `/clear` kažeš `/zabelezi`. To napravi `docs/plans/istrazivanja/<ime>/` sa `BELESKA.md` (pitanje, šta radi i kako se proverava, šta je odbačeno i zašto, šta nije provereno, zavisnosti) i samo onim skriptama koje su stvarno pokrenute, dopiše red u `IDEJE.md` i commit-uje. Možeš tako nizati više istraživanja, svako u svojoj sesiji. Kad kasnije pokreneš `/plan`, on pročita belešku, premesti folder u `proto/` plana i prenese isprobane komande u "Kako" i "Provera" koraka. Zaključak ne zavisi od tvog sećanja ni od sesije u kojoj je nastao.

**Kako koraci razgovaraju:** ne čitanjem jedni drugih, nego kroz mesta koja se ionako čitaju. Pri zatvaranju `/korak` isprazni "Otvoreno": ono što treba narednom koraku upisuje u sekciju "Napomene od prethodnih koraka" u fajlu tog koraka; nalaz o kodu modula u "Poznati problemi" tog modula; ideja za kasnije u `docs/plans/IDEJE.md` (koju `/plan` čita); primedba o alatu u skill ili pravilo. U izveštaju ostaje samo ono što je važno jedino tebi.

**`/provera` (jači model):** kad korak završi kao GOTOVO-DRUGAČIJE ili BLOKIRAN, `/korak` ti to preporuči. `/provera` pročita izveštaje samo nepregledanih koraka (red `Provereno do: NN` u planu, hook te podseti), uskladi naredne korake na licu mesta i pomeri oznaku. Nov korak dodaje samo ako je nužan za cilj, sa navedenim razlogom, pa plan ne raste od proveravanja.

## 6. Kako pišeš upit za male stvari (bez plana)

Loše: "Popravi da login radi."

Dobro: "U modulu `auth`, `service.login` vraća 500 kad korisnik ne postoji. Treba `NotFound`. Menjaj samo `service.py` i dodaj test. Ne diraj router."

Obrazac: **modul + fajl + šta se menja + šta se ne dira + kako se proverava.** Ako ne znaš gde je problem, prvo pitaj: "Gde se obrađuje login? Samo mi kaži fajl i funkciju, ne menjaj ništa." pa onda zadaj izmenu.

## 7. Popravka postojećeg projekta koji nije ovako organizovan

Prekopiraj `.claude/` iz ovog šablona u taj projekat (da `/sredi` uopšte postoji tamo), pa u tom projektu:
```
/sredi
```
Skill radi u fazama i posle svake traži da uradiš `/clear` pa `/sredi faza N`:

1. Snimi stanje, predloži podelu na celine i profil šablona (`fastapi` ili `cli`, uz razlog). Čeka tvoju potvrdu, ništa ne piše. Potvrđuješ sa `/sredi faza 2 <ispravke>` **bez** `/clear`, jer je predlog samo u toj sesiji; profil menjaš u istom redu, npr. `/sredi faza 2 profil cli`.
2. Napiše navigaciju: `ARHITEKTURA.md` (sa redom `Profil šablona: ...`), `CLAUDE.md` po celinama, kratak korenski `CLAUDE.md`. Kod se ne premešta.
3. Narasle `projekat.md` / `progress.md` pretoči: ono što je tačno i važno ide u navigaciju, ostalo u `docs/LOG.md` kao arhiva, original se briše.
4. Priključi projekat na šablon (komande iz odeljka 9, `sablon.py` sa `--profil` iz `ARHITEKTURA.md`): to donese `.claude/` alate sa pravilom ulaznog sloja za taj profil i `docs/plans/`.
5. (kasnije, po želji) fizičko premeštanje fajlova u foldere modula, kroz `/plan`, modul po modul.

## 8. Pravila održavanja (šta nikad ne sme da naraste)

- `CLAUDE.md` u korenu: ispod 80 linija. Ako raste, nešto u njemu pripada modulu ili arhitekturi.
- `CLAUDE.md` modula: ispod 40 linija. Opis, ne istorija.
- `docs/LOG.md`: neograničen, jer ga niko ne čita.
- Planovi: završeni ostaju u `docs/plans/`, niko ih ne učitava. Obriši kad hoćeš.
- `/clear` posle svakog koraka i posle svakog plana. `/compact` samo usred koraka koji se otegao, i tada sa uputstvom: `/compact zadrži samo šta je urađeno u ovom koraku i šta je ostalo`.
- Kad primetiš da Claude čita fajlove koje ne bi trebalo: to je znak da mu putokaz nedostaje. Dodaj red u mapu ili u `CLAUDE.md` modula, ne objašnjavaj mu u chatu (chat se briše, fajl ostaje).

## 9. Ažuriranje projekta kad se šablon promeni

Šablon živi na Forgejo-u: `http://192.168.31.10:3053/enindza/blank-start`. Menjaš ga samo tamo (u `Blank start`, pa push). Projekti ga ne kopiraju, nego prate.

Svaki projekat u `.claude/sablon.json` pamti commit šablona na kojem je. Hook pri startu sesije jednom dnevno proveri Forgejo i, ako ima novije, ispiše `Sablon: novija verzija ... (/sablon)`. Tada:

```
/sablon
```

To pokrene `.claude/scripts/sablon.py`, koji:
- pročita profil projekta iz `.claude/sablon.json`; stari projekat bez profila prepozna iz koda (`fastapi` u zavisnostima je fastapi, `typer` ili `click` je cli) i upiše ga, a ako ne može, `/sablon` te pita;
- prepiše fajlove koje šablon održava (`putanje` u `sablon.json`: ceo `.claude` osim tvojih `rules/projekat-*.md` i `settings.local.json`, pravilo ulaznog sloja za profil (`api.md` ili `cli.md`), `docs/plans/_sablon`, `docs/plans/istrazivanja/_sablon`, `app/modules/_sablon`, `UPUTSTVO.md`);
- obriše ono što je šablon obrisao, i pravilo drugog profila;
- doda fajlove koje šablon daje samo prvi put ako ih nema (`IDEJE.md`, `LOG.md`, `README.md` planova, a po profilu `ARHITEKTURA.md` i `conftest.py`);
- upiše novu verziju i commit-uje;
- ispiše ako je šablon menjao i fajlove koje projekat prilagođava (`CLAUDE.md`, `conftest.py`, `pyproject.toml`, `requirements.txt`, `app/core`), pa te Claude u `/sablon` provede kroz to ručno.

Zato u projektu ne menjaj fajlove koje šablon održava: sledeći `/sablon` ih prepiše. Pravila za projekat idu u nov fajl `.claude/rules/projekat-*.md`, lične dozvole u `.claude/settings.local.json`. Ako ti u šablonu nešto smeta, popravi u `Blank start` i push-uj: svi projekti dobiju popravku.

Stari projekat koji je nastao kopiranjem foldera (nema `sablon.json`) priključuješ jednom, ručno:
```
git remote add sablon http://192.168.31.10:3053/enindza/blank-start.git
git fetch sablon main
git checkout sablon/main -- .claude
python .claude/scripts/sablon.py
```
(ako ne prepozna profil: `--profil fastapi` ili `--profil cli`)

Skripta će reći "Prva sinhronizacija": uporedi `CLAUDE.md` sa šablonom za svoj profil, obriši sekciju `## Pravila rada` (sada je u `.claude/rules/rad.md`) i commit-uj. Isto radi `/sablon` ako ga pokreneš posle ovih komandi.

Skripta odbija da radi u samom repo-u šablona, koji se prepoznaje po lokalnoj oznaci `git config sablon.izvor true`: pokreće se samo u projektu.

## 10. Koji model za šta

| Posao | Model |
|---|---|
| `/plan`, `/zabelezi`, `/provera`, `/sredi` | Opus, skill ga sam postavi (`model:` u skill-u) |
| `/korak` | Sonnet, skill ga sam postavi |
| arhitekturne odluke, debug blokiranog koraka | Opus ili Fable, ručno sa `/model` |
| `/modul`, male jasno omeđene izmene | Sonnet |
| Pitanja "gde je šta" | agent `istrazivac` (Haiku), ne glavni model |

### Subagenti

Podrazumevano subagent nasleđuje model glavne sesije: ako sediš na Opusu, i pretraga troši Opus. Zato šablon to rešava podešavanjima, ne molbom:

- `.claude/settings.json` zabranjuje ugrađene agente `Explore`, `Plan` i `general-purpose` (`permissions.deny`) i tera svaki subagent na Haiku (`CLAUDE_CODE_SUBAGENT_MODEL` + `CLAUDE_CODE_SUBAGENT_MODEL_FORCE`).
- `.claude/agents/istrazivac.md` je jedini agent: samo Read/Glob/Grep, vraća najviše 10 redova (`putanja:linija`).
- `.claude/rules/subagenti.md`: pokreće se samo kad korisnik ili skill traži, prvo sam `Grep`/`Glob`, nikad više subagenata istovremeno, usko pitanje.
- Ograničenja po skill-u stoje u samim skill-ovima (`/plan` najviše 3 poziva, `/korak` najviše 1, `/provera` najviše 2, `/sredi` serijski).

Ako vidiš da subagent troši mnogo, pitanje je bilo preširoko: prekini i suzi ga na jednu stvar. Ako ti ikad zatreba agent na drugom modelu (npr. Sonnet), skini `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` iz `settings.json`. Agent specifičan za projekat nazovi `.claude/agents/projekat-<ime>.md`; ostale prepisuje `/sablon`.

## 11. Kad ovo savladaš

- `CLAUDE.local.md`: tvoje lične napomene koje ne idu u git.
- `paths:` u pravilima možeš da suziš na jedan modul (`app/modules/auth/**`) kad modul ima posebne konvencije.
- Plan mode (Shift+Tab dvaput) za istraživanje pre `/plan`: Claude samo čita i predlaže, ne piše.
