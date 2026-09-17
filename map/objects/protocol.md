# Wire protocol & world model

Plain Go structs, JSON-encoded, that define every message the client and
server exchange and the level/collision data both sides interpret.

## Why this shape

Both binaries import the same `shared` package so there is exactly one
definition of each message and world type — no hand-kept-in-sync schema
on each side. Everything is JSON over UDP wrapped in a single envelope
(`MessageWrapper{Type, Payload}`) so new message types don't require
protocol version bumps, just a new `Type` string and a `case`/`if` on
each side.

## Shape

- `MessageWrapper{Type string, Payload []byte}` — every packet on the
  wire is one of these; `Payload` is the JSON of the specific message.
  `shared/protocol.go:4-7`
- Client→server: `ClientMoveMsg`, `ClientAttackMsg`, `ClientInteractMsg`,
  `ClientLoadLevelMsg`, `ClientChatMsg`, `ClientReportPlayerMsg`.
  `shared/protocol.go:11-41`. Only `"Move"` is actually handled by the
  server today (see `server-anticheat.md`) — the rest are defined but not
  wired up.
- Server→client: `ServerLevelLoadedMsg`, `PlayerState`,
  `ServerPlayerStateMsg`, `ServerPlayerStatesMsg`, `ServerAttackResultMsg`,
  `ServerChatMsg`, `ServerMovementRejectedMsg`, `ServerBannedMsg`.
  `shared/protocol.go:45-84`. Only `"PlayerStates"` is actually sent today.
- World model: `Vec2`, `TileType` (+ `IsPassable`), `CollisionLayer`
  (byte grid + `Get`/`Set`/`IsBlocked`), `VisualLayer`, `GameObject`,
  `Level`. `shared/types.go:6-104`.
- Tunables both sides must agree on: `TileSize`, `MaxSpeed`,
  `SpeedTolerance`, `NetworkTickRate`, and the `Suspicion*` weights/
  thresholds. `shared/types.go:106-119`. These are compiled into both
  binaries — there's no runtime negotiation.

## Connected to

- Consumed by `client-prediction.md` (encodes `ClientMoveMsg`, decodes
  `ServerPlayerStatesMsg`) and `server-anticheat.md` (decodes
  `ClientMoveMsg`, encodes `ServerPlayerStatesMsg`).
- `docs/PROTOCOL_REFERENCE.md` now mirrors these struct definitions
  directly (rewritten 2026-09-17) and additionally marks, per message,
  whether it's actually sent/handled in code or only defined — read it
  for that per-message wire status; read this card or the source for the
  shape itself.

## If you change this

- **Hits:** both `client/` and `server/` (they both `import "game/shared"`
  and decode these exact structs) — a field rename or type change breaks
  the other side silently at runtime (JSON just drops unknown fields,
  it won't fail to compile the other binary since they're separate
  `package main`s).
- **Hits:** `docs/PROTOCOL_REFERENCE.md` and `docs/ARCHITECTURE.md`
  should be updated to match — they currently already lag (see
  `docs/CONTEXT.md`), don't let that drift grow.
- **Does not hit:** `animaker/` — it does not import `shared` and has no
  network code.

## Surfaces

Read/written by `client/main.go`, `client/prediction.go`,
`server/main.go`, `server/validation.go`. No human-facing UI reads this
directly.

## See

`shared/protocol.go`, `shared/types.go`
