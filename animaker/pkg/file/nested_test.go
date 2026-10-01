package file

import (
	"os"
	"path/filepath"
	"testing"

	"animaker/pkg/editor"
)

// writeTorch saves torch.anif (one flame part on its own "fire" sheet) in
// dir, the way the editor would have, and returns its path.
func writeTorch(t *testing.T, dir string) string {
	t.Helper()
	fire := importSheet(t, dir, "fire", "fire")
	track := editor.NewTrack("torch")
	flame := editor.AddPart(track, editor.NewSheetPart("flame", "", "fire"))
	editor.AddKeyframe(track.Directions[0], flame.ID, 0)
	path := filepath.Join(dir, "torch.anif")
	if err := SaveTrack(track, path, []SheetRef{{Name: "fire", SprshPath: fire.SprshPath}}); err != nil {
		t.Fatal(err)
	}
	return path
}

// Requested: torch.anif nested in walk_torch.anif, and reopening the
// parent brings the torch - with the torch's own sheet - back.
func TestReopenedTrackLoadsItsNestedAnimations(t *testing.T) {
	dir := t.TempDir()
	propsDir := filepath.Join(dir, "props")
	os.MkdirAll(propsDir, 0755)
	torchPath := writeTorch(t, propsDir)

	walk := editor.NewTrack("walk_torch")
	part := editor.AddPart(walk, editor.NewNestedAniPart("torch_1", editor.AnimKey(torchPath)))
	editor.AddKeyframe(walk.Directions[0], part.ID, 0)
	walkPath := filepath.Join(dir, "walk_torch.anif")
	if err := SaveTrack(walk, walkPath, nil); err != nil {
		t.Fatal(err)
	}

	loaded, _, err := LoadTrack(walkPath)
	if err != nil {
		t.Fatal(err)
	}
	anims := map[string]*editor.NestedAnim{}
	if problems := LoadNestedAnimsFor(loaded, anims); len(problems) > 0 {
		t.Fatalf("problems: %v", problems)
	}
	torch := anims[editor.AnimKey(torchPath)]
	if torch == nil {
		t.Fatalf("torch.anif not loaded; have %v", anims)
	}
	if torch.Sheets["fire"] == nil {
		t.Error("torch.anif's own sheet wasn't loaded with it")
	}
}

func TestMissingNestedAnimationIsAProblem(t *testing.T) {
	dir := t.TempDir()
	walk := editor.NewTrack("walk_torch")
	editor.AddPart(walk, editor.NewNestedAniPart("torch_1", filepath.Join(dir, "gone.anif")))

	problems := LoadNestedAnimsFor(walk, map[string]*editor.NestedAnim{})
	if len(problems) != 1 {
		t.Errorf("problems = %v, want one for gone.anif", problems)
	}
}

func TestSelfNestingAnimationLoadsOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loop.anif")
	track := editor.NewTrack("loop")
	editor.AddPart(track, editor.NewNestedAniPart("again", editor.AnimKey(path)))
	if err := SaveTrack(track, path, nil); err != nil {
		t.Fatal(err)
	}

	anims := map[string]*editor.NestedAnim{}
	if _, problems := LoadNestedAnim(path, anims); len(problems) > 0 {
		t.Fatalf("problems: %v", problems)
	}
	if len(anims) != 1 {
		t.Errorf("%d animations loaded, want 1", len(anims))
	}
}
