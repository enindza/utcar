# Pravila rada (uvek važe; ovaj fajl održava šablon, ne menjaj ga u projektu)
- Radi SAMO u obimu koji je zadat u upitu ili u aktivnom koraku plana. Ono što primetiš van obima ne popravljaj, nego zabeleži u izveštaj koraka.
- Pre izmene modula pročitaj njegov `app/modules/<ime>/CLAUDE.md`. Pre izmene veze između modula pročitaj `docs/ARHITEKTURA.md`.
- Ne čitaj `docs/LOG.md`. U njega se samo dopisuje.
- Za pretragu po kodu koristi `istrazivac` subagenta (Haiku), po pravilima iz `.claude/rules/subagenti.md`. Ne čitaj fajlove "za svaki slučaj".
- Tok rada za veće izmene: `/plan` (jači model) → `/korak` (Sonnet) → commit → `/clear` → `/korak` ... Posle koraka GOTOVO-DRUGAČIJE ili BLOKIRAN: `/provera` (jači model). Isti tok bez čoveka: `python .claude/scripts/vozi.py` iz terminala.
- Stavke za buduće planove idu u `docs/plans/IDEJE.md` (samo dopisivanje; čita ih `/plan`).
- Istraživanje van plana (probanje varijanti u slobodnoj sesiji) ne sme u `app/`. Kad zaključiš: `/zabelezi` pa `/clear`; to snima folder `docs/plans/istrazivanja/<ime>/` i red u `IDEJE.md`, a `/plan` ga kasnije preuzme.
- Fajlove pod `.claude/`, `docs/plans/_sablon/` i `docs/plans/istrazivanja/_sablon/` održava šablon (`/sablon`). Pravila specifična za projekat idu u nov fajl `.claude/rules/projekat-*.md`, ne u postojeće.
