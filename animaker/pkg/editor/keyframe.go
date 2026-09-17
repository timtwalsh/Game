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

// AddKeyframe inserts a keyframe into a part at the given time, keeping
// Keyframes sorted by TimeMs. If one already exists at that exact time,
// it's replaced rather than duplicated.
func AddKeyframe(part *Part, timeMs uint32) *Keyframe {
	for _, kf := range part.Keyframes {
		if kf.TimeMs == timeMs {
			return kf
		}
	}
	kf := NewKeyframe(timeMs)
	part.Keyframes = append(part.Keyframes, kf)
	sortKeyframes(part)
	reindexKeyframes(part)
	return kf
}

// DeleteKeyframe removes the keyframe at idx from a part.
func DeleteKeyframe(part *Part, idx int) error {
	if idx < 0 || idx >= len(part.Keyframes) {
		return fmt.Errorf("invalid keyframe index: %d", idx)
	}
	part.Keyframes = append(part.Keyframes[:idx], part.Keyframes[idx+1:]...)
	reindexKeyframes(part)
	return nil
}

// DuplicateKeyframe clones the keyframe at idx to a new time and inserts it.
func DuplicateKeyframe(part *Part, idx int, newTimeMs uint32) (*Keyframe, error) {
	if idx < 0 || idx >= len(part.Keyframes) {
		return nil, fmt.Errorf("invalid keyframe index: %d", idx)
	}
	nkf := part.Keyframes[idx].Clone()
	nkf.TimeMs = newTimeMs
	part.Keyframes = append(part.Keyframes, nkf)
	sortKeyframes(part)
	reindexKeyframes(part)
	return nkf, nil
}

// MoveKeyframe changes the time of the keyframe at idx.
func MoveKeyframe(part *Part, idx int, newTimeMs uint32) error {
	if idx < 0 || idx >= len(part.Keyframes) {
		return fmt.Errorf("invalid keyframe index: %d", idx)
	}
	part.Keyframes[idx].TimeMs = newTimeMs
	sortKeyframes(part)
	reindexKeyframes(part)
	return nil
}

func sortKeyframes(part *Part) {
	sort.Slice(part.Keyframes, func(i, j int) bool {
		return part.Keyframes[i].TimeMs < part.Keyframes[j].TimeMs
	})
}

func reindexKeyframes(part *Part) {
	for i, kf := range part.Keyframes {
		kf.ID = i
	}
}

// ResolvedTransform is a part's interpolated placement at a point in time.
type ResolvedTransform struct {
	X, Y, Z     float32
	RotationDeg float32
	Row, Col    int // Sheet kind only; step function, not interpolated
}

// ValueAt returns the part's interpolated transform at timeMs, linearly
// interpolating X/Y/Z/Rotation between the two surrounding keyframes.
// Row/Col hold at the earlier keyframe's value until the next keyframe is
// reached (a step function — cells aren't blended).
func (p *Part) ValueAt(timeMs uint32) ResolvedTransform {
	if len(p.Keyframes) == 0 {
		return ResolvedTransform{}
	}
	first := p.Keyframes[0]
	if len(p.Keyframes) == 1 || timeMs <= first.TimeMs {
		return kfToResolved(first)
	}
	last := p.Keyframes[len(p.Keyframes)-1]
	if timeMs >= last.TimeMs {
		return kfToResolved(last)
	}
	for i := 1; i < len(p.Keyframes); i++ {
		b := p.Keyframes[i]
		if timeMs <= b.TimeMs {
			a := p.Keyframes[i-1]
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
