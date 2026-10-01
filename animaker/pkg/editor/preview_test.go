package editor

import (
	"image"
	"testing"
)

func newTestSheet(name string, w, h, cellW, cellH int) *SpriteSheetTemplate {
	return NewSpriteSheetTemplate(name, name+".png", image.NewRGBA(image.Rect(0, 0, w, h)), cellW, cellH, 3, 4)
}

// A preview is sliced on the prop's default grid, as a runtime swap would
// be, and becomes the prop's override without touching the track.
func TestLoadPreviewSheetUsesThePropsGrid(t *testing.T) {
	p := NewProject("test")
	p.LoadedSheets["sprite"] = newTestSheet("sprite", 64, 48, 16, 24)
	EnsureProp(p.CurrentTrack, "body", "sprite")
	part := AddPart(p.CurrentTrack, NewSheetPart("body_1", "body", "sprite"))

	name, err := p.LoadPreviewSheet("body", "C:/art/sprite_red.png", image.NewRGBA(image.Rect(0, 0, 64, 48)))
	if err != nil {
		t.Fatal(err)
	}
	if name != "sprite_red" {
		t.Errorf("registered as %q, want sprite_red", name)
	}
	got := p.ResolveActiveSheet(part)
	if got == nil || got.Name != "sprite_red" {
		t.Fatalf("part resolves to %v, want the preview sheet", got)
	}
	if got.CellW != 16 || got.CellH != 24 || got.PivotX != 3 || got.PivotY != 4 {
		t.Errorf("preview grid = %dx%d pivot (%v,%v), want the default's 16x24 pivot (3,4)",
			got.CellW, got.CellH, got.PivotX, got.PivotY)
	}
	if _, inLoaded := p.LoadedSheets["sprite_red"]; inLoaded {
		t.Error("preview sheet leaked into LoadedSheets, where it could be chosen as authored art")
	}
	if p.CurrentTrack.Props[0].Default != "sprite" {
		t.Errorf("prop default changed to %q", p.CurrentTrack.Props[0].Default)
	}
}

func TestLoadPreviewSheetDoesNotShadowAnImportedSheet(t *testing.T) {
	p := NewProject("test")
	p.LoadedSheets["sprite"] = newTestSheet("sprite", 32, 32, 16, 16)
	EnsureProp(p.CurrentTrack, "body", "sprite")

	name, err := p.LoadPreviewSheet("body", "other/sprite.png", image.NewRGBA(image.Rect(0, 0, 32, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if name == "sprite" {
		t.Error("preview took an imported sheet's name")
	}
}

// While a preview shows, the palette still shows the authored sheet;
// dropping one of its tiles on the selected part must key that part, not
// spawn a new one.
func TestDropTileWhilePreviewingKeysTheSelectedPart(t *testing.T) {
	p := NewProject("test")
	p.LoadedSheets["sprite"] = newTestSheet("sprite", 32, 32, 16, 16)
	EnsureProp(p.CurrentTrack, "body", "sprite")
	idx, _ := p.DropTile("sprite", 0, 0, 0, 0)
	p.Selection.PartIndex = idx
	if _, err := p.LoadPreviewSheet("body", "sprite_red.png", image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}

	p.Seek(200)
	p.DropTile("sprite", 0, 1, 0, 0)

	if got := len(p.CurrentTrack.Parts); got != 1 {
		t.Errorf("rig has %d parts, want 1", got)
	}
}
