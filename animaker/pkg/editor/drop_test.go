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
