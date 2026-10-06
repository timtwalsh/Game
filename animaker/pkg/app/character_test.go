package app

import (
	"animaker/pkg/editor"
	"animaker/pkg/file"
	"animaker/pkg/ui"
	"path/filepath"
	"testing"
)

// A character's animations open as tracks; a new one is saved beside the
// .anichar from the open animation's rig, and joins the manifest.
func TestCharacterAddAndSelectAnimations(t *testing.T) {
	dir := t.TempDir()
	walk := editor.NewTrack("human_walk")
	editor.EnsureProp(walk, "hair", "hair_short")
	part := editor.AddPart(walk, editor.NewSheetPart("hair", "hair", "hair_short"))
	editor.AddKeyframe(walk.Directions[0], part.ID, 0)
	walkPath := filepath.Join(dir, "human_walk.anif")
	if err := file.SaveTrack(walk, walkPath, nil); err != nil {
		t.Fatal(err)
	}

	a := testApp(t)
	a.build()
	charPath := filepath.Join(dir, "human.anichar")
	a.openCharacter(editor.NewCharacter("human"), charPath)
	if !a.addAnimation("walk", walkPath) {
		t.Fatal("adding walk failed")
	}
	a.selectAnimation("walk")
	if a.Project.SavePath != walkPath || a.characterPanel.Open != "walk" {
		t.Fatalf("open %q / panel %q; want walk", a.Project.SavePath, a.characterPanel.Open)
	}

	a.addNewAnimation("idle", a.newAnimationPath("idle"), ui.FromRig, a.rigSource())
	idlePath := filepath.Join(dir, "human_idle.anif")
	tr := a.Project.CurrentTrack
	switch {
	case a.Project.SavePath != idlePath || a.Project.Dirty:
		t.Errorf("idle at %q dirty %v; want saved at %q", a.Project.SavePath, a.Project.Dirty, idlePath)
	case tr.Metadata.Name != "human_idle" || len(tr.Parts) != 1 || len(tr.Props) != 1:
		t.Errorf("idle is %q with %d parts, %d props; want walk's rig", tr.Metadata.Name, len(tr.Parts), len(tr.Props))
	case tr.Directions[0].TotalKeyframes() != 0:
		t.Error("the rig brought keyframes")
	case a.characterPanel.Open != "idle":
		t.Errorf("panel shows %q open", a.characterPanel.Open)
	}

	c, err := file.LoadCharacter(charPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Animations) != 2 || c.Find("idle").AnifPath != editor.AnimKey(idlePath) {
		t.Errorf("saved character: %+v", c.Animations)
	}

	// Markers show on the open animation's timeline only.
	a.character.Find("idle").SetMarker("blink", -1, 120)
	a.characterPanel.OnChanged()
	if got := a.timeline.Markers(); len(got) != 1 || got[0].Name != "blink" {
		t.Errorf("timeline markers %v", got)
	}
	a.selectAnimation("walk")
	if got := a.timeline.Markers(); len(got) != 0 {
		t.Errorf("walk shows idle's markers: %v", got)
	}
	if c, _ := file.LoadCharacter(charPath); len(c.Find("idle").Markers) != 1 {
		t.Error("marker edit wasn't saved")
	}

	a.onCloseCharacter()
	if a.Project.SavePath != walkPath || a.timeline.Markers() != nil {
		t.Error("closing the character should keep the track and drop its markers")
	}
}

// The character's footprint and hitboxes show on the canvas only while one
// of its animations is open, and a drag there saves the .anichar.
func TestCharacterShapesOnCanvas(t *testing.T) {
	dir := t.TempDir()
	walk := editor.NewTrack("ogre_walk")
	walkPath := filepath.Join(dir, "ogre_walk.anif")
	if err := file.SaveTrack(walk, walkPath, nil); err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	a.build()
	charPath := filepath.Join(dir, "ogre.anichar")
	c := editor.NewCharacter("ogre")
	c.SetFootprint(editor.Box{X: -10, Y: -5, W: 20, H: 10})
	a.openCharacter(c, charPath)
	if a.canvasWidget.Character() != nil {
		t.Error("shapes show while no animation of the character is open")
	}
	if !a.addAnimation("walk", walkPath) {
		t.Fatal("adding walk failed")
	}
	a.selectAnimation("walk")
	if a.canvasWidget.Character() != c {
		t.Fatal("shapes don't show while the character's walk is open")
	}

	a.characterPanel.OnSelectShape(ui.FootprintRef())
	if !a.canvasWidget.SelectedShape.Footprint {
		t.Fatal("selecting the footprint in the panel didn't select it on the canvas")
	}
	a.canvasWidget.OnShapeEdited(ui.FootprintRef(), editor.Box{X: -12, Y: -4, W: 24, H: 8})
	a.canvasWidget.OnShapeEditEnd()
	saved, err := file.LoadCharacter(charPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Footprint == nil || *saved.Footprint != (editor.Box{X: -12, Y: -4, W: 24, H: 8}) {
		t.Errorf("saved footprint = %+v", saved.Footprint)
	}

	a.onCloseCharacter()
	if a.canvasWidget.Character() != nil || !a.canvasWidget.SelectedShape.IsNone() {
		t.Error("closing the character left its shapes on the canvas")
	}
}
