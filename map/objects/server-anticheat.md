# Server authority, movement validation & anti-cheat

The authoritative game loop: accepts client moves, scores them for
cheating, and broadcasts confirmed positions to everyone at network-tick
rate (`shared.NetworkTickRate`, currently 20Hz).

## Why this shape

Design principle from `docs/ARCHITECTURE.md`: trust the client for
responsiveness, but validate and log server-side rather than reject
outright — false positives are worse than letting a probable cheat
through once while it accrues suspicion. Hence `ValidateMovement`
returns a list of *issues* that feed a per-player running `Score`
instead of a hard allow/deny per packet.

## Shape

- `Server{clients map[addr]playerID, players map[id]*PlayerState,
  suspicions map[id]*SuspicionTracker, validator MovementValidator}`
  — all shared mutable state behind one `sync.Mutex`. `server/main.go:12-30`.
- `handlePacket` — only handles `wrapper.Type == "Move"`. Looks up or
  creates the player (**MVP shortcut: trusts the client-supplied random
  `PlayerID`, comment says so explicitly** — `server/main.go:73`), runs
  `ValidateMovement`, feeds any issues into the player's
  `SuspicionTracker`, and applies the new position **unless** the
  tracker's status is `SuspicionStatusAutoBan`. `server/main.go:57-115`.
- **Spawn (2026-10-05; from the world 2026-10-06):** a new player is
  created at `Server.spawn` — the world's first `spawn` object
  (`world.Map.Spawn`), or `shared.SpawnPoint` without level files — and
  their first `Move` is validated from there like any other. The client
  reads the same spawn from the same files, so a normal first move is
  clean. The spawn is deliberately *not* taken from
  the first `Move`: a client-chosen spawn would be an unvalidated teleport,
  available to anyone who sends a fresh `PlayerID` or goes quiet for
  `playerTimeout` and rejoins. (Before this, new players started at (0,0)
  and every connect was flagged TooFast at ~2800 px/s.) **Known gap:** a
  client dropped by `dropIdle` that resumes sending is re-created at spawn,
  but the client keeps its own position, so its next move is flagged � the
  server has no "you were respawned" message to snap it back.
- `MovementValidator{Collision world.Collider}` — the server's own map
  (D32): `main` takes `-root` (default `.`) and `world.LoadMap`s it at
  startup, recompiling every level's properties from the files; a broken
  world is fatal, no `world/`+`levels/` means the open 100x100
  `world.Grid`. The status board's Recent log says which.
- `MovementValidator.CheckSpeed` — straight-line distance/time.
  `ValidateMovement` flags it above `MaxSpeedBetween(from, to) *
  SpeedTolerance`, where `MaxSpeedBetween` is `MaxSpeed*TileSize` times
  the larger interaction speed multiplier of the two ends' tiles (so
  stepping out of water isn't flagged). With `SpeedTolerance` 2.0 and
  swimming at 0.5, walking speed in water is exactly the limit — only
  more than twice swim speed is caught.
- `MovementValidator.CheckWallPhase` — Bresenham line from `from` to
  `to` in signed, floored world tiles (`world.TileOf`); a tile blocking
  the movement type's flag (`BlockGround` for walking, `BlockJump` for
  `MovementTypeJump`) on the path, including void outside every level,
  is `!Clean`, and a detour estimate against `MaxSpeedBetween` decides
  `Cheated`. `ValidateMovement` flags both. Teleport/knockback movement
  types skip this check. The server only ever passes
  `MovementTypeWalk` today.
- `SuspicionTracker{Score, Events, ReportCount}` — `AddEvent` adds a
  fixed weight per `SuspicionEventType` (`shared.Suspicion*` constants);
  `GetStatus` buckets the running score into
  Normal/FullLogging/FlagForReview/AutoBan. `server/validation.go:122-160`.
  **AutoBan only blocks position updates going forward — there is no
  disconnect/kick, and no persistence: `Score` resets to 0 on server
  restart.**
- `tickLoop` broadcasts every player's `PlayerState` to every known
  client address every `shared.NetworkTickRate` ms (currently 50ms),
  unconditionally (no interest management / area-of-interest filtering
  yet). `server/main.go:117-137`.
- `dropIdle` (2026-10-05), run at the top of each tick, forgets a player
  with no `Move` for `playerTimeout` (5s): their state, suspicion tracker
  and client address. That's the only way a player leaves. **It also
  throws away their suspicion score**, so going quiet for 5s resets it -
  no worse than today's trust-the-client player ID, but it matters once
  IDs are real accounts. `server/main_test.go`.

## Connected to

- Consumes `shared.ClientMoveMsg`, `shared.PlayerState`,
  `shared.ServerPlayerStatesMsg`, `shared.SuspicionEvent`,
  `shared.MaxSpeed`/`SpeedTolerance`/`Suspicion*` constants — see
  `protocol.md`.
- Consumes `shared/world` (`LoadMap`, `Collider`, `TileOf`, the
  `Block*` flags) — see `world.md`. The client loads the same files; if
  the two ever run from different `-root`s, honest players get flagged.
- `shared.SuspicionEvent.TileX/TileY` are signed `int` world tiles
  (server-internal; never sent).

## If you change this

- **Hits:** anti-cheat tuning constants live in `shared/types.go`, so
  changing them affects this file's thresholds directly — re-check
  `docs/ARCHITECTURE.md`'s anti-cheat section for intended values, they
  are not auto-synced (see root `CONTEXT.md`).
- **Hits:** `client-prediction.md`'s `ServerCorrection` path — any
  change to what position the server accepts/echoes back changes what
  the client snaps to.
- **Hits:** `client-prediction.md` — prediction must stay at least as
  strict as these checks (collision box containing the checked point,
  speed within `MaxSpeedBetween`), or honest players get flagged.
- **Does not hit:** `animaker/` — no dependency in either direction.

## Surfaces

Runs as `server/main.go`'s `main()` → `ListenAndServe`. Its terminal is a
live status board (`server/statusboard.go`), redrawn in place rather than
scrolled: one block per player, in connect order — `Player N
connected|disconnected` then `> latest action` (walking/turning, stopped,
jumped, anti-cheat flags, auto-ban, timed out). Actions are derived from
each `ClientMoveMsg` by `describeMove`; position changes alone aren't
actions. Disconnected players stay listed for `boardForgetAfter` (1 min).
Server-level messages (UDP errors) go to its `Event` "Recent" tail, not
`fmt.Print` — anything printed directly would be overwritten by the next
redraw. `ansi_windows.go` turns on VT processing for plain console
windows; if stdout isn't a terminal, frames are printed in full instead.

## See

`server/main.go`, `server/validation.go`, `server/statusboard.go`

Tests: `server/validation_test.go` — covers `CheckSpeed`, `CheckWallPhase`
(clean/blocked/feasible-detour/infeasible-detour/teleport-skip),
`ValidateMovement`, `SuspicionTracker` status thresholds, and against a
real `World` built from `world/terrains.toml`: wall phase across a level
boundary, negative coordinates and void; jumping a fence (ground+roll)
allowed while walking through it is caught; swimming at swim speed clean,
1.5x walking speed flagged in water but not on grass, and stepping out of
water clean.
`server/statusboard_test.go` covers the board's latest-action-per-player
rendering, forgetting disconnected players, and `describeMove`. `server/main.go`
isn't tested over a real socket, but `server/main_test.go` drives
`handlePacket` directly: idle-drop/rejoin, and that a first move near
`SpawnPoint` is clean while one far from it is flagged TooFast, players
spawn at the world's spawn, and a walk from the repo's sample spawn is
clean.
