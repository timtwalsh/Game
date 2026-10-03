package file

import (
	"animaker/pkg/editor"
	"os"
	"path/filepath"
	"testing"
)

// The direction count is saved, and an older file without it is judged by
// its keys.
func TestDirectionCountRoundTripsAndIsInferred(t *testing.T) {
	dir := t.TempDir()
	tr := editor.NewTrack("walk")
	p := editor.NewProject("walk")
	p.CurrentTrack = tr
	if _, err := p.SetDirectionCount(8, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "walk.anif")
	if err := SaveTrack(tr, path, nil); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadTrack(path)
	if err != nil || loaded.DirectionCount != 8 || len(loaded.Directions) != 8 {
		t.Fatalf("loaded count %d, %d directions, err %v; want 8 and 8", loaded.DirectionCount, len(loaded.Directions), err)
	}

	old := filepath.Join(dir, "old.anif")
	write(t, old, "[metadata]\nname = \"old\"\n\n[directions.0]\n[directions.3]\n")
	loaded, _, err = LoadTrack(old)
	if err != nil || loaded.Facings() != 4 {
		t.Errorf("old file: facings %d, err %v; want 4", loaded.Facings(), err)
	}
}

func TestDirectionOutsideTheCountIsRejected(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"key 5 of 4": "[metadata]\nname = \"x\"\ndirections = 4\n\n[directions.5]\n",
		"count 12":   "[metadata]\nname = \"x\"\ndirections = 12\n\n[directions.0]\n",
	} {
		path := filepath.Join(dir, "x.anif")
		write(t, path, body)
		if _, _, err := LoadTrack(path); err == nil {
			t.Errorf("%s: loaded, want an error", name)
		}
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
