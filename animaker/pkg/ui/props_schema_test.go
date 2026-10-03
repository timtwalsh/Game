package ui

import (
	"animaker/pkg/editor"
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The props schema's default is a picker, so it can be changed after the
// prop is made - an undoable track edit.
func TestPropsSchemaDefaultIsEditable(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	for _, n := range []string{"hair_short", "hair_long"} {
		p.LoadedSheets[n] = editor.NewSpriteSheetTemplate(n, n+".png", image.NewRGBA(image.Rect(0, 0, 16, 16)), 16, 16, 0, 0)
	}
	editor.EnsureProp(p.CurrentTrack, "hair", "hair_short")
	pp := NewPropertiesPanel(p)
	refreshed := 0
	pp.OnPropsChanged = func() { refreshed++; pp.Refresh() }
	w := test.NewWindow(pp.Build(widget.NewLabel("dir")))
	t.Cleanup(w.Close)

	row := pp.schemaBox.Objects[0].(*fyne.Container)
	var sel *widget.Select
	for _, o := range row.Objects {
		if s, ok := o.(*widget.Select); ok {
			sel = s
		}
	}
	if sel == nil || sel.Selected != "hair_short" {
		t.Fatalf("no default picker showing hair_short in %v", row.Objects)
	}
	sel.SetSelected("hair_long")
	if got := p.CurrentTrack.FindProp("hair").Default; got != "hair_long" || refreshed != 1 || !p.Dirty {
		t.Errorf("default %q, refreshed %d, dirty %v; want hair_long, 1, true", got, refreshed, p.Dirty)
	}
	if !p.Undo() || p.CurrentTrack.FindProp("hair").Default != "hair_short" {
		t.Error("undo didn't restore the old default")
	}
}
