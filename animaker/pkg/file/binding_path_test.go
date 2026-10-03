package file

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"animaker/pkg/editor"
)

// A fixed binding naming an .anif is written relative to the track, like
// every other nested path, and comes back absolute.
func TestFixedAnimBindingPathIsRelative(t *testing.T) {
	dir := t.TempDir()
	tr := editor.NewTrack("guard")
	torch := editor.AddPart(tr, editor.NewNestedAniPart("Torch", filepath.Join(dir, "torch.anif")))
	flame := filepath.Join(dir, "fx", "blue_flame.anif")
	torch.NestedBindings["flame"] = editor.PropBinding{StaticValue: flame}

	path := filepath.Join(dir, "guard.anif")
	if err := SaveTrack(tr, path, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `static_value = "fx/blue_flame.anif"`) {
		t.Errorf("binding not saved relative:\n%s", data)
	}
	got, _, err := LoadTrack(path)
	if err != nil {
		t.Fatal(err)
	}
	if v := got.Parts[0].NestedBindings["flame"].StaticValue; v != editor.AnimKey(flame) {
		t.Errorf("loaded %q, want %q", v, editor.AnimKey(flame))
	}
}
