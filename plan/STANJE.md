# Stanje realizacije

> Ovaj fajl je **kanal komunikacije između koraka**. Svaki korak ga čita na
> početku i ažurira na kraju (pre commit-a). Plan i ugovori: [`PLAN.md`](PLAN.md).

## Sledeći korak

**Korak 4 – Processor i integracija parsera u `handleConnection`**

## Grana

`claude/amazing-thompson-cucp4s` (ako sesija zadaje drugu granu – koristi nju i
upiši je ovde).

## Pregled

| # | Korak | Status | Commit |
|---|---|---|---|
| 0 | Priprema (go.mod, plan, `/korak`) | ✅ gotovo | (ovaj commit) |
| 1 | Model poruke i parser zaglavlja | ✅ gotovo | 1af1d53 |
| 2 | Parser SIA data bloka (događaji) | ✅ gotovo | parseEvents, parseBlockEvents |
| 3 | Tabela SIA kodova | ✅ gotovo | siacodes.go, DescribeSIA |
| 4 | Processor i integracija parsera | ⏳ sledeći | |
| 5 | SQLite store | ⬜ | |
| 6 | Integracija baze | ⬜ | |
| 7 | DC-09 okvir | ⬜ | |
| 8 | Forwarder | ⬜ | |
| 9 | Integracija prosleđivanja | ⬜ | |
| 10 | Dokumentacija, Docker, završna provera | ⬜ | |

Statusi: ⬜ nije počet · ⏳ sledeći · 🔧 u toku (prekinut) · ✅ gotovo · ⛔ blokiran

## Promene ugovora

(Ovde se upisuje svako odstupanje od API-ja/šeme/flagova iz `PLAN.md`, sa
brojem koraka. Sledeći koraci ovo imaju prednost nad `PLAN.md`.)

- nema

## Otvorena pitanja za korisnika

(Ako korak naiđe na odluku koju samo korisnik može da donese – upiše je ovde,
postavi pitanje i stane.)

- nema

## Poznati problemi (ne rešavati ako nisu deo koraka)

- `pusher_test.go` faktički ništa ne proverava: očekuje `POST` a kod šalje
  `PUT`, i šalje `http://` na TLS server, pa handler nikad nije pozvan. Pusher se
  po odluci korisnika ne dira.
- `utcar.go`: greška debug servera se proverava pre nego što goroutine stigne
  da je postavi.
- `pusher.go`: `InsecureSkipVerify: true`.
- `gofmt -l .` prijavljuje `pusher.go`, `util.go`, `util_test.go` – bili su
  neformatirani i pre koraka 1 (ne dirati `pusher.go`; ostale po želji u koraku 10).

## Dnevnik (handoff beleške)

Svaki korak dodaje unos **na vrh** ove sekcije, po šablonu:

```
### Korak N – naziv (YYYY-MM-DD)
- Urađeno: ...
- Fajlovi: ...
- Odluke/odstupanja: ...
- Provera: go vet ✅, go test ✅ (broj testova / šta je pokriveno)
- Za sledeći korak: ...
```

### Korak 3 – Tabela SIA kodova (2026-10-07)
- Urađeno: novi `siacodes.go` – `siaCodes map[string]string` (304 SIA DC-03
  koda, AA…ZU, engleski opisi) i `DescribeSIA(code) string` (`""` za nepoznat,
  osetljivo na velika/mala slova). `parseBlockEvents` u `sia.go` sada pravi
  događaj sa `Description: DescribeSIA(tok[:2])`.
- Fajlovi: `siacodes.go` (nov), `siacodes_test.go` (nov), `sia.go`, `sia_test.go`.
- Odluke/odstupanja: nema odstupanja od ugovora. Tabela je pisana po
  standardnoj SIA DC-03 listi; uključeni su i noviji kodovi (`BG/FG/HG/MG/PG/QG/UG`
  „Unverified Event“, `CP/CQ/OQ`, `NF/NL`). `NM` namerno nije u tabeli (kod ATS-a
  je to samo završetak teksta `*'...'NM`, a parser ga ionako ne vidi kao događaj).
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (novi: `TestDescribeSIA`,
  `TestSIACodesTable` – svaki ključ `[A-Z]{2}`, opis neprazan, ≥200 kodova,
  `TestParseMessageDescription`; `TestParseMessageEvents` sada očekivanim
  događajima postavlja `Description = DescribeSIA(Code)` pre poređenja).
- Za sledeći korak:
  - `m.Events[i].Description` je već popunjen posle `ParseMessage` – Processor
    ne treba ništa da dopunjuje; za log može koristiti `Code`+`Description`.
  - Za kompatibilnost sa pusher-om (`SIA` struct): `ParseSIA` u `parser.go` i
    dalje postoji i nije diran; `SIA` se može sastaviti iz `Message`
    (`Sequence`, `Receiver`, `Line`, `Account`, `Events[0].Code`, `Events[0].Zone`).

### Korak 2 – Parser SIA data bloka (događaji) (2026-10-07)
- Urađeno: `ParseMessage` na kraju (za `KindSIA` i Protocol `SIA-DCS`/`*SIA-DCS`)
  poziva `parseEvents(m.Blocks)` i puni `m.Events`. Za ostale protokole
  (`NULL`, `ADM-CID`, ...) `Events` ostaje `nil`.
- Fajlovi: `sia.go`, `sia_test.go`.
- Odluke/odstupanja (u okviru ugovora):
  - Blokovi: prvi blok uvek nosi događaje (sa `#acct|` ili bez njega);
    sledeći blokovi samo ako počinju sa `#` (npr. `[#001465|NUR021]`). Ostali
    (`[XE0.0]`, `[H12:30:45]` – DC-09 extended data) se ignorišu.
    `[#acct]` i `[]` nemaju događaje.
  - `N`/`O` na početku podataka bloka se skida samo ako ostatak počinje sa
    `[a-z]{2}` ili `[A-Z]{2}` (`OP001` → OP/001, `OBA1` → BA/1).
  - Tokeni se dele po `/` van `'...'`. Unutar tokena se redom čitaju
    modifikatori `[a-z]{2}[0-9:]*` (pa radi i `Nri2id7CL001` bez kosih crta),
    zatim `[A-Z]{2}` + ostatak = događaj; sve ostalo se ignoriše.
    Modifikatori važe do kraja bloka (ne prelaze u sledeći blok).
  - `*'tekst'XX` → `Text` = sadržaj između navodnika, `Zone` = deo pre `*`.
  - Nepoznat kod (`QQ`) se ipak vraća kao događaj; `Description` ostaje `""`.
  - Refaktor: `scanBlocks` sada koristi novu `blockEnd(s, start) (int, bool)`
    (jedan blok); `parseEvents` koristi istu funkciju.
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (novi:
  `TestParseMessageEvents` – 21 slučaj, `TestParseMessageEventsProtocol`).
- Za sledeći korak:
  - `Description` popuniti u `parseBlockEvents` (sia.go, mesto gde se pravi
    `ev := Event{Code: tok[:2], ...}`) sa `DescribeSIA(ev.Code)`, ili posle
    `parseEvents` u `ParseMessage` petljom po `m.Events`.
  - U `TestParseMessageEvents` očekivani događaji nemaju `Description` i
    porede se sa `reflect.DeepEqual` – kad se Description popuni, test treba
    prilagoditi (npr. postaviti `Description = ""` pre poređenja, ili dodati
    očekivane opise). Kod `QQ` mora ostati sa praznim opisom.

### Korak 1 – Model poruke i parser zaglavlja (2026-10-07)
- Urađeno: novi `sia.go` sa tipovima `MessageKind`, `Event`, `Message` (po
  ugovoru) i `ParseMessage`; `IsHeartbeat` premešten iz `parser.go` u `sia.go`
  sa ispravljenim regex-om (`^SR(\d{4})L(\d{4})[\s\x00]+(\w+)[\s\x00]+\[\w*\]$`,
  ulaz se prvo trim-uje od `\n\r\x00`). `ParseSIA` i `handleConnection` nisu dirani.
- Fajlovi: `sia.go` (nov), `sia_test.go` (nov), `parser.go` (uklonjen `IsHeartbeat`).
- Odluke/odstupanja (u okviru ugovora):
  - Zaglavlje: regex `headerRegex` traži bilo gde
    `"PROTO"\d{4}(R hex{1,6})?(L hex{1,6})?(#hex{3,16})?\[` – sekvenca je
    obavezna, mora odmah da sledi `[`. Protocol može biti i `*SIA-DCS`, `NULL`,
    `ADM-CID` (bilo šta bez navodnika/razmaka).
  - Account: prvo iz zaglavlja (`#acct`), inače iz prvog bloka (`[#acct|` ili `[#acct]`).
  - Posle blokova sme da stoji bilo šta (kod ATS-a stoji `7C9677F21948CC12|#001465`
    – ignoriše se, ostaje samo u `Raw`). `Timestamp` se traži regex-om
    `_\d{2}:\d{2}:\d{2},\d{2}-\d{2}-\d{4}` u delu posle blokova.
  - Unknown: `ParseError` je `"no heartbeat or SIA header found"` ili
    `"unterminated data block"`; kod drugog su Protocol/Sequence/... ipak
    popunjeni (korisno za bazu/log), `Blocks` je prazan.
  - Heartbeat: `Account` = treće polje bez završnih `X`; `Protocol`/`Sequence` prazni.
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (novi: `TestIsHeartbeatNUL`,
  `TestParseMessageHeartbeat`, `TestParseMessageSIA` – 8 slučajeva,
  `TestParseMessageUnknown` – 8 slučajeva; stari testovi i dalje prolaze).
- Za sledeći korak:
  - Događaje parsirati iz `m.Blocks` (sadrži zagrade, više blokova
    jedan za drugim, npr. `[#001465|NUA021*'hall [1]'NM][#001465|NUR021]`).
    Pomoćna funkcija `scanBlocks(s, start)` već zna da preskoči `]` unutar
    `'...'` – može se iskoristiti/proširiti za razbijanje na pojedinačne blokove.
  - Popunjavati `Events` samo kad je `Kind == KindSIA` i Protocol `SIA-DCS`
    ili `*SIA-DCS`; poziv dodati na kraj `ParseMessage` (posle `m.Kind = KindSIA`).
  - Prefiks `#acct|` u bloku treba preskočiti pre događaja; blok `[]` (NULL) nema događaja.
  - Primeri u `sia_test.go` (`TestParseMessageSIA`) mogu se proširiti očekivanim događajima.

### Korak 0 – Priprema (2026-10-07)
- Urađeno: dodat `go.mod` (modul `github.com/enindza/utcar`, `go 1.24`,
  cobra v1.10.1, viper v1.21.0) i `go.sum`; napravljeni `plan/PLAN.md`,
  `plan/STANJE.md`, skill `/korak` (`.claude/skills/korak/SKILL.md`) i `CLAUDE.md`.
- Odluke: korisnik je izabrao SQLite, nešifrovan DC-09 preko TCP-a, ACK alarmu
  odmah, pusher (openHAB) se ne dira.
- Provereno: `modernc.org/sqlite@v1.40.1` radi sa Go 1.24 (novije verzije
  podižu `go` direktivu na 1.26 – ne koristiti).
- Provera: go vet ✅, go test ✅ (postojeći testovi).
- Za sledeći korak: uvek `export GOTOOLCHAIN=local` pre `go` komandi.
  Primeri poruka za testove su u `README.md`, `parser_test.go`, `utcar_test.go`;
  `util_test.go` sadrži početak stvarne dekriptovane poruke
  (`"\n0101005B\"SIA-DC..."` – LF, pa 8 znakova pre navodnika).
