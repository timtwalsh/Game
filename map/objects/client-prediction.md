# Client prediction, interpolation & rendering

The client's local movement simulation (so input feels instant) and its
smoothing of every other player's position (since they only arrive at
10Hz over the network).

## Why this shape

Client-side prediction: the local player's position is computed
immediately from input, never waits for a server round-trip. Remote
players only get a position update 10x/second, so their movement is
interpolated between the last two known points rather than snapped,
to avoid visible stutter. This is the standard split for the
"trust the client, validate server-side" model described in
`docs/ARCHITECTURE.md`.

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
  `InterpolationDuration` (100ms, hardcoded). `ServerUpdate` resets the
  interpolation window whenever a new `PlayerState` arrives.
  `client/prediction.go:126-171`.
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
- **Hits:** perceived movement feel and the 10Hz send cadence in
  `client/main.go:140` if you change `NetworkTickRate` expectations —
  keep it matched to `shared.NetworkTickRate`'s intent even though the
  send loop currently hardcodes `100` rather than importing the constant.
- **Does not hit:** server-side validation logic — the server does not
  run this prediction code; it only sees the resulting `ClientMoveMsg`.

## Surfaces

Runs inside `client/main.go`'s raylib render loop (144fps target). No
other consumer.

## See

`client/prediction.go`, `client/main.go`, `client/renderer.go`
