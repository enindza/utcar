# Stanje realizacije

> Ovaj fajl je **kanal komunikacije između koraka**. Svaki korak ga čita na
> početku i ažurira na kraju (pre commit-a). Plan i ugovori: [`PLAN.md`](PLAN.md).

## Sledeći korak

**Korak 10 – Dokumentacija, Docker, završna provera**

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
| 4 | Processor i integracija parsera | ✅ gotovo | processor.go, handleConnection(c, p) |
| 5 | SQLite store | ✅ gotovo | store.go (OpenStore/Save/Close) |
| 6 | Integracija baze | ✅ gotovo | Processor.Store, --db, --db-heartbeats |
| 7 | DC-09 okvir | ✅ gotovo | dc09.go (CRC16, DC09Frame, BuildFrame, ParseResponse) |
| 8 | Forwarder | ✅ gotovo | forwarder.go (NewForwarder/Start/Enqueue/Stop) |
| 9 | Integracija prosleđivanja | ✅ gotovo | Processor.Forwarders, --forward* |
| 10 | Dokumentacija, Docker, završna provera | 🔧 u toku | |

Statusi: ⬜ nije počet · ⏳ sledeći · 🔧 u toku (prekinut) · ✅ gotovo · ⛔ blokiran

## Promene ugovora

(Ovde se upisuje svako odstupanje od API-ja/šeme/flagova iz `PLAN.md`, sa
brojem koraka. Sledeći koraci ovo imaju prednost nad `PLAN.md`.)

- Korak 4: `Processor` za sada ima samo polje `Push chan SIA`. Polja `Store` i
  `StoreHeartbeats` dodaje korak 6, `Forwarders` i `ForwardHeartbeats` korak 9
  (tipovi `Store`/`Forwarder` još ne postoje). Potpis `Process` je po ugovoru.
- Korak 4: `ParseSIA` (i `parser.go`, `parser_test.go`) uklonjeni.
- Korak 5: `go.mod` sada ima `go 1.24.0` (umesto `go 1.24`) – `modernc.org/sqlite`
  v1.40.1 i `golang.org/x/sys` v0.36.0 to traže; i dalje je Go 1.24. Linija
  `toolchain` je uklonjena (`go mod edit -toolchain=none`).
- Korak 6: `Processor` sada ima `Store *Store`, `Push chan SIA`,
  `StoreHeartbeats bool` (još bez `Forwarders`/`ForwardHeartbeats` – korak 9).
- Korak 7: `BuildFrame(m, "dc09")` vraća grešku i za SIA/heartbeat poruku bez
  `Account` (centar ne bi mogao da identifikuje objekat). Dodate konstante
  `FormatDC09`/`FormatRaw` i `ResponseACK`/`ResponseNAK`/`ResponseDUH`.
- Korak 8: `Enqueue` vraća `false` kad poruka **nije stavljena u red** iz bilo
  kog razloga: pun red, forwarder zaustavljen, ili `BuildFrame` greška (npr.
  `unknown` u `dc09`, nil poruka). U svim slučajevima `Enqueue` sam loguje
  upozorenje (osim posle `Stop`) – pozivalac ne treba da loguje. Dodata metoda
  `Format() string` i konstante `DefaultForward{Queue,Timeout,MinBackoff,MaxBackoff}`;
  nulte vrednosti u `ForwarderOptions` → default (1000, 10s, 1s, 60s).
- Korak 9: `Processor` je sada u potpunosti po ugovoru (`Store`, `Forwarders`,
  `Push`, `StoreHeartbeats`, `ForwardHeartbeats`). `unknown` poruku Processor
  ne prosleđuje forwarderima u `dc09` formatu (samo u `raw`). Nove pomoćne
  funkcije u `utcar.go`: `forwardSpecs(values)` (deli po zarezu – viper
  **ne** deli `UTCAR_FORWARD`, provereno) i `newForwarders(specs, opts)`.

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
- Pusher prima **svaki** događaj (ne samo prvi kao ranije); za sve kodove osim
  UA/UR `HttpPost` vraća grešku → u logu `Push error: Unsupported SIA command`
  (isto ponašanje kao ranije za npr. RP). `pchan` je nebaferovan, pa
  `handleConnection` čeka pusher (posle ACK-a, kao i ranije).
- CTRL-C (`receiveSignal`) radi `os.Exit(0)`, pa se `defer store.Close()` u
  `run()` ne izvršava. Podaci su bezbedni (svaki `Save` je commit-ovana
  transakcija, WAL se oporavlja pri sledećem otvaranju), ostaju `-wal`/`-shm`
  fajlovi. Isto važi za forwardere (`defer f.Stop()` u `run()` se ne izvršava;
  poruke u redu se gube pri gašenju – i inače se gube jer red nije trajan).
  Rešiti u koraku 10 ako treba (npr. signal handler koji zatvara resurse).
- Red forwardera je samo u memoriji: restart utcar-a gubi neposlate poruke
  (u bazi ostaju). Van plana – eventualno proširenje.
- `run()` otvara bazu posle `net.Listen` – greška baze je fatalna, ali port je
  kratko bio otvoren (bez posledica).

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

### Korak 9 – Integracija prosleđivanja (2026-10-07)
- Urađeno: `Processor` dobio `Forwarders []*Forwarder` i `ForwardHeartbeats
  bool`; `Process` posle `p.store(m)` zove `p.forward(m)` (nova metoda:
  heartbeat samo ako `ForwardHeartbeats`; `unknown` se preskače za forwardere
  sa `Format() == FormatDC09`, da ne bi bilo duplog upozorenja; povratna
  vrednost `Enqueue` se ignoriše). Flagovi `--forward` (StringSlice),
  `--forward-timeout` (`DefaultForwardTimeout`), `--forward-queue`
  (`DefaultForwardQueue`), `--forward-heartbeats` (`true`) + viper bind.
  `run()`: `newForwarders(forwardSpecs(viper.GetStringSlice("forward")), opts)`
  (greška = `log.Fatalf`), za svaki `Start()`, `defer Stop()`, log
  „Forwarding to <Name> (format <Format>, heartbeats: <bool>)“. Backoff ostaje
  default (1s/60s).
- Fajlovi: `processor.go`, `utcar.go`, `processor_test.go`, `utcar_test.go`.
- Odluke/odstupanja: vidi „Promene ugovora“ (Korak 9). Viper za env vraća
  `["a:1,b:2"]` (ne deli po zarezu), za flag deli (pflag CSV) – zato
  `forwardSpecs` deli svaku vrednost po zarezu, trim-uje i preskače prazne.
  `newForwarders` na grešci zaustavlja već napravljene (nisu startovani).
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (i `-race -count=3`).
  Novi testovi: `TestProcessForward` (dc09 + raw forwarder bez `Start`, red
  se čita preko `queuedFrames(f)`; sa/bez `ForwardHeartbeats`; unknown samo u
  raw), `TestProcessForwardQueueFull` (pun red ne blokira push),
  `TestHandleConnectionForward` (end-to-end: SIA + heartbeat + smeće preko
  `handleConnection` → 2 lažna centra; centar b ugašen, a dobija SIA i NULL
  odmah, pa b upaljen dobija isto; smeće ne stiže), `TestForwardSpecs`,
  `TestNewForwarders`. Ručno: binar sa `UTCAR_FORWARD="127.0.0.1:9001,
  tcp://127.0.0.1:9002?format=raw"` loguje 2 forwardera; `--forward udp://x:1`
  → „Failed to setup forwarding“, exit 1.
- Za sledeći korak:
  - README: opisati flagove `--db`, `--db-heartbeats`, `--forward`
    (`host:port`, `tcp://host:port[?format=dc09|raw]`, više puta ili zarezom;
    env `UTCAR_FORWARD` zarezom), `--forward-timeout`, `--forward-queue`,
    `--forward-heartbeats` (env: `UTCAR_` + veliko, `-` → `_`). Napomena da
    `raw` nije ispravan DC-09 okvir (vidi Korak 7 – nalaz o prefiksu `0101`),
    da se `unknown` šalje samo u `raw`, heartbeat kao `"NULL"` (jedan pokušaj),
    događaji se ponavljaju do ACK (backoff 1s→60s), DUH = odustaje.
  - Docker: `CGO_ENABLED=0` radi (modernc sqlite je pure Go); za bazu treba
    volume (npr. `--db /data/utcar.db`). Proveriti postojeći `Dockerfile`.
  - „Poznati problemi“: `gofmt` za `util.go`/`util_test.go` (po želji),
    CTRL-C ne zatvara bazu/forwardere (`os.Exit` u `receiveSignal`), greška
    debug servera. Odlučiti šta se rešava u koraku 10.
  - Sve komande u `run()` čitaju viper posle `initConfig` – `--help` prikazuje
    sve nove flagove.

### Korak 8 – Forwarder (2026-10-07)
- Urađeno: novi `forwarder.go`. `NewForwarder(spec, opts)` parsira spec
  (`parseForwardSpec`: `host:port`, `tcp://host:port[/]`,
  `?format=dc09|raw`; druge šeme/opcije/putanja/user, port 0 ili nenumerički,
  prazan host → greška) i pravi red `chan forwardItem` veličine `QueueSize`.
  `Enqueue` **odmah** pravi okvir (`BuildFrame`) – svi pokušaji šalju isti
  okvir (isti NULL broj). `Start` (sync.Once) pokreće jednu goroutine (`run`
  → `deliver` → `send`). `send`: `Dialer.DialContext` (Timeout), jedan
  `SetDeadline(now+Timeout)` za upis i čitanje, čitanje do `\r` (ili EOF sa
  podacima), `ParseResponse`; `context.AfterFunc` zatvara konekciju na `Stop`.
  Konekcija po poruci (zatvara se posle odgovora).
- Pravila: ACK → gotovo (log „delivered … (attempt N)“ samo za ne-heartbeat);
  DUH → upozorenje, odustaje; NAK/greška/timeout → za heartbeat samo log, bez
  ponavljanja; ostalo ponavlja sa backoff-om `MinBackoff`, ×2, do `MaxBackoff`
  (backoff se resetuje za svaku poruku). `retry = m.Kind != KindHeartbeat`
  (u `raw` formatu se i `unknown` šalje i ponavlja). `Stop`: cancel ctx
  (prekida dial/čitanje/backoff), čeka goroutine, loguje broj odbačenih
  poruka iz reda; višestruki `Stop` i `Stop` bez `Start` su bezbedni.
- Fajlovi: `forwarder.go` (nov), `forwarder_test.go` (nov).
- Odluke/odstupanja: vidi „Promene ugovora“ (Korak 8). `Name()` = `host:port`
  (bez formata).
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (i `-race -count=5`).
  Novi testovi (lažni centar `newFakeCenter(t, addr, respond)` beleži okvire,
  odgovara `respond(n, frame)`; nil → ne odgovara i drži konekciju):
  `TestForwarderACK`, `…NAKRetry` (isti okvir 2×), `…TimeoutRetry`,
  `…CenterDown` (centar upaljen kasnije na istoj adresi), `…DUH`,
  `…HeartbeatNoRetry`, `…HeartbeatCenterDown`, `…Raw` (i unknown u raw),
  `…Enqueue` (pun red, unknown/nil u dc09, posle Stop), `…Stop` (tokom
  backoff-a i tokom čekanja odgovora), `…StopNotStarted`, `TestNewForwarderSpec`.
- Za sledeći korak:
  - U `Processor` dodati `Forwarders []*Forwarder` i `ForwardHeartbeats bool`;
    u `Process` za svaki forwarder `f.Enqueue(m)` (heartbeat samo ako
    `ForwardHeartbeats`). Povratnu vrednost ne treba logovati (Enqueue loguje).
    `unknown` u dc09 formatu: `Enqueue` sam odbija uz upozorenje – ili ga
    Processor preskoči proverom `f.Format() == FormatDC09` da ne bi bilo
    duplog upozorenja (Processor već loguje „WARNING: unrecognized message“).
  - Flagovi: `--forward` (StringSlice; env `UTCAR_FORWARD` lista sa zarezom –
    proveriti da viper `GetStringSlice` deli env po zarezu), `--forward-timeout`
    (10s), `--forward-queue` (1000), `--forward-heartbeats` (true).
    `ForwarderOptions{QueueSize, Timeout}` – backoff ostaviti 0 (default 1s/60s).
    Greška `NewForwarder` = fatalna. Posle `Start()` logovati
    „Forwarding to <Name()> (format <Format()>)“.
  - `Stop` se ne poziva na CTRL-C (`os.Exit` u `receiveSignal`) – vidi
    „Poznati problemi“.
  - Test pomoćnici u `forwarder_test.go`: `newFakeCenter`, `c.next()`,
    `c.none()`, `centerResponse(id)`, `always(id)`, `testForwardOptions`
    (mali backoff), `startForwarder(t, spec, opts)` (Stop u Cleanup),
    `frameBody`. Poruke: `testForwardSIA`, `testForwardSIA2`, `testForwardHeartbeat`.

### Korak 7 – DC-09 okvir (2026-10-07)
- Urađeno: novi `dc09.go` – `CRC16` (CRC-16/ARC), `DC09Frame(body)`
  (`"\n" + %04X crc + %04X len + body + "\r"`), `BuildFrame(m, format)`,
  `ParseResponse(resp)`. `dc09`: SIA → `"PROTO"seq` + `R`rcvr (ako postoji) +
  `L`line (ili `L0`) + `#`acct + `Blocks` + `Timestamp` (deo posle blokova,
  npr. `7C9677F21948CC12|#001465`, se izostavlja); heartbeat → `"NULL"` +
  brojač 0001–9999 (globalni `nullSequence atomic.Uint32`, posle 9999 ide
  0001) + `R`rcvr `L`line `#`acct `[]`; unknown/nil/bez naloga/nepoznat format →
  greška. `raw`: `"\n" + m.Raw + "\r"` za sve vrste poruka (i unknown),
  greška samo za nil ili prazan `Raw`.
- `ParseResponse`: trim `\n\r\x00 `; goli `ACK`/`NAK`/`DUH` prihvata; inače
  traži `"ID"`; ako ispred navodnika ima nešto, mora biti tačno 8 hex znakova
  i proveravaju se CRC i dužina (ostatak od navodnika do kraja). ID mora biti
  ACK/NAK/DUH, inače greška. Sekvenca/nalog u odgovoru se ne proveravaju.
- **Nalaz o prefiksu dekriptovane poruke**: prva 4 znaka (`0101`) **nisu**
  DC-09 CRC. Za README primer `01010053"SIA-DCS"0007R0073L0011[#001365|NUA021*'detector hall'NM]7C9677F21948CC12|#001365`
  CRC tela od navodnika do kraja je `0FAC` (bez repa `7C96…`: `1980`), a
  dužina je 81 (`0x51`) prema `0053` (83) u poruci; `0101` je isti u svim
  primerima (izgleda kao konstanta ATS-a). Dakle `raw` **nije** ispravan DC-09
  okvir – za standardne DC-09 centre koristiti `dc09` (default); `raw` samo
  za centre koji primaju isti format kao ATS (OH+XSIA). (Napomena: tačni
  podaci stvarne poruke bi trebalo da se potvrde na pravom panelu – 2 bajta
  razlike u dužini mogu biti CR/LF ili izmenjen primer u README-u.)
- Fajlovi: `dc09.go` (nov), `dc09_test.go` (nov).
- Odluke/odstupanja: poruka bez naloga → greška (vidi „Promene ugovora“).
  Protokol se prepisuje doslovno (i `*SIA-DCS`/`ADM-CID`). Heartbeat
  `Receiver`/`Line` su decimalni iz `SR0001L0001` – prepisuju se kao jesu.
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (i `-race`). Novi
  testovi: `TestCRC16` (`123456789` → BB3D), `TestDC09Frame`,
  `TestBuildFrameSIA` (5 slučajeva: README, 2 bloka, timestamp, bez L, ADM-CID),
  `TestBuildFrameHeartbeat` (uzastopni brojevi, prelaz 9999→0001),
  `TestBuildFrameErrors`, `TestBuildFrameRaw`, `TestParseResponse`
  (ACK/NAK/DUH sa i bez okvira, smeće, loš CRC/dužina/hex).
- Za sledeći korak:
  - Okvir praviti **jednom po poruci** (pre ponovnih pokušaja) – ponovljeni
    pokušaj mora imati isti broj sekvence (bitno za NULL brojač); greška
    `BuildFrame` → log upozorenje, poruka se ne stavlja u red (ili se
    odbacuje u goroutine-i).
  - Format iz spec-a: `?format=raw` → `FormatRaw`, inače `FormatDC09`;
    nepoznat format → greška u `NewForwarder` (može se proveriti pozivom
    `BuildFrame(&Message{Kind: KindHeartbeat, Account: "1"}, format)` ili
    prostim poređenjem sa konstantama).
  - Odgovor centra se čita do `\r` (okvir počinje sa `\n`); `ParseResponse`
    sam skida `\n\r\x00`. U testovima lažni centar može odgovoriti sa
    `DC09Frame(`"ACK"0007R0075L0001#001465[]`)`, `DC09Frame(`"NAK"...`)`,
    `DC09Frame(`"DUH"...`)`.
  - `nullSequence` je globalan (paket) – u testovima ne pretpostavljati
    konkretan broj NULL sekvence.

### Korak 6 – Integracija baze (2026-10-07)
- Urađeno: `Processor` dobio `Store *Store` i `StoreHeartbeats bool`;
  `Process` posle `m == nil` provere zove `p.store(m)` (nova metoda: preskače
  ako `p`/`Store` nil ili heartbeat uz `!StoreHeartbeats`; greška `Save` → samo
  `log` „Database error: …“, obrada (log, push) se nastavlja). Snimaju se SIA i
  unknown poruke. Flagovi `--db` (`""`) i `--db-heartbeats` (`false`) + viper
  `SetEnvKeyReplacer("-", "_")` (`UTCAR_DB`, `UTCAR_DB_HEARTBEATS`). `run()`:
  ako je `--db` zadat → `OpenStore` (greška = `log.Fatalf`), `defer Close()`,
  log „Storing messages in … (heartbeats: …)“.
- Fajlovi: `processor.go`, `utcar.go`, `processor_test.go`, `utcar_test.go`.
- Odluke/odstupanja: u `run()` `OpenStore` koristi sopstveni `err` (unutar
  `if` bloka), jer goroutine debug servera piše u spoljašnji `err` (race).
  `Save` se poziva i kad je `p.Push` nil. Id iz `Save` se ne koristi.
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (i `-race`). Novi
  testovi: `TestProcessStore` (heartbeat/sia/garbage sa i bez
  `StoreHeartbeats` – broj poruka, događaja i redosled `kind`),
  `TestProcessStoreError` (zatvoren store → push i dalje radi),
  `TestHandleConnectionStore` (end-to-end: SIA + heartbeat preko
  `handleConnection` → 1 red u `messages` sa `received_at`, `remote`
  127.0.0.1:port, `raw` bez LF, 2 događaja sa opisima; push 2). Ručno:
  binar sa `UTCAR_DB`/`UTCAR_DB_HEARTBEATS` pravi bazu; nepostojeći dir → exit 1.
- Za sledeći korak:
  - Korak 7 je samo `dc09.go` (CRC16, DC09Frame, BuildFrame, ParseResponse) –
    bez integracije u `Processor`.
  - Test pomoćnik `countRows(t, s, table)` je u `processor_test.go`.
  - Primeri poruka: `"0101005B\"SIA-DCS\"0008R0075L0001[#001465|NUA021*'hall'NM][#001465|NUR022]"`
    – `Raw` sadrži sve, uključujući `0101005B` (CRC+dužina panela) ispred
    navodnika; `Protocol` je bez navodnika. Za DC-09 okvir
    telo treba sastaviti iz polja (`"SIA-DCS"0008R0075L0001#001465` + `Blocks`
    + `Timestamp`), ne iz `Raw`. Heartbeat → `"NULL"` telo (`Sequence` je
    prazan kod heartbeat-a – izabrati npr. `0000`), unknown u `dc09` → greška.

### Korak 5 – SQLite store (2026-10-07)
- Urađeno: zavisnost `modernc.org/sqlite@v1.40.1`. Novi `store.go`:
  `Store{db *sql.DB}`, `OpenStore(path)` (DSN `file:<path>?_pragma=busy_timeout(5000)
  &_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)`, `SetMaxOpenConns(1)`,
  kreira šemu iz `PLAN.md` – `CREATE ... IF NOT EXISTS`), `Save(m)` (jedna
  transakcija: red u `messages` + red po događaju u `events`; vraća id iz
  `messages`; `Save(nil)` → greška; nulto `m.Time` → `time.Now()`),
  `Close()`. `received_at` = `m.Time.UTC().Format(storeTimeFormat)`
  (`"2006-01-02T15:04:05.000Z"`).
- Fajlovi: `store.go` (nov), `store_test.go` (nov), `go.mod`, `go.sum`.
- Odluke/odstupanja: prazni stringovi se snimaju kao `''` (ne NULL), i za
  `parse_error`. Kolona `kind` = `string(m.Kind)`. `go 1.24.0` u `go.mod`
  (vidi „Promene ugovora“). `Store` nije vezan za `Processor` (to je korak 6).
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (i `-race`). Novi
  testovi: `TestStoreSaveSIA` (2 događaja iz 2 bloka sa ri/id/tekstom, UTC
  konverzija `received_at` iz CEST), `TestStoreSaveHeartbeatAndUnknown`,
  `TestStoreSaveNil`, `TestStoreReopen` (podaci ostaju posle ponovnog
  otvaranja, `journal_mode` = `wal`), `TestOpenStoreError` (nepostojeći dir).
- Za sledeći korak:
  - Dodati u `Processor` polja `Store *Store` i `StoreHeartbeats bool`; `Save`
    pozvati na početku `Process` (posle `m == nil` provere, pre `switch`-a), a
    heartbeat preskočiti ako `!p.StoreHeartbeats`. Greška `Save` → samo `log`
    (ne prekidati obradu).
  - Flagovi `--db` (default `""`) i `--db-heartbeats` (`false`) u `utcar.go`;
    viper treba `SetEnvKeyReplacer(strings.NewReplacer("-", "_"))` za
    `UTCAR_DB_HEARTBEATS`. U `run()`: ako `--db != ""` → `OpenStore`, greška =
    fatalna, `defer store.Close()`.
  - Test pomoćnici u `store_test.go`: `openTestStore(t)` (vraća `*Store` i
    putanju, zatvara u `t.Cleanup`), `readMessage(t, s, id)` →
    `storedMessage`, `readEvents(t, s, id)` → `[]Event`. Broj redova:
    `s.db.QueryRow("SELECT COUNT(*) FROM messages")`.
  - `Save` je bezbedan za istovremene pozive (jedna konekcija; `database/sql`
    serijalizuje).

### Korak 4 – Processor i integracija parsera (2026-10-07)
- Urađeno: novi `processor.go` – `Processor{Push chan SIA}` i
  `(p *Processor) Process(m *Message)` (radi i sa `p == nil` i `m == nil`):
  heartbeat → samo log; unknown → `log` „WARNING: unrecognized message …“
  (bez panic-a); SIA → `requests.Add(1)`, log zaglavlja i svakog događaja
  (`formatEvent(e)`), pa za svaki događaj `SIA{m.Time, Sequence, Receiver,
  Line, Account, e.Code, e.Zone}` u `p.Push` (ako nije nil).
  `handleConnection(c net.Conn, p *Processor)`: vreme prijema se uzima odmah
  posle `Read`; posle ACK-a `ParseMessage(data)`, `m.Time`, `m.Remote =
  c.RemoteAddr().String()`, `p.Process(m)`. `run()` pravi
  `p := &Processor{Push: pchan}` (pchan je nil ako nema `--addr`).
- Fajlovi: `processor.go` (nov), `processor_test.go` (nov), `utcar.go`,
  `utcar_test.go` (prepravljen), obrisani `parser.go` i `parser_test.go`.
- Odluke/odstupanja: `ParseSIA` uklonjen (nije bilo drugih korisnika);
  `TestIsHeartbeat` nije premeštan jer `TestIsHeartbeatNUL` u `sia_test.go`
  pokriva iste slučajeve. `requests` broji samo SIA poruke (kao ranije –
  heartbeat se nije brojao, unknown je ranije pucao pre brojanja).
  `Processor` još nema polja za bazu/prosleđivanje (vidi „Promene ugovora“).
- Provera: gofmt (moji fajlovi) ✅, go vet ✅, go test ✅ (i `-race`). Novi
  testovi: `TestProcessNil`, `TestProcessPush` (3 događaja iz 2 bloka),
  `TestProcessRequests`, `TestFormatEvent`, `TestHandleConnectionPush`
  (2 događaja → 2 SIA u kanalu, proverava i `time`),
  `TestHandleConnectionUnknown` (smeće i nezavršen blok → ACK, handler se
  vraća, ništa u Push), `TestHandleConnectionHeartbeat`.
- Za sledeći korak:
  - Korak 5 je samo `store.go` (OpenStore/Save/Close) – u `Processor` ga
    uvodi tek korak 6 (dodati polja `Store *Store`, `StoreHeartbeats bool`;
    snimanje staviti u `Process` pre `switch`-a/ranih `return`-ova jer se
    unknown i (opciono) heartbeat takođe snimaju).
  - `m.Time` je lokalno vreme (`time.Now()`); za `received_at` koristiti
    `m.Time.UTC().Format("2006-01-02T15:04:05.000Z")`.
  - Test pomoćnici u `utcar_test.go`: `startServer(t, p)` (jedna konekcija,
    vraća addr i `done`), `sendMessage(t, addr, msg)` (dopunjuje NUL-ovima do
    bloka od 8 bajtova i proverava ACK), `waitDone(t, done)`.
  - Testovi u package `main`, poruke za test: `ParseMessage([]byte(...))`
    sa primerima iz `processor_test.go`/`sia_test.go`.

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
