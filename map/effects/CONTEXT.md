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

- `build_local.ps1` now builds all three: `./server`, `./client` (from
  the root `game` module), and `animaker` (from within `animaker/`, since
  it's a separate module) — and by default launches all of them. Adding a
  fourth buildable component means editing it too. An animaker build
  failure is handled as non-fatal (warns and skips launching it) so the
  script still works for the game while animaker is broken.
- `.github/workflows/test.yml` has two jobs, both green as of 2026-09-17:
  `game` runs `go vet`/`go build ./...`/`go test ./... -v` for the root
  module; `animaker` runs `go build ./...` for that module. Both need
  native Linux packages installed first because `raylib-go` (game) and
  Fyne (animaker) use cgo — `gcc libgl1-mesa-dev xorg-dev`, plus
  `libwayland-dev libxkbcommon-dev` for `raylib-go` specifically (it
  fails at `go vet` without them: `fatal error:
  wayland-client-core.h: No such file or directory`). If you add a new
  cgo dependency, expect to extend this apt-get list, not just the
  Go module graph.
- No other external configs or scheduled jobs reference paths inside this
  tree as of this writing.

## Known gaps to close before this matters more

- `animaker/` builds fine in CI (Linux, with the apt packages above) but
  **not on this Windows dev machine**: it needs `CGO_ENABLED=1` and a
  real C compiler, and despite `go env CC` reporting `gcc` as the
  configured default, no `gcc` executable is actually on PATH here (in
  either PowerShell or Git Bash) — `go env CC` names the assumed
  compiler, it doesn't confirm one is installed. Until a MinGW-w64
  toolchain is installed and on PATH, `build_local.ps1`'s animaker step
  will keep failing locally even though the same code builds cleanly on
  every push.

## If the index and a card disagree

Fix the card, not this file — this file is a pointer, the card is the
source of truth for the waterfall.
