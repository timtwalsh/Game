# Network Protocol Reference

Quick reference for client-server communication patterns and workflows.
Struct shapes below are the real Go types from `shared/protocol.go` — where
a message isn't actually wired into `client/` or `server/` yet, that's
called out explicitly rather than implied.

---

## Message Types

### Client → Server

#### Movement — **live**
```go
type ClientMoveMsg struct {
    PlayerID  uint64 `json:"player_id"`
    Position  Vec2   `json:"position"`
    TimeMs    uint32 `json:"time_ms"`
    Direction uint8  `json:"direction"`
    ColorR    uint8  `json:"color_r"`
    ColorG    uint8  `json:"color_g"`
    ColorB    uint8  `json:"color_b"`
}
```
- **Sent:** Every 100ms (10 updates/sec) — `client/main.go:140-142`
- **Validation:** Speed check, wall-phase check — `server/validation.go`
- **Response:** Usually none (broadcast to other clients on the next tick)
- **On violation:** Server logs suspicion, no feedback to client
- Color fields exist because the MVP renders each player as a colored
  circle (`client/main.go:150-167`) — there's no sprite/animation system yet.
#### Attack — **defined, not wired up**
```go
type ClientAttackMsg struct {
    Direction uint8 `json:"direction"` // 0=up, 1=right, 2=down, 3=left
    Hit       bool  `json:"hit"`       // Did local hitbox test succeed?
}
```
- Nothing in `client/` sends this and the server's `handlePacket` only
  branches on `wrapper.Type == "Move"` (`server/main.go:65`) — an `"Attack"`
  packet would currently be silently ignored.
- Design intent (not yet built): cooldown check server-side, broadcast
  `ServerAttackResultMsg` to affected players, hitbox from a shared attack
  definition. See `docs/ARCHITECTURE.md#attack-prediction`.
#### Interact — **defined, not wired up**
```go
type ClientInteractMsg struct {
    TargetID uint64 `json:"target_id"` // NPC, door, chest ID
}
```
- Same status as Attack: type exists, no send or handle path in code.
#### Load Level — **defined, not wired up**
```go
type ClientLoadLevelMsg struct {
    LevelName string `json:"level_name"`
}
```
- Design intent: server responds with `ServerLevelLoadedMsg{Level}`. In
  code today there is no level to load — both client and server construct
  a hardcoded all-walkable `shared.CollisionLayer` in-process
  (`client/main.go:48`, `server/main.go:27`) instead of loading one.
#### Chat — **defined, not wired up**
```go
type ClientChatMsg struct {
    Message string `json:"message"`
}
```
- Design intent: length limit, rate limit (1 msg/2s), broadcast to nearby
  players. None of that exists in code — no rate limiting of any kind is
  implemented anywhere in this repo yet.
#### Report Player — **defined, not wired up**
```go
type ClientReportPlayerMsg struct {
    ReportedID uint64 `json:"reported_id"`
    Reason     string `json:"reason"` // "Speed hacking", "Griefing", etc.
}
```
- `SuspicionTracker.ReportCount` exists as a field (`server/validation.go:126`)
  but nothing increments it — there is no handler for this message type, so
  reports are not currently possible even though the field to count them
  is already there.
---

### Server → Client

#### Level Loaded — **defined, not sent**
```go
type ServerLevelLoadedMsg struct {
    Level Level `json:"level"`
}
```
`Level` (`shared/types.go:96-104`):
```go
type Level struct {
    Name         string         `json:"name"`
    Width        uint32         `json:"width"`
    Height       uint32         `json:"height"`
    Tileset      string         `json:"tileset"`
    Collision    CollisionLayer `json:"collision"`     // Layer 0
    VisualLayers []VisualLayer  `json:"visual_layers"` // Layers 1-2, 4, 6
    Objects      []GameObject   `json:"objects"`       // Layers 3, 5
}
```
- Never constructed or sent by the server today — there is no level-loading
  code path (see Load Level above).
#### Player State — **the shape is live, but only inside a batch**
```go
type PlayerState struct {
    PlayerID  uint64 `json:"player_id"`
    Position  Vec2   `json:"position"`
    Animation uint8  `json:"animation"`
    Direction uint8  `json:"direction"`
    ColorR    uint8  `json:"color_r"`
    ColorG    uint8  `json:"color_g"`
    ColorB    uint8  `json:"color_b"`
}
type ServerPlayerStateMsg struct {
    PlayerState
}
```
- `ServerPlayerStateMsg` (singular) is defined but never sent standalone —
  the server always sends the batched `ServerPlayerStatesMsg` below, even
  when there's only one player.
- `Animation` is populated in the struct but always `0` in practice — there
  is no animation system driving it yet (see `docs/ARCHITECTURE.md#attack-prediction`
  and the client renderer, which just draws a circle + direction line).
#### Multiple Player States (Batched) — **live**
```go
type ServerPlayerStatesMsg struct {
    States []PlayerState `json:"states"`
}
```
- **Sent:** Every 100ms, unconditionally, to every known client address —
  `server/main.go:117-137`. This is the *only* server→client message
  actually sent today.
- **Client does:** demuxes each `PlayerState` in `receiveLoop`
  (`client/main.go:56-93`) — if it's the local player's own ID, calls
  `ServerCorrection`; otherwise creates/updates a `PlayerInterpolation`.
- No area-of-interest filtering — "all players in area" currently means
  *all connected players*, full stop.
#### Attack Result — **defined, not sent**
```go
type ServerAttackResultMsg struct {
    AttackerID uint64 `json:"attacker_id"`
    TargetID   uint64 `json:"target_id"` // 0 if none (Go has no Option<T>; this is a sentinel)
    Damage     uint16 `json:"damage"`    // 0 if blocked/dodged
}
```
- No attack system exists server-side, so this is never constructed. See
  Attack above.
#### Chat — **defined, not sent**
```go
type ServerChatMsg struct {
    PlayerID uint64 `json:"player_id"`
    Message  string `json:"message"`
}
```
#### Movement Rejected — **defined, not sent**
```go
type ServerMovementRejectedMsg struct {
    Reason string `json:"reason"`
}
```
- The server never rejects a movement with feedback today. When a player's
  suspicion status reaches `SuspicionStatusAutoBan`, the server just stops
  applying that player's position updates (`server/main.go:106-112`) — the
  client is never told why, and keeps predicting locally with no
  correction arriving.
#### Banned — **defined, not sent**
```go
type ServerBannedMsg struct {
    Reason string `json:"reason"`
}
```
- No ban list, no disconnect logic, no login gate. See
  `docs/ARCHITECTURE.md#ban-list`.
---

## Workflow: Player Login → Movement → Other Player Sees

The steps below marked (live) match current code; the rest is the intended
future flow.

```
T+0ms: Client starts
  Client (live):
    ├─ Resolve server UDP address, bind a local UDP socket
    ├─ Generate a random PlayerID (time.Now().UnixNano()) and a random color
    └─ No login, no credentials, no session token — client/main.go:29-54

T+~0ms: Player moves (presses WASD/arrows)
  Client (live):
    ├─ Update PredictedPosition locally every frame (client/prediction.go)
    ├─ Render at new position immediately
    └─ Every 100ms, send a ClientMoveMsg to the server

T+~100ms: Server receives move (live)
  Server:
    ├─ Look up or create the player, trusting the client-sent PlayerID
    ├─ Check speed: MovementValidator.CheckSpeed
    ├─ Check wall-phase: MovementValidator.CheckWallPhase
    ├─ Add a SuspicionEvent to the tracker if either check flags it
    └─ Apply the new position, unless suspicion status is AutoBan

T+~100-200ms: Every player receives the next tick (live)
  Every Client:
    ├─ Receive ServerPlayerStatesMsg (all players, every 100ms)
    ├─ Own ID → ServerCorrection (snap Position to server's value)
    └─ Other IDs → PlayerInterpolation.ServerUpdate, then ease over 100ms
```

### Result
- **Player 1 experiences:** Movement is instant (local prediction)
- **Player 2 experiences:** Movement arrives on the next 100ms tick, then eases in over the following 100ms
- **Server validates:** Speed + wall-phase checks run on every move; violations raise suspicion but don't block movement until auto-ban
- **Anti-cheat:** As above — logging/review/ban-list steps beyond "stop applying position updates" are not implemented (see `docs/ARCHITECTURE.md`)

There is no login, no level load, and no "landing in world" step in code
today — a client that starts up is immediately in the same shared,
levelless space as everyone else.

---

## Workflow: Player Attacks (design only — not implemented)

Nothing described below exists in code yet: there is no `AttackDefinition`,
no server-side hit resolution, and `ClientAttackMsg`/`ServerAttackResultMsg`
are never sent or handled (see Attack above). This is kept as the intended
design for whoever picks up Phase 2 combat.

```
T+0ms: Player clicks attack (up direction)
  Client:
    ├─ Play attack animation locally
    ├─ Test local hitbox collision with nearby sprites
    ├─ Show hit effect if collision detected
    ├─ Set hit=true in message
    └─ Send Attack message to server

T+50ms: Server receives attack
  Server:
    ├─ Lookup target player position (from their last update)
    ├─ Lookup attack definition ("sword_slash_up")
    ├─ Test if attack hitbox overlaps target
    ├─ If yes: Apply damage, broadcast damage
    ├─ If no: Broadcast miss
    └─ Add to attack log (for stats/replays)

T+150ms: All clients receive AttackResult
  All Clients:
    ├─ Update damage numbers
    ├─ Update health bars
    ├─ Play hit/miss animations
    └─ Audio cues

T+200ms: Next attack can fire (cooldown ~200ms)
```

### Key Points
- **Client prediction:** Player sees hit instantly
- **Server authority:** Server decides if actually hit based on positions
- **Hitbox data:** Same attack definition on client & server
- **Timeline:** Large latency window due to movement updates
---

## Workflow: Anti-Cheat Detection

The detection math below (live) matches `server/validation.go`. The
consequences past "raise suspicion" (logging, human review queue, ban
list, disconnect) are design intent, not current behavior — see the
callouts inline.

### Scenario: Speed Hack

```
T+0ms: Cheater tries to teleport 50 tiles away
  Cheater Client (modified):
    ├─ Sends: ClientMoveMsg{ Position: (+50,0), TimeMs: 100 }
    └─ (Normal player: +5 tiles in 100ms)

T+~100ms: Server receives move (live: CheckSpeed)
  Server:
    ├─ Calculate speed: 50 tiles / 0.1s = 500 tiles/sec
    ├─ MaxSpeed = 5, SpeedTolerance = 2x → threshold = 10 tiles/sec
    ├─ 500 > 10 → flagged TooFast
    ├─ tracker.AddEvent → Score += SuspicionSpeedHack (0.5)
    │
    └─ Position is still applied (Score is below AutoBan) — the cheater's
       jump is visible to other players via the next broadcast tick.

T+~1000ms+: Cheater keeps moving fast, Score keeps climbing
  Server (live):
    └─ GetStatus() crosses SuspicionEnableLogging (5.0), then
       SuspicionFlagReview (8.0) thresholds — but nothing reads these
       statuses except the AutoBan check, so nothing visibly happens yet.

T+~2000ms: Score crosses SuspicionAutoBan (10.0)
  Server (live):
    └─ handlePacket stops applying this player's position updates
       (server/main.go:106-112). No ServerBannedMsg is sent, no
       disconnect happens — the player just stops moving from every other
       client's point of view.
```
> **Design intent, not implemented:** player-report counting, a 24h
> report window, a human review queue, a persistent ban list, and a login
> check against that list. See `docs/ARCHITECTURE.md#ban-list`.

### Scenario: Wall-Phasing

```
T+0ms: Cheater edits local collision, removes a wall
  Cheater Client:
    ├─ Remove wall from local collision layer (modified client)
    ├─ Walk through where wall was
    └─ Send: ClientMoveMsg{ Position: (+1,0), TimeMs: 100 }

T+~100ms: Server receives move (live: CheckWallPhase)
  Server:
    ├─ Check speed: 1 tile / 0.1s = 10 tiles/sec — at the threshold, not over it
    ├─ Check wall-phase: walk the tile line from `from` to `to`
    ├─ If a blocked tile lies on the path, estimate whether a detour
    │  was speed-feasible
    ├─ If not feasible: flagged WallPhase, Score += SuspicionWallPhase (2.0)
    └─ Position is still applied (broadcast as normal)
```
> **Caveat that applies to both scenarios above:** both client and server
> currently run against the same hardcoded all-walkable placeholder
> collision grid (`shared.NewCollisionLayer(100, 100)`), not a real loaded
> level. Wall-phase detection cannot actually trigger in the running game
> today because there are no walls to phase through yet — this scenario
> becomes real once level loading is implemented.

---

## Data Size Examples

### Move Message
```json
{
  "type": "Move",
  "player_id": 12345,
  "position": {"x": 100.5, "y": 200.3},
  "time_ms": 100,
  "direction": 4,
  "color_r": 128, "color_g": 64, "color_b": 200
}
```
- **JSON (actual shape above):** ~140 bytes — larger than the simplified
  3-field example in earlier drafts of this doc, because `ClientMoveMsg`
  also carries `player_id`, `direction`, and the three color bytes.
- **Binary equivalent:** ~20 bytes (uint64 + 2×float32 + 4×uint8) if this
  were ever packed instead of JSON-encoded — no binary encoding exists in
  code today, everything is JSON via `encoding/json`.
### PlayerState (inside a batch)
```json
{
  "player_id": 12345,
  "position": {"x": 100.5, "y": 200.3},
  "animation": 0,
  "direction": 4,
  "color_r": 128, "color_g": 64, "color_b": 200
}
```
- **JSON:** ~110 bytes per entry inside a `ServerPlayerStatesMsg.states` array
- **Binary equivalent:** ~20 bytes (uint64 + 2×float32 + 3×uint8)
### Level Message (not sent today — see Level Loaded above)
```json
{
  "type": "LevelLoaded",
  "level": {
    "name": "area_1",
    "width": 256,
    "height": 256,
    "collision": [0,0,1,1,0],
    "visual_layers": [],
    "objects": []
  }
}
```
- **Total (design estimate, real level):** ~150KB (one-time per level load)
- **Compressed (zstd):** ~20-30KB — no compression exists in code
---

## Network Stats (100 concurrent players) — design estimate, not measured

```
Inbound (to server):
├─ 100 players × 10 moves/sec = 1000 Move messages/sec
├─ ~140 bytes/message (see Move Message above) = ~140KB/sec
└─ Total: ~140KB/sec inbound (Attack/Chat/Report add nothing — unimplemented)

Outbound (from server):
├─ One ServerPlayerStatesMsg per connected client, every 100ms
├─ 100 players → each message batches ~100 PlayerState entries (~110 bytes each) ≈ 11KB
├─ Sent to 100 clients × 10/sec = ~11MB/sec outbound
└─ This is much higher than earlier drafts assumed, because there is no
   area-of-interest filtering — every client gets every player, every tick.
```
> The earlier per-message batching estimate in this doc understated
> outbound bandwidth by assuming interest-managed broadcasts. Until
> area-of-interest filtering exists, outbound traffic scales with
> `O(players²)`, not `O(players)` — this is worth fixing before testing
> with more than a handful of concurrent clients.

---

## Rate Limiting — design only, not implemented

```
Per-player limits:
├─ Movement: 10/sec (automatic, enforced by tick)
├─ Attack: 1/sec (configurable per weapon)
├─ Chat: 1/2sec (spam prevention)
├─ Interact: 1/sec
└─ Reports: 1/minute
```
> **Not implemented.** The server processes every packet it receives with
> no per-player rate limiting of any kind — a modified client sending
> `ClientMoveMsg`s at 1000/sec would be processed 1000 times/sec. The
> "automatic, enforced by tick" movement limit above describes the intent
> for a *well-behaved* client, not an actual server-side enforcement
> mechanism.

---

## Latency Tolerance

```
Network latency (typical): 50-200ms
Movement interpolation window: 100ms
┌─────────────────────────┐
│ Interpolate over window │
├─────────────────────────┤
0ms          50ms         100ms        150ms
Send         Receive      Display      Next update
             update       here         received
```

If latency > 200ms: Interpolation will catch up, might look jerky

If latency < 50ms: Interpolation finishes early, waits for next update

This matches the live `InterpolationDuration = 100` in
`client/prediction.go:144`.

---

## Common Edge Cases

### Client Behind on Movement — live behavior
```
Client predicts: Player at (100, 100)
Server says: Player should be at (90, 90)
├─ Server always wins — ServerCorrection sets Position directly, no
│  smoothing or thresholding on the magnitude of the correction
└─ Client snaps to the server value on its next PlayerStates receive
```
Unlike the design note in earlier drafts, there is no "difference too
large" check — every correction is applied immediately and unconditionally
(`client/prediction.go:122-124`).

### Zone Transitions — not implemented
> No level/zone concept exists yet (see Load Level above), so there is
> nothing to transition between.

### Player Disconnects — not implemented
> The server never removes a player from `Server.players`. A client that
> stops sending `ClientMoveMsg` simply stops updating its entry — its last
> known position keeps being broadcast to everyone else indefinitely.
> There is no heartbeat, no timeout, and no disconnect broadcast in code.

### Impossible Collision
```
Server receives: Move to position outside level bounds
├─ CheckSpeed / CheckWallPhase: operate on tile coordinates, don't
│  independently bounds-check against level width/height
├─ Server: still applies the position if not flagged
└─ Client: receives and renders it
```
Moot today in the sense that there is no real level with bounds to be
outside of — this remains a real gap once level loading exists.

---

## Testing Checklist

- [ ] Move message round-trip: <200ms total
- [ ] Multiple clients see each other move
- [ ] Attack hits registered correctly — blocked on Attack being implemented at all
- [ ] Chat broadcasts to nearby players — blocked on Chat being implemented at all
- [ ] Level change works (zone transition) — blocked on level loading existing
- [ ] Lag simulation: Add 100ms+ delay
  - [ ] Movement still feels responsive
  - [ ] Other players still interpolate smoothly
- [ ] Speed hack: Send 50 tiles/100ms
  - [ ] Server detects, logs (raises Score — there is no separate log yet)
  - [ ] No immediate disconnect
  - [ ] Player can continue (to collect data)
- [ ] Wall-phase: Move through wall at normal speed — blocked on a real level existing to have walls
- [ ] Ban: Manually ban player, try to login — blocked on auth/ban-list being implemented at all

None of these have automated tests behind them yet — there are no
`_test.go` files in this repo as of this writing.
---

End of protocol reference. See `ARCHITECTURE.md` for high-level overview.
