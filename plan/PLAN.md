# Plan realizacije

Ovaj fajl je **stabilan**: opisuje ciljeve, odluke, ugovore između koraka i sadržaj
svakog koraka. Trenutno stanje (šta je urađeno, šta je sledeće, beleške za sledeći
korak) je u [`STANJE.md`](STANJE.md). Plan se menja samo ako se promeni odluka –
i to se onda zapiše u dnevnik u `STANJE.md`.

## Cilj

1. **Sve SIA komande** – parser prepoznaje svaku SIA-DCS poruku (bilo koji kod,
   zona promenljive dužine, particija, više događaja, tekst), opisuje kod iz
   tabele, a nepoznate kodove ne odbacuje.
2. **Snimanje u bazu** – svaka primljena poruka (i događaji iz nje) se snima u
   SQLite.
3. **Prosleđivanje ka monitoring centrima** – dekriptovane poruke se prosleđuju
   na N (konkretno 2) centra, nešifrovanim SIA DC-09 protokolom preko TCP-a.

## Odluke (potvrdio korisnik)

| Tema | Odluka |
|---|---|
| Baza | **SQLite**, drajver `modernc.org/sqlite` **v1.40.1** (pure Go, radi sa `CGO_ENABLED=0` i u `FROM scratch` image-u, kompatibilan sa Go 1.24) |
| Protokol ka centrima | **Nešifrovan TCP, SIA DC-09 okvir** (`LF crc 0LLL "id"seq Rrcvr Lpref #acct[data] CR`), čeka se ACK centra |
| ACK alarmu | **Odmah** (kao do sada); prosleđivanje ide u pozadini, sa redom čekanja i ponovnim pokušajima |
| openHAB (`pusher.go`) | **Ne dira se.** Mora i dalje da se kompajlira i radi za UA/UR; dobija `SIA` strukturu kao do sada |

## Tehničke odluke (moje, mogu se menjati uz zapis u dnevnik)

- Go verzija u `go.mod` ostaje **1.24**. Uvek raditi sa `GOTOOLCHAIN=local`, da
  `go get` ne podigne `go` direktivu. Ako zavisnost traži noviji Go – izabrati
  stariju verziju zavisnosti.
- `SIA` struct iz `utcar.go` ostaje nepromenjen (koristi ga pusher i
  `pusher_test.go`). Novi model poruke je poseban tip `Message`.
- Neprepoznata poruka **ne izaziva panic** – snima se u bazu kao `unknown`,
  loguje se upozorenje.
- Heartbeat (`SR0001L0001 001465XX [ID...]`) se prema centrima šalje kao DC-09
  **`"NULL"`** poruka (nadzor veze). Može se isključiti flagom.
- Heartbeat se **ne snima** u bazu po defaultu (stiže svakog minuta); može se
  uključiti flagom.
- Format prema centru po defaultu je **`dc09`** (okvir se ponovo sastavlja sa
  ispravnim CRC-om i dužinom). Opcija `raw` šalje dekriptovanu poruku doslovno
  (`LF + poruka + CR`) – za centre koji primaju isti format kao ATS panel.
- U `dc09` formatu poruka tipa `unknown` se ne prosleđuje (centar bi je
  odbio) – samo upozorenje u logu.
- Svaki centar ima **svoj red i svoju goroutine**; pad jednog ne blokira drugi
  ni prijem od alarma. Redosled poruka ka jednom centru se čuva.
- Ponovni pokušaji: događaji – dok ne uspe (backoff 1s → max 60s); heartbeat
  (NULL) – jedan pokušaj, bez ponavljanja. (Korak 10, odluka korisnika: posle
  5 uzastopnih `NAK`-ova SIA poruka se odbacuje; `unknown` u `raw` – jedan pokušaj.) Pun red → poruka se odbacuje uz
  `log` upozorenje.
- Odgovor centra: `"ACK"` = uspeh; `"NAK"` ili timeout/prekid = ponovi;
  `"DUH"` = centar ne podržava poruku → odustani (log).
- CRC za DC-09: **CRC-16/ARC** (poly 0x8005 reflektovan = 0xA001, init 0).
  Kontrolna vrednost: `crc16("123456789") == 0xBB3D`.

## Ugovori između koraka (API)

Koraci se oslanjaju na ove tipove i funkcije. Ako korak mora da odstupi, to
upisuje u dnevnik u `STANJE.md` (sekcija „Promene ugovora“).

```go
// sia.go
type MessageKind string
const (
    KindHeartbeat MessageKind = "heartbeat"
    KindSIA       MessageKind = "sia"
    KindUnknown   MessageKind = "unknown"
)

type Event struct {
    Code        string // dva slova, npr. "BA"
    Description string // iz tabele kodova, "" ako je nepoznat (korak 3 popunjava)
    Zone        string // adresa/zona kako je stigla, npr. "021", "1", ""
    Area        string // iz modifikatora ri, "" ako ga nema
    User        string // iz modifikatora id
    Time        string // iz modifikatora ti, npr. "12:30"
    Text        string // tekst iz *'...'
}

type Message struct {
    Time      time.Time // vreme prijema (postavlja handleConnection)
    Remote    string    // adresa panela (postavlja handleConnection)
    Raw       string    // dekriptovana poruka, bez LF/CR/NUL na krajevima
    Kind      MessageKind
    Protocol  string // npr. "SIA-DCS" (bez navodnika)
    Sequence  string // "0007"
    Receiver  string // "0075" (bez 'R'), "" ako ga nema
    Line      string // "0001" (bez 'L'), "" ako ga nema
    Account   string // "001465" (bez '#')
    Blocks    string // svi data blokovi doslovno, sa zagradama: "[#001465|NRP000*'DECKERS'NM]"
    Timestamp string // DC-09 vremenska oznaka ako postoji ("_HH:MM:SS,MM-DD-YYYY"), inače ""
    Events    []Event
    ParseError string // "" ako je parsiranje uspelo
}

func ParseMessage(data []byte) *Message   // nikad ne vraća nil, nikad ne panikuje
func IsHeartbeat(input []byte) bool        // postoji, ispravlja se regex

// siacodes.go
func DescribeSIA(code string) string       // "" za nepoznat kod

// processor.go
type Processor struct {
    Store             *Store        // nil = bez baze
    Forwarders        []*Forwarder  // prazno = bez prosleđivanja
    Push              chan SIA      // nil = bez openHAB-a (postojeći pusher)
    StoreHeartbeats   bool
    ForwardHeartbeats bool
}
func (p *Processor) Process(m *Message)    // p može biti nil (testovi)
// handleConnection(c net.Conn, p *Processor)

// store.go
func OpenStore(path string) (*Store, error)
func (s *Store) Save(m *Message) (int64, error) // vraća id iz tabele messages
func (s *Store) Close() error

// dc09.go
func CRC16(data []byte) uint16
func DC09Frame(body string) []byte                  // "\n" + %04X crc + %04X len + body + "\r"
func BuildFrame(m *Message, format string) ([]byte, error) // format: "dc09" | "raw"
func ParseResponse(resp []byte) (status string, err error)  // "ACK" | "NAK" | "DUH"

// forwarder.go
type ForwarderOptions struct {
    QueueSize  int
    Timeout    time.Duration
    MinBackoff time.Duration
    MaxBackoff time.Duration
}
func NewForwarder(spec string, opts ForwarderOptions) (*Forwarder, error)
// spec: "host:port" | "tcp://host:port" | "tcp://host:port?format=raw"
func (f *Forwarder) Start()
func (f *Forwarder) Enqueue(m *Message) bool // false = red pun, poruka odbačena
func (f *Forwarder) Stop()                    // prekida i ponovne pokušaje
func (f *Forwarder) Name() string
```

### Šema baze (korak 5)

```sql
CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    received_at TEXT NOT NULL,      -- UTC, "2006-01-02T15:04:05.000Z"
    remote      TEXT,
    kind        TEXT NOT NULL,      -- heartbeat | sia | unknown
    protocol    TEXT,
    sequence    TEXT,
    receiver    TEXT,
    line        TEXT,
    account     TEXT,
    raw         TEXT NOT NULL,
    parse_error TEXT
);
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id  INTEGER NOT NULL REFERENCES messages(id),
    code        TEXT NOT NULL,
    description TEXT,
    zone        TEXT,
    area        TEXT,
    user_id     TEXT,
    event_time  TEXT,
    text        TEXT
);
CREATE INDEX IF NOT EXISTS idx_messages_received_at ON messages(received_at);
CREATE INDEX IF NOT EXISTS idx_events_code ON events(code);
CREATE INDEX IF NOT EXISTS idx_events_zone ON events(zone);
```

### Novi flagovi (env: `UTCAR_` + ime velikim slovima, `-` → `_`)

| Flag | Default | Korak |
|---|---|---|
| `--db` | `""` (bez baze) | 6 |
| `--db-heartbeats` | `false` | 6 |
| `--forward` (može više puta; env: lista odvojena zarezom) | prazno | 9 |
| `--forward-timeout` | `10s` | 9 |
| `--forward-queue` | `1000` | 9 |
| `--forward-heartbeats` | `true` | 9 |

Napomena za env sa crticom: viper treba `SetEnvKeyReplacer(strings.NewReplacer("-", "_"))`.

---

## Koraci

Svaki korak: mala zaokružena celina, posle koje **`go vet ./...` i
`go test ./...` prolaze**, sve je commit-ovano i push-ovano.

### Korak 0 – Priprema
- `go.mod` (Go 1.24, cobra, viper), postojeći testovi prolaze.
- Plan (`plan/PLAN.md`), stanje (`plan/STANJE.md`), komanda `/korak`
  (`.claude/skills/korak/SKILL.md`), `CLAUDE.md`.

### Korak 1 – Model poruke i parser zaglavlja
- Novi fajl `sia.go`: tipovi `MessageKind`, `Event`, `Message`; `ParseMessage`.
- `ParseMessage` u ovom koraku: heartbeat → `KindHeartbeat` (+ Receiver, Line,
  Account iz `SR0001L0001 001465XX [ID..]`; Account bez završnih `X`);
  SIA zaglavlje (`"PROTO"seq R.. L.. #acct? [blokovi] _timestamp?`, traži se bilo
  gde u poruci – ispred navodnika stoje CRC/dužina) → `KindSIA`, popunjava
  Protocol, Sequence, Receiver, Line, Account (iz zaglavlja ili iz `#acct|` u
  prvom bloku), Blocks, Timestamp. Sve ostalo → `KindUnknown` + `ParseError`.
- Blokovi: uzastopne `[...]` grupe; `]` unutar `'...'` teksta ne zatvara blok.
- Ispraviti regex u `IsHeartbeat` (problem sa `|` koji deli ceo izraz; NUL
  karakteri umesto razmaka dozvoljeni: `[\s\x00]+`).
- **Ne** dirati još `ParseSIA` i `handleConnection`.
- Testovi: `sia_test.go` – primeri iz README-a i testova, heartbeat sa NUL
  karakterima, poruka sa hex nalogom, poruka sa timestamp-om, smeće → unknown.

### Korak 2 – Parser SIA data bloka (događaji)
- U `sia.go`: popuniti `Message.Events` za `KindSIA` kada je `Protocol` `SIA-DCS`
  ili `*SIA-DCS`.
- Data blok: `#acct|` + događaji. Početno `N` (nov) / `O` (star) se skida samo
  ako ostatak i dalje počinje validnim tokenom (inače npr. `OP001` bez prefiksa
  bi postao `P001`).
- Tokeni odvojeni sa `/` (osim unutar `'...'`). Token od 2 mala slova na početku
  = modifikator (`ri` → Area, `id` → User, `ti` → Time, ostali se ignorišu);
  modifikatori važe za sledeće događaje u istom bloku. Token `[A-Z]{2}` + ostatak
  = događaj (Code, Zone = ostatak). Sufiks `*'tekst'XX` → Text.
- Testovi: `NUA021*'detector hall'NM`, `NRP000*'DECKERS'NM`,
  `Nri1/BA001`, `Nri01/CL501/ri02/CL501`, `Nti12:30/id5/OP001`, `NBA1`,
  `NYR` (bez zone), više događaja, nepoznat kod `NQQ123`.

### Korak 3 – Tabela SIA kodova
- Novi fajl `siacodes.go`: `map[string]string` sa svim standardnim SIA DC-03
  kodovima (AA…ZU) i engleskim opisima; `DescribeSIA`.
- `ParseMessage` popunjava `Event.Description`.
- Testovi: poznati kodovi (BA, UA, UR, CL, OP, RP, YT…), nepoznat → "".

### Korak 4 – Processor i integracija parsera u `handleConnection`
- Novi fajl `processor.go`: `Processor` i `Process` (za sada: log poruke i
  događaja, `requests` brojač, slanje u `Push` kanal – za svaki događaj
  `SIA{m.Time, m.Sequence, m.Receiver, m.Line, m.Account, e.Code, e.Zone}`).
- `handleConnection(c, p *Processor)`: posle ACK-a → `ParseMessage`, postavi
  `Time`/`Remote`, `p.Process(m)`. Ukloniti `log.Panicf` za nepoznatu poruku.
- Ukloniti stari `ParseSIA` (i njegov test) – ili ostaviti kao omotač, po izboru;
  zapisati u dnevnik.
- `run()` pravi `Processor` i prosleđuje ga; pusher logika ostaje ista.
- Testovi: postojeći `utcar_test.go` prilagoditi (`handleConnection(conn, nil)`),
  plus test da nepoznata poruka i dalje dobija ACK i ne ruši server.

### Korak 5 – SQLite store
- `GOTOOLCHAIN=local go get modernc.org/sqlite@v1.40.1`.
- Novi fajl `store.go`: `OpenStore` (kreira šemu; `SetMaxOpenConns(1)`,
  `busy_timeout`, WAL), `Save` (transakcija: messages + events), `Close`.
- Testovi: `store_test.go` sa `t.TempDir()` – snimi SIA poruku sa 2 događaja,
  heartbeat, unknown; pročitaj nazad i proveri kolone.

### Korak 6 – Integracija baze
- Flagovi `--db`, `--db-heartbeats` (+ env, `SetEnvKeyReplacer`).
- `run()` otvara store ako je `--db` zadat; `Processor.Process` snima
  (heartbeat samo ako `StoreHeartbeats`). Greška baze se loguje, ne ruši prijem.
- Test: end-to-end preko `handleConnection` sa store-om u temp fajlu.

### Korak 7 – DC-09 okvir
- Novi fajl `dc09.go`: `CRC16`, `DC09Frame`, `BuildFrame`, `ParseResponse`.
- `BuildFrame(m, "dc09")`:
  - SIA: telo `"PROTO"seq` + `R`rcvr (ako postoji) + `L`line (ili `L0`) +
    `#`acct + Blocks + Timestamp.
  - Heartbeat: telo `"NULL"` + seq (brojač 0001–9999) + `R`rcvr `L`line
    `#`acct + `[]`.
  - Unknown: greška.
- `BuildFrame(m, "raw")`: `"\n" + m.Raw + "\r"`.
- Proveriti na primerima iz README-a/testova da li su prva 4 hex znaka posle
  LF u dekriptovanoj poruci već CRC (i sledeća 4 dužina) – zapisati nalaz u
  dnevnik (utiče na to da li je `raw` dovoljan za DC-09 centre).
- Testovi: `CRC16("123456789") == 0xBB3D`, format okvira, dužina, NULL okvir,
  `ParseResponse` za ACK/NAK/DUH/smeće.

### Korak 8 – Forwarder
- Novi fajl `forwarder.go` po ugovoru: red (kanal), goroutine, za svaku poruku
  `net.DialTimeout` → write okvir → čitanje odgovora do `\r` sa deadline-om →
  `ParseResponse`. Ponovni pokušaji po pravilima iz „Tehničkih odluka“. `Stop`
  prekida i čekanje na backoff.
- Testovi `forwarder_test.go` sa lažnim centrom (`net.Listen` na `127.0.0.1:0`):
  ACK → isporučeno jednom; NAK pa ACK → 2 pokušaja; centar ugašen pa upaljen →
  isporučeno posle backoff-a (mali backoff u testu); DUH → odustaje; heartbeat
  neuspeh → nema ponavljanja; pun red → `Enqueue` vraća false; spec parsiranje.

### Korak 9 – Integracija prosleđivanja
- Flagovi `--forward`, `--forward-timeout`, `--forward-queue`,
  `--forward-heartbeats` (+ env; `UTCAR_FORWARD` lista odvojena zarezom).
- `run()` pravi i startuje forwardere; `Processor.Process` šalje svakom
  (`Enqueue`), poštuje `ForwardHeartbeats`.
- Test: end-to-end – panel (test klijent) → utcar → **2 lažna centra**, oba
  primaju ispravan DC-09 okvir; jedan centar ugašen ne utiče na drugi.

### Korak 10 – Dokumentacija, Docker, završna provera
- README: nove funkcije, flagovi, env, primeri (2 centra, baza), primeri SQL
  upita, napomena o redu čekanja (gubi se pri gašenju programa).
- Dockerfile/README: `CGO_ENABLED=0` build, volume za bazu (`-v`), primer.
- `go vet`, `gofmt -l .`, `go test ./...`, pregled celog diff-a od koraka 0.
- Zatvoriti plan u `STANJE.md` (svi koraci gotovi) i predložiti PR.

## Moguća proširenja (van plana)

- Trajni red za prosleđivanje (u bazi), da se ne gubi pri restartu.
- Tabela statusa isporuke po centru (audit).
- Contact ID (`ADM-CID`) parsiranje.
- DC-09 AES šifrovanje prema centrima, UDP.
