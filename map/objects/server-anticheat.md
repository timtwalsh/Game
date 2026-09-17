# Server authority, movement validation & anti-cheat

The authoritative game loop: accepts client moves, scores them for
cheating, and broadcasts confirmed positions to everyone at 10Hz.

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
  client address every 100ms, unconditionally (no interest management /
  area-of-interest filtering yet). `server/main.go:117-137`.

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

Runs headless as `server/main.go`'s `main()` → `ListenAndServe`. No UI.

## See

`server/main.go`, `server/validation.go`

Tests: `server/validation_test.go` — covers `CheckSpeed`, `CheckWallPhase`
(clean/blocked/feasible-detour/infeasible-detour/teleport-skip),
`ValidateMovement`, and `SuspicionTracker` status thresholds. `server/main.go`
(the UDP loop itself) is untested — it's network glue, not scored logic.
