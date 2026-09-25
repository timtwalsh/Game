package editor

import "testing"

// dirWithKeyframes builds a direction holding one part's keyframes, for the
// interpolation tests below. Returns the part ID to evaluate against.
func dirWithKeyframes(kfs ...*Keyframe) (*Direction, int) {
	const partID = 1
	return &Direction{Keyframes: map[int][]*Keyframe{partID: kfs}}, partID
}

func TestValueAtInterpolatesBetweenSurroundingKeyframes(t *testing.T) {
	dir, partID := dirWithKeyframes(
		&Keyframe{TimeMs: 0, X: 0, Row: 0, Col: 0},
		&Keyframe{TimeMs: 100, X: 100, Row: 1, Col: 2},
	)

	got := dir.ValueAt(partID, 50)
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
	dir, partID := dirWithKeyframes(
		&Keyframe{TimeMs: 50, X: 10},
		&Keyframe{TimeMs: 150, X: 20},
	)

	if got := dir.ValueAt(partID, 0); got.X != 10 {
		t.Errorf("ValueAt before first keyframe = %v, want clamped to 10", got.X)
	}
	if got := dir.ValueAt(partID, 9999); got.X != 20 {
		t.Errorf("ValueAt after last keyframe = %v, want clamped to 20", got.X)
	}
}

// A part that exists on the rig but isn't posed in this facing resolves to
// the zero value rather than erroring — the canvas just skips drawing it.
func TestValueAtUnposedPartReturnsZeroValue(t *testing.T) {
	dir := NewDirection()
	if got := dir.ValueAt(99, 100); got != (ResolvedTransform{}) {
		t.Errorf("ValueAt on a part with no keyframes here = %+v, want zero value", got)
	}
}

func TestAddKeyframeKeepsSortedOrder(t *testing.T) {
	dir := NewDirection()
	AddKeyframe(dir, 1, 200)
	AddKeyframe(dir, 1, 0)
	AddKeyframe(dir, 1, 100)

	kfs := dir.KeyframesFor(1)
	want := []uint32{0, 100, 200}
	if len(kfs) != len(want) {
		t.Fatalf("got %d keyframes, want %d", len(kfs), len(want))
	}
	for i, w := range want {
		if kfs[i].TimeMs != w {
			t.Errorf("keyframe[%d].TimeMs = %v, want %v (not sorted)", i, kfs[i].TimeMs, w)
		}
		if kfs[i].ID != i {
			t.Errorf("keyframe[%d].ID = %v, want %v (not reindexed after sort)", i, kfs[i].ID, i)
		}
	}
}

func TestAddKeyframeAtExistingTimeReturnsExistingNotDuplicate(t *testing.T) {
	dir := NewDirection()
	first := AddKeyframe(dir, 1, 100)
	first.X = 42
	second := AddKeyframe(dir, 1, 100)

	if got := len(dir.KeyframesFor(1)); got != 1 {
		t.Fatalf("got %d keyframes, want 1 (re-adding at an existing time shouldn't duplicate)", got)
	}
	if second.X != 42 {
		t.Errorf("AddKeyframe at an existing time returned a fresh keyframe instead of the existing one")
	}
}

// Keyframes are per-direction even though the part is shared, so posing a
// part in one facing must not touch any other.
func TestKeyframesAreIndependentPerDirection(t *testing.T) {
	track := NewTrack("human_walk")
	part := AddPart(track, NewSheetPart("Body", "", "body"))

	AddKeyframe(track.Directions[0], part.ID, 0).X = 10
	AddKeyframe(track.Directions[2], part.ID, 0).X = 99

	if got := track.Directions[0].ValueAt(part.ID, 0).X; got != 10 {
		t.Errorf("direction 0 X = %v, want 10 - direction 2 leaked into it", got)
	}
	if got := len(track.Directions[1].KeyframesFor(part.ID)); got != 0 {
		t.Errorf("direction 1 has %d keyframes, want 0 - it was never posed", got)
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
	part := AddPart(track, NewSheetPart("Hair", "hair", ""))
	AddKeyframe(track.Directions[2], part.ID, 0)

	cp := track.DeepCopy()
	cp.Directions[2].KeyframesFor(part.ID)[0].X = 999
	cp.Parts[0].Name = "Renamed"

	if track.Directions[2].KeyframesFor(part.ID)[0].X == 999 {
		t.Error("mutating the copy's keyframe affected the original - not actually independent")
	}
	if track.Parts[0].Name == "Renamed" {
		t.Error("mutating the copy's part affected the original - not actually independent")
	}
}

func TestDirectionTotalDurationMsIsMaxAcrossParts(t *testing.T) {
	dir := NewDirection()
	AddKeyframe(dir, 1, 0)
	AddKeyframe(dir, 1, 100)
	AddKeyframe(dir, 2, 0)
	AddKeyframe(dir, 2, 300)

	if got := dir.TotalDurationMs(); got != 300 {
		t.Errorf("TotalDurationMs() = %v, want 300 (the longer part's last keyframe)", got)
	}
}

// Parts belong to the rig, so adding one makes it exist in every direction
// at once; removing it takes its keyframes out of every direction too,
// rather than leaving them orphaned for a later part to inherit.
func TestPartsAreSharedAcrossDirections(t *testing.T) {
	track := NewTrack("human_walk")
	body := AddPart(track, NewSheetPart("Body", "", "body"))
	hair := AddPart(track, NewSheetPart("Hair", "", "hair"))

	if body.ID == hair.ID {
		t.Fatalf("parts share ID %d; IDs must be unique", body.ID)
	}
	if len(track.Parts) != 2 {
		t.Fatalf("track has %d parts, want 2", len(track.Parts))
	}

	for _, key := range track.SortedDirectionKeys() {
		AddKeyframe(track.Directions[key], body.ID, 0)
		AddKeyframe(track.Directions[key], hair.ID, 0)
	}

	if err := RemovePart(track, 0); err != nil { // remove Body
		t.Fatalf("RemovePart: %v", err)
	}
	if len(track.Parts) != 1 || track.Parts[0].ID != hair.ID {
		t.Fatalf("after removal the rig is %+v, want just Hair", track.Parts)
	}
	for _, key := range track.SortedDirectionKeys() {
		if got := len(track.Directions[key].KeyframesFor(body.ID)); got != 0 {
			t.Errorf("direction %d still has %d keyframes for the removed part", key, got)
		}
		if got := len(track.Directions[key].KeyframesFor(hair.ID)); got != 1 {
			t.Errorf("direction %d has %d keyframes for Hair, want 1 - removal hit the wrong part", key, got)
		}
	}

	// A part added after a removal must not reuse the freed ID, or it would
	// silently adopt any keyframes that outlived it.
	if next := AddPart(track, NewSheetPart("Sword", "", "sword")); next.ID == body.ID {
		t.Errorf("new part reused removed part's ID %d", next.ID)
	}
}

// Switching facing keeps the selected part, because the part list is the
// track's; only the keyframe selection is direction-specific.
func TestSetActiveDirectionKeepsPartSelection(t *testing.T) {
	p := NewProject("test")
	AddPart(p.CurrentTrack, NewSheetPart("Body", "", "body"))
	p.Selection.PartIndex = 0
	p.Selection.KeyframeIndex = 3

	p.SetActiveDirection(2)

	if p.Selection.PartIndex != 0 {
		t.Errorf("PartIndex = %d after a direction change, want it kept at 0", p.Selection.PartIndex)
	}
	if p.Selection.KeyframeIndex != -1 {
		t.Errorf("KeyframeIndex = %d after a direction change, want -1", p.Selection.KeyframeIndex)
	}
}
