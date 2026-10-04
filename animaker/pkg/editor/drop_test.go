package editor

import "testing"

// Regression: reported as dragging sprite (0,1) on at ~200ms "creates a new
// timeline for that sprite, but it should just insert it onto the timeline
// of the previous sprite". With the first part still selected, the second
// drop must key that part, not add another.
func TestDropTileWithPartSelectedKeysThatPart(t *testing.T) {
	p := NewProject("test")
	dir := p.ActiveDirection()

	idx, _ := p.DropTile("sprite", 0, 0, 10, 20)
	p.Selection.PartIndex = idx // the app selects what was just dropped

	p.Seek(200)
	idx2, kf := p.DropTile("sprite", 0, 1, 12, 20)

	if got := len(p.CurrentTrack.Parts); got != 1 {
		t.Fatalf("rig has %d parts, want 1", got)
	}
	if idx2 != idx {
		t.Errorf("second drop landed on part %d, want %d", idx2, idx)
	}
	kfs := dir.KeyframesFor(p.CurrentTrack.Parts[0].ID)
	if len(kfs) != 2 {
		t.Fatalf("part has %d keyframes, want 2", len(kfs))
	}
	if kf != kfs[1] || kf.TimeMs != 200 || kf.Row != 0 || kf.Col != 1 || kf.X != 12 {
		t.Errorf("second keyframe = %+v, want (0,1) at 200ms, x=12", *kf)
	}
	// The first cell holds right up to the second keyframe - no extra
	// "hold" keyframe needed.
	if got := dir.ValueAt(p.CurrentTrack.Parts[0].ID, 199); got.Col != 0 {
		t.Errorf("at 199ms showing col %d, want 0", got.Col)
	}
}

// Decided 2026-10-03: a new part starts at the origin wherever it's
// dropped, since a rig's pieces are usually drawn on one shared grid. A
// drop that keys the selected part still lands where it's dropped.
func TestDropTileNewPartStartsAtOrigin(t *testing.T) {
	p := NewProject("test")
	if !p.DropMakesNewPart("sprite") {
		t.Fatal("nothing selected: a drop should make a new part")
	}
	idx, kf := p.DropTile("sprite", 0, 0, 37, -12)
	if kf.X != 0 || kf.Y != 0 {
		t.Errorf("new part at (%v,%v), want the origin", kf.X, kf.Y)
	}
	p.Selection.PartIndex = idx
	if p.DropMakesNewPart("sprite") || !p.DropMakesNewPart("other_sheet") {
		t.Error("DropMakesNewPart should follow the selection and its sheet")
	}
	p.Seek(100)
	if _, kf := p.DropTile("sprite", 0, 1, 37, -12); kf.X != 37 || kf.Y != -12 {
		t.Errorf("keyed drop at (%v,%v), want the drop point (37,-12)", kf.X, kf.Y)
	}
}

func TestDropTileOnExistingKeyframeReCellsIt(t *testing.T) {
	p := NewProject("test")
	idx, _ := p.DropTile("sprite", 0, 0, 0, 0)
	p.Selection.PartIndex = idx

	p.DropTile("sprite", 2, 3, 5, 5) // still at 0ms

	kfs := p.ActiveDirection().KeyframesFor(p.CurrentTrack.Parts[0].ID)
	if len(kfs) != 1 {
		t.Fatalf("part has %d keyframes, want 1", len(kfs))
	}
	if kfs[0].Row != 2 || kfs[0].Col != 3 {
		t.Errorf("keyframe cell = (%d,%d), want (2,3)", kfs[0].Row, kfs[0].Col)
	}
}

func TestDropTileWithNothingSelectedAddsAPart(t *testing.T) {
	p := NewProject("test")
	p.DropTile("sprite", 0, 0, 0, 0)
	p.Selection.PartIndex = -1

	idx, _ := p.DropTile("sprite", 1, 0, 0, 0)

	if got := len(p.CurrentTrack.Parts); got != 2 || idx != 1 {
		t.Errorf("rig has %d parts and drop landed on %d, want 2 parts and index 1", got, idx)
	}
}

// A part draws every keyframe from one sheet, so a cell from another sheet
// can't become one of its frames - it has to be a new part.
func TestDropTileFromOtherSheetAddsAPart(t *testing.T) {
	p := NewProject("test")
	idx, _ := p.DropTile("body", 0, 0, 0, 0)
	p.Selection.PartIndex = idx

	p.DropTile("sword", 0, 0, 0, 0)

	if got := len(p.CurrentTrack.Parts); got != 2 {
		t.Errorf("rig has %d parts, want 2", got)
	}
}

// A sheet imported as a prop's art produces parts linked to that prop, so
// they swap when the prop's value changes.
func TestDropTileFromPropSheetLinksThePart(t *testing.T) {
	p := NewProject("test")
	EnsureProp(p.CurrentTrack, "hair", "hair_short")

	idx, _ := p.DropTile("hair_short", 0, 0, 0, 0)

	part := p.CurrentTrack.Parts[idx]
	if part.GoverningProp != "hair" || part.FixedSheet != "hair_short" || part.Name != "hair_1" {
		t.Errorf("part = %+v, want hair_1 governed by hair, fixed hair_short", *part)
	}
}

func TestEnsurePropKeepsAnExistingDefault(t *testing.T) {
	track := NewTrack("t")
	if !EnsureProp(track, "hair", "hair_short") {
		t.Fatal("first EnsureProp didn't create the prop")
	}
	if EnsureProp(track, "hair", "hair_long") {
		t.Error("second EnsureProp created a duplicate prop")
	}
	if len(track.Props) != 1 || track.Props[0].Default != "hair_short" {
		t.Errorf("props = %+v, want one hair prop defaulting to hair_short", track.Props)
	}
}

// Reported: select a part, click a tile - nothing happened unless a
// timeline marker was selected. A click now keys the selected part at the
// playhead with that cell, keeping its position.
func TestTapTileKeysSelectedPartAtPlayheadKeepingPose(t *testing.T) {
	p := NewProject("test")
	dir := p.ActiveDirection()
	idx, first := p.DropTile("sprite", 0, 0, 10, 20)
	first.X, first.Y = 10, 20                                  // dragged off the origin
	p.Selection.PartIndex, p.Selection.KeyframeIndex = idx, -1 // as clicking the part leaves it
	p.Scrub(200)

	kf, err := p.TapTile("sprite", 0, 3)
	if err != nil {
		t.Fatalf("TapTile: %v", err)
	}
	kfs := dir.KeyframesFor(p.CurrentTrack.Parts[idx].ID)
	if len(kfs) != 2 || kf != kfs[1] {
		t.Fatalf("keyframes = %d, want a new one at the playhead", len(kfs))
	}
	if kf.TimeMs != 200 || kf.Col != 3 || kf.X != 10 || kf.Y != 20 {
		t.Errorf("keyframe = %+v, want col 3 at 200ms, still at (10,20)", *kf)
	}
	if kfs[0].Col != 0 {
		t.Errorf("first keyframe re-celled to %d, want it left at 0", kfs[0].Col)
	}

	// On a keyed time it re-cells that keyframe rather than adding one.
	if _, err := p.TapTile("sprite", 0, 2); err != nil || len(dir.KeyframesFor(p.CurrentTrack.Parts[idx].ID)) != 2 || kf.Col != 2 {
		t.Errorf("second tap: err %v, col %d, want the 200ms keyframe re-celled to 2", err, kf.Col)
	}
}

func TestTapTileRefusesWithoutAPartOrFromAnotherSheet(t *testing.T) {
	p := NewProject("test")
	if _, err := p.TapTile("sprite", 0, 1); err == nil {
		t.Error("tap with nothing selected: want an error, not a silent no-op")
	}

	idx, kf := p.DropTile("body", 0, 0, 0, 0)
	p.Selection.PartIndex = idx
	if _, err := p.TapTile("hair", 2, 5); err == nil {
		t.Error("tap from another sheet: want an error")
	}
	if kf.Row != 0 || kf.Col != 0 {
		t.Errorf("refused tap changed the keyframe to (%d,%d)", kf.Row, kf.Col)
	}
}
