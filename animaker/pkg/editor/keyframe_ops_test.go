package editor

import (
	"errors"
	"testing"
)

// threeKeyframes builds one part keyed at 0, 250 and 450ms with distinct
// cells and positions, so a test can tell exactly which keyframe a
// resolved value came from.
func threeKeyframes() (*Direction, int) {
	dir := NewDirection()
	const partID = 1
	for i, ms := range []uint32{0, 250, 450} {
		kf := AddKeyframe(dir, partID, ms)
		kf.X = float32(i * 10)
		kf.Row, kf.Col = i, i
	}
	return dir, partID
}

// Regression: a playhead sitting exactly on a *middle* keyframe resolved
// to the previous keyframe's cell. The segment test treated equality as
// t=1 of the preceding segment, which gets position right but takes
// Row/Col from the segment's start, since cells step rather than blend.
// Clicking a timeline marker seeks exactly onto its keyframe, so this is
// the case the editor hits constantly.
func TestValueAtExactlyOnAMiddleKeyframeUsesThatKeyframesCell(t *testing.T) {
	dir, partID := threeKeyframes()

	got := dir.ValueAt(partID, 250)
	if got.Row != 1 || got.Col != 1 {
		t.Errorf("at exactly 250ms, cell = (%d,%d), want (1,1) - the keyframe at 250ms, not the one before it", got.Row, got.Col)
	}
	if got.X != 10 {
		t.Errorf("at exactly 250ms, X = %v, want 10", got.X)
	}

	// Between keyframes the step function still holds the earlier cell.
	if mid := dir.ValueAt(partID, 300); mid.Row != 1 {
		t.Errorf("at 300ms, Row = %d, want 1 (held from 250ms until 450ms)", mid.Row)
	}
}

func TestMoveKeyframeReordersAndTheKeyframeTracksItsNewIndex(t *testing.T) {
	dir, partID := threeKeyframes()
	moving := dir.KeyframesFor(partID)[0] // the 0ms keyframe, Row 0

	if err := MoveKeyframe(dir, partID, moving.Index, 300); err != nil {
		t.Fatalf("MoveKeyframe: %v", err)
	}

	kfs := dir.KeyframesFor(partID)
	want := []uint32{250, 300, 450}
	for i, w := range want {
		if kfs[i].TimeMs != w {
			t.Errorf("keyframe[%d] at %dms, want %dms (not re-sorted)", i, kfs[i].TimeMs, w)
		}
	}
	// A retime drag holds the *Keyframe and reads its ID after every move,
	// so ID must follow the keyframe to its new slot.
	if moving.Index != 1 || kfs[moving.Index] != moving {
		t.Errorf("moved keyframe has ID %d; kfs[ID] is not it - selection would point at the wrong keyframe", moving.Index)
	}
}

// Two keyframes of one part at the same instant give ValueAt a
// zero-length segment and make "the keyframe at the playhead" ambiguous,
// so a move onto an occupied time is refused and changes nothing.
func TestMoveKeyframeOntoAnOccupiedTimeIsRefused(t *testing.T) {
	dir, partID := threeKeyframes()

	err := MoveKeyframe(dir, partID, 0, 250)
	if !errors.Is(err, ErrKeyframeTimeTaken) {
		t.Fatalf("MoveKeyframe onto 250ms: err = %v, want ErrKeyframeTimeTaken", err)
	}
	kfs := dir.KeyframesFor(partID)
	if len(kfs) != 3 || kfs[0].TimeMs != 0 || kfs[1].TimeMs != 250 {
		t.Errorf("a refused move still changed the keyframes: %v", times(kfs))
	}
}

// Moving a keyframe onto its own current time is not a collision.
func TestMoveKeyframeToItsOwnTimeIsAllowed(t *testing.T) {
	dir, partID := threeKeyframes()
	if err := MoveKeyframe(dir, partID, 1, 250); err != nil {
		t.Errorf("MoveKeyframe to its own time: %v, want nil", err)
	}
}

func TestDuplicateKeyframeOntoFreeTimeInsertsACopy(t *testing.T) {
	dir, partID := threeKeyframes()

	dup, err := DuplicateKeyframe(dir, partID, 1, 350) // copy the 250ms pose
	if err != nil {
		t.Fatalf("DuplicateKeyframe: %v", err)
	}
	if got := len(dir.KeyframesFor(partID)); got != 4 {
		t.Fatalf("part has %d keyframes, want 4", got)
	}
	if dup.TimeMs != 350 || dup.X != 10 || dup.Row != 1 {
		t.Errorf("copy = %+v, want the 250ms pose (X 10, Row 1) at 350ms", *dup)
	}
	if dir.KeyframesFor(partID)[dup.Index] != dup {
		t.Error("returned keyframe's Index doesn't index it")
	}
}

// Onto a time that's already keyed, the pose is pasted over that keyframe
// rather than stacking a second keyframe at the same instant.
func TestDuplicateKeyframeOntoOccupiedTimePastesThePose(t *testing.T) {
	dir, partID := threeKeyframes()

	dup, err := DuplicateKeyframe(dir, partID, 0, 450) // 0ms pose onto 450ms
	if err != nil {
		t.Fatalf("DuplicateKeyframe: %v", err)
	}
	if got := len(dir.KeyframesFor(partID)); got != 3 {
		t.Errorf("part has %d keyframes, want 3 - a duplicate onto an occupied time must not add one", got)
	}
	if dup.TimeMs != 450 || dup.X != 0 || dup.Row != 0 {
		t.Errorf("keyframe at 450ms = %+v, want the 0ms pose pasted onto it", *dup)
	}
}

func TestDuplicateKeyframeOntoItsOwnTimeIsANoOp(t *testing.T) {
	dir, partID := threeKeyframes()
	src := dir.KeyframesFor(partID)[1]

	dup, err := DuplicateKeyframe(dir, partID, 1, 250)
	if err != nil || dup != src {
		t.Errorf("duplicate onto own time = (%p, %v), want the source back and no error", dup, err)
	}
	if got := len(dir.KeyframesFor(partID)); got != 3 {
		t.Errorf("part has %d keyframes, want 3", got)
	}
}

func TestDuplicateTargetMs(t *testing.T) {
	if got := DuplicateTargetMs(250, 400); got != 400 {
		t.Errorf("playhead elsewhere: target = %d, want the playhead (400)", got)
	}
	if got := DuplicateTargetMs(250, 250); got != 250+DuplicateOffsetMs {
		t.Errorf("playhead on source: target = %d, want %d", got, 250+DuplicateOffsetMs)
	}
}

// EnsureKeyframe backs both "New Keyframe" and auto-keying on a canvas
// drag. An existing keyframe must come back untouched: re-seeding it from
// ValueAt would overwrite authored values.
func TestEnsureKeyframeReturnsAnExistingKeyframeUntouched(t *testing.T) {
	dir, partID := threeKeyframes()
	existing := dir.KeyframesFor(partID)[1]
	existing.RotationDeg = 45

	kf, created := EnsureKeyframe(dir, partID, 250)
	if created || kf != existing {
		t.Fatalf("EnsureKeyframe at an existing time: created=%v, same=%v - want the existing one", created, kf == existing)
	}
	if kf.RotationDeg != 45 || kf.Row != 1 {
		t.Errorf("existing keyframe was modified: %+v", *kf)
	}
	if got := len(dir.KeyframesFor(partID)); got != 3 {
		t.Errorf("part has %d keyframes, want 3", got)
	}
}

// A new keyframe continues the animation from where it is at that moment,
// so auto-keying mid-motion doesn't snap the part back to the origin.
func TestEnsureKeyframeSeedsANewKeyframeFromTheInterpolatedPose(t *testing.T) {
	dir, partID := threeKeyframes()

	kf, created := EnsureKeyframe(dir, partID, 125) // halfway from X 0 to X 10
	if !created {
		t.Fatal("EnsureKeyframe at a free time reported it already existed")
	}
	if kf.X != 5 {
		t.Errorf("seeded X = %v, want 5 (interpolated)", kf.X)
	}
	if kf.Row != 0 {
		t.Errorf("seeded Row = %d, want 0 (held from the 0ms keyframe)", kf.Row)
	}
}

func times(kfs []*Keyframe) []uint32 {
	out := make([]uint32, len(kfs))
	for i, kf := range kfs {
		out[i] = kf.TimeMs
	}
	return out
}
