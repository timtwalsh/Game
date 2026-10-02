package file

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"animaker/pkg/editor"
)

// A hand-edited .anif with keyframes out of order, and two at one time,
// loads sorted with one keyframe per time (the later one in the file).
func TestLoadTrackSortsKeyframes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "walk.anif")
	src := `[metadata]
name = "walk"

[[parts]]
id = 1
name = "Body"
kind = "sheet"
fixed_sheet = "body"

[[directions.0.keyframes]]
part_id = 1
time_ms = 400
x = 4.0
[[directions.0.keyframes]]
part_id = 1
time_ms = 0
x = 0.0
[[directions.0.keyframes]]
part_id = 1
time_ms = 400
x = 9.0
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	tr, _, err := LoadTrack(path)
	if err != nil {
		t.Fatal(err)
	}
	kfs := tr.Directions[0].KeyframesFor(1)
	if len(kfs) != 2 || kfs[0].TimeMs != 0 || kfs[1].TimeMs != 400 || kfs[1].X != 9 {
		t.Fatalf("keyframes = %+v %+v, want 0ms then 400ms (x=9)", *kfs[0], *kfs[len(kfs)-1])
	}
	for i, kf := range kfs {
		if kf.ID != i {
			t.Errorf("keyframe %d has ID %d", i, kf.ID)
		}
	}
}

// A .sprsh stores its image path with forward slashes, as written on any
// platform, and loads back from a subfolder.
func TestSprshImagePathUsesForwardSlashes(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "art", "hair.png")
	os.MkdirAll(filepath.Dir(imgPath), 0755)
	f, _ := os.Create(imgPath)
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 16, 16)))
	f.Close()

	sprsh := filepath.Join(dir, "hair.sprsh")
	tmpl := editor.NewSpriteSheetTemplate("hair", imgPath, image.NewRGBA(image.Rect(0, 0, 16, 16)), 8, 8, 4, 4)
	if err := SaveSheetTemplate(tmpl, sprsh); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(sprsh)
	if !strings.Contains(string(data), `file_path = "art/hair.png"`) {
		t.Errorf("image path not stored with forward slashes:\n%s", data)
	}
	if _, err := LoadSheetTemplate(sprsh); err != nil {
		t.Errorf("reload: %v", err)
	}
}

// Saving replaces the file whole and leaves no temp file behind.
func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "walk.anif")
	for i := 0; i < 2; i++ { // the second save replaces an existing file
		if err := SaveTrack(editor.NewTrack("walk"), path, nil); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("files after saving: %v, want only walk.anif", names)
	}
}
