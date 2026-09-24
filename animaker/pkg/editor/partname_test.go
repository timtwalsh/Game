package editor

import "testing"

// Dragging a tile out of the palette names the new part after its sheet,
// so dropping several tiles from one sheet must not produce a rig full of
// identically-named parts - the part list and timeline are labelled by
// name, and identical labels make them unusable even though identity is
// really the ID.
func TestUniquePartNameAvoidsCollisions(t *testing.T) {
	track := NewTrack("human_walk")

	var names []string
	for i := 0; i < 3; i++ {
		name := UniquePartName(track, "sprite")
		AddPart(track, NewSheetPart(name, "", "sprite"))
		names = append(names, name)
	}

	want := []string{"sprite_1", "sprite_2", "sprite_3"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("part %d named %q, want %q", i, names[i], w)
		}
	}

	// A gap left by a deletion gets reused for the name (unlike the ID,
	// which must never be reused) - names are cosmetic, so the lowest free
	// suffix is the least surprising choice.
	if err := RemovePart(track, 1); err != nil { // removes sprite_2
		t.Fatalf("RemovePart: %v", err)
	}
	if got := UniquePartName(track, "sprite"); got != "sprite_2" {
		t.Errorf("after removing sprite_2, next name = %q, want sprite_2", got)
	}
}

func TestUniquePartNameIsPerBase(t *testing.T) {
	track := NewTrack("human_walk")
	AddPart(track, NewSheetPart(UniquePartName(track, "body"), "", "body"))

	if got := UniquePartName(track, "hair"); got != "hair_1" {
		t.Errorf("first hair part named %q, want hair_1 - an unrelated base shouldn't be crowded out", got)
	}
}

// The palette stands on its own now, so it must resolve a sheet with no
// part selected at all.
func TestPaletteSheetTemplateIndependentOfSelection(t *testing.T) {
	p := NewProject("test")
	if p.PaletteSheetTemplate() != nil {
		t.Error("a fresh project resolved a palette sheet, want nil")
	}

	sheet := NewSpriteSheetTemplate("sprite", "sprite.png",
		buildTestSheetImage(2, 2, 16, 16), 16, 16, 8, 8)
	p.LoadedSheets["sprite"] = sheet
	p.PaletteSheet = "sprite"

	if p.Selection.PartIndex != -1 {
		t.Fatalf("precondition: a part is selected (%d)", p.Selection.PartIndex)
	}
	if got := p.PaletteSheetTemplate(); got != sheet {
		t.Errorf("palette resolved %v with nothing selected, want the sheet itself", got)
	}
}
