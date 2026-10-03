package lint

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"animaker/pkg/editor"
	"animaker/pkg/file"
)

// keyedAt gives a track one part keyed at 0 and endMs, so it lasts endMs.
func keyedAt(endMs uint32) func(*editor.Track) {
	return func(tr *editor.Track) {
		p := editor.AddPart(tr, &editor.Part{Name: "Body", Kind: editor.PartKindSheet})
		editor.AddKeyframe(tr.Directions[0], p.ID, 0)
		editor.AddKeyframe(tr.Directions[0], p.ID, endMs)
	}
}

func saveChar(t *testing.T, dir string, c *editor.Character) string {
	t.Helper()
	path := filepath.Join(dir, c.Name+".anichar")
	if err := file.SaveCharacter(c, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCharacterIsItsOwnFamily(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"hair": "long"}, keyedAt(500))
	idle := save(t, dir, "human_idle", map[string]string{"hair": "long"}, keyedAt(500))
	// Matches human_* but isn't one of the character's animations.
	save(t, dir, "human_portrait", map[string]string{}, nil)

	c := editor.NewCharacter("human")
	w, _ := c.AddAnimation("walk", walk)
	w.SetMarker("footstep", -1, 250)
	c.AddAnimation("idle", idle)
	if fs := Character(saveChar(t, dir, c)); len(fs) != 0 {
		t.Errorf("findings for a clean character:\n%s", messages(fs))
	}
}

func TestCharacterChecksPropsAndMarkers(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"hair": "long"}, keyedAt(500))
	idle := save(t, dir, "human_idle", nil, keyedAt(500))
	c := editor.NewCharacter("human")
	w, _ := c.AddAnimation("walk", walk)
	w.SetMarker("footstep", -1, 900)
	c.AddAnimation("idle", idle)
	c.AddAnimation("run", filepath.Join(dir, "human_run.anif")) // never saved
	fs := Character(saveChar(t, dir, c))
	msgs := messages(fs)
	for _, want := range []string{`lacks prop "hair"`, `marker "footstep" at 900ms is past the track's end (500ms)`, "human_run.anif: can't read"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("want %q in:\n%s", want, msgs)
		}
	}
}

func TestRunLintsCharacters(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"hair": "long"}, keyedAt(500))
	c := editor.NewCharacter("human")
	c.AddAnimation("walk", walk)
	saveChar(t, dir, c)
	var out bytes.Buffer
	if code := Run([]string{filepath.Join(dir, "*.anichar")}, &out); code != 0 {
		t.Errorf("exit %d for a clean character:\n%s", code, out.String())
	}
	c.Find("walk").SetMarker("hit", -1, 5000)
	saveChar(t, dir, c)
	out.Reset()
	if code := Run([]string{filepath.Join(dir, "*.anichar")}, &out); code != 1 || !strings.Contains(out.String(), "human.anichar: 1 finding(s)") {
		t.Errorf("exit %d for a late marker:\n%s", code, out.String())
	}
}
