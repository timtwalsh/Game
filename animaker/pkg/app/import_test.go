package app

import (
	"image"
	"path/filepath"
	"testing"

	"animaker/pkg/editor"
	"animaker/pkg/file"
	"animaker/pkg/ui"
)

// An image with a saved template: re-importing prefills from it, and only
// changed settings count as replacing it.
func TestImportPrefillsAndFlagsAChangedTemplate(t *testing.T) {
	a := testApp(t)
	dir := t.TempDir()
	img := filepath.Join(dir, "hair.png")
	saved := editor.NewSpriteSheetTemplate("hair_long", img, image.NewRGBA(image.Rect(0, 0, 64, 64)), 32, 48, 16, 40)
	if err := file.SaveSheetTemplate(saved, file.SprshPathFor(img)); err != nil {
		t.Fatal(err)
	}

	pre, ok := existingSheetSettings(img)
	want := ui.SheetImport{Name: "hair_long", CellW: 32, CellH: 48, PivotX: 16, PivotY: 40}
	if !ok || pre != want {
		t.Fatalf("prefill = %+v, %v; want %+v", pre, ok, want)
	}

	same := want
	same.FilePath = img
	if got := a.importReplaces(same); len(got) != 0 {
		t.Errorf("unchanged re-import flagged: %v", got)
	}
	changed := same
	changed.CellW = 16
	if got := a.importReplaces(changed); len(got) != 1 {
		t.Errorf("changed grid: replaces = %v, want the saved template", got)
	}
}

func TestImportFlagsALoadedSheetOfTheSameName(t *testing.T) {
	a := testApp(t)
	dir := t.TempDir()
	px := image.NewRGBA(image.Rect(0, 0, 32, 32))
	a.Project.LoadedSheets["body"] = editor.NewSpriteSheetTemplate("body", filepath.Join(dir, "body.png"), px, 16, 16, 8, 8)
	editor.AddPart(a.Project.CurrentTrack, editor.NewSheetPart("Body", "", "body"))

	other := ui.SheetImport{FilePath: filepath.Join(dir, "body_v2.png"), Name: "body", CellW: 16, CellH: 16, PivotX: 8, PivotY: 8}
	got := a.importReplaces(other)
	if len(got) != 1 {
		t.Fatalf("replaces = %v, want the loaded sheet", got)
	}
	if fresh := (ui.SheetImport{FilePath: filepath.Join(dir, "x.png"), Name: "new", CellW: 16, CellH: 16}); len(a.importReplaces(fresh)) != 0 {
		t.Error("a new sheet was flagged as replacing something")
	}
}

func TestNoPrefillForANewImage(t *testing.T) {
	if _, ok := existingSheetSettings(filepath.Join(t.TempDir(), "new.png")); ok {
		t.Error("prefill for an image with no .sprsh")
	}
}
