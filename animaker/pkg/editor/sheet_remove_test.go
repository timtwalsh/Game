package editor

import (
	"errors"
	"testing"
)

// Reported: an accidentally imported "flame.png" was stuck in the .anif
// with no way to remove it.
func TestRemoveUnusedSheet(t *testing.T) {
	p := NewProject("t")
	p.LoadedSheets["body"] = newTestSheet("body", 32, 32, 16, 16)
	p.LoadedSheets["flame"] = newTestSheet("flame", 32, 32, 16, 16)
	p.PaletteSheet = "flame"

	if err := p.RemoveSheet("flame"); err != nil {
		t.Fatal(err)
	}
	if _, still := p.LoadedSheets["flame"]; still {
		t.Error("flame is still loaded")
	}
	if p.PaletteSheet == "flame" {
		t.Error("palette still shows the removed sheet")
	}
	if _, ok := p.LoadedSheets["body"]; !ok {
		t.Error("removing flame unloaded body too")
	}
}

func TestRemoveSheetRefusedWhileInUse(t *testing.T) {
	p := NewProject("t")
	p.LoadedSheets["flame"] = newTestSheet("flame", 32, 32, 16, 16)
	AddPart(p.CurrentTrack, NewSheetPart("torch_fire", "", "flame"))
	EnsureProp(p.CurrentTrack, "fire", "flame")

	err := p.RemoveSheet("flame")
	var inUse *SheetInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("err = %v, want SheetInUseError", err)
	}
	if len(inUse.Users) != 2 {
		t.Errorf("users = %v, want the part and the prop", inUse.Users)
	}
	if _, ok := p.LoadedSheets["flame"]; !ok {
		t.Error("a refused removal still unloaded the sheet")
	}
}

func TestRemoveSheetClearsPreviewOverridesNamingIt(t *testing.T) {
	p := NewProject("t")
	p.LoadedSheets["body"] = newTestSheet("body", 32, 32, 16, 16)
	p.LoadedSheets["body_alt"] = newTestSheet("body_alt", 32, 32, 16, 16)
	EnsureProp(p.CurrentTrack, "body", "body")
	p.PreviewProps["body"] = "body_alt"

	if err := p.RemoveSheet("body_alt"); err != nil {
		t.Fatal(err)
	}
	if v, ok := p.PreviewProps["body"]; ok {
		t.Errorf("preview override still names the removed sheet: %q", v)
	}
}
