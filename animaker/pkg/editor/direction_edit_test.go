package editor

import (
	"errors"
	"testing"
)

// fourFacings is a project with directions 0-3, posed only in 0 and 2 -
// a file saved with the old four-direction default.
func fourFacings(t *testing.T) (*Project, *Part) {
	t.Helper()
	p := NewProject("t")
	AddStandardDirections(p.CurrentTrack)
	part := AddPart(p.CurrentTrack, NewSheetPart("body", "", "s"))
	AddKeyframe(p.CurrentTrack.Directions[0], part.ID, 0).X = 5
	AddKeyframe(p.CurrentTrack.Directions[2], part.ID, 0).X = 7
	return p, part
}

func TestDeleteDirection(t *testing.T) {
	p, _ := fourFacings(t)
	p.SetActiveDirection(2)
	if err := p.DeleteDirection(2); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.CurrentTrack.Directions[2]; ok {
		t.Error("direction 2 still there")
	}
	if a := p.Playback.ActiveDirection; a != 1 {
		t.Errorf("active direction %d after deleting the active 2, want the nearest, 1", a)
	}
	for _, k := range []int{0, 1, 3} {
		if err := p.DeleteDirection(k); err != nil && k != 3 {
			t.Fatal(err)
		}
	}
	if err := p.DeleteDirection(3); !errors.Is(err, ErrLastDirection) {
		t.Errorf("deleting the last direction: %v, want ErrLastDirection", err)
	}
}

func TestChangeDirectionKeyMovesTheKeyframes(t *testing.T) {
	p, part := fourFacings(t)
	p.SetActiveDirection(0)

	// Posed "up" but meant "right": 1 exists but is empty, so it's replaced.
	if err := p.ChangeDirectionKey(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.CurrentTrack.Directions[0]; ok {
		t.Error("direction 0 still there")
	}
	if kfs := p.CurrentTrack.Directions[1].KeyframesFor(part.ID); len(kfs) != 1 || kfs[0].X != 5 {
		t.Errorf("direction 1 keyframes %v, want direction 0's pose (x=5)", kfs)
	}
	if p.Playback.ActiveDirection != 1 {
		t.Errorf("active %d, want it to follow the move to 1", p.Playback.ActiveDirection)
	}

	// 2 is posed: refused, nothing moved.
	if err := p.ChangeDirectionKey(1, 2); err == nil {
		t.Error("moving onto posed direction 2 was allowed")
	}
	if p.CurrentTrack.Directions[2].KeyframesFor(part.ID)[0].X != 7 {
		t.Error("refused move changed direction 2")
	}
	if err := p.ChangeDirectionKey(1, -1); err == nil {
		t.Error("negative key allowed")
	}
}

func TestRemoveEmptyDirectionsKeepsOne(t *testing.T) {
	p, _ := fourFacings(t)
	p.SetActiveDirection(3)
	removed := p.RemoveEmptyDirections()
	if len(removed) != 2 || removed[0] != 1 || removed[1] != 3 {
		t.Errorf("removed %v, want [1 3]", removed)
	}
	if got := p.CurrentTrack.SortedDirectionKeys(); len(got) != 2 {
		t.Errorf("left %v, want [0 2]", got)
	}
	if a := p.Playback.ActiveDirection; a != 2 {
		t.Errorf("active %d after its direction went, want the nearest, 2", a)
	}

	all := NewProject("t")
	AddStandardDirections(all.CurrentTrack)
	all.SetActiveDirection(2)
	all.RemoveEmptyDirections()
	if got := all.CurrentTrack.SortedDirectionKeys(); len(got) != 1 || got[0] != 2 {
		t.Errorf("all empty: left %v, want just the active one, [2]", got)
	}
	if again := all.RemoveEmptyDirections(); again != nil {
		t.Errorf("second run removed %v, want nothing", again)
	}
}
