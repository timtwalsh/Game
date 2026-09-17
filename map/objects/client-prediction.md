# Client prediction, interpolation & rendering

The client's local movement simulation (so input feels instant) and its
smoothing of every other player's position (since they only arrive at
network-tick rate, currently 20Hz).

## Why this shape

Client-side prediction: the local player's position is computed
immediately from input, never waits for a server round-trip. Remote
players only get a position update once per tick, so their movement is
interpolated between the last two known points rather than snapped,
to avoid visible stutter. This is the standard split for the
"trust the client, validate server-side" model described in
`docs/ARCHITECTURE.md`.

Until 2026-09-17, the client's send interval, the server's broadcast
interval, and this interpolation window were three independently
hardcoded `100`s that had drifted apart from `shared.NetworkTickRate`
(which nothing actually read). Stacked together they added ~150-300ms of
perceived latency to a remote player's rendered position — fully
reproducible on localhost, nothing to do with real network RTT. All
three now derive from `shared.NetworkTickRate` (50ms/20Hz), which halved
that stacked delay. If you retune it, the "Hits" list below is where to
look.

That same day, a second and more visible bug was found and fixed: fixing
the tick rate alone didn't fix a "freeze then rush to catch up" jitter,
because `InterpolationDuration` was a *fixed* constant regardless of how
much real time actually passed between two `ServerUpdate` calls. The
client's send loop and the server's broadcast ticker are two independent,
unsynchronized timers, so the real gap between updates for a given
remote player naturally drifts around the nominal tick rate. When a gap
ran long, the old code froze at the target (its internal clock literally
stopped once "arrived" — see `Update`'s old guard) and then had to cover
the resulting larger distance in the same fixed short window once the
next update landed, i.e. a visible stall-then-snap. `ServerUpdate` now
measures the real elapsed time since the previous update (clamped to
`[interpolationDurationMin, interpolationDurationMax]`, both derived from
`shared.NetworkTickRate`) and uses that as the next segment's duration,
so `Update` always eases at the pace updates are actually arriving.

## Shape

- `PlayerInput{Up, Down, Left, Right, Attack, Interact bool}` +
  `GetMovementVector()` (normalized diagonal) and `GetDirection()`
  (0-7 octant, 255 = none). `client/prediction.go:8-59`.
- `PlayerController` — the *local* player only: `Position` (last
  server-confirmed), `PredictedPosition` (what's drawn), `Velocity`,
  `Direction`, and its own `Collision` copy. `UpdatePrediction` moves
  `PredictedPosition` each frame with wall-sliding via `CanMoveTo`;
  `ServerCorrection` snaps `Position` when a server update arrives.
  `client/prediction.go:61-124`.
- `PlayerInterpolation` — one per *remote* player: `CurrentPosition`
  eases from `LastPosition` toward `TargetPosition` over
  `InterpolationDuration`, which `ServerUpdate` recomputes on every call
  from the real elapsed time since the previous call (clamped between
  `interpolationDurationMin`/`Max`) — not a fixed constant.
  `client/prediction.go:126-199`.
- `Client` (in `client/main.go:16-27`) owns one `PlayerController` for
  self and a `map[uint64]*PlayerInterpolation` for everyone else;
  `receiveLoop` demuxes incoming `ServerPlayerStatesMsg` into corrections
  vs. remote interpolation updates. `client/main.go:56-93`.
- `RenderableObject` / `SortObjectsForRendering` in `client/renderer.go` —
  Z-based draw-order and shadow-length calc for world objects
  (`shared.GameObject`), independent of players.

## Connected to

- Consumes `shared.Vec2`, `shared.CollisionLayer`, `shared.PlayerState`,
  `shared.ClientMoveMsg`, `shared.ServerPlayerStatesMsg` — see
  `protocol.md`.
- `PlayerController`'s own `Collision` is a fresh
  `shared.NewCollisionLayer(100, 100)` (`client/main.go:48`) — an
  all-walkable placeholder, **not** loaded from any real level data.
  Wall-sliding today can't actually hit a wall.

## If you change this

- **Hits:** nothing outside `client/` — `PlayerController` and
  `PlayerInterpolation` are not imported by `server/` or `animaker/`.
- **Hits:** perceived movement feel everywhere — `client/main.go:141`
  (send interval), `server/main.go`'s `tickLoop` (broadcast interval),
  and this file's `InterpolationDuration` all read `shared.NetworkTickRate`
  directly now, so changing the one constant retunes all three together.
  Also note `client/main.go:142` now sends the *actual* elapsed ms since
  last send (not a hardcoded `100`) as `ClientMoveMsg.TimeMs` — this
  feeds directly into `MovementValidator.CheckSpeed` on the server, so a
  bug here would skew anti-cheat speed math, not just visuals.
- **Does not hit:** server-side validation logic — the server does not
  run this prediction code; it only sees the resulting `ClientMoveMsg`.

## Surfaces

Runs inside `client/main.go`'s raylib render loop (144fps target). No
other consumer.

## See

`client/prediction.go`, `client/main.go`, `client/renderer.go`

Tests: `client/prediction_test.go` — covers `PlayerInput` direction/vector
math, `PlayerController` movement + wall-stopping + server correction,
`PlayerInterpolation` easing, and (as regression coverage for the jitter
fix above) that a late update adapts `InterpolationDuration` upward
instead of re-snapping to a fixed window, and that a near-zero gap
clamps to `interpolationDurationMin`. `client/main.go` and
`client/renderer.go` are untested (network/render glue and depth-sort
only, respectively).
