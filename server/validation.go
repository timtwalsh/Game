package main

import (
	"game/shared"
	"game/shared/world"
	"math"
)

type MovementType int

const (
	MovementTypeWalk MovementType = iota
	MovementTypeTeleport
	MovementTypeKnockedBack
	MovementTypeJump
)

// WallPhaseResult's tile is in signed world tile coordinates.
type WallPhaseResult struct {
	Clean   bool
	Cheated bool
	TileX   int
	TileY   int
}

type MovementValidation struct {
	Valid     bool
	TooFast   bool
	Speed     float32
	WallPhase bool
	TileX     int
	TileY     int
}

// MovementValidator checks moves against the server's own copy of the map
// (D32): blocking and speed multipliers come from the Collider, never from
// the client.
type MovementValidator struct {
	Collision world.Collider
}

func NewMovementValidator(collision world.Collider) MovementValidator {
	return MovementValidator{Collision: collision}
}

// blockFlag is the blocking flag a movement type must not cross: walking
// tests ground, a jump tests jump (so it clears a fence that blocks only
// ground and roll).
func blockFlag(mType MovementType) uint16 {
	if mType == MovementTypeJump {
		return world.BlockJump
	}
	return world.BlockGround
}

// MaxSpeedBetween is the fastest an honest player may move from one
// position to another, in px/s: walking speed times the larger speed
// multiplier of the start and end tiles. Taking the larger one avoids
// flagging someone who stepped out of water onto grass, where the client
// switched to full speed partway through the move.
func (mv *MovementValidator) MaxSpeedBetween(from, to shared.Vec2) float32 {
	m := max(
		mv.Collision.SpeedMultiplier(world.TileOf(from.X), world.TileOf(from.Y)),
		mv.Collision.SpeedMultiplier(world.TileOf(to.X), world.TileOf(to.Y)),
	)
	return shared.MaxSpeed * shared.TileSize * m
}

func (mv *MovementValidator) CheckSpeed(from, to shared.Vec2, timeMs uint32) float32 {
	if timeMs == 0 {
		return math.MaxFloat32
	}
	dist := from.DistanceTo(to)
	timeSec := float32(timeMs) / 1000.0
	return dist / timeSec
}

func (mv *MovementValidator) CheckWallPhase(from, to shared.Vec2, timeMs uint32, mType MovementType) WallPhaseResult {
	if mType == MovementTypeTeleport || mType == MovementTypeKnockedBack {
		return WallPhaseResult{Clean: true}
	}

	// Basic line drawing approximation, in signed world tiles: floor, not a
	// truncating cast, so -0.5 px is tile -1 rather than tile 0.
	fromX, fromY := world.TileOf(from.X), world.TileOf(from.Y)
	toX, toY := world.TileOf(to.X), world.TileOf(to.Y)
	flag := blockFlag(mType)

	dx := math.Abs(float64(toX - fromX))
	dy := math.Abs(float64(toY - fromY))
	sx, sy := -1, -1
	if toX > fromX {
		sx = 1
	}
	if toY > fromY {
		sy = 1
	}

	err := dx - dy
	x, y := fromX, fromY

	for {
		if mv.Collision.Blocks(x, y, flag) {
			// Simple cheat check: could they go around?
			directDist := from.DistanceTo(to)

			// Estimate detour
			rightDist := directDist + shared.TileSize // Simplified
			timeSec := float32(timeMs) / 1000.0
			speedForDetour := rightDist / timeSec

			if speedForDetour > mv.MaxSpeedBetween(from, to) {
				return WallPhaseResult{Clean: false, Cheated: true, TileX: x, TileY: y}
			}
			return WallPhaseResult{Clean: false, Cheated: false, TileX: x, TileY: y}
		}

		if x == toX && y == toY {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}

	return WallPhaseResult{Clean: true}
}

func (mv *MovementValidator) ValidateMovement(from, to shared.Vec2, timeMs uint32, mType MovementType) []MovementValidation {
	var issues []MovementValidation

	speed := mv.CheckSpeed(from, to, timeMs)
	if speed > mv.MaxSpeedBetween(from, to)*shared.SpeedTolerance {
		issues = append(issues, MovementValidation{TooFast: true, Speed: speed})
	}

	wp := mv.CheckWallPhase(from, to, timeMs, mType)
	if wp.Cheated || !wp.Clean {
		issues = append(issues, MovementValidation{WallPhase: true, TileX: wp.TileX, TileY: wp.TileY})
	}

	return issues
}

type SuspicionStatus int

const (
	SuspicionStatusNormal SuspicionStatus = iota
	SuspicionStatusFullLogging
	SuspicionStatusFlagForReview
	SuspicionStatusAutoBan
)

type SuspicionTracker struct {
	PlayerID    uint64
	Score       float32
	Events      []shared.SuspicionEvent
	ReportCount uint32
}

func NewSuspicionTracker(id uint64) *SuspicionTracker {
	return &SuspicionTracker{
		PlayerID: id,
		Score:    0.0,
		Events:   make([]shared.SuspicionEvent, 0),
	}
}

func (st *SuspicionTracker) AddEvent(event shared.SuspicionEvent) {
	var weight float32
	switch event.Type {
	case shared.SuspicionEventTooFast:
		weight = shared.SuspicionSpeedHack
	case shared.SuspicionEventWallPhase:
		weight = shared.SuspicionWallPhase
	case shared.SuspicionEventPlayerReport:
		weight = shared.SuspicionPlayerReport
	}
	st.Score += weight
	st.Events = append(st.Events, event)
}

func (st *SuspicionTracker) GetStatus() SuspicionStatus {
	if st.Score >= shared.SuspicionAutoBan {
		return SuspicionStatusAutoBan
	} else if st.Score >= shared.SuspicionFlagReview {
		return SuspicionStatusFlagForReview
	} else if st.Score >= shared.SuspicionEnableLogging {
		return SuspicionStatusFullLogging
	}
	return SuspicionStatusNormal
}
