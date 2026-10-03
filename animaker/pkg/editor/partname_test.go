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

func TestRenamePart(t *testing.T) {
	track := NewTrack("t")
	AddPart(track, NewSheetPart("sprite_1", "", "sprite"))
	AddPart(track, NewSheetPart("sprite_2", "", "sprite"))

	if err := RenamePart(track, 0, "  head  "); err != nil {
		t.Fatalf("rename to head: %v", err)
	}
	if got := track.Parts[0].Name; got != "head" {
		t.Errorf("name = %q, want %q (trimmed)", got, "head")
	}
	if err := RenamePart(track, 1, "head"); err != ErrPartNameTaken {
		t.Errorf("rename onto a taken name: err = %v, want ErrPartNameTaken", err)
	}
	if err := RenamePart(track, 1, "   "); err != ErrPartNameEmpty {
		t.Errorf("rename to blank: err = %v, want ErrPartNameEmpty", err)
	}
	if got := track.Parts[1].Name; got != "sprite_2" {
		t.Errorf("refused rename changed the name to %q", got)
	}
	if err := RenamePart(track, 0, "head"); err != nil {
		t.Errorf("rename to its own name: %v", err)
	}
}

func TestCheckNewPart(t *testing.T) {
	tr := NewTrack("t")
	AddProp(tr, "hair", "hair_a")
	AddProp(tr, "held", "torch.anif")
	AddPart(tr, NewSheetPart("Body", "", "body"))

	gov := func(p *Part, prop string) *Part { p.GoverningProp = prop; return p }
	for _, tc := range []struct {
		name string
		part *Part
		ok   bool
	}{
		{"sheet with fixed sheet", NewSheetPart(" Head ", "", "head"), true},
		{"sheet with sheet prop", NewSheetPart("Hair", "hair", ""), true},
		{"sheet with nothing", NewSheetPart("Hair", "", ""), false},
		{"sheet with anim prop", NewSheetPart("Hair", "held", ""), false},
		{"nested with path", NewNestedAniPart("Torch", "torch.anif"), true},
		{"nested with anim prop", gov(NewNestedAniPart("Torch", ""), "held"), true},
		{"nested with sheet prop", gov(NewNestedAniPart("Torch", "torch.anif"), "hair"), false},
		{"nested with nothing", NewNestedAniPart("Torch", ""), false},
		{"taken name", NewSheetPart("Body", "", "body"), false},
		{"blank name", NewSheetPart("  ", "", "body"), false},
		{"unknown prop", NewSheetPart("X", "nope", ""), false},
	} {
		err := CheckNewPart(tr, tc.part)
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
	p := NewSheetPart(" Head ", "", "head")
	CheckNewPart(tr, p)
	if p.Name != "Head" {
		t.Errorf("name not trimmed: %q", p.Name)
	}
}
