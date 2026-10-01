package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"animaker/pkg/editor"
)

// importSheet does what the editor's import does on disk: a PNG with a
// .sprsh beside it, named after the image file.
func importSheet(t *testing.T, dir, imageName, sheetName string) *editor.SpriteSheetTemplate {
	t.Helper()
	imgPath := filepath.Join(dir, imageName+".png")
	writeTestPNG(t, imgPath, 64, 32)
	img, err := LoadImage(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	s := editor.NewSpriteSheetTemplate(sheetName, imgPath, img, 16, 16, 8, 8)
	s.SprshPath = filepath.Join(dir, imageName+".sprsh")
	if err := SaveSheetTemplate(s, s.SprshPath); err != nil {
		t.Fatal(err)
	}
	return s
}

func trackUsing(sheets ...string) *editor.Track {
	track := editor.NewTrack("walk")
	for _, s := range sheets {
		part := editor.AddPart(track, editor.NewSheetPart(s+"_1", "", s))
		editor.AddKeyframe(track.Directions[0], part.ID, 0)
	}
	return track
}

// Regression: reported as "I saved my WIP .anif file and when I loaded it
// again it failed to load the sprite sheet". Nothing loaded sheets on open.
func TestReopenedTrackLoadsItsSheets(t *testing.T) {
	dir := t.TempDir()
	artDir := filepath.Join(dir, "art", "elsewhere", "deep", "down")
	if err := os.MkdirAll(artDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Deliberately outside the .anif's folder search, so this passes only
	// via the path the .anif recorded.
	sheet := importSheet(t, artDir, "sprite_v2", "sprite")

	anif := filepath.Join(dir, "walk.anif")
	if err := SaveTrack(trackUsing("sprite"), anif, []SheetRef{{Name: "sprite", SprshPath: sheet.SprshPath}}); err != nil {
		t.Fatal(err)
	}

	track, refs, err := LoadTrack(anif)
	if err != nil {
		t.Fatal(err)
	}
	sheets, missing, problems := LoadSheetsForTrack(anif, refs, track.ReferencedSheetNames())
	if len(missing) > 0 || len(problems) > 0 {
		t.Fatalf("missing %v, problems %v", missing, problems)
	}
	if s := sheets["sprite"]; s == nil || s.CellW != 16 || s.Cols() != 4 {
		t.Errorf("sprite sheet = %+v, want the 16px-cell, 4-column sheet", s)
	}
}

// A track saved before the .anif recorded sheet paths (the user's WIP
// file) still opens with its art when the sheet sits beside it - and the
// .sprsh is found by the name inside it, not its filename.
func TestOldTrackFindsSheetBesideIt(t *testing.T) {
	dir := t.TempDir()
	importSheet(t, dir, "character_sheet", "sprite")

	anif := filepath.Join(dir, "walk.anif")
	if err := SaveTrack(trackUsing("sprite"), anif, nil); err != nil {
		t.Fatal(err)
	}
	track, refs, err := LoadTrack(anif)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("precondition: old-style track has refs %v", refs)
	}

	sheets, missing, _ := LoadSheetsForTrack(anif, refs, track.ReferencedSheetNames())
	if sheets["sprite"] == nil || len(missing) > 0 {
		t.Errorf("sprite not found beside the .anif (missing %v)", missing)
	}
}

// Moving a whole folder (track + art) leaves the recorded paths valid,
// since they're relative to the .anif.
func TestRecordedSheetPathsAreRelative(t *testing.T) {
	dir := t.TempDir()
	sheet := importSheet(t, dir, "sprite", "sprite")
	anif := filepath.Join(dir, "walk.anif")
	if err := SaveTrack(trackUsing("sprite"), anif, []SheetRef{{Name: "sprite", SprshPath: sheet.SprshPath}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(anif)
	if err != nil {
		t.Fatal(err)
	}
	if want := `path = "sprite.sprsh"`; !strings.Contains(string(data), want) {
		t.Errorf(".anif doesn't record %s:\n%s", want, data)
	}
}

func TestUnfoundSheetIsReportedMissing(t *testing.T) {
	dir := t.TempDir()
	anif := filepath.Join(dir, "walk.anif")
	if err := SaveTrack(trackUsing("sprite"), anif, nil); err != nil {
		t.Fatal(err)
	}
	track, refs, _ := LoadTrack(anif)

	_, missing, _ := LoadSheetsForTrack(anif, refs, track.ReferencedSheetNames())
	if len(missing) != 1 || missing[0] != "sprite" {
		t.Errorf("missing = %v, want [sprite]", missing)
	}
}

// A .sprsh that exists but whose image was deleted is a problem worth
// naming, not a silent miss.
func TestSheetWithMissingImageIsAProblem(t *testing.T) {
	dir := t.TempDir()
	sheet := importSheet(t, dir, "sprite", "sprite")
	if err := os.Remove(filepath.Join(dir, "sprite.png")); err != nil {
		t.Fatal(err)
	}
	anif := filepath.Join(dir, "walk.anif")
	if err := SaveTrack(trackUsing("sprite"), anif, []SheetRef{{Name: "sprite", SprshPath: sheet.SprshPath}}); err != nil {
		t.Fatal(err)
	}
	track, refs, _ := LoadTrack(anif)

	_, missing, problems := LoadSheetsForTrack(anif, refs, track.ReferencedSheetNames())
	if len(problems) == 0 {
		t.Error("no problem reported for a .sprsh whose image is gone")
	}
	if len(missing) != 1 {
		t.Errorf("missing = %v, want [sprite]", missing)
	}
}
