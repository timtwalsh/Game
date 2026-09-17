package main

import (
	"game/shared"
	"testing"
)

func TestCheckSpeed(t *testing.T) {
	mv := NewMovementValidator(shared.NewCollisionLayer(10, 10))

	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 100, Y: 0}

	if got := mv.CheckSpeed(from, to, 1000); got != 100 {
		t.Errorf("CheckSpeed over 1s = %v, want 100", got)
	}
}

func TestCheckSpeedZeroTimeIsMaxFloat(t *testing.T) {
	mv := NewMovementValidator(shared.NewCollisionLayer(10, 10))
	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 10, Y: 0}

	got := mv.CheckSpeed(from, to, 0)
	if got < 1e30 {
		t.Errorf("CheckSpeed with timeMs=0 = %v, want a very large sentinel value", got)
	}
}

func TestCheckWallPhaseCleanPath(t *testing.T) {
	// All-walkable grid: a legitimate move should be Clean.
	mv := NewMovementValidator(shared.NewCollisionLayer(10, 10))
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
	collision := shared.NewCollisionLayer(10, 10)
	collision.Set(1, 0, shared.TileTypeBlocked)
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
	collision := shared.NewCollisionLayer(10, 10)
	collision.Set(1, 0, shared.TileTypeBlocked)
	mv := NewMovementValidator(collision)

	from := shared.Vec2{X: 0, Y: 0}
	to := shared.Vec2{X: 16, Y: 0}

	result := mv.CheckWallPhase(from, to, 200, MovementTypeWalk) // fast: 5 tiles/sec
	if !result.Cheated {
		t.Errorf("CheckWallPhase at high speed into a wall: Cheated = false, want true (result=%+v)", result)
	}
}

func TestCheckWallPhaseSkipsTeleportAndKnockback(t *testing.T) {
	collision := shared.NewCollisionLayer(10, 10)
	collision.Set(1, 0, shared.TileTypeBlocked)
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
	mv := NewMovementValidator(shared.NewCollisionLayer(10, 10))
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
