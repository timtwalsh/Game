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

// Requested: a nested part's direction can be static, inherited, or set per
// keyframe - all three must survive save and load.
func TestNestedDirectionModesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	track := editor.NewTrack("walk_torch")
	static := editor.AddPart(track, editor.NewNestedAniPart("torch_static", filepath.Join(dir, "torch.anif")))
	static.DirectionMode, static.StaticDirection = editor.NestedDirStatic, 3
	perKf := editor.AddPart(track, editor.NewNestedAniPart("torch_turning", filepath.Join(dir, "torch.anif")))
	perKf.DirectionMode = editor.NestedDirPerKeyframe
	editor.AddKeyframe(track.Directions[0], perKf.ID, 0).Direction = 1
	editor.AddKeyframe(track.Directions[0], perKf.ID, 200).Direction = 2
	inherit := editor.AddPart(track, editor.NewNestedAniPart("torch_follow", filepath.Join(dir, "torch.anif")))
	_ = inherit

	path := filepath.Join(dir, "walk_torch.anif")
	if err := SaveTrack(track, path, nil); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadTrack(path)
	if err != nil {
		t.Fatal(err)
	}

	if p := loaded.Parts[0]; p.DirectionMode != editor.NestedDirStatic || p.StaticDirection != 3 {
		t.Errorf("static part = mode %v dir %d, want static 3", p.DirectionMode, p.StaticDirection)
	}
	p := loaded.Parts[1]
	if p.DirectionMode != editor.NestedDirPerKeyframe {
		t.Errorf("per-keyframe part mode = %v", p.DirectionMode)
	}
	kfs := loaded.Directions[0].KeyframesFor(p.ID)
	if len(kfs) != 2 || kfs[0].Direction != 1 || kfs[1].Direction != 2 {
		t.Errorf("per-keyframe directions didn't round-trip: %+v %+v", *kfs[0], *kfs[1])
	}
	if p := loaded.Parts[2]; p.DirectionMode != editor.NestedDirInherit {
		t.Errorf("inherit part mode = %v", p.DirectionMode)
	}
}

// Files saved before direction modes pinned a direction with a "direction"
// binding; that becomes static.
func TestLegacyStaticDirectionBindingMigrates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.anif")
	legacy := `[metadata]
name = "old"
version = "1.0"

[[parts]]
id = 1
name = "torch_1"
kind = "nested_ani"
nested_ani_path = "torch.anif"

[parts.nested_bindings.direction]
static_value = "2"

[directions.0]
`
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadTrack(path)
	if err != nil {
		t.Fatal(err)
	}
	p := loaded.Parts[0]
	if p.DirectionMode != editor.NestedDirStatic || p.StaticDirection != 2 {
		t.Errorf("migrated to mode %v dir %d, want static 2", p.DirectionMode, p.StaticDirection)
	}
	if len(p.NestedBindings) != 0 {
		t.Errorf("legacy binding kept: %+v", p.NestedBindings)
	}
}
