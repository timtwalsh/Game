package ui

import (
	"image"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// bindingPanel is a parent track holding torch.anif, which declares a
// torch_base sheet prop and has its own sheets torchbase_wood/_metal.
func bindingPanel(t *testing.T, parentProps ...string) (*PropertiesPanel, *editor.Project, *editor.Part) {
	t.Helper()
	test.NewTempApp(t)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	torch := editor.NewTrack("torch")
	editor.AddProp(torch, "torch_base", "torchbase_wood")
	path := editor.AnimKey("torch.anif")

	p := editor.NewProject("guard_walk")
	p.LoadedAnims[path] = &editor.NestedAnim{Path: path, Track: torch, Sheets: map[string]*editor.SpriteSheetTemplate{
		"torchbase_wood":  editor.NewSpriteSheetTemplate("torchbase_wood", "w.png", img, 8, 8, 0, 0),
		"torchbase_metal": editor.NewSpriteSheetTemplate("torchbase_metal", "m.png", img, 8, 8, 0, 0),
	}}
	for _, name := range parentProps {
		editor.AddProp(p.CurrentTrack, name, "torchbase_wood")
	}
	part := editor.AddPart(p.CurrentTrack, editor.NewNestedAniPart("torch_1", path))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)

	pp := NewPropertiesPanel(p)
	w := test.NewWindow(pp.Build(widget.NewLabel("dir")))
	t.Cleanup(w.Close)
	pp.SelectPart(0)
	return pp, p, part
}

// The city guard pins its torch's base: Fixed, picked from the torch's
// own sheets. Undoable.
func TestBindingFixedValue(t *testing.T) {
	pp, p, part := bindingPanel(t)

	mode := findSelect(t, pp.partLinkBox, bindDefault)
	if mode == nil {
		t.Fatal("no binding row for torch_base")
	}
	mode.SetSelected(bindStatic)
	value := findSelect(t, pp.partLinkBox, "torchbase_wood") // seeded from the default
	if value == nil {
		t.Fatal("no fixed-value picker")
	}
	if !contains(value.Options, "torchbase_metal") {
		t.Fatalf("options %v lack the torch's own torchbase_metal", value.Options)
	}
	value.SetSelected("torchbase_metal")

	if got := part.NestedBindings["torch_base"]; got != (editor.PropBinding{StaticValue: "torchbase_metal"}) {
		t.Fatalf("binding = %+v", got)
	}
	p.Undo()
	if got := p.CurrentTrack.Parts[0].NestedBindings["torch_base"]; got.StaticValue != "torchbase_wood" {
		t.Errorf("after undo binding = %+v, want the step before (fixed to the default)", got)
	}
}

// The player passes its own torch_base prop through; a parent prop of
// the same name is preselected.
func TestBindingPassthroughPrefersSameName(t *testing.T) {
	pp, _, part := bindingPanel(t, "colour", "torch_base")

	findSelect(t, pp.partLinkBox, bindDefault).SetSelected(bindPassthrough)

	if got := part.NestedBindings["torch_base"]; got != (editor.PropBinding{PassthroughFrom: "torch_base"}) {
		t.Fatalf("binding = %+v, want passthrough from torch_base", got)
	}
	from := findSelect(t, pp.partLinkBox, "torch_base")
	if from == nil || !contains(from.Options, "colour") {
		t.Fatal("no parent-prop picker listing the parent's props")
	}
	from.SetSelected("colour")
	if got := part.NestedBindings["torch_base"].PassthroughFrom; got != "colour" {
		t.Errorf("PassthroughFrom = %q, want colour", got)
	}

	findSelect(t, pp.partLinkBox, bindPassthrough).SetSelected(bindDefault)
	if _, ok := part.NestedBindings["torch_base"]; ok {
		t.Error("Default left a binding behind")
	}
}
