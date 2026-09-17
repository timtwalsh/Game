# Change-impact index

"I'm changing X, what inside this tree moves." First-order only — open
the named card for the full waterfall.

| Changing... | Open these cards | Why |
|---|---|---|
| `shared/protocol.go` (message shapes) | [protocol.md](../objects/protocol.md), [client-prediction.md](../objects/client-prediction.md), [server-anticheat.md](../objects/server-anticheat.md) | Both binaries decode these structs independently; a mismatch fails silently at runtime, not at compile time. |
| `shared/types.go` constants (`MaxSpeed`, `SpeedTolerance`, `Suspicion*`, `TileSize`) | [protocol.md](../objects/protocol.md), [server-anticheat.md](../objects/server-anticheat.md), `docs/ARCHITECTURE.md` | Anti-cheat thresholds and movement speed are compiled into both client and server; no runtime negotiation, no shared config file. |
| `client/prediction.go` | [client-prediction.md](../objects/client-prediction.md) | Self-contained to `client/`; does not affect `server/`. |
| `server/validation.go` | [server-anticheat.md](../objects/server-anticheat.md) | Self-contained to `server/`; does not affect `client/`. |
| `animaker/pkg/**` | [animaker.md](../objects/animaker.md) | Isolated module; verified no import from/to `client/`, `server/`, `shared/`. |
| Level/collision format (`shared.Level`, `CollisionLayer`) | [protocol.md](../objects/protocol.md) | Both `client/main.go` and `server/main.go` currently construct a placeholder all-walkable `CollisionLayer` instead of loading one — adding real level loading touches both call sites. |

## Outside this tree

- `build_local.ps1` hardcodes the two build targets (`./server`,
  `./client`) and the two output binary names — adding a third
  buildable component means editing it too.
- `.github/workflows/test.yml` runs `go vet`, `go build ./...`, and
  `go test ./... -v` for the root `game` module on every push/PR. It does
  **not** build or test `animaker/` (separate module, currently broken —
  see below) — if you fix that, add a second job for it here and in the
  workflow.
- No other external configs or scheduled jobs reference paths inside this
  tree as of this writing.

## Known gaps to close before this matters more

- `animaker/` currently fails `go build` (missing `go.sum` entries for its
  Fyne/toml dependencies) — flagged separately, not yet fixed. Changing
  anything in `animaker/pkg/` won't get CI feedback until that's resolved.

## If the index and a card disagree

Fix the card, not this file — this file is a pointer, the card is the
source of truth for the waterfall.
