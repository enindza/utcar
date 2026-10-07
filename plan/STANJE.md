# Stanje realizacije

> Ovaj fajl je **kanal komunikacije između koraka**. Svaki korak ga čita na
> početku i ažurira na kraju (pre commit-a). Plan i ugovori: [`PLAN.md`](PLAN.md).

## Sledeći korak

**Korak 1 – Model poruke i parser zaglavlja**

## Grana

`claude/amazing-thompson-cucp4s` (ako sesija zadaje drugu granu – koristi nju i
upiši je ovde).

## Pregled

| # | Korak | Status | Commit |
|---|---|---|---|
| 0 | Priprema (go.mod, plan, `/korak`) | ✅ gotovo | (ovaj commit) |
| 1 | Model poruke i parser zaglavlja | ⏳ sledeći | |
| 2 | Parser SIA data bloka (događaji) | ⬜ | |
| 3 | Tabela SIA kodova | ⬜ | |
| 4 | Processor i integracija parsera | ⬜ | |
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
