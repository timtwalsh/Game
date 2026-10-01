package ui

import (
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// selectsIn lists the dropdowns under o, in layout order.
func selectsIn(o fyne.CanvasObject) []*widget.Select {
	var out []*widget.Select
	switch v := o.(type) {
	case *widget.Select:
		out = append(out, v)
	case *fyne.Container:
		for _, c := range v.Objects {
			out = append(out, selectsIn(c)...)
		}
	}
	return out
}

// findSelect returns the dropdown currently showing one of options.
func findSelect(t *testing.T, box *fyne.Container, options ...string) *widget.Select {
	t.Helper()
	for _, s := range selectsIn(box) {
		for _, o := range options {
			if s.Selected == o {
				return s
			}
		}
	}
	return nil
}

func nestedPanel(t *testing.T) (*PropertiesPanel, *editor.Project, *editor.Part) {
	t.Helper()
	test.NewTempApp(t)
	p := editor.NewProject("walk_torch")
	part := editor.AddPart(p.CurrentTrack, editor.NewNestedAniPart("torch_1", editor.AnimKey("torch.anif")))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)
	pp := NewPropertiesPanel(p)
	w := test.NewWindow(pp.Build(widget.NewLabel("dir")))
	t.Cleanup(w.Close)
	pp.SelectPart(0)
	return pp, p, part
}

// Requested: choose a nested animation's direction as static, inherited
// from the parent, or static per keyframe.
func TestNestedDirectionPickers(t *testing.T) {
	pp, p, part := nestedPanel(t)

	mode := findSelect(t, pp.partLinkBox, "Inherit from parent")
	if mode == nil {
		t.Fatal("no Direction picker showing \"Inherit from parent\" for a new nested part")
	}

	// Static shows a picker for which direction.
	mode.SetSelected("Static")
	if part.DirectionMode != editor.NestedDirStatic {
		t.Fatalf("mode = %v, want static", part.DirectionMode)
	}
	plays := findSelect(t, pp.partLinkBox, "0 (up)")
	if plays == nil {
		t.Fatal("no static direction picker")
	}
	plays.SetSelected("3 (left)")
	if part.StaticDirection != 3 {
		t.Errorf("static direction = %d, want 3", part.StaticDirection)
	}

	// Per keyframe: the keyframe section gets a Direction picker that sets
	// the keyframe at the playhead. Switching seeded it with 3, what it was
	// showing.
	mode = findSelect(t, pp.partLinkBox, "Static")
	mode.SetSelected("Per keyframe")
	kfDir := findSelect(t, pp.keyframeBox, "3 (left)")
	if kfDir == nil {
		t.Fatal("no per-keyframe Direction picker (or it wasn't seeded with 3)")
	}
	kfDir.SetSelected("1 (right)")
	if got := p.ActiveDirection().KeyframesFor(part.ID)[0].Direction; got != 1 {
		t.Errorf("keyframe direction = %d, want 1", got)
	}
}
