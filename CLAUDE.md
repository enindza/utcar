# utcar

Go program koji prima poruke od ATS alarmnog sistema (OH+XSIA, 3DES), parsira
SIA poruke i opcionalno ih šalje openHAB-u. U toku je proširenje (sve SIA
komande, SQLite baza, prosleđivanje ka monitoring centrima po DC-09).

## Rad po koracima

- Plan: `plan/PLAN.md` (odluke, ugovori između koraka, sadržaj koraka).
- Stanje i beleške između koraka: `plan/STANJE.md`.
- Korisnik radi `/clear` pa `/korak` – skill `.claude/skills/korak/SKILL.md`
  izvršava sledeći korak. Ako `/korak` nije dostupan kao komanda, pročitaj taj
  fajl i uradi po njemu.

## Konvencije

- Uvek `export GOTOOLCHAIN=local` pre `go` komandi (`go.mod` ostaje na Go 1.24).
- Provera: `gofmt -l .`, `go vet ./...`, `go test ./...`.
- `pusher.go` (openHAB) se ne menja.
- Komunikacija sa korisnikom na srpskom.
