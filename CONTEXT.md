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
.\build_local.ps1        # builds bin\server.exe and bin\client.exe
.\build_local.ps1 -Run   # also launches server + 2 test clients
```

`animaker` builds independently: `cd animaker; go build`.

## Human checks

- Before changing `shared/protocol.go` or `shared/types.go`: read
  [map/effects/CONTEXT.md](map/effects/CONTEXT.md) — both client and server
  decode these structs and a silent format change breaks the other side.
- Before changing anti-cheat thresholds in `shared/types.go`
  (`Suspicion*`, `MaxSpeed`, `SpeedTolerance`): read the anti-cheat design
  in `docs/ARCHITECTURE.md` first — the numbers there and in code should
  stay in sync manually, there is no shared source of truth for them yet.
