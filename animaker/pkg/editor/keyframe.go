package editor

import (
	"errors"
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

// ErrKeyframeTimeTaken is returned when a keyframe would be moved onto a
// time another keyframe of the same part already occupies.
var ErrKeyframeTimeTaken = errors.New("another keyframe of this part is already at that time")

// DuplicateKeyframe copies a part's keyframe at idx to newTimeMs and
// returns the keyframe now holding that pose. If the part already has a
// keyframe at newTimeMs, the pose is pasted onto it rather than inserting a
// second one at the same instant: two keyframes sharing a time give
// ValueAt a zero-length segment and make "the keyframe at the playhead"
// ambiguous, which the canvas drag and the selection both depend on.
// Duplicating onto the source's own time is a no-op that returns the
// source.
func DuplicateKeyframe(dir *Direction, partID, idx int, newTimeMs uint32) (*Keyframe, error) {
	kfs := dir.KeyframesFor(partID)
	if idx < 0 || idx >= len(kfs) {
		return nil, fmt.Errorf("invalid keyframe index: %d", idx)
	}
	src := kfs[idx]
	if src.TimeMs == newTimeMs {
		return src, nil
	}
	// Taken before AddKeyframe, whose insert may reallocate the slice.
	pose := *src
	dst := AddKeyframe(dir, partID, newTimeMs)
	dst.X, dst.Y, dst.Z = pose.X, pose.Y, pose.Z
	dst.RotationDeg = pose.RotationDeg
	dst.Row, dst.Col = pose.Row, pose.Col
	dst.Direction = pose.Direction
	return dst, nil
}

// MoveKeyframe changes the time of a part's keyframe at idx, refusing with
// ErrKeyframeTimeTaken if another keyframe of the same part is already at
// newTimeMs. The keyframes are re-sorted afterwards, so the moved
// keyframe's index can change — callers tracking it should hold the
// *Keyframe and read its ID, not keep using idx.
func MoveKeyframe(dir *Direction, partID, idx int, newTimeMs uint32) error {
	kfs := dir.KeyframesFor(partID)
	if idx < 0 || idx >= len(kfs) {
		return fmt.Errorf("invalid keyframe index: %d", idx)
	}
	for i, kf := range kfs {
		if i != idx && kf.TimeMs == newTimeMs {
			return ErrKeyframeTimeTaken
		}
	}
	kfs[idx].TimeMs = newTimeMs
	normalizeKeyframes(dir, partID)
	return nil
}

// EnsureKeyframe returns a part's keyframe at timeMs in this direction,
// creating one if there isn't one. A created keyframe is seeded from the
// part's interpolated pose at that moment, so it starts as a continuation
// of the animation rather than snapping to the origin; an existing one is
// returned untouched. created reports which happened.
func EnsureKeyframe(dir *Direction, partID int, timeMs uint32) (kf *Keyframe, created bool) {
	for _, existing := range dir.KeyframesFor(partID) {
		if existing.TimeMs == timeMs {
			return existing, false
		}
	}
	// Resolved before inserting, while the surrounding keyframes still
	// describe the pose being continued.
	seed := dir.ValueAt(partID, timeMs)
	kf = AddKeyframe(dir, partID, timeMs)
	kf.X, kf.Y, kf.Z = seed.X, seed.Y, seed.Z
	kf.RotationDeg = seed.RotationDeg
	kf.Row, kf.Col = seed.Row, seed.Col
	kf.Direction = seed.Direction
	return kf, true
}

// DuplicateOffsetMs is how far after the source "Duplicate Keyframe" puts
// the copy when the playhead is sitting on the source itself.
const DuplicateOffsetMs = 100

// DuplicateTargetMs picks where "Duplicate Keyframe" puts its copy: at the
// playhead, since that's where the artist has scrubbed to — or, when the
// playhead is on the source keyframe, DuplicateOffsetMs after it, since a
// copy at the source's own time would be a no-op.
func DuplicateTargetMs(sourceMs, playheadMs uint32) uint32 {
	if playheadMs == sourceMs {
		return sourceMs + DuplicateOffsetMs
	}
	return playheadMs
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
	Direction   int // NestedAni per-keyframe direction; stepped like Row/Col
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
		// Exactly on a keyframe resolves to that keyframe outright. Folding
		// equality into the segment test below (timeMs <= b.TimeMs) lands
		// on t=1, which gets X/Y/Z right but takes Row/Col from the
		// *previous* keyframe, since cells step rather than blend — so a
		// playhead parked on a middle keyframe (exactly where clicking its
		// marker puts it) drew the wrong cell.
		if timeMs == b.TimeMs {
			return kfToResolved(b)
		}
		if timeMs < b.TimeMs {
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
				Direction:   a.Direction,
			}
		}
	}
	return kfToResolved(last)
}

func kfToResolved(kf *Keyframe) ResolvedTransform {
	return ResolvedTransform{X: kf.X, Y: kf.Y, Z: kf.Z, RotationDeg: kf.RotationDeg, Row: kf.Row, Col: kf.Col, Direction: kf.Direction}
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}
