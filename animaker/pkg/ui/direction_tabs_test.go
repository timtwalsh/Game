package ui

import (
	"animaker/pkg/editor"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestDirectionTabLabelNamesTheFacing(t *testing.T) {
	cases := map[[2]int]string{
		{0, 0}: "Up (0)",
		{1, 5}: "Right (5)",
		{2, 1}: "Down (1)",
		{3, 0}: "Left (0)",
		{7, 2}: "Dir 7 (2)",
	}
	for in, want := range cases {
		if got := DirectionTabLabel(in[0], in[1]); got != want {
			t.Errorf("DirectionTabLabel(%d, %d) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// One tab per direction: the active one highlighted, unposed ones in the
// warning colour, and clicking another reports its key.
func TestDirectionTabsShowEachFacing(t *testing.T) {
	test.NewTempApp(t)
	track := editor.NewTrack("walk")
	editor.AddStandardDirections(track)
	part := editor.AddPart(track, editor.NewSheetPart("body", "", "sheet"))
	editor.AddKeyframe(track.Directions[0], part.ID, 0)
	editor.AddKeyframe(track.Directions[2], part.ID, 0)

	dt := NewDirectionTabs()
	var picked []int
	dt.OnSelect = func(k int) { picked = append(picked, k) }
	dt.Refresh(track, 2)

	grid := dt.Object().(*fyne.Container).Objects[0].(*fyne.Container)
	if len(grid.Objects) != 4 {
		t.Fatalf("%d tabs, want 4", len(grid.Objects))
	}
	btns := make([]*widget.Button, 4)
	for i, o := range grid.Objects {
		btns[i] = o.(*widget.Button)
	}
	if btns[0].Text != "Up (1)" || btns[1].Text != "Right (0)" {
		t.Errorf("tabs = %q, %q, want Up (1), Right (0)", btns[0].Text, btns[1].Text)
	}
	if btns[2].Importance != widget.HighImportance {
		t.Error("active direction (Down) isn't highlighted")
	}
	if btns[1].Importance != widget.WarningImportance || btns[0].Importance != widget.MediumImportance {
		t.Error("want unposed facings in the warning colour and posed ones plain")
	}

	test.Tap(btns[2]) // the active one: nothing to switch to
	test.Tap(btns[3])
	if len(picked) != 1 || picked[0] != 3 {
		t.Errorf("picked %v, want [3]", picked)
	}
}
