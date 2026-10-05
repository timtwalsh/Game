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
- **Spawn (2026-10-05):** a new player is created at `shared.SpawnPoint`
  (`shared/types.go`), and their first `Move` is validated from there like
  any other. The client starts its prediction at the same constant, so a
  normal first move is clean. The spawn is deliberately *not* taken from
  the first `Move`: a client-chosen spawn would be an unvalidated teleport,
  available to anyone who sends a fresh `PlayerID` or goes quiet for
  `playerTimeout` and rejoins. (Before this, new players started at (0,0)
  and every connect was flagged TooFast at ~2800 px/s.) **Known gap:** a
  client dropped by `dropIdle` that resumes sending is re-created at spawn,
  but the client keeps its own position, so its next move is flagged � the
  server has no "you were respawned" message to snap it back.
- `MovementValidator.CheckSpeed` — straight-line distance/time vs.
  `MaxSpeed*TileSize*SpeedTolerance`. `server/validation.go:41-48`.
- `MovementValidator.CheckWallPhase` — Bresenham line from `from` to
  `to` in tile space; if a blocked tile lies on the path, estimates
  whether a detour was speed-feasible to decide `Cheated` vs. merely
  `!Clean`. `server/validation.go:50-95`. Teleport/knockback movement
  types skip this check.
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
- The `validator`'s `Collision` is `shared.NewCollisionLayer(100, 100)`
  (`server/main.go:27`) — same all-walkable placeholder as the client's,
  so `CheckWallPhase` cannot currently trigger against real level
  geometry either.

## If you change this

- **Hits:** anti-cheat tuning constants live in `shared/types.go`, so
  changing them affects this file's thresholds directly — re-check
  `docs/ARCHITECTURE.md`'s anti-cheat section for intended values, they
  are not auto-synced (see root `CONTEXT.md`).
- **Hits:** `client-prediction.md`'s `ServerCorrection` path — any
  change to what position the server accepts/echoes back changes what
  the client snaps to.
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
`ValidateMovement`, and `SuspicionTracker` status thresholds.
`server/statusboard_test.go` covers the board's latest-action-per-player
rendering, forgetting disconnected players, and `describeMove`. `server/main.go`
isn't tested over a real socket, but `server/main_test.go` drives
`handlePacket` directly: idle-drop/rejoin, and that a first move near
`SpawnPoint` is clean while one far from it is flagged TooFast.
