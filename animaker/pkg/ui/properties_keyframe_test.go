package ui

import (
	"image"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// entriesIn lists the text entries under o, in layout order.
func entriesIn(o fyne.CanvasObject) []*selectAllEntry {
	var out []*selectAllEntry
	switch v := o.(type) {
	case *selectAllEntry:
		out = append(out, v)
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, entriesIn(c)...)
		}
	}
	return out
}

// panelWithPart builds the properties panel for a project with one part
// keyed at 0ms at x=10, selected, with the playhead at playheadMs.
func panelWithPart(t *testing.T, playheadMs uint32) (*PropertiesPanel, *editor.Project, *editor.Part) {
	t.Helper()
	test.NewTempApp(t)
	p := editor.NewProject("t")
	p.LoadedSheets["s"] = editor.NewSpriteSheetTemplate("s", "s.png", image.NewRGBA(image.Rect(0, 0, 16, 16)), 16, 16, 0, 0)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("body", "", "s"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0).X = 10
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 400).X = 30

	pp := NewPropertiesPanel(p)
	w := test.NewWindow(pp.Build(widget.NewLabel("dir")))
	t.Cleanup(w.Close)
	p.Seek(playheadMs)
	pp.SelectPart(0) // as a click in the part list does: no keyframe selected
	return pp, p, part
}

// Requested: the position panel is always visible when a part is selected,
// not only after a keyframe has been picked.
func TestPositionFieldsShowForASelectedPart(t *testing.T) {
	pp, p, _ := panelWithPart(t, 200)
	if p.Selection.KeyframeIndex != -1 {
		t.Fatalf("precondition: a keyframe is selected (%d)", p.Selection.KeyframeIndex)
	}
	entries := entriesIn(container.NewVBox(pp.keyframeBox.Objects...))
	if len(entries) < 4 {
		t.Fatalf("%d entries shown, want X/Y/Z/Rotation", len(entries))
	}
	// Halfway between x=10 and x=30: the pose the part is showing.
	if got := entries[0].Text; got != "20" {
		t.Errorf("X shows %q, want the interpolated 20", got)
	}
}

// With no keyframe at the playhead, the first edit adds one there.
func TestEditingPositionKeysThePartAtThePlayhead(t *testing.T) {
	pp, p, part := panelWithPart(t, 200)
	x := entriesIn(container.NewVBox(pp.keyframeBox.Objects...))[0]

	x.SetText("25")

	kfs := p.ActiveDirection().KeyframesFor(part.ID)
	if len(kfs) != 3 {
		t.Fatalf("%d keyframes, want a new one at 200ms", len(kfs))
	}
	if kfs[1].TimeMs != 200 || kfs[1].X != 25 {
		t.Errorf("new keyframe = %+v, want x=25 at 200ms", *kfs[1])
	}
	if !p.UndoStack.CanUndo() {
		t.Error("adding the keyframe isn't undoable")
	}
}

// On an existing keyframe the fields edit it, without adding another.
func TestEditingPositionOnAKeyframeEditsIt(t *testing.T) {
	pp, p, part := panelWithPart(t, 400)
	entries := entriesIn(container.NewVBox(pp.keyframeBox.Objects...))
	// With a keyframe there, the Time (ms) field comes first.
	if entries[0].Text != "400" {
		t.Fatalf("first field = %q, want the 400ms time field", entries[0].Text)
	}
	entries[1].SetText("35")

	kfs := p.ActiveDirection().KeyframesFor(part.ID)
	if len(kfs) != 2 || kfs[1].X != 35 {
		t.Errorf("keyframes = %d, x at 400ms = %v; want 2 and 35", len(kfs), kfs[1].X)
	}
}
