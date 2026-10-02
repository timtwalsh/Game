package editor

import (
	"errors"
	"testing"
)

func TestRemovePropRefusedWhileUsed(t *testing.T) {
	p := NewProject("t")
	AddProp(p.CurrentTrack, "hair", "hair_a")
	AddPart(p.CurrentTrack, NewSheetPart("Hair", "hair", "hair_a"))

	err := p.RemoveProp(0)
	var inUse *PropInUseError
	if !errors.As(err, &inUse) || len(inUse.Users) != 1 {
		t.Fatalf("err = %v, want a PropInUseError naming the part", err)
	}
	if p.CurrentTrack.FindProp("hair") == nil {
		t.Error("prop removed despite the refusal")
	}
	if p.UndoStack.CanUndo() {
		t.Error("a refused removal recorded an undo step")
	}
}

// A nested part passing the prop through to its animation uses it too.
func TestRemovePropRefusedWhilePassedThrough(t *testing.T) {
	p := NewProject("t")
	AddProp(p.CurrentTrack, "torch_base", "metal")
	torch := AddPart(p.CurrentTrack, NewNestedAniPart("Torch", "torch.anif"))
	torch.NestedBindings["torch_base"] = PropBinding{PassthroughFrom: "torch_base"}

	if err := p.RemoveProp(0); err == nil {
		t.Fatal("removed a prop a nested part passes through")
	}
}

func TestRemoveUnusedProp(t *testing.T) {
	p := NewProject("t")
	AddProp(p.CurrentTrack, "hair", "hair_a")
	p.PreviewProps["hair"] = "hair_b"

	if err := p.RemoveProp(0); err != nil {
		t.Fatal(err)
	}
	if len(p.CurrentTrack.Props) != 0 {
		t.Error("prop still there")
	}
	if _, ok := p.PreviewProps["hair"]; ok {
		t.Error("its preview override was left behind")
	}
	if !p.Undo() || p.CurrentTrack.FindProp("hair") == nil {
		t.Error("removal isn't undoable")
	}
}

// A nested part's fixed binding names a sheet: it's needed to draw, and
// removing it is refused like any other use.
func TestFixedBindingCountsAsSheetUse(t *testing.T) {
	tr := NewTrack("guard")
	torch := AddPart(tr, NewNestedAniPart("Torch", "torch.anif"))
	torch.NestedBindings["torch_base"] = PropBinding{StaticValue: "torchbase_metal"}
	torch.NestedBindings["flame"] = PropBinding{StaticValue: "blue_flame.anif"}

	if got := tr.ReferencedSheetNames(); len(got) != 1 || got[0] != "torchbase_metal" {
		t.Errorf("ReferencedSheetNames = %v", got)
	}
	if got := tr.SheetUsers("torchbase_metal"); len(got) != 1 {
		t.Errorf("SheetUsers = %v, want the torch part", got)
	}
	found := false
	for _, a := range tr.ReferencedAnimPaths() {
		found = found || a == "blue_flame.anif"
	}
	if !found {
		t.Errorf("ReferencedAnimPaths = %v, lacks the fixed .anif value", tr.ReferencedAnimPaths())
	}
}
