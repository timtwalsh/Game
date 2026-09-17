# How to walk this map

**Universes** (per the ICM system-map convention):

- **live** — implement and cite against these. Everything catalogued here
  is live unless marked otherwise: this is a small, working codebase, not
  a large one with much dead weight yet.
- **leftover** — none identified yet.
- **ghost** — `docs/ANI_MAKER_SPEC.md` describes features `animaker/pkg/`
  does not implement yet (see its object card). The game side has no
  integration code that *loads* animaker's output files — that link is
  designed in docs, not built in code. `docs/ARCHITECTURE.md` and
  `docs/PROTOCOL_REFERENCE.md` also describe a good deal of designed-but-
  unbuilt behavior (attacks, level loading, logging, bans, auth, rate
  limiting) — as of 2026-09-17 both docs mark every such section inline,
  so read them directly rather than assuming this map repeats that list.

**Name collisions worth knowing up front:**

- "Player" is not one type. The server holds `shared.PlayerState`
  (authoritative, networked). The client holds a `PlayerController`
  (local, predicted) for itself and a `PlayerInterpolation` per remote
  player. They are related but not interchangeable — see
  `objects/client-prediction.md`.

**Walking rule:** open the routing table in `CLAUDE.md`, open the one
object card you need, follow its `See` link into the real source. Don't
open every card in `objects/` for a single change.
