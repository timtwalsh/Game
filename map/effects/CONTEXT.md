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

- `build_local.ps1` builds all three: `./server`, `./client` (from the
  root `game` module), and `animaker` (from within `animaker/`, since
  it's a separate module) — and by default launches all of them. Adding a
  fourth buildable component means editing it too. Before building
  animaker it checks for `gcc` on PATH and, if missing, looks for the
  WinLibs GCC install under
  `%LOCALAPPDATA%\Microsoft\WinGet\Packages\BrechtSanders.WinLibs*` and
  temporarily prepends its `bin/` to PATH plus sets `CGO_ENABLED=1` for
  just that build step (restored afterward). An animaker build failure
  is still handled as non-fatal (warns and skips launching it) so the
  script keeps working for the game if animaker's build breaks again for
  an unrelated reason.
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

## Resolved gaps (kept for context)

- As of 2026-09-17, `animaker/` builds locally on the Windows dev machine
  too: WinLibs GCC (MinGW-w64) is installed via
  `winget install BrechtSanders.WinLibs.POSIX.UCRT` (a ~150-250MB portable
  zip extract, no registry/system-wide install), and `build_local.ps1`
  finds it automatically (see above) without needing it permanently on
  PATH. If animaker's build starts failing locally again, check whether
  that winget package is still present before assuming a code regression
  — `winget list --id BrechtSanders.WinLibs.POSIX.UCRT`.

## If the index and a card disagree

Fix the card, not this file — this file is a pointer, the card is the
source of truth for the waterfall.
