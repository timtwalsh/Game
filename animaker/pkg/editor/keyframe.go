package editor

import "fmt"

// KeyFrame represents a single frame in an animation.
type KeyFrame struct {
	ID           int
	Duration     uint32          // milliseconds
	Sprite       SpriteReference // which sprite to display
	Speed        float32         // movement speed modifier (1.0 = normal)
	HitBox       *Box            // collision box (nil if none)
	AttackHitBox *Box            // attack/damage box (nil if none)
	Events       []Event         // sound, particles, shake, flash
	DisplayOffset Point          // for variable sprite sizes
}

// Box represents a rectangle used for hitboxes.
type Box struct {
	X, Y, W, H int
}

// Point represents a 2D coordinate.
type Point struct {
	X, Y int
}

// SpriteReference identifies which sprite to display from a sheet.
type SpriteReference struct {
	SheetName string // e.g. "player_sheet"
	Index     int    // sprite index in grid

	// Fallback if sheet not available
	Absolute *AbsoluteSpriteRef
}

// AbsoluteSpriteRef directly references a region in an image file.
type AbsoluteSpriteRef struct {
	FilePath    string
	X, Y, W, H int
}

// NewKeyFrame creates a keyframe with default values.
func NewKeyFrame(id int) *KeyFrame {
	return &KeyFrame{
		ID:       id,
		Duration: 100, // 100ms default
		Speed:    1.0,
		Events:   []Event{},
	}
}

// Clone creates a deep copy of the keyframe with a new ID.
func (kf *KeyFrame) Clone(newID int) *KeyFrame {
	newKF := &KeyFrame{
		ID:            newID,
		Duration:      kf.Duration,
		Sprite:        kf.Sprite,
		Speed:         kf.Speed,
		DisplayOffset: kf.DisplayOffset,
	}

	// Deep copy hitboxes
	if kf.HitBox != nil {
		newKF.HitBox = &Box{kf.HitBox.X, kf.HitBox.Y, kf.HitBox.W, kf.HitBox.H}
	}
	if kf.AttackHitBox != nil {
		newKF.AttackHitBox = &Box{kf.AttackHitBox.X, kf.AttackHitBox.Y, kf.AttackHitBox.W, kf.AttackHitBox.H}
	}

	// Deep copy events
	newKF.Events = make([]Event, len(kf.Events))
	for i, e := range kf.Events {
		newKF.Events[i] = CopyEvent(e)
	}

	// Deep copy absolute sprite ref
	if kf.Sprite.Absolute != nil {
		abs := *kf.Sprite.Absolute
		newKF.Sprite.Absolute = &abs
	}

	return newKF
}

// AddKeyFrame inserts a new keyframe after the given index. If afterIdx is -1,
// the frame is appended at the end. Returns the new keyframe.
func AddKeyFrame(anim *Animation, afterIdx int) *KeyFrame {
	newID := len(anim.KeyFrames)
	kf := NewKeyFrame(newID)

	if afterIdx < 0 || afterIdx >= len(anim.KeyFrames)-1 {
		// Append at end
		anim.KeyFrames = append(anim.KeyFrames, kf)
	} else {
		// Insert after afterIdx
		pos := afterIdx + 1
		anim.KeyFrames = append(anim.KeyFrames, nil)
		copy(anim.KeyFrames[pos+1:], anim.KeyFrames[pos:])
		anim.KeyFrames[pos] = kf
		// Re-index
		reindexKeyFrames(anim)
	}

	return kf
}

// DeleteKeyFrame removes the keyframe at the given index.
func DeleteKeyFrame(anim *Animation, idx int) error {
	if idx < 0 || idx >= len(anim.KeyFrames) {
		return fmt.Errorf("invalid keyframe index: %d", idx)
	}

	anim.KeyFrames = append(anim.KeyFrames[:idx], anim.KeyFrames[idx+1:]...)
	reindexKeyFrames(anim)
	return nil
}

// DuplicateKeyFrame clones the keyframe at idx and inserts it after.
func DuplicateKeyFrame(anim *Animation, idx int) (*KeyFrame, error) {
	if idx < 0 || idx >= len(anim.KeyFrames) {
		return nil, fmt.Errorf("invalid keyframe index: %d", idx)
	}

	src := anim.KeyFrames[idx]
	newKF := src.Clone(len(anim.KeyFrames))

	// Insert after source
	pos := idx + 1
	anim.KeyFrames = append(anim.KeyFrames, nil)
	copy(anim.KeyFrames[pos+1:], anim.KeyFrames[pos:])
	anim.KeyFrames[pos] = newKF

	reindexKeyFrames(anim)
	return newKF, nil
}

// MoveKeyFrame moves the keyframe at fromIdx to toIdx.
func MoveKeyFrame(anim *Animation, fromIdx, toIdx int) error {
	if fromIdx < 0 || fromIdx >= len(anim.KeyFrames) {
		return fmt.Errorf("invalid source index: %d", fromIdx)
	}
	if toIdx < 0 || toIdx >= len(anim.KeyFrames) {
		return fmt.Errorf("invalid target index: %d", toIdx)
	}
	if fromIdx == toIdx {
		return nil
	}

	kf := anim.KeyFrames[fromIdx]
	// Remove from old position
	anim.KeyFrames = append(anim.KeyFrames[:fromIdx], anim.KeyFrames[fromIdx+1:]...)
	// Insert at new position
	if toIdx > fromIdx {
		toIdx-- // Adjust for removal
	}
	anim.KeyFrames = append(anim.KeyFrames, nil)
	copy(anim.KeyFrames[toIdx+1:], anim.KeyFrames[toIdx:])
	anim.KeyFrames[toIdx] = kf

	reindexKeyFrames(anim)
	return nil
}

// reindexKeyFrames updates all keyframe IDs to match their position.
func reindexKeyFrames(anim *Animation) {
	for i, kf := range anim.KeyFrames {
		kf.ID = i
	}
}
