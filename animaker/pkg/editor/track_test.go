package editor

import "testing"

func TestValueAtInterpolatesBetweenSurroundingKeyframes(t *testing.T) {
	part := NewSheetPart("Arm_Left", "arms", "")
	part.Keyframes = []*Keyframe{
		{TimeMs: 0, X: 0, Row: 0, Col: 0},
		{TimeMs: 100, X: 100, Row: 1, Col: 2},
	}

	got := part.ValueAt(50)
	if got.X != 50 {
		t.Errorf("ValueAt(50).X = %v, want 50 (halfway)", got.X)
	}
	// Row/Col is a step function - holds the earlier keyframe's cell until
	// the next keyframe's time is reached.
	if got.Row != 0 || got.Col != 0 {
		t.Errorf("ValueAt(50) cell = (%d,%d), want (0,0) - step function shouldn't blend cells", got.Row, got.Col)
	}
}

func TestValueAtClampsBeforeFirstAndAfterLast(t *testing.T) {
	part := NewSheetPart("Body", "", "human_body_default")
	part.Keyframes = []*Keyframe{
		{TimeMs: 50, X: 10},
		{TimeMs: 150, X: 20},
	}

	if got := part.ValueAt(0); got.X != 10 {
		t.Errorf("ValueAt before first keyframe = %v, want clamped to 10", got.X)
	}
	if got := part.ValueAt(9999); got.X != 20 {
		t.Errorf("ValueAt after last keyframe = %v, want clamped to 20", got.X)
	}
}

func TestValueAtEmptyPartReturnsZeroValue(t *testing.T) {
	part := NewSheetPart("Empty", "", "")
	got := part.ValueAt(100)
	if got != (ResolvedTransform{}) {
		t.Errorf("ValueAt on a part with no keyframes = %+v, want zero value", got)
	}
}

func TestAddKeyframeKeepsSortedOrder(t *testing.T) {
	part := NewSheetPart("Head", "", "human_head_default")
	AddKeyframe(part, 200)
	AddKeyframe(part, 0)
	AddKeyframe(part, 100)

	want := []uint32{0, 100, 200}
	if len(part.Keyframes) != len(want) {
		t.Fatalf("got %d keyframes, want %d", len(part.Keyframes), len(want))
	}
	for i, w := range want {
		if part.Keyframes[i].TimeMs != w {
			t.Errorf("keyframe[%d].TimeMs = %v, want %v (not sorted)", i, part.Keyframes[i].TimeMs, w)
		}
		if part.Keyframes[i].ID != i {
			t.Errorf("keyframe[%d].ID = %v, want %v (not reindexed after sort)", i, part.Keyframes[i].ID, i)
		}
	}
}

func TestAddKeyframeAtExistingTimeReturnsExistingNotDuplicate(t *testing.T) {
	part := NewSheetPart("Head", "", "human_head_default")
	first := AddKeyframe(part, 100)
	first.X = 42
	second := AddKeyframe(part, 100)

	if len(part.Keyframes) != 1 {
		t.Fatalf("got %d keyframes, want 1 (re-adding at an existing time shouldn't duplicate)", len(part.Keyframes))
	}
	if second.X != 42 {
		t.Errorf("AddKeyframe at an existing time returned a fresh keyframe instead of the existing one")
	}
}

func TestResolveActiveSheetNamePrecedence(t *testing.T) {
	track := NewTrack("human_walk")
	AddProp(track, "arms", "leather")
	part := NewSheetPart("Arm_Left", "arms", "")

	proj := NewProject("test")
	proj.CurrentTrack = track

	if got := proj.ResolveActiveSheetName(part); got != "leather" {
		t.Errorf("with no preview override, resolved sheet = %q, want prop default %q", got, "leather")
	}

	proj.PreviewProps["arms"] = "chainmail"
	if got := proj.ResolveActiveSheetName(part); got != "chainmail" {
		t.Errorf("with a preview override set, resolved sheet = %q, want override %q", got, "chainmail")
	}

	fixedPart := NewSheetPart("Body", "", "human_body_default")
	if got := proj.ResolveActiveSheetName(fixedPart); got != "human_body_default" {
		t.Errorf("a part with no governing prop resolved to %q, want its FixedSheet %q", got, "human_body_default")
	}
}

func TestDeepCopyIsIndependent(t *testing.T) {
	track := NewTrack("human_walk")
	dir := AddDirection(track, 2) // 2 = "down" by the game's direction convention
	part := NewSheetPart("Hair", "hair", "")
	AddKeyframe(part, 0)
	AddPart(dir, part)

	copy := track.DeepCopy()
	copy.Directions[2].Parts[0].Keyframes[0].X = 999

	if track.Directions[2].Parts[0].Keyframes[0].X == 999 {
		t.Error("mutating the deep copy affected the original - not actually independent")
	}
}

func TestDirectionTotalDurationMsIsMaxAcrossParts(t *testing.T) {
	dir := &Direction{}
	p1 := NewSheetPart("A", "", "")
	p1.Keyframes = []*Keyframe{{TimeMs: 0}, {TimeMs: 100}}
	p2 := NewSheetPart("B", "", "")
	p2.Keyframes = []*Keyframe{{TimeMs: 0}, {TimeMs: 300}}
	AddPart(dir, p1)
	AddPart(dir, p2)

	if got := dir.TotalDurationMs(); got != 300 {
		t.Errorf("TotalDurationMs() = %v, want 300 (the longer part's last keyframe)", got)
	}
}
