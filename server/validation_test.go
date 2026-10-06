package main

import (
	"game/shared"
	"game/shared/world"
	"testing"
)

func TestCheckSpeed(t *testing.T) {
	mv := NewMovementValidator(world.NewGrid(10, 10))

	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 100, Y: 0}

	if got := mv.CheckSpeed(from, to, 1000); got != 100 {
		t.Errorf("CheckSpeed over 1s = %v, want 100", got)
	}
}

func TestCheckSpeedZeroTimeIsMaxFloat(t *testing.T) {
	mv := NewMovementValidator(world.NewGrid(10, 10))
	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 10, Y: 0}

	got := mv.CheckSpeed(from, to, 0)
	if got < 1e30 {
		t.Errorf("CheckSpeed with timeMs=0 = %v, want a very large sentinel value", got)
	}
}

func TestCheckWallPhaseCleanPath(t *testing.T) {
	// All-walkable grid: a legitimate move should be Clean.
	mv := NewMovementValidator(world.NewGrid(10, 10))
	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 80, Y: 0} // 5 tiles at TileSize=16

	result := mv.CheckWallPhase(from, to, 1000, MovementTypeWalk)
	if !result.Clean {
		t.Errorf("CheckWallPhase over open ground: Clean = false, want true (result=%+v)", result)
	}
}

func TestCheckWallPhaseBlockedButFeasibleDetourIsNotCheating(t *testing.T) {
	// Block the tile directly in the path (tile x=1). At normal speed, a
	// detour is time-feasible, so this should be flagged for logging but
	// not marked Cheated.
	collision := world.NewGrid(10, 10)
	collision.SetBlocking(1, 0, world.BlockAll)
	mv := NewMovementValidator(collision)

	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 16, Y: 0} // 1 tile, lands exactly on the blocked tile boundary

	result := mv.CheckWallPhase(from, to, 1000, MovementTypeWalk) // slow: 1 tile/sec
	if result.Clean {
		t.Fatalf("expected the blocked tile to be detected, got Clean=true")
	}
	if result.Cheated {
		t.Errorf("CheckWallPhase at legitimate speed: Cheated = true, want false (result=%+v)", result)
	}
}

func TestCheckWallPhaseBlockedAndInfeasibleDetourIsCheating(t *testing.T) {
	// Same blocked tile, but reached far too fast for any detour to be
	// time-feasible: should be marked Cheated.
	collision := world.NewGrid(10, 10)
	collision.SetBlocking(1, 0, world.BlockAll)
	mv := NewMovementValidator(collision)

	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 16, Y: 0}

	result := mv.CheckWallPhase(from, to, 200, MovementTypeWalk) // fast: 5 tiles/sec
	if !result.Cheated {
		t.Errorf("CheckWallPhase at high speed into a wall: Cheated = false, want true (result=%+v)", result)
	}
}

func TestCheckWallPhaseSkipsTeleportAndKnockback(t *testing.T) {
	collision := world.NewGrid(10, 10)
	collision.SetBlocking(1, 0, world.BlockAll)
	mv := NewMovementValidator(collision)

	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 16, Y: 0}

	for _, mType := range []MovementType{MovementTypeTeleport, MovementTypeKnockedBack} {
		result := mv.CheckWallPhase(from, to, 10, mType)
		if !result.Clean {
			t.Errorf("CheckWallPhase with movement type %v should skip wall-phase checks, got Clean=false", mType)
		}
	}
}

func TestValidateMovementFlagsTooFast(t *testing.T) {
	mv := NewMovementValidator(world.NewGrid(10, 10))
	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 500, Y: 0} // way beyond MaxSpeed*TileSize*SpeedTolerance

	issues := mv.ValidateMovement(from, to, 1000, MovementTypeWalk)

	found := false
	for _, issue := range issues {
		if issue.TooFast {
			found = true
		}
	}
	if !found {
		t.Errorf("ValidateMovement did not flag an obviously too-fast move: issues=%+v", issues)
	}
}

func TestSuspicionTrackerStatusThresholds(t *testing.T) {
	tracker := NewSuspicionTracker(1)

	if got := tracker.GetStatus(); got != SuspicionStatusNormal {
		t.Fatalf("fresh tracker status = %v, want SuspicionStatusNormal", got)
	}

	// shared.SuspicionSpeedHack = 0.5; 10 events crosses EnableLogging (5.0)
	// but stays under FlagReview (8.0).
	for i := 0; i < 10; i++ {
		tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventTooFast})
	}
	if got := tracker.GetStatus(); got != SuspicionStatusFullLogging {
		t.Errorf("after 10 TooFast events (score=%v), status = %v, want SuspicionStatusFullLogging", tracker.Score, got)
	}

	// 6 more events (+3.0, score=8.0) crosses FlagReview (8.0) but stays under AutoBan (10.0).
	for i := 0; i < 6; i++ {
		tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventTooFast})
	}
	if got := tracker.GetStatus(); got != SuspicionStatusFlagForReview {
		t.Errorf("after 16 TooFast events (score=%v), status = %v, want SuspicionStatusFlagForReview", tracker.Score, got)
	}

	// One WallPhase event (+2.0) pushes score to 10.0, crossing AutoBan.
	tracker.AddEvent(shared.SuspicionEvent{Type: shared.SuspicionEventWallPhase})
	if got := tracker.GetStatus(); got != SuspicionStatusAutoBan {
		t.Errorf("after crossing 10.0 (score=%v), status = %v, want SuspicionStatusAutoBan", tracker.Score, got)
	}
}

// testLevel builds a level from the repo's sample terrain definitions:
// rows of terrain letters (g grass, w water, b black) placed at pos.
func testLevel(t *testing.T, defs *world.Defs, name string, pos world.Point, rows ...string) *world.Level {
	t.Helper()
	l, err := world.NewLevel(name, pos, len(rows[0]), len(rows))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[byte]string{'g': "grass", 'w': "water", 'b': "black"}
	for y, row := range rows {
		for x := range row {
			l.SetTerrain(x, y, defs.TerrainByName(ids[row[x]]).ID)
		}
	}
	return l
}

func testWorld(t *testing.T, levels ...func(*world.Defs) *world.Level) *world.World {
	t.Helper()
	defs, err := world.LoadDefs("../world/terrains.toml")
	if err != nil {
		t.Fatal(err)
	}
	w := world.NewWorld(defs)
	for _, mk := range levels {
		if err := w.Add(mk(defs)); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range w.Levels {
		w.SolveLevel(l)
	}
	return w
}

func px(tile int) float32 { return float32(tile) * shared.TileSize }

func TestWallPhaseCaughtAcrossLevelBoundary(t *testing.T) {
	// Level a is open grass; level b, to its east, starts with a column of
	// black (blocked). Walking from a into b's black column is caught even
	// though the two tiles belong to different levels.
	w := testWorld(t,
		func(d *world.Defs) *world.Level { return testLevel(t, d, "a", world.Point{X: -4, Y: 0}, "gggg") },
		func(d *world.Defs) *world.Level { return testLevel(t, d, "b", world.Point{X: 0, Y: 0}, "bggg") },
	)
	mv := NewMovementValidator(w)
	from := shared.Vec2{X: px(-1), Y: 0}
	to := shared.Vec2{X: px(0) + 4, Y: 0}
	if r := mv.CheckWallPhase(from, to, 50, MovementTypeWalk); r.Clean || r.TileX != 0 {
		t.Errorf("walking into level b's wall: %+v, want blocked at tile 0", r)
	}
	// Negative coordinates floor correctly: -0.5 px is tile -1 (open grass),
	// not tile 0 (black).
	if r := mv.CheckWallPhase(shared.Vec2{X: -8, Y: 0}, shared.Vec2{X: -0.5, Y: 0}, 50, MovementTypeWalk); !r.Clean {
		t.Errorf("moving within tile -1 was flagged: %+v", r)
	}
	// Void beyond both levels blocks.
	if r := mv.CheckWallPhase(shared.Vec2{X: px(-4), Y: 0}, shared.Vec2{X: px(-5), Y: 0}, 50, MovementTypeWalk); r.Clean {
		t.Error("walking off the edge of the world into void was not caught")
	}
}

func TestJumpingAFenceIsAllowedWalkingThroughIsNot(t *testing.T) {
	// A fence blocks ground and roll, but not jump.
	g := world.NewGrid(10, 10)
	g.SetBlocking(1, 0, world.BlockGround|world.BlockRoll)
	mv := NewMovementValidator(g)
	from, to := shared.Vec2{X: 0, Y: 0}, shared.Vec2{X: px(2), Y: 0}
	if r := mv.CheckWallPhase(from, to, 1000, MovementTypeJump); !r.Clean {
		t.Errorf("jumping the fence was flagged: %+v", r)
	}
	if r := mv.CheckWallPhase(from, to, 1000, MovementTypeWalk); r.Clean {
		t.Error("walking through the fence was not caught")
	}
}

func tooFast(issues []MovementValidation) bool {
	for _, i := range issues {
		if i.TooFast {
			return true
		}
	}
	return false
}

func TestSpeedCheckHonoursSwimming(t *testing.T) {
	w := testWorld(t, func(d *world.Defs) *world.Level {
		return testLevel(t, d, "a", world.Point{}, "gggggggggg", "wwwwwwwwww")
	})
	mv := NewMovementValidator(w)
	swimSpeed := shared.MaxSpeed * shared.TileSize * 0.5 // water's multiplier in world/terrains.toml
	// One network tick (50 ms) of movement at a given speed along a row.
	move := func(row int, speed float32) []MovementValidation {
		from := shared.Vec2{X: px(2), Y: px(row)}
		to := shared.Vec2{X: from.X + speed*0.05, Y: from.Y}
		return mv.ValidateMovement(from, to, 50, MovementTypeWalk)
	}
	if issues := move(1, swimSpeed); len(issues) != 0 {
		t.Errorf("swimming at swim speed was flagged: %+v", issues)
	}
	// SpeedTolerance (2.0) allows up to twice the honest speed, so walking
	// speed in water is exactly the limit; 1.5x walking speed is past it.
	walk := shared.MaxSpeed * shared.TileSize
	if !tooFast(move(1, walk*1.5)) {
		t.Error("1.5x walking speed in water was not flagged")
	}
	if tooFast(move(0, walk*1.5)) {
		t.Error("1.5x walking speed on grass was flagged; the multiplier leaked out of the water")
	}
	// Climbing out of the water onto grass uses the larger multiplier.
	from := shared.Vec2{X: px(2), Y: px(1)}
	to := shared.Vec2{X: from.X, Y: from.Y - walk*0.05}
	if issues := mv.ValidateMovement(from, to, 50, MovementTypeWalk); len(issues) != 0 {
		t.Errorf("stepping from water onto grass at walking speed was flagged: %+v", issues)
	}
}
