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
- **Character animation (2026-10-04/05).** `client/anim` is the runtime
  for animaker assets, raylib-free. `Library` loads `.anichar` → `.anif`
  (and the `.anif`s they nest) → `.sprsh` once and shares tracks/sheets by
  path; a missing sheet or nested file is a `Library.Problems` entry (the
  client prints them), not an error, as in the editor. `Instance` is
  per-player playback (`Play`/`Restart`/`SetFacing`/`SetProp`/`Seek`/
  `Advance`/`Finished`/`AppendSprites`/`AppendCrossedMarkers`). It
  matches the editor: X/Y/Z/rotation lerp, Row/Col/direction step; the
  game's 8-way facing maps to the track's directions (diagonals →
  sideways), and an unposed direction plays the first posed one. Nested
  parts play on the instance's own clock (looping at their own length),
  pick direction by inherit/static/per-keyframe, take props by
  passthrough/static bindings (or an `.anif`-valued prop swaps the nested
  track), stay in their part's Z slot, and are offset and turned by their
  part. All of that is resolved into a per-instance node tree when the
  animation or props change; direction lookup is two array reads off
  tables built at load, keyframes are binary-searched. A frame allocates
  nothing (`TestFrameDoesNotAllocate`, `TestNestedFrameDoesNotAllocate`;
  `BenchmarkFrame` ~80ns for the baby).
  `client/character.go` `CharacterAnimator` is the engine-owned state
  machine: the `animStates` table maps each `shared.Anim*` state to an
  animation (idle stands in as walk frame 0 when there's no "idle"; jump
  is a one-shot). Adding a state = constant + table row + its trigger in
  `nextLocalState`. Local players `Step`; remote ones `Apply` the relayed
  `Animation`/`AnimSeq`.
  `client/sprites.go` loads sheet textures and draws a frame. How a
  character sits in the world (`lookDef`: `.anichar` path, scale, origin
  offset from the collision box centre) is engine-side data, not in
  animaker's formats; every player uses `playerLook` (the baby).
- **Frame loop & leaving (2026-10-05).** `main` updates prediction,
  interpolation and animation under `Client.mutex`, snapshots what to draw,
  and unlocks before drawing, so `receiveLoop` isn't blocked by rendering.
  `remoteAnims` is main-goroutine-only for that reason. `applyPlayerStates`
  drops any remote player missing from a broadcast (the server's list is
  complete, and it times players out - see server-anticheat.md).
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

`client/prediction.go`, `client/main.go`, `client/renderer.go`,
`client/character.go`, `client/sprites.go`, `client/anim/`

Tests: `client/prediction_test.go` — covers `PlayerInput` direction/vector
math, `PlayerController` movement + wall-stopping + server correction,
`PlayerInterpolation` easing, and (as regression coverage for the jitter
fix above) that a late update adapts `InterpolationDuration` upward
instead of re-snapping to a fixed window, and that a near-zero gap
clamps to `interpolationDurationMin`. `client/anim/anim_test.go`
loads the real baby assets (modes, sheets, direction mapping, cell
stepping, loop wrap, jump interpolation, prop fallback);
`client/character_test.go` covers the state machine (one-shots run to
the end, back-to-back jumps bump `AnimSeq`, remote restarts on seq,
unknown states show idle).
`client/main.go`, `client/sprites.go` and
`client/renderer.go` are untested (network/render glue and depth-sort
only, respectively).
