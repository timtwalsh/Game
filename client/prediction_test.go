package main

import (
	"game/shared"
	"game/shared/world"
	"math"
	"testing"
	"time"
)

func almostEqual(a, b float32) bool {
	const eps = 0.001
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < eps
}

func TestGetMovementVectorCardinal(t *testing.T) {
	cases := []struct {
		name  string
		input PlayerInput
		want  shared.Vec2
	}{
		{"right", PlayerInput{Right: true}, shared.Vec2{X: 1, Y: 0}},
		{"left", PlayerInput{Left: true}, shared.Vec2{X: -1, Y: 0}},
		{"up", PlayerInput{Up: true}, shared.Vec2{X: 0, Y: -1}},
		{"down", PlayerInput{Down: true}, shared.Vec2{X: 0, Y: 1}},
		{"none", PlayerInput{}, shared.Vec2{X: 0, Y: 0}},
	}
	for _, c := range cases {
		got := c.input.GetMovementVector()
		if !almostEqual(got.X, c.want.X) || !almostEqual(got.Y, c.want.Y) {
			t.Errorf("%s: GetMovementVector() = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestGetMovementVectorDiagonalIsNormalized(t *testing.T) {
	input := PlayerInput{Up: true, Right: true}
	got := input.GetMovementVector()
	length := float32(math.Sqrt(float64(got.X*got.X + got.Y*got.Y)))
	if !almostEqual(length, 1.0) {
		t.Errorf("diagonal movement vector length = %v, want 1.0 (got=%+v)", length, got)
	}
}

func TestGetDirection(t *testing.T) {
	cases := []struct {
		name  string
		input PlayerInput
		want  uint8
	}{
		{"none", PlayerInput{}, 255},
		{"north", PlayerInput{Up: true}, 0},
		{"northeast", PlayerInput{Up: true, Right: true}, 1},
		{"east", PlayerInput{Right: true}, 2},
		{"south", PlayerInput{Down: true}, 4},
		{"west", PlayerInput{Left: true}, 6},
	}
	for _, c := range cases {
		if got := c.input.GetDirection(); got != c.want {
			t.Errorf("%s: GetDirection() = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestPlayerControllerMovesOnOpenGround(t *testing.T) {
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, world.NewGrid(10, 10))
	input := PlayerInput{Right: true}

	pc.UpdatePrediction(&input, 100*time.Millisecond)

	// speed = MaxSpeed*TileSize = 80 units/sec; 100ms -> 8 units.
	if !almostEqual(pc.PredictedPosition.X, 8) {
		t.Errorf("PredictedPosition.X = %v, want 8", pc.PredictedPosition.X)
	}
	if pc.Direction != 2 { // East
		t.Errorf("Direction = %d, want 2 (East)", pc.Direction)
	}
}

func TestPlayerControllerStopsAtWall(t *testing.T) {
	collision := world.NewGrid(10, 10)
	collision.SetBlocking(3, 0, world.BlockGround) // covers x in [48,64)

	pc := NewPlayerController(shared.Vec2{X: 20, Y: 0}, collision)
	input := PlayerInput{Right: true}

	// 20 -> 28 -> 36: the 12px box's far edge reaches 47.98, still in tile 2.
	pc.UpdatePrediction(&input, 100*time.Millisecond)
	pc.UpdatePrediction(&input, 100*time.Millisecond)
	if !almostEqual(pc.PredictedPosition.X, 36) {
		t.Fatalf("after two steps, PredictedPosition.X = %v, want 36", pc.PredictedPosition.X)
	}

	// The next step would put the box's far edge in the blocked tile 3.
	pc.UpdatePrediction(&input, 100*time.Millisecond)
	if !almostEqual(pc.PredictedPosition.X, 36) {
		t.Errorf("after moving into a wall, PredictedPosition.X = %v, want unchanged at 36", pc.PredictedPosition.X)
	}
}

func TestPlayerControllerBoxCannotOverlapAWall(t *testing.T) {
	// A wall below the player: moving down stops while the box's bottom
	// edge is still above it, not when the top-left point reaches it.
	collision := world.NewGrid(10, 10)
	collision.SetBlocking(0, 2, world.BlockGround) // y in [32,48)
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, collision)
	input := PlayerInput{Down: true}
	for i := 0; i < 60; i++ {
		pc.UpdatePrediction(&input, simStep)
	}
	if bottom := pc.PredictedPosition.Y + playerHull; bottom > 32 {
		t.Errorf("box bottom = %v, overlaps the wall starting at y=32", bottom)
	}
	if pc.PredictedPosition.Y < 32-playerHull-2 {
		t.Errorf("stopped at y=%v, well short of the wall", pc.PredictedPosition.Y)
	}
}

func TestPlayerControllerNegativeCoordinatesFloor(t *testing.T) {
	// Tile -1 blocks; standing at x=1 (tile 0) and stepping left to x=-0.5
	// puts the box's left edge in tile -1, which must stop the move. A
	// truncating conversion would read -0.5 as tile 0 and let it through.
	collision := world.NewGrid(10, 10) // everything outside 0..9 blocks
	pc := NewPlayerController(shared.Vec2{X: 1, Y: 0}, collision)
	if pc.CanMoveTo(-0.5, 0) {
		t.Error("CanMoveTo(-0.5, 0) = true; -0.5 px is tile -1, outside the grid")
	}
}

func TestPlayerControllerSwimsAtTheWatersSpeed(t *testing.T) {
	collision := world.NewGrid(10, 10)
	for x := 0; x < 10; x++ {
		collision.SetSpeed(x, 0, 0.5)
	}
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, collision)
	input := PlayerInput{Right: true}
	pc.UpdatePrediction(&input, 100*time.Millisecond)
	// Half of 80 px/s for 100ms.
	if !almostEqual(pc.PredictedPosition.X, 4) {
		t.Errorf("PredictedPosition.X = %v, want 4 at half speed", pc.PredictedPosition.X)
	}
}

func TestPlayerControllerServerCorrection(t *testing.T) {
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, world.NewGrid(10, 10))
	pc.ServerCorrection(shared.Vec2{X: 42, Y: 7})

	if pc.Position.X != 42 || pc.Position.Y != 7 {
		t.Errorf("Position after ServerCorrection = %+v, want {42 7}", pc.Position)
	}
}

func TestPlayerInterpolationEasesThenSnaps(t *testing.T) {
	pi := NewPlayerInterpolation(shared.Vec2{X: 0, Y: 0})
	pi.ServerUpdate(shared.PlayerState{Position: shared.Vec2{X: 100, Y: 0}})

	half := networkTick / 2
	pi.Update(half) // half of InterpolationDuration, whatever it's currently tuned to
	if !almostEqual(pi.CurrentPosition.X, 50) {
		t.Errorf("halfway through interpolation, CurrentPosition.X = %v, want 50", pi.CurrentPosition.X)
	}

	pi.Update(networkTick - half) // remaining half
	if !almostEqual(pi.CurrentPosition.X, 100) {
		t.Errorf("after full interpolation window, CurrentPosition.X = %v, want 100 (snapped to target)", pi.CurrentPosition.X)
	}
}

func TestPlayerInterpolationAdaptsToALateUpdate(t *testing.T) {
	// Regression test for the "freeze then snap" jitter bug: if the real
	// gap between updates runs longer than InterpolationDuration, the old
	// code froze at the target (Update's outer guard stopped the clock),
	// then had to cover the resulting larger distance in a fixed short
	// window next time - a visible stall-then-rush pattern. The fix
	// measures the real elapsed gap and uses it for the next segment.
	pi := NewPlayerInterpolation(shared.Vec2{X: 0, Y: 0})
	pi.ServerUpdate(shared.PlayerState{Position: shared.Vec2{X: 100, Y: 0}})

	// Let the first segment finish, then simulate updates arriving late:
	// several ticks pass with no new ServerUpdate - enough to exceed
	// interpolationDurationMax, so this also exercises the upper clamp.
	lateGap := networkTick * 5
	pi.Update(lateGap)
	if !almostEqual(pi.CurrentPosition.X, 100) {
		t.Fatalf("should have reached the first target during the gap, got CurrentPosition.X = %v", pi.CurrentPosition.X)
	}

	// The delayed update finally arrives. Its interpolation duration
	// should reflect the real ~3-tick gap since the last ServerUpdate,
	// not silently reset to a single tick.
	pi.ServerUpdate(shared.PlayerState{Position: shared.Vec2{X: 200, Y: 0}})
	if pi.InterpolationDuration <= networkTick {
		t.Errorf("InterpolationDuration after a %v gap = %v, want it adapted upward (was pinned at %v pre-fix)", lateGap, pi.InterpolationDuration, networkTick)
	}
	if pi.InterpolationDuration != interpolationDurationMax {
		t.Errorf("InterpolationDuration = %v, want it clamped to interpolationDurationMax (%v) for a gap this large", pi.InterpolationDuration, interpolationDurationMax)
	}

	// A single network tick's worth of progress should now cover
	// proportionally less distance than before (since it must be spread
	// over the longer, adapted window) - i.e. no more instant-looking snap.
	pi.Update(networkTick)
	if pi.CurrentPosition.X >= 150 {
		t.Errorf("after one normal tick into an adapted (longer) window, CurrentPosition.X = %v, want well short of the target (100) - a snap indicates the duration didn't adapt", pi.CurrentPosition.X)
	}
}

func TestPlayerInterpolationClampsAVeryShortGap(t *testing.T) {
	pi := NewPlayerInterpolation(shared.Vec2{X: 0, Y: 0})
	pi.ServerUpdate(shared.PlayerState{Position: shared.Vec2{X: 100, Y: 0}})

	// Two updates arrive back-to-back with essentially no time between them.
	pi.Update(time.Millisecond)
	pi.ServerUpdate(shared.PlayerState{Position: shared.Vec2{X: 200, Y: 0}})

	if pi.InterpolationDuration != interpolationDurationMin {
		t.Errorf("InterpolationDuration after a ~1ms gap = %v, want it clamped to interpolationDurationMin (%v)", pi.InterpolationDuration, interpolationDurationMin)
	}
}

func TestPlayerControllerMovementIsFrameRateIndependent(t *testing.T) {
	// Regression test for the frame-timing bug: the client used to step the
	// simulation by each render frame's elapsed time truncated to whole
	// milliseconds, so at 144fps (~6.94ms frames counted as 6ms) the player
	// moved ~13% slower than MaxSpeed, and the error varied with frame rate.
	// The simulation now always advances in fixed simSteps, so one second of
	// simulated time covers exactly MaxSpeed tiles.
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, world.NewGrid(100, 100))
	input := PlayerInput{Right: true}

	for i := uint32(0); i < shared.SimTickHz; i++ {
		pc.UpdatePrediction(&input, simStep)
	}

	want := shared.MaxSpeed * shared.TileSize
	if !almostEqual(pc.PredictedPosition.X, want) {
		t.Errorf("after 1s of sim steps, PredictedPosition.X = %v, want %v", pc.PredictedPosition.X, want)
	}
}

func TestPlayerControllerRenderPositionBlendsSteps(t *testing.T) {
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, world.NewGrid(10, 10))
	input := PlayerInput{Right: true}
	pc.UpdatePrediction(&input, 100*time.Millisecond) // 0 -> 8

	if got := pc.RenderPosition(0); !almostEqual(got.X, 0) {
		t.Errorf("RenderPosition(0).X = %v, want 0 (previous step)", got.X)
	}
	if got := pc.RenderPosition(0.5); !almostEqual(got.X, 4) {
		t.Errorf("RenderPosition(0.5).X = %v, want 4", got.X)
	}
	if got := pc.RenderPosition(1); !almostEqual(got.X, 8) {
		t.Errorf("RenderPosition(1).X = %v, want 8 (current step)", got.X)
	}
}

func TestTicksPerNetworkSendMatchesNetworkTickRate(t *testing.T) {
	// The client sends once every ticksPerNetworkSend sim steps; that must
	// still add up to shared.NetworkTickRate, since TimeMs reported to the
	// server's speed check is derived from it.
	got := (time.Duration(ticksPerNetworkSend) * simStep).Round(time.Millisecond)
	if want := networkTick; got != want {
		t.Errorf("ticksPerNetworkSend * simStep = %v, want %v", got, want)
	}
}
