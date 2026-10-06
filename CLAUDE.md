# Game — entry point

2D top-down MMO, ALTTP-style, written in Go. Two independent Go modules live
here: the game itself and a standalone sprite-animation editor. See
[CONTEXT.md](CONTEXT.md) for how they relate and how to build them.

## Where things live

| I want to... | Go to |
|---|---|
| Understand the overall design/network model/anti-cheat | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| See *why* a tech decision was made (Go vs Rust vs C++, GC tuning) | [docs/IMPLEMENTATION_NOTES.md](docs/IMPLEMENTATION_NOTES.md) |
| Look up a wire message format, and whether it's actually wired up | [docs/PROTOCOL_REFERENCE.md](docs/PROTOCOL_REFERENCE.md) |
| Read the animation-editor tool's design | [docs/ANI_MAKER_SPEC.md](docs/ANI_MAKER_SPEC.md) |
| Read the level maker / world file format / autotile design (steps 1-5 built: `shared/world`, art templates, game-side loading, `cmd/levelmaker` v1) | [docs/LEVEL_MAKER_SPEC.md](docs/LEVEL_MAKER_SPEC.md) |
| Design or build UI/HUD/menus/dialogue — principles, targets, system plan, prioritised task list | [docs/UI_UX.md](docs/UI_UX.md) |
| Edit client-side prediction/rendering/input | [client/](client/) |
| Edit server authority, movement validation, anti-cheat | [server/](server/) |
| Edit wire types and world model shared by both | [shared/](shared/) |
| Edit level files, autotile, cell properties (`shared/world`), or the sample `world/` + `levels/` data | [map/objects/world.md](map/objects/world.md) |
| Edit the level maker tool | [cmd/levelmaker/](cmd/levelmaker/) — card: [map/objects/levelmaker.md](map/objects/levelmaker.md) |
| Edit the sprite/animation editor tool | [animaker/](animaker/) |
| Understand what a code change will hit before touching it | [map/CLAUDE.md](map/CLAUDE.md) |
| Build/run everything locally (game + tools) | [build_local.ps1](build_local.ps1) — builds server, client, levelmaker, blobtemplate, animaker; runs server + 2 clients + levelmaker + animaker by default (`-NoRun` to skip, `-Root` for another world) |
| Run tests before pushing | `go test ./...`, and `go test ./...` inside `animaker/` when touching it — see [CONTEXT.md](CONTEXT.md#testing) |
| Check/change what CI runs | [.github/workflows/test.yml](.github/workflows/test.yml) |

## Rule of thumb

- `docs/` is the design bible — Go-framed and checked against the code as of 2026-09-17 (see `docs/CONTEXT.md`), but a design doc can still drift from code over time, so verify against `map/` or the source when it matters.
- `map/` is a live index of the actual code — trust it over `docs/` when they disagree; the code wins over both.
- `client/`, `server/`, `shared/` are one Go module (`module game`); `animaker/` is a separate module. Don't cross-import them.
