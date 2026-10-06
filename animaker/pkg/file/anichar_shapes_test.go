package file

import (
	"animaker/pkg/editor"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCharacterShapesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := editor.NewCharacter("ogre")
	c.Scale = 0.5
	c.SetFootprint(editor.Box{X: -20, Y: -6, W: 40, H: 12})
	c.AddHitbox()
	c.SetHitbox(0, editor.Hitbox{Name: "body", Kind: editor.ShapeOval, Box: editor.Box{X: -30, Y: -90, W: 60, H: 90}})
	c.AddHitbox()
	c.SetHitbox(1, editor.Hitbox{Name: "head", Kind: editor.ShapeCircle, Box: editor.Box{X: -10, Y: -110, W: 20, H: 20}})
	path := filepath.Join(dir, "ogre.anichar")
	if err := SaveCharacter(c, path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, want := range []string{"scale = 0.5", "[footprint]", "[[hitboxes]]", `shape = "oval"`, `name = "head"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("file lacks %s:\n%s", want, raw)
		}
	}
	got, err := LoadCharacter(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Errorf("round trip changed the character:\ngot  %+v\nwant %+v", got, c)
	}
}

func TestCharacterWithoutShapesWritesNoShapeKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torch.anichar")
	if err := SaveCharacter(editor.NewCharacter("torch"), path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, key := range []string{"scale", "footprint", "hitboxes"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("a character without %s wrote it:\n%s", key, raw)
		}
	}
}

func TestLoadCharacterRejectsBadShapes(t *testing.T) {
	cases := map[string]string{
		"zero footprint":   "[footprint]\nx = 0\ny = 0\nw = 0\nh = 4\n",
		"unknown shape":    "[[hitboxes]]\nname = \"body\"\nshape = \"star\"\nw = 4\nh = 4\n",
		"duplicate name":   "[[hitboxes]]\nname = \"body\"\nw = 4\nh = 4\n[[hitboxes]]\nname = \"body\"\nw = 4\nh = 4\n",
		"bad name":         "[[hitboxes]]\nname = \"my body\"\nw = 4\nh = 4\n",
		"negative scale":   "scale = -1\n",
		"zero-size hitbox": "[[hitboxes]]\nname = \"body\"\nw = 4\nh = 0\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "x.anichar")
			os.WriteFile(path, []byte("name = \"x\"\n"+body), 0o644)
			if _, err := LoadCharacter(path); err == nil {
				t.Errorf("loaded:\n%s", body)
			}
		})
	}
}
