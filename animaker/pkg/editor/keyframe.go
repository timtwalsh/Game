package editor

import (
	"fmt"
	"sort"
)

// NewKeyframe creates a keyframe with default values at the given time.
func NewKeyframe(timeMs uint32) *Keyframe {
	return &Keyframe{TimeMs: timeMs}
}

// Clone creates a deep copy of a keyframe (it has no pointer fields, so
// this is just a value copy, but kept as a method for call-site clarity
// and to insulate callers from that implementation detail).
func (kf *Keyframe) Clone() *Keyframe {
	c := *kf
	return &c
}

// AddKeyframe inserts a keyframe for a part in one direction, keeping that
// part's keyframes sorted by TimeMs. If one already exists at that exact
// time it's returned rather than duplicated.
func AddKeyframe(dir *Direction, partID int, timeMs uint32) *Keyframe {
	if dir.Keyframes == nil {
		dir.Keyframes = map[int][]*Keyframe{}
	}
	for _, kf := range dir.Keyframes[partID] {
		if kf.TimeMs == timeMs {
			return kf
		}
	}
	kf := NewKeyframe(timeMs)
	dir.Keyframes[partID] = append(dir.Keyframes[partID], kf)
	normalizeKeyframes(dir, partID)
	return kf
}

// DeleteKeyframe removes a part's keyframe at idx in one direction.
func DeleteKeyframe(dir *Direction, partID, idx int) error {
	kfs := dir.KeyframesFor(partID)
	if idx < 0 || idx >= len(kfs) {
		return fmt.Errorf("invalid keyframe index: %d", idx)
	}
	dir.Keyframes[partID] = append(kfs[:idx], kfs[idx+1:]...)
	normalizeKeyframes(dir, partID)
	return nil
}

// DuplicateKeyframe clones a part's keyframe at idx to a new time.
func DuplicateKeyframe(dir *Direction, partID, idx int, newTimeMs uint32) (*Keyframe, error) {
	kfs := dir.KeyframesFor(partID)
	if idx < 0 || idx >= len(kfs) {
		return nil, fmt.Errorf("invalid keyframe index: %d", idx)
	}
	nkf := kfs[idx].Clone()
	nkf.TimeMs = newTimeMs
	dir.Keyframes[partID] = append(kfs, nkf)
	normalizeKeyframes(dir, partID)
	return nkf, nil
}

// MoveKeyframe changes the time of a part's keyframe at idx.
func MoveKeyframe(dir *Direction, partID, idx int, newTimeMs uint32) error {
	kfs := dir.KeyframesFor(partID)
	if idx < 0 || idx >= len(kfs) {
		return fmt.Errorf("invalid keyframe index: %d", idx)
	}
	kfs[idx].TimeMs = newTimeMs
	normalizeKeyframes(dir, partID)
	return nil
}

// normalizeKeyframes re-sorts a part's keyframes by time and renumbers
// their IDs to match, so a Keyframe.ID is always its index in the slice.
func normalizeKeyframes(dir *Direction, partID int) {
	kfs := dir.Keyframes[partID]
	sort.Slice(kfs, func(i, j int) bool { return kfs[i].TimeMs < kfs[j].TimeMs })
	for i, kf := range kfs {
		kf.ID = i
	}
}

// ResolvedTransform is a part's interpolated placement at a point in time.
type ResolvedTransform struct {
	X, Y, Z     float32
	RotationDeg float32
	Row, Col    int // Sheet kind only; step function, not interpolated
}

// ValueAt returns a part's interpolated transform at timeMs in this
// direction, linearly interpolating X/Y/Z/Rotation between the two
// surrounding keyframes. Row/Col hold at the earlier keyframe's value until
// the next keyframe is reached (a step function — cells aren't blended).
// A part with no keyframes in this direction resolves to the zero value;
// the canvas skips drawing it.
func (d *Direction) ValueAt(partID int, timeMs uint32) ResolvedTransform {
	kfs := d.KeyframesFor(partID)
	if len(kfs) == 0 {
		return ResolvedTransform{}
	}
	first := kfs[0]
	if len(kfs) == 1 || timeMs <= first.TimeMs {
		return kfToResolved(first)
	}
	last := kfs[len(kfs)-1]
	if timeMs >= last.TimeMs {
		return kfToResolved(last)
	}
	for i := 1; i < len(kfs); i++ {
		b := kfs[i]
		if timeMs <= b.TimeMs {
			a := kfs[i-1]
			var t float32
			if span := float32(b.TimeMs - a.TimeMs); span > 0 {
				t = float32(timeMs-a.TimeMs) / span
			}
			return ResolvedTransform{
				X:           lerp(a.X, b.X, t),
				Y:           lerp(a.Y, b.Y, t),
				Z:           lerp(a.Z, b.Z, t),
				RotationDeg: lerp(a.RotationDeg, b.RotationDeg, t),
				Row:         a.Row,
				Col:         a.Col,
			}
		}
	}
	return kfToResolved(last)
}

func kfToResolved(kf *Keyframe) ResolvedTransform {
	return ResolvedTransform{X: kf.X, Y: kf.Y, Z: kf.Z, RotationDeg: kf.RotationDeg, Row: kf.Row, Col: kf.Col}
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}
