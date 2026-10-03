package file

import (
	"animaker/pkg/editor"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCharacterRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := editor.NewCharacter("human")
	c.Controller = "humanoid"
	walk, _ := c.AddAnimation("walk", filepath.Join(dir, "human_walk.anif"))
	walk.SetMarker("footstep", -1, 250)
	walk.SetMarker("footstep", -1, 0)
	atk, _ := c.AddAnimation("sword_attack", filepath.Join(dir, "anims", "human_sword_attack.anif"))
	atk.Mode = editor.PlayOnce
	atk.SetMarker("hit", -1, 300)
	path := filepath.Join(dir, "human.anichar")
	if err := SaveCharacter(c, path); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(path)
	for _, want := range []string{`anif = "anims/human_sword_attack.anif"`, `hit = 300`, `footstep = [0, 250]`, `mode = "once"`} {
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

func TestLoadCharacterIssueExample(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "human.anichar")
	os.WriteFile(path, []byte(`name = "human"
controller = "humanoid"

[animations.idle]
anif = "human_idle.anif"
mode = "loop"

[animations.walk]
anif = "human_walk.anif"
mode = "loop"
markers = { footstep = [0, 250] }

[animations.sword_attack]
anif = "human_sword_attack.anif"
mode = "once"
markers = { hit = 300, can_cancel = 450 }

[animations.death]
anif = "human_death.anif"
mode = "hold"
`), 0644)
	c, err := LoadCharacter(path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range c.Animations {
		names = append(names, a.Name)
	}
	if want := []string{"death", "idle", "sword_attack", "walk"}; !reflect.DeepEqual(names, want) {
		t.Errorf("animations %v, want %v", names, want)
	}
	atk := c.Find("sword_attack")
	if atk.Mode != editor.PlayOnce || atk.AnifPath != editor.AnimKey(filepath.Join(dir, "human_sword_attack.anif")) {
		t.Errorf("sword_attack = %+v", atk)
	}
	if want := []editor.Marker{{Name: "hit", TimeMs: 300}, {Name: "can_cancel", TimeMs: 450}}; !reflect.DeepEqual(atk.Markers, want) {
		t.Errorf("markers %v, want %v", atk.Markers, want)
	}
	if c.Find("death").Mode != editor.PlayHold {
		t.Error("death should hold")
	}
}

func TestLoadCharacterRejectsBadInput(t *testing.T) {
	for name, body := range map[string]string{
		"bad mode":      "name = \"x\"\n[animations.a]\nanif = \"a.anif\"\nmode = \"pingpong\"\n",
		"no anif":       "name = \"x\"\n[animations.a]\nmode = \"loop\"\n",
		"negative time": "name = \"x\"\n[animations.a]\nanif = \"a.anif\"\nmarkers = { hit = -5 }\n",
		"text time":     "name = \"x\"\n[animations.a]\nanif = \"a.anif\"\nmarkers = { hit = \"soon\" }\n",
		"bad name":      "name = \"x\"\n[animations.\"sword attack\"]\nanif = \"a.anif\"\n",
	} {
		path := filepath.Join(t.TempDir(), "c.anichar")
		os.WriteFile(path, []byte(body), 0644)
		if _, err := LoadCharacter(path); err == nil {
			t.Errorf("%s: loaded without error", name)
		}
	}
}
