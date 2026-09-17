package main

import (
	"game/shared"
	"math"
)

type PlayerInput struct {
	Up       bool
	Down     bool
	Left     bool
	Right    bool
	Attack   bool
	Interact bool
}

func (pi *PlayerInput) GetMovementVector() shared.Vec2 {
	var x, y float32
	if pi.Up { y -= 1.0 }
	if pi.Down { y += 1.0 }
	if pi.Left { x -= 1.0 }
	if pi.Right { x += 1.0 }
	
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
	if pi.Up { dy -= 1.0 }
	if pi.Down { dy += 1.0 }
	if pi.Left { dx -= 1.0 }
	if pi.Right { dx += 1.0 }
	
	if dx == 0 && dy == 0 {
		return 255 // No movement
	}
	
	angle := math.Atan2(dy, dx)
	// Convert -PI..PI to 0..2PI
	if angle < 0 {
		angle += 2 * math.Pi
	}
	
	// Add Pi/8 to rotate the sectors so that East is centered at 0
	octant := int(math.Floor((angle + math.Pi/8) / (math.Pi / 4))) % 8
	
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
	Collision         shared.CollisionLayer
}

func NewPlayerController(pos shared.Vec2, collision shared.CollisionLayer) *PlayerController {
	return &PlayerController{
		Position:          pos,
		PredictedPosition: pos,
		LastPosition:      pos,
		Direction:         4, // South by default
		Collision:         collision,
	}
}

func (pc *PlayerController) UpdatePrediction(input *PlayerInput, deltaMs uint32) {
	pc.LastPosition = pc.PredictedPosition
	deltaSec := float32(deltaMs) / 1000.0
	movement := input.GetMovementVector()
	
	speed := shared.MaxSpeed * shared.TileSize
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

func (pc *PlayerController) CanMoveTo(x, y float32) bool {
	tileX := uint32(x / shared.TileSize)
	tileY := uint32(y / shared.TileSize)
	return !pc.Collision.IsBlocked(tileX, tileY)
}

func (pc *PlayerController) ServerCorrection(serverPos shared.Vec2) {
	pc.Position = serverPos
}

type PlayerInterpolation struct {
	TargetPosition        shared.Vec2
	CurrentPosition       shared.Vec2
	LastPosition          shared.Vec2
	InterpolationTime     uint32
	InterpolationDuration uint32
	Direction             uint8
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
		InterpolationTime:     shared.NetworkTickRate,
		InterpolationDuration: shared.NetworkTickRate,
	}
}

// interpolationDurationMin/Max bound the adaptive duration ServerUpdate
// derives from real inter-update spacing, so one very short or very long
// gap (jitter, a dropped packet) doesn't produce an unplayable snap or an
// unplayably slow crawl for the next segment.
const (
	interpolationDurationMin = shared.NetworkTickRate / 2
	interpolationDurationMax = shared.NetworkTickRate * 4
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
	pi.ColorR = state.ColorR
	pi.ColorG = state.ColorG
	pi.ColorB = state.ColorB
}

func (pi *PlayerInterpolation) Update(deltaMs uint32) {
	// InterpolationTime accumulates unconditionally, even past
	// InterpolationDuration (i.e. even once we've visually "arrived") -
	// ServerUpdate reads it to measure the real elapsed time since the
	// previous update. Capping it at InterpolationDuration here would hide
	// how late a delayed update actually was.
	pi.InterpolationTime += deltaMs
	if pi.InterpolationTime >= pi.InterpolationDuration {
		pi.CurrentPosition = pi.TargetPosition
	} else {
		t := float32(pi.InterpolationTime) / float32(pi.InterpolationDuration)
		pi.CurrentPosition.X = pi.LastPosition.X + (pi.TargetPosition.X - pi.LastPosition.X)*t
		pi.CurrentPosition.Y = pi.LastPosition.Y + (pi.TargetPosition.Y - pi.LastPosition.Y)*t
	}
}
