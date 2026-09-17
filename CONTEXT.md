# Workspace contract

**What this is:** a 2D top-down multiplayer RPG (ALTTP-style), UDP client/server,
client-side prediction with server-side movement validation and anti-cheat
scoring. Plus a separate desktop tool (`animaker`) for authoring sprite
animations consumed by the game.

## Modules

| Module | go.mod | Contains | Depends on |
|---|---|---|---|
| `game` | [go.mod](go.mod) | `client/`, `server/`, `shared/` | raylib-go |
| `animaker` | [animaker/go.mod](animaker/go.mod) | `animaker/pkg/` | Fyne |

The two modules do not import each other. `animaker` produces animation
asset files the game is meant to consume; as of this writing that
integration is not wired up in code — see [map/effects/CONTEXT.md](map/effects/CONTEXT.md).

## Layout

- `docs/` — factory: design reference (architecture, protocol, decisions,
  the animaker spec). Stable across changes; read before making a design
  call, but verify against code if it's been a while — see `docs/CONTEXT.md`
  for what's been checked and when.
- `map/` — factory: a live-maintained index of the actual code (nouns,
  what a change hits). Update it when you add/rename a type or change a
  cross-cutting behavior (protocol format, anti-cheat thresholds).
- `client/`, `server/`, `shared/`, `animaker/` — product: the actual code.
- `assets/` — product: game art (currently just `tileset.png`).
- `bin/` — product: build output from `build_local.ps1`. Not source; safe
  to delete and rebuild.

## Build

```powershell
.\build_local.ps1          # builds bin\server.exe, bin\client.exe, bin\animaker.exe,
                            # then launches server + 2 test clients + animaker
.\build_local.ps1 -NoRun   # build only, don't launch anything
```

Animaker is built as one of the "tools" in the same script (from within
`animaker/`, since it's a separate module). It needs cgo (`CGO_ENABLED=1`)
and a real C compiler — as of 2026-09-17 this machine has WinLibs GCC
(MinGW-w64) installed via `winget install BrechtSanders.WinLibs.POSIX.UCRT`,
and `build_local.ps1` finds it automatically (checks PATH first, falls back
to the winget install location) without needing it permanently on PATH.
On a fresh machine without that package installed, this step will fail;
the script treats that as non-fatal (warns and skips launching it) so it
still works for the game.

## Workflow

**As of 2026-09-18: feature branch + PR, not direct-to-main.** New work
goes on its own branch, opened as a PR against `main` for review before
merging. This replaces the earlier prototyping-phase practice of pushing
straight to `main` (2026-09-17 through 2026-09-18) — commits from that
window are already on `main` directly and were not retroactively moved
onto branches/PRs. CI (`.github/workflows/test.yml`) still runs on every
push and on every PR; with this change it can now actually gate a merge
instead of only catching problems after the fact.

## Testing

```powershell
go test ./...          # runs shared/, client/, server/ test suites
go test ./... -v       # verbose, per-test output
```

Coverage as of 2026-09-17: `shared/types_test.go` (Vec2, TileType,
CollisionLayer), `client/prediction_test.go` (input/movement/interpolation
math), `server/validation_test.go` (speed/wall-phase checks, suspicion
thresholds). These are regression tests against current behavior, not a
full TDD suite — `client/main.go` and `server/main.go` (the network/render
loops) are untested glue code. Extend this pattern (one `_test.go` beside
the file it tests, in the same package) as new logic is added; update the
relevant `map/objects/*.md` card's "Tests:" line when you do, so the map
stays accurate.

`.github/workflows/test.yml` has two jobs, both green as of 2026-09-17:
`game` (`go vet`, `go build ./...`, `go test ./... -v`) and `animaker`
(`go build ./...`). Both install native Linux packages first — `raylib-go`
and Fyne are cgo-based, not pure Go, so the runner needs X11/Wayland/GL
dev headers (`gcc libgl1-mesa-dev xorg-dev libwayland-dev
libxkbcommon-dev`) or `go vet`/`go build` fail before ever reaching your
code. This job silently failed on every push for the first several
commits of this repo's history until that was diagnosed and fixed — if
CI goes red on a change that looks unrelated to your diff, check whether
it's actually a missing native dependency before assuming your code broke
it.

## Human checks

- Before changing `shared/protocol.go` or `shared/types.go`: read
  [map/effects/CONTEXT.md](map/effects/CONTEXT.md) — both client and server
  decode these structs and a silent format change breaks the other side.
- Before changing anti-cheat thresholds in `shared/types.go`
  (`Suspicion*`, `MaxSpeed`, `SpeedTolerance`): read the anti-cheat design
  in `docs/ARCHITECTURE.md` first — the numbers there and in code should
  stay in sync manually, there is no shared source of truth for them yet.
