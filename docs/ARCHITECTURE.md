# 2D Top-Down MMO - Architecture & Design Document

A Go-based ALTTP-style MMO running at 144fps with distributed client-side prediction and server-side validation.

---

## Table of Contents

1. [Overview](#overview)
2. [Technology Stack](#technology-stack)
3. [System Architecture](#system-architecture)
4. [Network Model](#network-model)
5. [Anti-Cheat System](#anti-cheat-system)
6. [Client Architecture](#client-architecture)
7. [Server Architecture](#server-architecture)
8. [Data Formats](#data-formats)
9. [Codebase Structure](#codebase-structure)
---

## Overview

This is a multiplayer RPG where:
- **Gameplay**: Top-down 2D action/adventure (ALTTP-style), targeting 144fps
- **Scale**: 100-1000s of concurrent players per server
- **Cheat Detection**: Automated + community reports + tournament oversight
- **Design Philosophy**: Trust the client, validate with post-analysis
Key principle: Client predicts everything locally for responsiveness. Server trusts the client but logs suspicious behavior for analysis.

---

## Technology Stack

### Language
- **Go** (fast compilation, built-in concurrency with goroutines, memory-safe)
### Client
- `raylib-go` - Graphics/rendering
- `net` (UDP) - Networking
- `encoding/json` - Serialization
### Server
- `net` (UDP) - Networking, goroutines for concurrent clients
- `encoding/json` - Serialization
### Shared
- `encoding/json` - Serialization format
- Game constants and type definitions
---

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     GAME WORLD (Server)                     │
│  ┌──────────────────┐  ┌──────────────────┐                │
│  │ Level State      │  │ Player Manager   │                │
│  │ ├─ Collision     │  │ ├─ Positions     │                │
│  │ ├─ Objects       │  │ ├─ Animations    │                │
│  │ └─ NPCs          │  │ └─ Inventory     │                │
│  └──────────────────┘  └──────────────────┘                │
│           ↓                      ↓                           │
│  ┌──────────────────────────────────────┐                  │
│  │ Movement Validator                   │                  │
│  │ ├─ Speed check                       │                  │
│  │ └─ Wall-phase detection              │                  │
│  └──────────────────────────────────────┘                  │
│           ↓                                                  │
│  ┌──────────────────────────────────────┐                  │
│  │ Anti-Cheat System                    │                  │
│  │ ├─ Suspicion tracking                │                  │
│  │ ├─ Event logging                     │                  │
│  │ └─ Ban management                    │                  │
│  └──────────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────────┘
              ↑                              ↑
              │                              │
        Updates (10/sec)              Position broadcasts
              │                              │
        ┌─────────────┐              ┌──────────────┐
        │ Client 1    │              │ Client 2     │
        │ Prediction  │←────────────→│ Interpolation│
        │ Rendering   │              │ Rendering    │
        └─────────────┘              └──────────────┘
```

> **Status:** "Level State" collision/objects/NPCs and "Player Manager" inventory in
> this diagram are the *design target*. Today's server (`server/main.go`) only holds
> `map[uint64]*shared.PlayerState` and a single hardcoded all-walkable
> `shared.CollisionLayer` — there is no level loading, no NPCs, and no inventory yet.
> "Event logging" and "Ban management" are likewise designed but not built; see
> [Anti-Cheat System](#anti-cheat-system) and [Server Architecture](#server-architecture) below.

---

## Network Model

### Tick Rate
- **Server → Client**: ~10 position updates per second (100ms interval)
- **Client → Server**: ~10 movement updates per second (100ms interval)
- **Bandwidth**: ~5-10 KB/s per player at full load
### Message Flow

```
Client Side:
  Input → Prediction → Network Send (every 100ms)
    ↓
    └─→ Render (every 7ms at 144fps)

Server Side:
  Receive Update → Validate (speed, wall-phase) → Broadcast to nearby
    ↓
    └─→ Log suspicious activity → Update suspicion score
```

> "Broadcast to nearby" is aspirational — the server currently broadcasts every
> player's state to every connected client unconditionally
> (`server/main.go:117-137`), with no area-of-interest filtering. "Log suspicious
> activity" means incrementing an in-memory `SuspicionTracker.Score`
> (`server/validation.go`); there is no persistent event log yet.

### Protocol

The wire format is JSON, one `shared.MessageWrapper{Type, Payload}` envelope per
packet. These are the real Go struct definitions from `shared/protocol.go` —
see [docs/PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md) for the full reference
and per-message wire status.

**Client → Server** (`shared/protocol.go:11-41`):
```go
type ClientMoveMsg struct {
    PlayerID  uint64
    Position  Vec2
    TimeMs    uint32
    Direction uint8
}
type ClientAttackMsg struct {
    Direction uint8
    Hit       bool
}
type ClientInteractMsg struct {
    TargetID uint64
}
type ClientLoadLevelMsg struct {
    LevelName string
}
type ClientChatMsg struct {
    Message string
}
type ClientReportPlayerMsg struct {
    ReportedID uint64
    Reason     string
}
```
> Only `ClientMoveMsg` (wire type `"Move"`) is actually decoded by the server
> today (`server/main.go:65`). The other five are defined but nothing sends or
> handles them yet — no attack, interact, level-load, chat, or report flow is
> wired up in code.

**Server → Client** (`shared/protocol.go:45-84`):
```go
type ServerLevelLoadedMsg struct {
    Level Level
}
type PlayerState struct {
    PlayerID  uint64
    Position  Vec2
    Animation uint8
    Direction uint8
}
type ServerPlayerStatesMsg struct {
    States []PlayerState
}
type ServerAttackResultMsg struct {
    AttackerID uint64
    TargetID   uint64 // 0 if none — Go has no Option<T>, this is a sentinel value
    Damage     uint16
}
type ServerChatMsg struct {
    PlayerID uint64
    Message  string
}
type ServerMovementRejectedMsg struct {
    Reason string
}
type ServerBannedMsg struct {
    Reason string
}
```
> Only `ServerPlayerStatesMsg` (wire type `"PlayerStates"`) is actually sent
> today, on every server tick (`server/main.go:126-134`) and handled by the
> client (`client/main.go:70-91`). The rest are defined but unused — the server
> never rejects a movement or sends a ban message, it silently stops applying
> position updates once a player crosses the auto-ban suspicion threshold
> (`server/main.go:106`).

---

## Anti-Cheat System

### 4-Layer Defense

#### Layer A: Automated Speed Detection
```
Every movement check:
├─ Calculate speed = distance / time
├─ If speed > MAX_SPEED * 2.0:
│  └─ Suspicion += 0.5
└─ Log the violation
```
- **Cost:** 1 float operation per movement
- **Catches:** Obvious speedhacks
- **FP Rate:** <0.1% (network jitter is tolerated by 2x multiplier)
- **Implemented as:** `MovementValidator.CheckSpeed` (`server/validation.go:41-48`)
#### Layer B: Automated Wall-Phase Detection
```
Every movement check:
├─ Test each tile along path
├─ If path crosses blocked tile:
│  ├─ Can player go around in available time?
│  ├─ If speed_for_detour > MAX_SPEED:
│  │  └─ Suspicion += 2.0 (likely cheating)
│  └─ Else: just log (might be legitimate edge case)
```
- **Cost:** O(distance) line-drawing + collision checks (~1-5ms)
- **Catches:** Wall-phasing, clipping through geometry
- **FP Rate:** <1% (edge cases at level boundaries)
- **Implemented as:** `MovementValidator.CheckWallPhase` (`server/validation.go:50-95`).
  Since both client and server currently run against a hardcoded all-walkable
  collision grid (no real level loaded), this check cannot actually trigger
  against real geometry yet — it only becomes meaningful once level loading
  exists.
#### Layer C: Escalation Thresholds
```
Suspicion Score Ranges:
├─ 0.0 - 5.0:   Normal (sampling enabled)
├─ 5.0 - 8.0:   Full logging enabled for player
├─ 8.0 - 10.0:  Flag for human review
└─ 10.0+:       Auto-ban + human confirmation
```
- **Implemented as:** `SuspicionTracker.GetStatus` (`server/validation.go:151-160`),
  thresholds are `shared.SuspicionEnableLogging/FlagReview/AutoBan`
  (`shared/types.go:106-119`). "Full logging" and "flag for human review" are
  status values only — there is no logging subsystem or review queue reading
  them yet; only "Auto-ban" has an effect (it stops the player's position
  updates from being applied, `server/main.go:106`).
#### Layer D: Community + Oversight
```
Player Reports:
├─ 5+ reports in 24h → Enable full logging
├─ Human moderator reviews → Manual decision

Tournaments:
├─ Game master spectates live
├─ 100% logging enabled
├─ Instant action on violations
```
> **Not implemented.** `ClientReportPlayerMsg` exists in the protocol
> (see above) but the server never handles a `"ReportPlayer"` wire type, so
> there is no report counter, no 24h window, and no tournament/spectator mode
> in code.

### Suspicion Events

Real shape, from `shared/protocol.go:87-101` and `shared/types.go:111-115`:
```go
type SuspicionEventType string

const (
    SuspicionEventTooFast      SuspicionEventType = "TooFast"
    SuspicionEventWallPhase    SuspicionEventType = "WallPhase"
    SuspicionEventPlayerReport SuspicionEventType = "PlayerReport"
)

type SuspicionEvent struct {
    Type   SuspicionEventType
    Speed  float32
    TileX  uint32
    TileY  uint32
    Reason string
}
```
Weights (`shared/types.go`): `TooFast +0.5`, `WallPhase +2.0`,
`PlayerReport +0.5`. There is no `CommunityFlagged` event type or weight in
code today, despite being listed in earlier drafts of this doc — only the
three above exist.

### Ban List

```
player_id → (reason, timestamp, evidence_link)
├─ Checked on login
├─ Prevents banned players from connecting
└─ Appealable (human review)
```
> **Not implemented.** There is no ban list, no login flow, and no
> persistence at all — `SuspicionTracker.Score` lives only in server memory
> and resets to zero on restart. "Auto-ban" today just means the server stops
> accepting that player's position updates for the remainder of the process
> lifetime.

---

## Client Architecture

### Core Components

#### Player Controller

Real shape, `client/prediction.go:61-124`:
```go
type PlayerController struct {
    Position          shared.Vec2 // last server-confirmed position
    PredictedPosition shared.Vec2 // local prediction, what's rendered
    LastPosition      shared.Vec2
    Velocity          shared.Vec2
    Direction         uint8
    Collision         shared.CollisionLayer
}

func (pc *PlayerController) UpdatePrediction(input *PlayerInput, deltaMs uint32)
func (pc *PlayerController) CanMoveTo(x, y float32) bool
func (pc *PlayerController) ServerCorrection(serverPos shared.Vec2)
```
There is no separate `GetDisplayPosition()` method — callers read
`PredictedPosition` directly (see `client/main.go:165-166`).

#### Movement Prediction
```
Input → Local collision test → Position update (every frame)
  ↓                              ↓
 5 inputs/sec            144 display updates/sec
```

#### Attack Prediction
> **Not implemented.** No `AttackDefinition`, `AttackFrame`, `AttackHitbox`, or
> `AttackPredictor` exists in `client/`. `ClientAttackMsg` is defined in the
> protocol but nothing in the client sends it, and there is no local hitbox
> test or attack animation code yet. This section describes a designed
> feature, not current behavior.

#### Interpolation (Other Players)

Real shape, `client/prediction.go:126-171`:
```go
type PlayerInterpolation struct {
    TargetPosition        shared.Vec2
    CurrentPosition       shared.Vec2
    LastPosition          shared.Vec2
    InterpolationTime     uint32
    InterpolationDuration uint32 // 100ms, hardcoded in NewPlayerInterpolation
    Direction             uint8
}

func (pi *PlayerInterpolation) ServerUpdate(state shared.PlayerState)
func (pi *PlayerInterpolation) Update(deltaMs uint32)
```
```
Last update ──(interpolate)──→ Next update
├─ Linear interpolation over 100ms
├─ Smooth movement between server updates
└─ Handles network latency transparently
```

### Rendering (Z-Range System)

Instead of fixed layers, use z-coordinates for depth ordering. Objects render based on their position in 3D space:

```
Z-Range System:
  0-10:    Underground visuals (caves, basements)
  11-50:   Ground-level objects + visuals (players, NPCs, items)
           └─ Sorted by (y - z/2) for depth ordering
  51-70:   Above-ground visuals (trees, roofs, structures)
  71-100:  High air objects (clouds, effects, UI)
  
Depth Sorting: Objects render in order of sort_key = (y - z/2)
  └─ Objects lower on screen (higher y) render in front
  └─ UNLESS they're tall (higher z) and behind
  └─ Automatically produces natural depth ordering with shadows
```

**Rendering Algorithm:**
1. Render visual layer z=0-10 (underground)
2. Collect all objects in z=11-50 (ground level)
3. Sort by (y - z/2)
4. Render shadows based on z-height
5. Render sorted objects
6. Render visual layer z=51-70 (above-ground)
7. Render objects z=71+ (air/UI)

> **Partially implemented.** `client/renderer.go` implements steps 2-3 today:
> `SortObjectsForRendering` filters `shared.GameObject`s to z=11-50, computes
> `ShadowLength` and a `SortKey = int32(obj.Y) - int32(obj.Z)/2`, and sorts by
> it. There is no visual-layer rendering (steps 1, 6, 7) or shadow *drawing*
> yet — `CalculateShadowLength` computes a length but nothing draws a shadow
> with it.

**Advantages:**
- Seamless depth sorting (no layer boundaries)
- Shadows work naturally (taller objects cast longer shadows)
- More intuitive (objects have real heights, like ALTTP)
- Flexible z-ranges for custom effects
- No sorting errors (y-ordering handled automatically)
---

## Server Architecture

### Core Components

#### World State

Real shape, `server/main.go:12-30`:
```go
type Server struct {
    conn       *net.UDPConn
    clients    map[string]uint64                  // UDP addr -> player ID
    players    map[uint64]*shared.PlayerState
    suspicions map[uint64]*SuspicionTracker
    mutex      sync.Mutex
    validator  MovementValidator
    nextID     uint64
}
```
There is no separate `World` type, no `levels map[string]Level`, no
`objects []GameObject`, and no per-level `MovementValidator` — there is
exactly one `MovementValidator` for the whole server, built against a
single hardcoded collision grid (`server/main.go:27`). Multi-level support
does not exist yet.

#### Movement Validator

Real shape, `server/validation.go:33-111`:
```go
type MovementValidator struct {
    Collision shared.CollisionLayer
}

func (mv *MovementValidator) CheckSpeed(from, to shared.Vec2, timeMs uint32) float32
func (mv *MovementValidator) CheckWallPhase(from, to shared.Vec2, timeMs uint32, mType MovementType) WallPhaseResult
func (mv *MovementValidator) ValidateMovement(from, to shared.Vec2, timeMs uint32, mType MovementType) []MovementValidation
```
No separate `estimate_detour_distance()` method exists — the detour estimate
is inlined inside `CheckWallPhase` as a simplified `directDist + TileSize`
approximation (`server/validation.go:74-83`), not a real pathfinding detour.

#### Suspicion Tracking

Real shape, `server/validation.go:122-160` (the design doc's "Suspicion
Manager" is, in code, one `SuspicionTracker` per player, held in
`Server.suspicions`):
```go
type SuspicionTracker struct {
    PlayerID    uint64
    Score       float32
    Events      []shared.SuspicionEvent
    ReportCount uint32 // field exists, nothing increments it yet
}

func (st *SuspicionTracker) AddEvent(event shared.SuspicionEvent)
func (st *SuspicionTracker) GetStatus() SuspicionStatus
```
There is no `is_banned()` — `GetStatus() == SuspicionStatusAutoBan` is
checked directly at the call site (`server/main.go:106`).

#### Event Logger
> **Not implemented.** There is no `EventLogger`, no sampling, and no
> persisted event log. `SuspicionTracker.Events` is an in-memory slice that
> is never sampled, written to disk, or exposed anywhere.

#### Ban List
> **Not implemented.** See [Ban List](#ban-list) under Anti-Cheat System
> above — same gap, no code exists for this yet.

### Update Loop

Real shape: `handlePacket` (`server/main.go:57-115`) runs once per received
UDP packet (not batched), and `tickLoop` (`server/main.go:117-137`) runs
every 100ms independently:
```
On each received packet (handlePacket):
  ├─ Only "Move" is handled; other message types are ignored
  ├─ Validate speed + wall-phase
  ├─ Add suspicion events if any issues found
  └─ Apply new position, unless status == AutoBan

Every 100ms (tickLoop):
  └─ Broadcast every player's state to every known client address
```
There is no sampled logging step and no explicit escalation-threshold check
in the loop — thresholds are only evaluated lazily, inside `GetStatus()`,
whenever `AddEvent` or the auto-ban check happens to call it.

**Per-frame cost:** ~1-5ms for 1000 concurrent players
**Memory:** ~1MB per player (position, state, log buffer)

*(Both figures above are targets from the original design, not measured —
there is no load test in this repo yet.)*

---

## Data Formats

### Level File (TOML)

```toml
[metadata]
name = "area_1_lake"
width = 256
height = 256
tileset = "grassland"

# Collision layer (terrain, for pathfinding)
[collision]
# Raw byte data
# Each byte = TileType (0=blocked, 1=walkable, etc.)
data = "AAABBBCCCC..."

# Visual layers with z-ranges
[[visual_layers]]
name = "underground"
z_min = 0
z_max = 10
data = "..."

[[visual_layers]]
name = "ground"
z_min = 11
z_max = 50
data = "..."

[[visual_layers]]
name = "treeline"
z_min = 51
z_max = 70
data = "..."

# Objects (with z-coordinates for depth)
[[objects]]
id = 1
kind = "npc"
name = "merchant"
x = 100.5
y = 200.3
z = 0              # Ground level
collision_type = "solid"
dialogue = "merchant"

[[objects]]
id = 2
kind = "tree"
x = 150.0
y = 180.0
z = 32             # Tall object (casts shadow)
collision_type = "solid"

[[objects]]
id = 3
kind = "door"
x = 50.0
y = 50.0
z = 0
collision_type = "solid"
destination = "cave_1"
```

> **Not implemented on the game side.** `shared.Level`, `shared.CollisionLayer`,
> `shared.VisualLayer`, and `shared.GameObject` exist as Go structs
> (`shared/types.go:69-104`) with JSON tags, but nothing in `client/` or
> `server/` reads a TOML level file or populates these from disk — both
> binaries build a level in memory via `shared.NewCollisionLayer(100, 100)`
> instead (`client/main.go:48`, `server/main.go:27`). `animaker` (see
> `docs/ANI_MAKER_SPEC.md`) uses TOML for its own project files via
> `github.com/BurntSushi/toml`, unrelated to this level format.

### Attack Definition (TOML)

```toml
[sword_slash_right]
duration_ms = 300

[[sword_slash_right.frames]]
frame = 0
# No hitbox (startup)

[[sword_slash_right.frames]]
frame = 5
hitbox_x = 1
hitbox_y = 0
hitbox_w = 1
hitbox_h = 2

[[sword_slash_right.frames]]
frame = 10
# No hitbox (recovery)
```

> **Not implemented.** No Go type for this exists anywhere in the repo yet —
> see [Attack Prediction](#attack-prediction) above.

### Network Messages (JSON)

```json
{
  "type": "Move",
  "player_id": 12345,
  "position": { "x": 100.5, "y": 200.3 },
  "time_ms": 100,
  "direction": 4
}

{
  "type": "PlayerStates",
  "states": [
    { "player_id": 12345, "position": { "x": 100.5, "y": 200.3 }, "animation": 0, "direction": 4 }
  ]
}
```
These match the actual `json:"..."` tags on `ClientMoveMsg` and
`ServerPlayerStatesMsg` in `shared/protocol.go` — see
[docs/PROTOCOL_REFERENCE.md](PROTOCOL_REFERENCE.md) for the full set of
message shapes and which ones are actually wired up.

---

## Codebase Structure

This is the actual current layout (Go, not Cargo/Rust) — see the repo root
[CLAUDE.md](../CLAUDE.md) for a routing table and [map/](../map/CLAUDE.md)
for a maintained code map with file:line citations:

```
F:\Game\
├── go.mod, go.sum         (module "game")
├── shared/                 (imported by client/ and server/)
│   ├── types.go            (Vec2, TileType, CollisionLayer, Level, GameObject, constants)
│   └── protocol.go         (Client*Msg, Server*Msg, SuspicionEvent)
│
├── client/                 (package main, imports "game/shared")
│   ├── main.go             (raylib window, input, network send/receive loop)
│   ├── prediction.go       (PlayerController, PlayerInterpolation, PlayerInput)
│   └── renderer.go         (z-range sort for shared.GameObject)
│
├── server/                 (package main, imports "game/shared")
│   ├── main.go             (Server, UDP listen loop, tickLoop broadcast)
│   └── validation.go       (MovementValidator, SuspicionTracker)
│
├── animaker/                (separate module "animaker", not imported by the above)
│   ├── go.mod
│   ├── main.go
│   └── pkg/{app,editor,ui,file}/
│
├── docs/                   (this design reference)
├── map/                    (maintained code map — nouns, what a change hits)
├── assets/                 (tileset.png)
├── bin/                    (build output — client.exe, server.exe)
└── build_local.ps1
```

There is no `tools/level_editor` or `tools/asset_compiler` — only
`animaker/` exists today, and it is a sprite/animation editor
(see `docs/ANI_MAKER_SPEC.md`), not a level editor or asset compiler. A
level editor and asset compiler remain unbuilt; if you're picking up that
work, it does not yet have a home in this tree.

---

## Performance Targets

### Client
- **Render FPS:** 144fps consistently
- **Prediction latency:** <16ms (one frame)
- **Memory:** <200MB per client
- **Network:** 5-10 KB/s
### Server
- **Concurrent players:** 1000+
- **Validation latency:** <5ms per movement
- **Memory:** ~1MB per player
- **Network:** 50KB/s per 100 players
### Validation
```
Speed check:    <0.1ms per move
Wall-phase:     <1-5ms per move (depends on distance)
Suspicion:      <0.1ms per update
Logging:        <0.5ms per event (when enabled)
```

> None of the figures in this section have been measured against the actual
> code — they're targets carried over from the original design, not
> benchmarks. There's no load-test harness in this repo yet to produce real
> numbers.

---

## Development Phases

### Phase 1: Core Loop
- [x] Network transport (UDP, JSON envelope)
- [ ] Level loading (collision + visual)
- [x] Basic movement (prediction + server-applied position)
- [x] Speed detection
### Phase 2: Combat
- [ ] Attack definitions
- [ ] Hitbox testing
- [ ] Animation system
- [ ] Damage calculation
### Phase 3: Multi-player
- [x] Player state broadcast
- [x] Interpolation
- [ ] Attack synchronization
- [ ] Arena/zone management
### Phase 4: Tooling
- [ ] Level editor
- [x] Animation maker (`animaker/`, partial — see `docs/ANI_MAKER_SPEC.md`)
- [ ] Asset compiler
### Phase 5: Anti-Cheat
- [x] Wall-phase detection (logic exists; can't trigger against real geometry until level loading exists)
- [x] Suspicion tracking
- [ ] Event logging
- [ ] Ban system (persistence, login check, appeal)
### Phase 6: Content
- [ ] Level design
- [ ] NPCs/enemies
- [ ] Items/economy
- [ ] Quests
---

## Scaling Considerations

### Single Server
- **Capacity:** ~5000 concurrent players
- **Hardware:** $50-100/month cloud VM
### Multiple Servers (Future)
```
Load Balancer
├─ Zone Server 1 (area_1, area_2)
├─ Zone Server 2 (area_3, area_4)
└─ Shared state (Redis for economy, leaderboards)
```

### Chat/Social
- Use separate TCP connection for chat
- Doesn't need game loop timing
- Can be on different server

> Everything in this section is forward-looking design; the current server
> is a single UDP process with no load balancing, no zoning, and no chat
> transport at all (`ClientChatMsg`/`ServerChatMsg` are defined but unhandled).

---

## Security Notes

- **No encryption on game data** (collision, attacks)
  - Attack surface is small (movement validation)
  - Cheaters caught through behavior analysis
  
- **HTTPS for authentication** (separate from game server)
  - Login endpoint returns session token
  - Game server validates token
- **Rate limiting** on sensitive operations
  - Chat messages (spam)
  - Player reports (false flagging)

> **Not implemented.** There is no authentication, no session tokens, and no
> rate limiting anywhere in the current code — the server accepts a
> client-supplied random `PlayerID` at face value
> (`server/main.go:73-74`, comment: "MVP Shortcut: Trust client's random
> PlayerID"). Treat this whole section as a pre-launch requirement, not a
> description of current behavior.

---

## Testing Strategy

### Unit Tests
```
shared/
├─ Vector math
├─ Tile calculations
├─ Message serialization

client/
├─ Prediction logic
├─ Interpolation math
├─ Input handling

server/
├─ Speed detection
├─ Wall-phase detection
├─ Suspicion escalation
```

### Integration Tests
```
├─ Client-server roundtrip
├─ Multiple client sync
├─ Anti-cheat detection
└─ Ban system workflow
```

### Load Tests
```
├─ 100 concurrent players
├─ 1000 concurrent players
└─ Bandwidth/latency profiles
```

> **None of these tests exist yet.** There are no `_test.go` files anywhere
> in this repo as of this writing. This section is a test plan, not a
> description of existing coverage.

---

## Next Steps

1. **Read the code:**
   - `shared/types.go`, `shared/protocol.go` — all shared definitions
   - `server/validation.go` — validation + anti-cheat
   - `client/prediction.go` — client-side logic
   - [map/CLAUDE.md](../map/CLAUDE.md) — a maintained map of what each piece does and what changing it hits
2. **Start with what Phase 1 is missing:**
   - Level loading (collision + visual layers from real data, not the hardcoded placeholder grid)
3. **Use Claude for:**
   - Implementing each module
   - Testing edge cases
   - Debugging integration issues
---

## Questions?

This document covers the high-level design. Each module has detailed code comments. Ask for clarification on any part.
