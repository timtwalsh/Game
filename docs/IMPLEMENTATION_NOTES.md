# Implementation Notes & Decisions
 
This document captures architectural decisions, trade-offs, and implementation details discussed during design.
 
---
 
## Technology Stack (FINAL DECISION)
 
**Chosen: Go + raylib-go + goroutines**
 
### Why Go Over Alternatives
 
| Factor | Go | Rust | C++ | C |
|--------|----|----|-----|---|
| Networking | ⭐⭐⭐ Goroutines are natural | ⭐⭐ Tokio is complex | ⭐⭐ Boost.asio is verbose | ⭐ Callback hell |
| Iteration Speed | ⭐⭐⭐ 15s compile | ⭐⭐ 30s compile | ⭐ 10min compile | ⭐⭐ Fast but slow to write |
| Code Readability | ⭐⭐⭐ Crystal clear | ⭐⭐ Needs Rust knowledge | ⭐⭐ Complex templates | ⭐ Manual memory |
| Development Speed | ⭐⭐⭐ Fast | ⭐⭐ Medium | ⭐ Slow | ⭐ Very slow |
| Copiloting UX | ⭐⭐⭐ Easy to understand | ⭐⭐ Learning curve | ⭐ Hard to follow | ⭐ Segfaults |
 
### Go-Specific Considerations
 
**GC Tuning Strategy:**
```
Default: GOGC=100
Tuning: Set GOGC=200-500 (higher = less frequent pauses)
Arena allocation: Allocate large chunks at frame start, reuse for frame
Expected result: 144fps achievable with minimal tuning
```
 
**Network Code Pattern:**
```go
// Spawn goroutine per player
go handlePlayer(connection)
 
// Channels for safe communication
results := make(chan PlayerUpdate)
go func() { results <- update }()
```
 
This is vastly cleaner than async/await or thread management.
 
---
 
## Anti-Cheat System (FINAL DESIGN)
 
**4-Layer Approach:**
 
### Layer A: Automated Speed Detection
- **Trigger**: Every movement packet
- **Check**: `speed = distance / time > MAX_SPEED * 2.0`
- **Cost**: O(1), negligible
- **Action**: Add suspicion +0.5
- **False Positive Rate**: <0.1%
### Layer B: Automated Wall-Phase Detection
- **Trigger**: Every movement packet
- **Check**: Bresenham line-drawing along path, test each tile
- **Logic**:
  - If blocked tile in path: Could they go around?
  - If detour would require `speed > MAX_SPEED`: Definitely cheating
- **Cost**: O(distance), ~1-5ms per move
- **Action**: Add suspicion +2.0 (if cheating detected)
- **False Positive Rate**: <1%
### Layer C: Escalation Thresholds
```
Suspicion Score Ranges:
├─ 0.0 - 5.0:   Normal (10% sampling)
├─ 5.0 - 8.0:   Full logging enabled
├─ 8.0 - 10.0:  Flag for human review
└─ 10.0+:       Auto-ban (human confirms)
```
 
### Layer D: Community + Oversight
```
Player Reports:
├─ 1 report: Log reason
├─ 5+ reports in 24h: Enable full logging, flag for review
└─ Human decision: Ban or dismiss
 
Tournaments:
├─ Game master present
├─ 100% logging enabled
├─ Real-time monitoring
└─ Instant action capability
```
 
### Sampling Strategy
```
10% of movements are logged (deterministic random per player)
Sampling rate survives: Most cheaters caught within 10 hours
Timeline: Obvious cheaters (speedhacking) caught within 1-2 hours
         Subtle cheaters (wall-phasing) caught within 10+ hours
```
 
**Why sampling works:**
- Cheaters produce obvious patterns (multiple violations per hour)
- 10% sample: High probability of catching patterns
- Casual cheaters: Eventually caught
- Determined cheaters: Require significant time investment + detection
- Cost: Negligible (log only 10% of events)
---
 
## Movement Validation
 
### Server-Side Validation
 
**What we validate:**
- Speed: Distance/time must be <= MAX_SPEED * tolerance
- Wall-phase: Path cannot cross blocked tiles unless detour possible
**What we trust:**
- Position is accurate (client predicts, we verify pattern)
- Attack hits are legitimate (hitbox data is same on both sides)
**Movement history:**
```
Server keeps: Last 500ms of position updates per player
Purpose: Validate attacks against historical position (not current)
Timeline: Attack at T, validate against position at T (not T+latency)
```
 
### Client-Side Prediction
 
**Client does:**
1. Player presses input
2. Local collision test against terrain + objects
3. Update predicted position
4. Render immediately (no lag)
5. Send to server every tick (shared.NetworkTickRate, 50ms as of 2026-09-17)
**Server does:**
1. Receive position
2. Validate speed + wall-phase
3. Broadcast to nearby players
4. Log if sampled
5. Track suspicion
---
 
## Network Protocol Details
 
### Message Format
- **Serialization**: JSON (human-readable, easy debugging) or MessagePack (compact)
- **Update Frequency**: 20 per second (50ms) — `shared.NetworkTickRate`,
  raised from the original 10/sec (100ms) on 2026-09-17 to cut perceived
  cross-client latency; see `docs/PROTOCOL_REFERENCE.md`'s Workflow section
- **Bandwidth**: ~10-20 KB/s per player (roughly double the original
  estimate, since it scales with update frequency)
### Critical Timing
 
**Attack Validation Timeline:**
```
T+0ms:    Client fires attack
          ├─ Local hitbox test (immediate feedback)
          └─ Send: {type: Attack, direction: u8, time: T}
 
T+100ms:  Server receives
          ├─ Look up attack definition
          ├─ Get player position at time T (from history)
          ├─ Get target position at time T
          ├─ Test if hitbox overlaps
          └─ Broadcast: {type: AttackResult, hit: bool}
 
T+200ms:  Client receives result
          ├─ If miss: Show miss animation
          └─ If hit: Already showed hit (confirmed)
```
 
**Clock Synchronization:**
```
Client clock may drift. Solution:
├─ Server sends authoritative time periodically
├─ Client adjusts with round-trip latency
├─ Attacks validated within ±50ms tolerance
└─ Prevents backdating attacks
```
 
---
 
## Collision Data Handling
 
### Decision: No Encryption
 
**Why we decided against it:**
- Collision layer is tiny (~64KB for 256x256, ~1MB for 1000x1000)
- Network overhead of encryption > benefit
- Server validates anyway (catches cheating regardless)
- Checksum is overkill (TCP handles corruption)
**What we send:**
```json
{
  "level": "area_1",
  "collision": "AAABBBCCCC...",  // Raw bytes, base64 encoded
  "visual_layers": [...],
  "objects": [...]
}
```
 
---
 
## Client Prediction Strategy
 
### Movement Prediction Cycle
 
```
Every frame (7ms at 144fps):
├─ Read input (keyboard)
├─ Test local collision
├─ Update predicted_position
├─ Render at predicted_position
 
Every tick (shared.NetworkTickRate, 50ms):
├─ Send position to server
├─ Server broadcasts to others
├─ Other clients interpolate over one tick (50ms)
└─ Next network update arrives
```
 
### Interpolation (Other Players)
 
```
Receive: PlayerState { position: (100, 50) }
├─ Set target_position = (100, 50)
├─ Over next tick (50ms): Lerp from old_position → target_position
├─ Render smoothly
└─ At T+50ms: At target, wait for next update
```
 
### Server Correction
 
```
If server disagrees (rare):
├─ Receive: {position: (90, 90)} (corrected)
├─ Set corrected position
├─ Resume prediction from there
└─ Looks like lag to player (acceptable)
```
 
---
 
## Development Methodology
 
### Copiloting Model
 
**You (pilot):**
- Make architectural decisions
- Provide game design direction
- Review code for logic correctness
- Test and iterate
- Make trade-off calls
**Me (co-pilot writing code):**
- Implement modules based on architecture
- Handle language-specific complexity
- Debug technical issues
- Optimize performance
- Write tests
### Why This Works for Go
 
```
Iteration cycle:
1. You say: "Add anti-cheat check for X"
2. I write code: 30 lines
3. Compile: 15 seconds (you wait minimally)
4. Test: Works
5. Repeat
 
Compare to C++:
1. You say: "Add anti-cheat check for X"
2. I write code: 50 lines (more boilerplate)
3. Compile: 10 minutes (you go get coffee)
4. Test: Linker error, fix, recompile: 10 more minutes
5. Repeat
```
 
**Go is best for copiloting** because code is readable and iteration is fast.
 
---
 
## Performance Targets & Validation
 
### Frame Budget (at 144fps)
```
Total: 6.9ms per frame
├─ Input: 0.1ms
├─ Prediction: 1.0ms
├─ Rendering: 4.0ms
├─ Network: 1.0ms
└─ Buffer: 0.8ms
```
 
### Server Load (100 concurrent players)
```
Per second:
├─ 1000 position updates (100 players × 10/sec)
├─ Speed validation: ~100ms total (0.1ms each)
├─ Wall-phase validation: ~500ms total (0.5ms each, 10%)
├─ Serialization: ~50ms
└─ Broadcasting: ~100ms
Total: ~750ms (well within 1000ms available)
```
 
### Anti-Cheat Cost
```
Speed check: O(1), <0.1ms per move
Wall-phase check: O(distance), ~1-5ms per move (10% sampled)
Suspicion tracking: O(1), <0.1ms per update
Event logging: O(1) append, minimal
Total: Negligible impact
```
 
---
 
## Asset Pipeline
 
### Level Files (TOML)
```toml
[metadata]
name = "area_1"
width = 256
height = 256
 
[collision]
data = "raw bytes as base64"
 
[[visual_layers]]
name = "underground"
z_min = 0
z_max = 10
data = "tile IDs"
 
[[objects]]
id = 1
kind = "tree"
x = 150.0
y = 180.0
z = 32
collision_type = "solid"
```
 
### Attack Definitions (TOML)
```toml
[sword_slash_right]
duration_ms = 300
 
[[sword_slash_right.frames]]
frame = 5
hitbox = {x: 1, y: 0, w: 1, h: 2}
```
 
### Compilation Pipeline
```
Raw Assets (human-editable)
├─ levels/*.level.toml
├─ attacks.toml
└─ tilesets/*.png
 
    ↓ (asset_compiler)
 
Compiled Assets (game-ready)
├─ levels/*.level.bin or .json
├─ attacks.bin
└─ tilesets.atlas
 
    ↓ (packaged with)
 
Game Binary
├─ Client loads from disk
└─ Server loads from disk
```
 
---
 
## Scaling Considerations
 
### Single Server
- Capacity: 5000 concurrent players
- Hardware: $50-100/month cloud VM
- Per player memory: ~1MB
- Bandwidth: ~5-10 KB/s per player
### Multiple Servers (Future)
```
Load Balancer
├─ Zone Server 1 (area_1, area_2)
├─ Zone Server 2 (area_3, area_4)
└─ Shared state server (Redis)
    ├─ Ban list
    ├─ Economy/items
    └─ Leaderboards
```
 
### Chat Server (Separate)
```
Dedicated chat server (can be different tech)
├─ TCP persistent connections
├─ Doesn't need game loop timing
├─ Can be simpler, older tech
└─ Scales independently
```
 
---
 
## Ban Appeal Process
 
### Automated Bans
```
Score >= 10.0
├─ Auto-ban triggers
├─ Player receives: "Auto-detected cheating"
├─ Ban creates ticket for human review
└─ Human confirms or resets
```
 
### Manual Bans
```
Human moderator
├─ Reviews full log
├─ Makes decision
├─ Records reason
└─ Player can appeal
```
 
### Appeal Process
```
Player submits appeal
├─ Admin reviews ticket + appeal
├─ Options:
│  ├─ Confirm ban (deny appeal)
│  ├─ Reduce ban duration
│  └─ Lift ban (unusual)
└─ Communicate decision
```
 
---
 
## Tournament Mode
 
### During Tournament
 
```
Game Master Privileges:
├─ Spectate any player (see predicted position)
├─ View real-time logs
├─ Instant muting/kicking
└─ Replay review capability
 
Logging:
├─ 100% of all events logged
├─ No sampling
├─ Available for instant review
└─ Stored for post-tournament analysis
 
Detection:
├─ Same automated checks (speed, wall-phase)
├─ Immediate escalation
├─ GM can observe and act
└─ Zero tolerance
```
 
---
 
## Testing Strategy
 
### Unit Tests
```go
// Speed validation
TestSpeedCheck_Normal()
TestSpeedCheck_TooFast()
 
// Wall-phase
TestWallPhase_Clean()
TestWallPhase_Detected()
TestWallPhase_Cheated()
 
// Suspicion escalation
TestSuspicion_AddEvent()
TestSuspicion_ThresholdEscalation()
```
 
### Integration Tests
```
MultiPlayerMovementSync()
  ├─ Player 1 moves
  ├─ Player 2 sees update
  ├─ Latency: 100ms
  └─ Assert smooth interpolation
 
AntiCheatDetection()
  ├─ Simulate speedhack
  ├─ Assert suspicion++
  ├─ Continue to escalation
  └─ Assert auto-ban
 
AttackValidation()
  ├─ Player 1 attacks
  ├─ Server validates hitbox
  ├─ Player 2 takes damage (or doesn't)
  └─ Assert correct result
```
 
### Load Tests
```
100 Players:    1000 position updates/sec
1000 Players:   10,000 position updates/sec
Measure:
  ├─ Validation latency
  ├─ Broadcast latency
  ├─ Server CPU
  └─ Memory usage
```
 
---
 
## Known Limitations & Future Work
 
### Current Scope
```
✅ Single-player movement + collision
✅ Multi-player synchronization
✅ Combat with hitboxes
✅ Basic anti-cheat
✅ 144fps client rendering
✅ 1000 concurrent player scale
```
 
### Not in Scope (v1)
```
❌ Physics engine (intentional: keep simple)
❌ Advanced AI (procedural, learning)
❌ Economy system
❌ Guilds/social features
❌ Dungeons/instances
❌ Skill trees/progression
❌ Persistent saves (planned, not required)
```
 
### Future Enhancements
```
Phase 7: Persistence
├─ Save player progress
├─ Character database
└─ Progression tracking
 
Phase 8: Social
├─ Friend lists
├─ Guilds
└─ Social chat
 
Phase 9: Content
├─ Multiple maps
├─ Quests
├─ NPCs with dialogue trees
└─ Economy
```
 
---
 
## Decision Log
 
### Why Not Client-Side Validation?
```
Considered: Trust client completely
Problem: Zero anti-cheat
Decision: Validate at least speed + wall-phase (catches 95% of cheating)
Cost: Minimal (automated, not expensive)
```
 
### Why Trust Client Positions?
```
Considered: Simulate everything server-side
Problem: Can't simulate 1000 players' physics + collisions efficiently
Decision: Validate critical aspects (speed, geometry)
Trust: Client position (but log for analysis)
Result: Scale to 1000+ players
```
 
### Why 10% Sampling?
```
Considered: Log everything (too much data)
Considered: Log nothing (no anti-cheat)
Decision: 10% sampling
Reason: High probability of catching patterns + negligible cost
Math: Cheater with 100+ violations/hour: ~90% caught in 10 hours
```
 
### Why Not EAC/BattlEye?
```
Considered: Use external anti-cheat engine
Problem: Licensing costs, overkill for casual MMO
Problem: Black box (can't debug)
Decision: Build custom, lightweight
Result: Simple, auditable, free
```
 
---
 
## Open Questions / Decisions Needed
 
1. **Persistence**: How much data save per player?
   - Current position? Character stats? Inventory?
   
2. **Economy**: Will there be items/currency?
   - If yes: Trade security becomes important
   - If no: Simplifies design
   
3. **PvP Rules**: Can players attack each other freely?
   - Yes: Need damage validation server-side
   - No: Only NPCs take damage
   
4. **Zones**: How are areas divided?
   - Single world? Instanced? Zone-based?
   - Affects networking and scaling strategy
   
5. **Progression**: Do players level up?
   - If yes: Stats affect combat validation
   - If no: All players same power level
---
 
End of implementation notes. Refer back here when building specific systems.
