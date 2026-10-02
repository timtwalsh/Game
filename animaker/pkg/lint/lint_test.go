package lint

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"animaker/pkg/editor"
	"animaker/pkg/file"
)

// save writes a track with the given props (name -> default) to dir.
func save(t *testing.T, dir, name string, props map[string]string, edit func(*editor.Track)) string {
	t.Helper()
	tr := editor.NewTrack(name)
	for _, n := range sortedKeys(props) {
		editor.AddProp(tr, n, props[n])
	}
	if edit != nil {
		edit(tr)
	}
	path := filepath.Join(dir, name+".anif")
	if err := file.SaveTrack(tr, path, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func messages(fs []Finding) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(f.String() + "\n")
	}
	return b.String()
}

func TestAgreeingFamilyIsClean(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"hair": "long", "arms": "leather"}, nil)
	idle := save(t, dir, "human_idle", map[string]string{"hair": "long", "arms": "leather"}, nil)
	if fs := Family([]string{walk, idle}); len(fs) != 0 {
		t.Errorf("findings for an agreeing family:\n%s", messages(fs))
	}
}

func TestMissingPropIsAnError(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"hair": "long", "arms": "leather"}, nil)
	idle := save(t, dir, "human_idle", map[string]string{"hair": "long"}, nil)
	fs := Family([]string{walk, idle})
	if !HasErrors(fs) || len(fs) != 1 || fs[0].File != idle || !strings.Contains(fs[0].Message, `"arms"`) {
		t.Errorf("want one error on human_idle about arms, got:\n%s", messages(fs))
	}
}

func TestPropKindMismatchIsAnError(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"held": "torch_sheet"}, nil)
	run := save(t, dir, "human_run", map[string]string{"held": filepath.Join(dir, "torch.anif")}, nil)
	fs := Family([]string{walk, run})
	if !HasErrors(fs) || !strings.Contains(messages(fs), "holds animations here but sprite sheets") {
		t.Errorf("want a kind mismatch error, got:\n%s", messages(fs))
	}
}

func TestDifferentDefaultsAreInfoOnly(t *testing.T) {
	dir := t.TempDir()
	walk := save(t, dir, "human_walk", map[string]string{"hair": "long"}, nil)
	idle := save(t, dir, "human_idle", map[string]string{"hair": "short"}, nil)
	fs := Family([]string{walk, idle})
	if HasErrors(fs) || len(fs) != 1 || fs[0].Severity != Info {
		t.Errorf("want one info finding, got:\n%s", messages(fs))
	}
}

// The city guard / player torch example: bindings must name the torch's
// props, and a passthrough must name one of the parent's.
func TestNestedBindingsAreChecked(t *testing.T) {
	dir := t.TempDir()
	torch := save(t, dir, "torch", map[string]string{"torch_base": "torchbase_wood"}, nil)
	nest := func(b map[string]editor.PropBinding) func(*editor.Track) {
		return func(tr *editor.Track) {
			p := editor.AddPart(tr, editor.NewNestedAniPart("Torch", torch))
			p.NestedBindings = b
		}
	}
	good := save(t, dir, "player_torch_walk", map[string]string{"torch_base": "torchbase_wood"},
		nest(map[string]editor.PropBinding{"torch_base": {PassthroughFrom: "torch_base"}}))
	if fs := Family([]string{good}); len(fs) != 0 {
		t.Errorf("valid passthrough flagged:\n%s", messages(fs))
	}
	bad := save(t, dir, "guard_torch_walk", nil, nest(map[string]editor.PropBinding{
		"torch_bse":  {StaticValue: "torchbase_metal"},   // typo: not a torch prop
		"torch_base": {PassthroughFrom: "missing_param"}, // parent has no such prop
	}))
	fs := Family([]string{bad})
	msg := messages(fs)
	if len(fs) != 2 || !strings.Contains(msg, `binds "torch_bse"`) || !strings.Contains(msg, `no prop "missing_param"`) {
		t.Errorf("want the typo and the missing parent prop, got:\n%s", msg)
	}
}

func TestRunExitCodes(t *testing.T) {
	dir := t.TempDir()
	save(t, dir, "human_walk", map[string]string{"hair": "long"}, nil)
	save(t, dir, "human_idle", map[string]string{"hair": "long"}, nil)
	save(t, dir, "guard_walk", map[string]string{"helmet": "iron"}, nil)
	save(t, dir, "guard_idle", nil, nil)

	var out bytes.Buffer
	if code := Run([]string{filepath.Join(dir, "human_*.anif")}, &out); code != 0 {
		t.Errorf("clean family: exit %d\n%s", code, out.String())
	}
	out.Reset()
	if code := Run([]string{filepath.Join(dir, "human_*.anif"), filepath.Join(dir, "guard_*.anif")}, &out); code != 1 {
		t.Errorf("drifted family: exit %d, want 1\n%s", code, out.String())
	}
	if code := Run(nil, &out); code != 2 {
		t.Errorf("no args: exit %d, want 2", code)
	}
	if code := Run([]string{filepath.Join(dir, "nobody_*.anif")}, &out); code != 1 {
		t.Errorf("glob matching nothing: exit %d, want 1", code)
	}
}
