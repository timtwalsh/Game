package main

import (
	"game/shared"
	"math"
)

type MovementType int

const (
	MovementTypeWalk MovementType = iota
	MovementTypeTeleport
	MovementTypeKnockedBack
	MovementTypeJump
)

type WallPhaseResult struct {
	Clean   bool
	Cheated bool
	TileX   uint32
	TileY   uint32
}

type MovementValidation struct {
	Valid     bool
	TooFast   bool
	Speed     float32
	WallPhase bool
	TileX     uint32
	TileY     uint32
}

type MovementValidator struct {
	Collision shared.CollisionLayer
}

func NewMovementValidator(collision shared.CollisionLayer) MovementValidator {
	return MovementValidator{Collision: collision}
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
	
	// Basic line drawing approximation
	fromX := int(from.X / shared.TileSize)
	fromY := int(from.Y / shared.TileSize)
	toX := int(to.X / shared.TileSize)
	toY := int(to.Y / shared.TileSize)
	
	dx := math.Abs(float64(toX - fromX))
	dy := math.Abs(float64(toY - fromY))
	sx, sy := -1, -1
	if toX > fromX { sx = 1 }
	if toY > fromY { sy = 1 }
	
	err := dx - dy
	x, y := fromX, fromY
	
	for {
		if x >= 0 && y >= 0 {
			if mv.Collision.IsBlocked(uint32(x), uint32(y)) {
				// Simple cheat check: could they go around?
				directDist := from.DistanceTo(to)
				
				// Estimate detour
				rightDist := directDist + shared.TileSize // Simplified
				timeSec := float32(timeMs) / 1000.0
				speedForDetour := rightDist / timeSec
				
				if speedForDetour > shared.MaxSpeed*shared.TileSize {
					return WallPhaseResult{Clean: false, Cheated: true, TileX: uint32(x), TileY: uint32(y)}
				}
				return WallPhaseResult{Clean: false, Cheated: false, TileX: uint32(x), TileY: uint32(y)}
			}
		}
		
		if x == toX && y == toY { break }
		e2 := 2 * err
		if e2 > -dy { err -= dy; x += sx }
		if e2 < dx { err += dx; y += sy }
	}
	
	return WallPhaseResult{Clean: true}
}

func (mv *MovementValidator) ValidateMovement(from, to shared.Vec2, timeMs uint32, mType MovementType) []MovementValidation {
	var issues []MovementValidation
	
	speed := mv.CheckSpeed(from, to, timeMs)
	if speed > shared.MaxSpeed*shared.TileSize*shared.SpeedTolerance {
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
