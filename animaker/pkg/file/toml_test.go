package file

import (
	"animaker/pkg/editor"
	"path/filepath"
	"testing"
)

func TestSaveLoadTrackRoundTrip(t *testing.T) {
	track := editor.NewTrack("human_walk")
	editor.AddProp(track, "hair", "long_blonde")
	editor.AddProp(track, "arms", "leather")

	dir := editor.AddDirection(track, 2) // 2 = "down" by the game's direction convention

	hair := editor.NewSheetPart("Hair", "hair", "")
	editor.AddKeyframe(hair, 0).Row = 4
	editor.AddKeyframe(hair, 0).Col = 2 // same call target as above (time 0) - sets Col too
	kf := editor.AddKeyframe(hair, 100)
	kf.X, kf.Y, kf.Z, kf.RotationDeg = 1.5, -2.5, 20, 90
	editor.AddPart(dir, hair)

	body := editor.NewSheetPart("Body", "", "human_body_default")
	editor.AddPart(dir, body)

	torch := editor.NewNestedAniPart("Torch", "base_wood_torch.anif")
	torch.NestedBindings["direction"] = editor.PropBinding{PassthroughFrom: "direction"}
	torch.NestedBindings["torch_sheet"] = editor.PropBinding{StaticValue: "rusty"}
	editor.AddKeyframe(torch, 0)
	editor.AddPart(dir, torch)

	path := filepath.Join(t.TempDir(), "human_walk.anif")
	if err := SaveTrack(track, path); err != nil {
		t.Fatalf("SaveTrack failed: %v", err)
	}

	loaded, err := LoadTrack(path)
	if err != nil {
		t.Fatalf("LoadTrack failed: %v", err)
	}

	if loaded.Metadata.Name != "human_walk" {
		t.Errorf("loaded name = %q, want %q", loaded.Metadata.Name, "human_walk")
	}
	if loaded.RefBoxWidth != track.RefBoxWidth || loaded.RefBoxHeight != track.RefBoxHeight {
		t.Errorf("loaded ref box = %dx%d, want %dx%d",
			loaded.RefBoxWidth, loaded.RefBoxHeight, track.RefBoxWidth, track.RefBoxHeight)
	}
	// NewTrack seeds 0-3 and the test adds 2, which already exists.
	if got := loaded.SortedDirectionKeys(); len(got) != 4 {
		t.Errorf("loaded directions = %v, want the 4 defaults", got)
	}
	if len(loaded.Props) != 2 {
		t.Fatalf("loaded %d props, want 2", len(loaded.Props))
	}
	if got := loaded.FindProp("hair"); got == nil || got.Default != "long_blonde" {
		t.Errorf("loaded 'hair' prop = %+v, want default long_blonde", got)
	}

	ldir, ok := loaded.Directions[2]
	if !ok {
		t.Fatalf("loaded track missing direction 2")
	}
	if len(ldir.Parts) != 3 {
		t.Fatalf("loaded %d parts, want 3", len(ldir.Parts))
	}

	var lHair, lBody, lTorch *editor.Part
	for _, p := range ldir.Parts {
		switch p.Name {
		case "Hair":
			lHair = p
		case "Body":
			lBody = p
		case "Torch":
			lTorch = p
		}
	}

	if lHair == nil || lHair.Kind != editor.PartKindSheet || lHair.GoverningProp != "hair" {
		t.Fatalf("Hair part round-tripped wrong: %+v", lHair)
	}
	if len(lHair.Keyframes) != 2 || lHair.Keyframes[1].X != 1.5 || lHair.Keyframes[1].RotationDeg != 90 {
		t.Errorf("Hair keyframes round-tripped wrong: %+v", lHair.Keyframes)
	}

	if lBody == nil || lBody.Kind != editor.PartKindSheet || lBody.FixedSheet != "human_body_default" {
		t.Fatalf("Body part round-tripped wrong: %+v", lBody)
	}

	if lTorch == nil || lTorch.Kind != editor.PartKindNestedAni || lTorch.NestedAniPath != "base_wood_torch.anif" {
		t.Fatalf("Torch part round-tripped wrong: %+v", lTorch)
	}
	if b := lTorch.NestedBindings["direction"]; b.PassthroughFrom != "direction" {
		t.Errorf("Torch direction binding round-tripped wrong: %+v", b)
	}
	if b := lTorch.NestedBindings["torch_sheet"]; b.StaticValue != "rusty" {
		t.Errorf("Torch torch_sheet binding round-tripped wrong: %+v", b)
	}
}

func TestSaveLoadSheetTemplateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "human_hair_v2_template.png")
	writeTestPNG(t, imgPath, 128, 192)

	tmpl := editor.NewSpriteSheetTemplate("human_hair_v2_template", imgPath, nil, 32, 48, 16, 24)
	sprshPath := filepath.Join(dir, "human_hair_v2_template.sprsh")
	if err := SaveSheetTemplate(tmpl, sprshPath); err != nil {
		t.Fatalf("SaveSheetTemplate failed: %v", err)
	}

	loaded, err := LoadSheetTemplate(sprshPath)
	if err != nil {
		t.Fatalf("LoadSheetTemplate failed: %v", err)
	}

	if loaded.Name != "human_hair_v2_template" {
		t.Errorf("loaded name = %q, want %q", loaded.Name, "human_hair_v2_template")
	}
	if loaded.CellW != 32 || loaded.CellH != 48 {
		t.Errorf("loaded cell size = %dx%d, want 32x48", loaded.CellW, loaded.CellH)
	}
	if loaded.PivotX != 16 || loaded.PivotY != 24 {
		t.Errorf("loaded pivot = (%v,%v), want (16,24)", loaded.PivotX, loaded.PivotY)
	}
	if loaded.Cols() != 4 || loaded.Rows() != 4 {
		t.Errorf("derived grid = %dx%d cells, want 4x4 for a 128x192 image at 32x48 cells", loaded.Cols(), loaded.Rows())
	}
}
