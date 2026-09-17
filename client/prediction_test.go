package main

import (
	"game/shared"
	"math"
	"testing"
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
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, shared.NewCollisionLayer(10, 10))
	input := PlayerInput{Right: true}

	pc.UpdatePrediction(&input, 100) // 100ms

	// speed = MaxSpeed*TileSize = 80 units/sec; 100ms -> 8 units.
	if !almostEqual(pc.PredictedPosition.X, 8) {
		t.Errorf("PredictedPosition.X = %v, want 8", pc.PredictedPosition.X)
	}
	if pc.Direction != 2 { // East
		t.Errorf("Direction = %d, want 2 (East)", pc.Direction)
	}
}

func TestPlayerControllerStopsAtWall(t *testing.T) {
	collision := shared.NewCollisionLayer(10, 10)
	collision.Set(2, 0, shared.TileTypeBlocked) // covers x in [32,48)

	pc := NewPlayerController(shared.Vec2{X: 20, Y: 0}, collision)
	input := PlayerInput{Right: true}

	// First step: 20 -> 28, still inside tile 1 (walkable).
	pc.UpdatePrediction(&input, 100)
	if !almostEqual(pc.PredictedPosition.X, 28) {
		t.Fatalf("after first step, PredictedPosition.X = %v, want 28", pc.PredictedPosition.X)
	}

	// Second step would land at 36, inside the blocked tile 2 - must be stopped.
	pc.UpdatePrediction(&input, 100)
	if !almostEqual(pc.PredictedPosition.X, 28) {
		t.Errorf("after moving into a wall, PredictedPosition.X = %v, want unchanged at 28", pc.PredictedPosition.X)
	}
}

func TestPlayerControllerServerCorrection(t *testing.T) {
	pc := NewPlayerController(shared.Vec2{X: 0, Y: 0}, shared.NewCollisionLayer(10, 10))
	pc.ServerCorrection(shared.Vec2{X: 42, Y: 7})

	if pc.Position.X != 42 || pc.Position.Y != 7 {
		t.Errorf("Position after ServerCorrection = %+v, want {42 7}", pc.Position)
	}
}

func TestPlayerInterpolationEasesThenSnaps(t *testing.T) {
	pi := NewPlayerInterpolation(shared.Vec2{X: 0, Y: 0})
	pi.ServerUpdate(shared.PlayerState{Position: shared.Vec2{X: 100, Y: 0}})

	pi.Update(50) // half of the 100ms InterpolationDuration
	if !almostEqual(pi.CurrentPosition.X, 50) {
		t.Errorf("halfway through interpolation, CurrentPosition.X = %v, want 50", pi.CurrentPosition.X)
	}

	pi.Update(50) // remaining half
	if !almostEqual(pi.CurrentPosition.X, 100) {
		t.Errorf("after full interpolation window, CurrentPosition.X = %v, want 100 (snapped to target)", pi.CurrentPosition.X)
	}
}
