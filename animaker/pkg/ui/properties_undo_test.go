package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Typing a value into a field on an existing keyframe is one undo step,
// however many keystrokes it took; leaving the field and typing again
// is a second one.
func TestTypingInAFieldIsOneUndoStepPerBurst(t *testing.T) {
	pp, p, part := panelWithPart(t, 400) // on the keyframe at x=30
	x := entriesIn(container.NewVBox(pp.keyframeBox.Objects...))[1]

	x.FocusGained()
	for _, typed := range []string{"-", "-1", "-12", "-12.", "-12.5"} {
		x.SetText(typed)
	}
	x.FocusLost()
	x.FocusGained()
	x.SetText("7")
	x.FocusLost()

	kf := func() float32 { return p.ActiveDirection().KeyframesFor(part.ID)[1].X }
	if kf() != 7 {
		t.Fatalf("x = %v, want 7", kf())
	}
	p.Undo()
	if kf() != -12.5 {
		t.Errorf("after one undo x = %v, want -12.5 (the end of the first burst)", kf())
	}
	p.Undo()
	if kf() != 30 {
		t.Errorf("after two undos x = %v, want the original 30", kf())
	}
}

func TestNudgeIsUndoable(t *testing.T) {
	pp, p, part := panelWithPart(t, 400)
	var xPlus *widget.Button
	for _, b := range buttonsIn(container.NewVBox(pp.keyframeBox.Objects...)) {
		if b.Text == "X+" {
			xPlus = b
		}
	}
	if xPlus == nil {
		t.Fatal("no X+ nudge button")
	}
	test.Tap(xPlus)
	test.Tap(xPlus)
	p.Undo()
	if got := p.ActiveDirection().KeyframesFor(part.ID)[1].X; got != 31 {
		t.Errorf("after one undo x = %v, want 31 (one click back)", got)
	}
}

func TestFixedSheetChangeIsUndoable(t *testing.T) {
	pp, p, part := panelWithPart(t, 0)
	p.LoadedSheets["other"] = p.LoadedSheets["s"]
	pp.Refresh()
	var fixed *widget.Select
	for _, s := range selectsIn(container.NewVBox(pp.partLinkBox.Objects...)) {
		if s.Selected == "s" {
			fixed = s
		}
	}
	if fixed == nil {
		t.Fatal("no fixed-sheet picker showing s")
	}
	fixed.SetSelected("other")
	if part.FixedSheet != "other" {
		t.Fatalf("FixedSheet = %q", part.FixedSheet)
	}
	p.Undo()
	if got := p.CurrentTrack.Parts[0].FixedSheet; got != "s" {
		t.Errorf("after undo FixedSheet = %q, want s", got)
	}
}

// buttonsIn lists the buttons under o, in layout order.
func buttonsIn(o fyne.CanvasObject) []*widget.Button {
	var out []*widget.Button
	switch v := o.(type) {
	case *widget.Button:
		out = append(out, v)
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, buttonsIn(c)...)
		}
	}
	return out
}
