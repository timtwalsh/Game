package main

import (
	"game/shared"
	"game/shared/world"
	"math"
	"time"
)

type PlayerInput struct {
	Up       bool
	Down     bool
	Left     bool
	Right    bool
	Attack   bool
	Interact bool
	Jump     bool // pressed this frame (not held)
}

func (pi *PlayerInput) GetMovementVector() shared.Vec2 {
	var x, y float32
	if pi.Up {
		y -= 1.0
	}
	if pi.Down {
		y += 1.0
	}
	if pi.Left {
		x -= 1.0
	}
	if pi.Right {
		x += 1.0
	}

	if x != 0 && y != 0 {
		len := float32(math.Sqrt(float64(x*x + y*y)))
		x /= len
		y /= len
	}
	return shared.Vec2{X: x, Y: y}
}

// GetDirection returns a 0-7 value (0=N, 1=NE, 2=E, 3=SE, 4=S, 5=SW, 6=W, 7=NW)
func (pi *PlayerInput) GetDirection() uint8 {
	var dx, dy float64
	if pi.Up {
		dy -= 1.0
	}
	if pi.Down {
		dy += 1.0
	}
	if pi.Left {
		dx -= 1.0
	}
	if pi.Right {
		dx += 1.0
	}

	if dx == 0 && dy == 0 {
		return 255 // No movement
	}

	angle := math.Atan2(dy, dx)
	// Convert -PI..PI to 0..2PI
	if angle < 0 {
		angle += 2 * math.Pi
	}

	// Add Pi/8 to rotate the sectors so that East is centered at 0
	octant := int(math.Floor((angle+math.Pi/8)/(math.Pi/4))) % 8

	// Map to our 0=N clockwise standard
	// Math: 0=E, 1=SE, 2=S, 3=SW, 4=W, 5=NW, 6=N, 7=NE
	// We want 0=N, 1=NE, 2=E, 3=SE, 4=S, 5=SW, 6=W, 7=NW
	// N is 6 in math, we want 0.
	mapping := []uint8{2, 3, 4, 5, 6, 7, 0, 1}
	return mapping[octant]
}

type PlayerController struct {
	Position          shared.Vec2
	PredictedPosition shared.Vec2
	LastPosition      shared.Vec2
	Velocity          shared.Vec2
	Direction         uint8
	// Collision is the same map the server validates against (loaded from
	// the same files), so prediction only goes where the server agrees.
	Collision world.Collider
}

func NewPlayerController(pos shared.Vec2, collision world.Collider) *PlayerController {
	return &PlayerController{
		Position:          pos,
		PredictedPosition: pos,
		LastPosition:      pos,
		Direction:         4, // South by default
		Collision:         collision,
	}
}

// UpdatePrediction advances the local simulation by one step of length dt.
// The render loop always calls it with the fixed simStep (see client/main.go),
// never with a frame's elapsed time, so movement is independent of frame rate.
// LastPosition keeps the pre-step position so rendering can blend between the
// two (RenderPosition).
func (pc *PlayerController) UpdatePrediction(input *PlayerInput, dt time.Duration) {
	pc.LastPosition = pc.PredictedPosition
	deltaSec := float32(dt.Seconds())
	movement := input.GetMovementVector()

	// The speed multiplier (swimming...) comes from the tile under the
	// position's top-left point - the point the server checks - at the start
	// of the step. The server allows the larger multiplier of a move's two
	// ends, so this never outruns it.
	mult := pc.Collision.SpeedMultiplier(world.TileOf(pc.PredictedPosition.X), world.TileOf(pc.PredictedPosition.Y))
	speed := shared.MaxSpeed * shared.TileSize * mult
	distance := speed * deltaSec

	if movement.X != 0 || movement.Y != 0 {
		pc.Velocity.X = movement.X * speed
		pc.Velocity.Y = movement.Y * speed

		newX := pc.PredictedPosition.X + movement.X*distance
		newY := pc.PredictedPosition.Y + movement.Y*distance

		if pc.CanMoveTo(newX, newY) {
			pc.PredictedPosition.X = newX
			pc.PredictedPosition.Y = newY
		} else {
			// Slide
			if pc.CanMoveTo(newX, pc.PredictedPosition.Y) {
				pc.PredictedPosition.X = newX
			} else if pc.CanMoveTo(pc.PredictedPosition.X, newY) {
				pc.PredictedPosition.Y = newY
			}
		}

		dir := input.GetDirection()
		if dir != 255 {
			pc.Direction = dir
		}
	} else {
		pc.Velocity = shared.Vec2{X: 0, Y: 0}
	}
}

// RenderPosition blends between the previous and current simulation steps;
// alpha is the fraction of a step that has elapsed since the last one ran.
func (pc *PlayerController) RenderPosition(alpha float32) shared.Vec2 {
	return shared.Vec2{
		X: pc.LastPosition.X + (pc.PredictedPosition.X-pc.LastPosition.X)*alpha,
		Y: pc.LastPosition.Y + (pc.PredictedPosition.Y-pc.LastPosition.Y)*alpha,
	}
}

// playerHull is the size of the player's collision box, in pixels. The box
// spans [x, x+playerHull) on each axis from the position, so it contains the
// position itself: the one point the server checks (CheckWallPhase).
// Prediction is therefore never more permissive than the server. It's a
// little smaller than a tile so one-tile gaps don't need pixel-perfect
// lining up.
const playerHull = 12

// CanMoveTo reports whether the player's collision box at (x, y) touches no
// tile that blocks walking.
func (pc *PlayerController) CanMoveTo(x, y float32) bool {
	const inside = playerHull - 1.0/64 // the box's far edge, just inside it
	x0, y0 := world.TileOf(x), world.TileOf(y)
	x1, y1 := world.TileOf(x+inside), world.TileOf(y+inside)
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			if pc.Collision.Blocks(tx, ty, world.BlockGround) {
				return false
			}
		}
	}
	return true
}

func (pc *PlayerController) ServerCorrection(serverPos shared.Vec2) {
	pc.Position = serverPos
}

type PlayerInterpolation struct {
	TargetPosition        shared.Vec2
	CurrentPosition       shared.Vec2
	LastPosition          shared.Vec2
	InterpolationTime     time.Duration
	InterpolationDuration time.Duration
	Direction             uint8
	Animation             uint8
	AnimSeq               uint8
	ColorR                uint8
	ColorG                uint8
	ColorB                uint8
}

func NewPlayerInterpolation(pos shared.Vec2) *PlayerInterpolation {
	return &PlayerInterpolation{
		TargetPosition:  pos,
		CurrentPosition: pos,
		LastPosition:    pos,
		Direction:       4,
		// Seeded so the first ServerUpdate's elapsed-time measurement
		// (see ServerUpdate) reads as exactly one tick, matching
		// InterpolationDuration below, instead of reading as 0 and being
		// clamped down to interpolationDurationMin.
		InterpolationTime:     networkTick,
		InterpolationDuration: networkTick,
	}
}

// interpolationDurationMin/Max bound the adaptive duration ServerUpdate
// derives from real inter-update spacing, so one very short or very long
// gap (jitter, a dropped packet) doesn't produce an unplayable snap or an
// unplayably slow crawl for the next segment.
const (
	networkTick              = time.Duration(shared.NetworkTickRate) * time.Millisecond
	interpolationDurationMin = networkTick / 2
	interpolationDurationMax = networkTick * 4
)

func (pi *PlayerInterpolation) ServerUpdate(state shared.PlayerState) {
	// pi.InterpolationTime, just before we reset it below, holds the real
	// elapsed time since the previous ServerUpdate call. Using that (rather
	// than always assuming exactly shared.NetworkTickRate) matches the
	// interpolation speed to how updates are actually arriving, instead of
	// freezing when they arrive late and snapping too fast to catch up.
	elapsed := pi.InterpolationTime
	if elapsed < interpolationDurationMin {
		elapsed = interpolationDurationMin
	} else if elapsed > interpolationDurationMax {
		elapsed = interpolationDurationMax
	}

	pi.LastPosition = pi.CurrentPosition
	pi.TargetPosition = state.Position
	pi.InterpolationTime = 0
	pi.InterpolationDuration = elapsed
	pi.Direction = state.Direction
	pi.Animation = state.Animation
	pi.AnimSeq = state.AnimSeq
	pi.ColorR = state.ColorR
	pi.ColorG = state.ColorG
	pi.ColorB = state.ColorB
}

// Update advances the visual easing by the real elapsed time dt. This is
// presentation only (remote positions are authored by the server), so it runs
// once per render frame with full-precision time rather than in sim steps.
func (pi *PlayerInterpolation) Update(dt time.Duration) {
	// InterpolationTime accumulates unconditionally, even past
	// InterpolationDuration (i.e. even once we've visually "arrived") -
	// ServerUpdate reads it to measure the real elapsed time since the
	// previous update. Capping it at InterpolationDuration here would hide
	// how late a delayed update actually was.
	pi.InterpolationTime += dt
	if pi.InterpolationTime >= pi.InterpolationDuration {
		pi.CurrentPosition = pi.TargetPosition
	} else {
		t := float32(pi.InterpolationTime) / float32(pi.InterpolationDuration)
		pi.CurrentPosition.X = pi.LastPosition.X + (pi.TargetPosition.X-pi.LastPosition.X)*t
		pi.CurrentPosition.Y = pi.LastPosition.Y + (pi.TargetPosition.Y-pi.LastPosition.Y)*t
	}
}
