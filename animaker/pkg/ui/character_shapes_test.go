package ui

import (
	"animaker/pkg/editor"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// shapePanel is a character panel showing a character with a footprint and
// one hitbox, counting OnChanged calls.
func shapePanel(t *testing.T) (*CharacterPanel, *editor.Character, *int) {
	t.Helper()
	test.NewApp()
	c := editor.NewCharacter("ogre")
	c.SetFootprint(editor.Box{X: -10, Y: -5, W: 20, H: 10})
	c.AddHitbox()
	changed := 0
	cp := NewCharacterPanel()
	cp.OnChanged = func() { changed++ }
	cp.Build()
	cp.Character = c
	cp.Refresh()
	return cp, c, &changed
}

// buttons finds the panel's shape-section buttons with this label.
func shapeButtons(cp *CharacterPanel, label string) []*widget.Button {
	var out []*widget.Button
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == label {
			out = append(out, b)
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
		}
	}
	walk(cp.shapesBox)
	return out
}

func TestPanelSelectsShapes(t *testing.T) {
	cp, _, _ := shapePanel(t)
	var got []ShapeRef
	cp.OnSelectShape = func(r ShapeRef) { got = append(got, r) }
	sel := shapeButtons(cp, "select")
	if len(sel) != 2 {
		t.Fatalf("%d select buttons, want 2 (footprint, hitbox)", len(sel))
	}
	test.Tap(sel[0])
	if len(got) != 1 || !got[0].Footprint {
		t.Fatalf("selections = %v, want the footprint", got)
	}
	// Tapping the selected shape's button again deselects it.
	test.Tap(shapeButtons(cp, "select")[0])
	if len(got) != 2 || !got[1].IsNone() {
		t.Errorf("selections = %v, want deselected", got)
	}
	test.Tap(shapeButtons(cp, "select")[1])
	if cp.Selected != HitboxRef(0) {
		t.Errorf("selected %v, want hitbox 0", cp.Selected)
	}
}

func TestPanelAddsAndRemovesShapes(t *testing.T) {
	cp, c, changed := shapePanel(t)
	test.Tap(shapeButtons(cp, "+ Add Hitbox")[0])
	if len(c.Hitboxes) != 2 || cp.Selected != HitboxRef(1) || *changed != 1 {
		t.Errorf("after add: %d hitboxes, selected %v, %d changes", len(c.Hitboxes), cp.Selected, *changed)
	}
	// The × buttons: footprint first, then each hitbox.
	test.Tap(shapeButtons(cp, "×")[0])
	if c.Footprint != nil {
		t.Error("removing the footprint left it")
	}
	if len(shapeButtons(cp, "+ Add Footprint")) != 1 {
		t.Error("no Add Footprint button once it's gone")
	}
	test.Tap(shapeButtons(cp, "+ Add Footprint")[0])
	if c.Footprint == nil || *c.Footprint != editor.DefaultFootprint || !cp.Selected.Footprint {
		t.Errorf("added footprint = %+v, selected %v", c.Footprint, cp.Selected)
	}
}

func TestPanelBoxFieldsApply(t *testing.T) {
	cp, c, changed := shapePanel(t)
	var entries []*selectAllEntry
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if e, ok := o.(*selectAllEntry); ok {
			entries = append(entries, e)
		}
		if ct, ok := o.(*fyne.Container); ok {
			for _, ch := range ct.Objects {
				walk(ch)
			}
		}
	}
	walk(cp.shapesBox)
	// scale, then the footprint's x y w h.
	if len(entries) < 5 {
		t.Fatalf("found %d entries", len(entries))
	}
	w := entries[3]
	w.SetText("32")
	w.OnSubmitted(w.Text)
	if c.Footprint.W != 32 || *changed != 1 {
		t.Errorf("footprint w = %v after typing 32 (%d changes)", c.Footprint.W, *changed)
	}
	// An invalid size is refused and the field reverts.
	entries = nil
	walk(cp.shapesBox)
	w = entries[3]
	w.SetText("0")
	w.OnSubmitted(w.Text)
	if c.Footprint.W != 32 || w.Text != "32" {
		t.Errorf("a zero width was applied: w=%v field=%q", c.Footprint.W, w.Text)
	}
}
