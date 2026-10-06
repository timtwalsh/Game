package anim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const babyPath = "../../assets/characters/baby/baby.anichar"

func loadBaby(t *testing.T) *Character {
	t.Helper()
	l := NewLibrary()
	c, err := l.LoadCharacter(babyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Problems) > 0 {
		t.Fatalf("problems loading the baby: %v", l.Problems)
	}
	return c
}

// files writes name -> content into a temp dir and returns it. Every
// sheet in these tests is "s.sprsh" (16x16 cells, pivot 8,8) over a
// placeholder image - the runtime never decodes it.
func files(t *testing.T, fs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	fs["s.png"] = "png"
	if _, ok := fs["s.sprsh"]; !ok {
		fs["s.sprsh"] = "name = \"s\"\nfile_path = \"s.png\"\ncell_w = 16\ncell_h = 16\npivot_x = 8.0\npivot_y = 8.0\n"
	}
	for name, content := range fs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// oneAnim is a character playing a single looping track.
func oneAnim(t *testing.T, path string) (*Instance, *Library) {
	t.Helper()
	l := NewLibrary()
	tr, err := l.LoadTrack(path)
	if err != nil {
		t.Fatal(err)
	}
	inst := NewInstance(&Character{Animations: map[string]*Animation{"a": {Name: "a", Track: tr}}})
	inst.Play("a")
	return inst, l
}

func TestLoadBabyCharacter(t *testing.T) {
	c := loadBaby(t)
	for name, mode := range map[string]PlayMode{"walk": PlayLoop, "jump": PlayOnce, "dead": PlayHold} {
		a := c.Animations[name]
		if a == nil {
			t.Fatalf("no %q animation", name)
		}
		if a.Mode != mode {
			t.Errorf("%s mode = %v, want %v", name, a.Mode, mode)
		}
		if a.Track.DirectionCount != 4 || len(a.Track.Directions) != 4 {
			t.Errorf("%s: %d directions (%d loaded), want 4", name, a.Track.DirectionCount, len(a.Track.Directions))
		}
		s := a.Track.Sheets["sprite"]
		if s == nil || s.CellW != 160 || s.CellH != 180 || s.PivotX != 80 || s.PivotY != 90 {
			t.Fatalf("%s: sprite sheet = %+v", name, s)
		}
		if _, err := os.Stat(s.ImagePath); err != nil {
			t.Errorf("%s: sheet image: %v", name, err)
		}
	}
	// One library shares the sheet between every track.
	if c.Animations["walk"].Track.Sheets["sprite"] != c.Animations["jump"].Track.Sheets["sprite"] {
		t.Error("walk and jump loaded separate copies of the same sheet")
	}
	if d := c.Animations["jump"].Track.Directions[0]; d.DurationMs != 800 {
		t.Errorf("jump duration = %d, want 800", d.DurationMs)
	}
}

func TestDirectionMapping(t *testing.T) {
	keys := func(n int) []int {
		tr := &Track{DirectionCount: n, Directions: map[int]*Direction{}}
		tr.index()
		out := make([]int, 8)
		for f := range out {
			out[f] = tr.byCompass[f*2]
		}
		return out
	}
	// N, NE, E, SE, S, SW, W, NW on a 4-direction (N, E, S, W) track:
	// diagonals go to the sideways direction.
	for n, want := range map[int][]int{
		4: {0, 1, 1, 1, 2, 3, 3, 3},
		8: {0, 1, 2, 3, 4, 5, 6, 7},
		1: {0, 0, 0, 0, 0, 0, 0, 0},
	} {
		got := keys(n)
		for f := range want {
			if got[f] != want[f] {
				t.Errorf("%d directions: facing %d -> key %d, want %d", n, f, got[f], want[f])
			}
		}
	}
	two := keys(2) // E, W
	for f, w := range map[int]int{1: 0, 2: 0, 3: 0, 5: 1, 6: 1, 7: 1} {
		if two[f] != w {
			t.Errorf("2 directions: facing %d -> key %d, want %d", f, two[f], w)
		}
	}
}

func TestUnposedDirectionFallsBackToFirstPosed(t *testing.T) {
	dir := files(t, map[string]string{"a.anif": `
[metadata]
  directions = 4
[[parts]]
  id = 1
  kind = "sheet"
  fixed_sheet = "s"
[directions.1]
  [[directions.1.keyframes]]
    part_id = 1
    col = 7
[directions.0]
`})
	inst, _ := oneAnim(t, filepath.Join(dir, "a.anif"))
	inst.SetFacing(0) // N exists but has no keyframes
	if s := inst.AppendSprites(nil); len(s) != 1 || s[0].Col != 7 {
		t.Errorf("facing an unposed direction drew %+v, want the posed one", s)
	}
}

func TestWalkLoopsAndStepsCells(t *testing.T) {
	inst := NewInstance(loadBaby(t))
	inst.SetFacing(2) // E: the baby's column 1
	inst.Play("walk")
	cell := func() (int, int) {
		s := inst.AppendSprites(nil)
		if len(s) != 1 {
			t.Fatalf("%d sprites, want 1", len(s))
		}
		return s[0].Row, s[0].Col
	}
	if r, c := cell(); r != 0 || c != 1 {
		t.Errorf("t=0: cell (%d,%d), want (0,1)", r, c)
	}
	inst.Advance(141) // still before the 142ms keyframe: cells step, not blend
	if r, _ := cell(); r != 0 {
		t.Errorf("t=141: row %d, want 0", r)
	}
	inst.Advance(1)
	if r, _ := cell(); r != 1 {
		t.Errorf("t=142: row %d, want 1", r)
	}
	inst.Advance(510) // 652ms wraps to 142 in a 510ms loop
	if r, _ := cell(); r != 1 || inst.Finished() {
		t.Errorf("after wrap: row %d finished %v, want row 1 and still playing", r, inst.Finished())
	}
}

func TestJumpInterpolatesAndFinishes(t *testing.T) {
	inst := NewInstance(loadBaby(t))
	inst.SetFacing(4) // S: column 2
	inst.Restart("jump")
	inst.Advance(70) // halfway from y=0 (0ms) to y=-10 (140ms)
	if s := inst.AppendSprites(nil)[0]; s.Y != -5 || s.Row != 1 || s.Col != 2 {
		t.Errorf("t=70: %+v, want y=-5 at cell (1,2)", s)
	}
	inst.Advance(1000)
	if !inst.Finished() {
		t.Error("jump not finished after its 800ms")
	}
	if s := inst.AppendSprites(nil)[0]; s.Y != 0 || s.Row != 1 {
		t.Errorf("finished jump shows %+v, want its last frame (y=0, row 1)", s)
	}
	// Play on what's already playing doesn't restart it; Restart does.
	inst.Play("jump")
	if !inst.Finished() {
		t.Error("Play restarted the playing animation")
	}
	inst.Restart("jump")
	if inst.Finished() {
		t.Error("Restart didn't restart")
	}
}

func TestPropSwapCarriesAcrossAnimations(t *testing.T) {
	track := `
[metadata]
  directions = 1
[[props]]
  name = "body"
  default = "s"
[[parts]]
  id = 1
  kind = "sheet"
  governing_prop = "body"
  fixed_sheet = "s"
[directions.0]
  [[directions.0.keyframes]]
    part_id = 1
`
	dir := files(t, map[string]string{"idle.anif": track, "walk.anif": track,
		"red.sprsh": "name = \"red\"\nfile_path = \"s.png\"\ncell_w = 16\ncell_h = 16\n"})
	l := NewLibrary()
	c := &Character{Animations: map[string]*Animation{}}
	for _, n := range []string{"idle", "walk"} {
		tr, err := l.LoadTrack(filepath.Join(dir, n+".anif"))
		if err != nil {
			t.Fatal(err)
		}
		c.Animations[n] = &Animation{Name: n, Track: tr}
	}
	red, err := l.LoadSheet(filepath.Join(dir, "red.sprsh"))
	if err != nil {
		t.Fatal(err)
	}
	inst := NewInstance(c)
	inst.Play("idle")
	if got := inst.AppendSprites(nil)[0].Sheet.Name; got != "s" {
		t.Errorf("default sheet = %q", got)
	}
	inst.SetProp("body", PropValue{Sheet: red}) // any loaded sheet, not just ones the track names
	if got := inst.AppendSprites(nil)[0].Sheet; got != red {
		t.Errorf("prop swap drew %q", got.Name)
	}
	inst.Play("walk")
	if got := inst.AppendSprites(nil)[0].Sheet; got != red {
		t.Errorf("swap lost on changing animation: drew %q", got.Name)
	}
	inst.SetProp("body", PropValue{})
	if got := inst.AppendSprites(nil)[0].Sheet.Name; got != "s" {
		t.Errorf("clearing the swap drew %q, want the default", got)
	}
}

func TestMissingSheetIsAProblemNotAnError(t *testing.T) {
	dir := files(t, map[string]string{"x.anif": `
[metadata]
  directions = 1
[[parts]]
  id = 1
  kind = "sheet"
  fixed_sheet = "nowhere"
[directions.0]
  [[directions.0.keyframes]]
    part_id = 1
`})
	inst, l := oneAnim(t, filepath.Join(dir, "x.anif"))
	if len(l.Problems) != 1 || !strings.Contains(l.Problems[0], `"nowhere"`) {
		t.Errorf("problems = %v, want the missing sheet", l.Problems)
	}
	if s := inst.AppendSprites(nil); len(s) != 0 {
		t.Errorf("a part with no sheet drew %+v", s)
	}
}

// A frame - turning, advancing, resolving sprites into last frame's
// buffer - must not allocate: it runs for every character at 144fps.
func TestFrameDoesNotAllocate(t *testing.T) {
	inst := NewInstance(loadBaby(t))
	inst.Play("walk")
	buf := inst.AppendSprites(nil)
	f := uint8(0)
	allocs := testing.AllocsPerRun(1000, func() {
		f++
		inst.SetFacing(f % 8)
		inst.Advance(7)
		_ = inst.Finished()
		buf = inst.AppendSprites(buf[:0])
	})
	if allocs != 0 {
		t.Errorf("%v allocations per frame, want 0", allocs)
	}
}

func TestNestedFrameDoesNotAllocate(t *testing.T) {
	inst, _ := oneAnim(t, filepath.Join(nestFiles(t, "", ""), "parent.anif"))
	buf := inst.AppendSprites(nil)
	f := uint8(0)
	allocs := testing.AllocsPerRun(1000, func() {
		f++
		inst.SetFacing(f % 8)
		inst.Advance(7)
		buf = inst.AppendSprites(buf[:0])
	})
	if allocs != 0 {
		t.Errorf("%v allocations per frame with a nested part, want 0", allocs)
	}
}

func TestMarkers(t *testing.T) {
	dir := files(t, map[string]string{
		"w.anif": `
[metadata]
  directions = 1
[[parts]]
  id = 1
  kind = "sheet"
  fixed_sheet = "s"
[directions.0]
  [[directions.0.keyframes]]
    part_id = 1
  [[directions.0.keyframes]]
    part_id = 1
    time_ms = 500
`,
		"c.anichar": `
[animations.walk]
  anif = "w.anif"
  markers = { footstep = [0, 250], halfway = 250 }
`})
	c, err := NewLibrary().LoadCharacter(filepath.Join(dir, "c.anichar"))
	if err != nil {
		t.Fatal(err)
	}
	inst := NewInstance(c)
	inst.Play("walk")
	crossed := func(ms uint32) string {
		inst.Advance(ms)
		var names []string
		for _, m := range inst.AppendCrossedMarkers(nil) {
			names = append(names, m.Name)
		}
		return strings.Join(names, ",")
	}
	if got := crossed(10); got != "footstep" {
		t.Errorf("first advance after starting crossed %q, want the 0ms footstep", got)
	}
	if got := crossed(200); got != "" {
		t.Errorf("10-210ms crossed %q", got)
	}
	if got := crossed(100); got != "footstep,halfway" {
		t.Errorf("210-310ms crossed %q", got)
	}
	if got := crossed(300); got != "footstep" { // 310 -> 610 wraps to 110, passing 0
		t.Errorf("across the loop crossed %q, want the 0ms footstep", got)
	}
	inst.Seek(0)
	if got := crossed(0); got != "" {
		t.Errorf("seeking crossed %q", got)
	}
}

// nestFiles is a 4-direction parent with a body sheet part (z=1) and a
// nested part (z=2) playing an 8-direction child. The child's own
// sprite has z=-100, its column says which direction is playing, and it
// moves from x=0 to x=30 over 300ms. The nested part's settings are
// spliced in from extra.
func nestFiles(t *testing.T, extra, parentKeyframes string) string {
	child := "[metadata]\n  directions = 8\n[[props]]\n  name = \"skin\"\n  default = \"s\"\n" +
		"[[parts]]\n  id = 1\n  kind = \"sheet\"\n  governing_prop = \"skin\"\n"
	for k := 0; k < 8; k++ {
		child += strings.ReplaceAll(`[directions.K]
  [[directions.K.keyframes]]
    part_id = 1
    col = K
    z = -100.0
  [[directions.K.keyframes]]
    part_id = 1
    time_ms = 300
    x = 30.0
    col = K
    z = -100.0
`, "K", string(rune('0'+k)))
	}
	parent := `
[metadata]
  directions = 4
[[props]]
  name = "skin"
  default = "s"
[[parts]]
  id = 1
  kind = "sheet"
  fixed_sheet = "s"
[[parts]]
  id = 2
  kind = "nested_ani"
  nested_ani_path = "child.anif"
` + extra + "\n"
	for k := 0; k < 4; k++ {
		parent += strings.ReplaceAll(`[directions.K]
  [[directions.K.keyframes]]
    part_id = 1
    z = 1.0
  [[directions.K.keyframes]]
    part_id = 2
    x = 100.0
    z = 2.0
  [[directions.K.keyframes]]
    part_id = 2
    time_ms = 1000
    x = 100.0
    z = 2.0
`, "K", string(rune('0'+k)))
	}
	parent += parentKeyframes
	return files(t, map[string]string{"parent.anif": parent, "child.anif": child,
		"red.sprsh": "name = \"red\"\nfile_path = \"s.png\"\ncell_w = 16\ncell_h = 16\n"})
}

// nested is the sprite the nested part draws (the second; the body is
// first).
func nested(t *testing.T, inst *Instance) Sprite {
	t.Helper()
	s := inst.AppendSprites(nil)
	if len(s) != 2 {
		t.Fatalf("%d sprites, want body + nested: %+v", len(s), s)
	}
	if s[0].Sheet.Name != "s" || s[0].X != 0 {
		t.Fatalf("draw order: %+v, want the body first - a nested sprite keeps its part's slot whatever its own Z", s)
	}
	return s[1]
}

func TestNestedInheritsDirectionByFacing(t *testing.T) {
	inst, l := oneAnim(t, filepath.Join(nestFiles(t, "", ""), "parent.anif"))
	if len(l.Problems) > 0 {
		t.Fatal(l.Problems)
	}
	// The parent has 4 directions, so facing NE plays its E, and the
	// 8-direction child turns with the parent's E - not with NE.
	for facing, wantCol := range map[uint8]int{0: 0, 1: 2, 2: 2, 4: 4, 5: 6, 6: 6} {
		inst.SetFacing(facing)
		if s := nested(t, inst); s.Col != wantCol {
			t.Errorf("facing %d: child plays direction %d, want %d", facing, s.Col, wantCol)
		}
	}
}

func TestNestedStaticAndPerKeyframeDirection(t *testing.T) {
	inst, _ := oneAnim(t, filepath.Join(nestFiles(t, `  direction_mode = "static"
  static_direction = 5`, ""), "parent.anif"))
	for _, f := range []uint8{0, 2, 4} {
		inst.SetFacing(f)
		if s := nested(t, inst); s.Col != 5 {
			t.Errorf("static, facing %d: child plays %d, want 5", f, s.Col)
		}
	}

	// Per keyframe: the direction steps at each of the part's keyframes.
	perKF := nestFiles(t, `  direction_mode = "keyframe"`, "")
	parent, _ := os.ReadFile(filepath.Join(perKF, "parent.anif"))
	src := strings.Replace(string(parent), "    part_id = 2\n    x = 100.0\n    z = 2.0\n",
		"    part_id = 2\n    x = 100.0\n    z = 2.0\n    direction = 3\n", 1)
	src = strings.Replace(src, "    part_id = 2\n    time_ms = 1000\n",
		"    part_id = 2\n    time_ms = 500\n    x = 100.0\n    z = 2.0\n    direction = 7\n  [[directions.0.keyframes]]\n    part_id = 2\n    time_ms = 1000\n", 1)
	os.WriteFile(filepath.Join(perKF, "parent.anif"), []byte(src), 0o644)
	inst, _ = oneAnim(t, filepath.Join(perKF, "parent.anif"))
	inst.SetFacing(0)
	if s := nested(t, inst); s.Col != 3 {
		t.Errorf("per keyframe at 0ms: child plays %d, want 3", s.Col)
	}
	inst.Advance(600)
	if s := nested(t, inst); s.Col != 7 {
		t.Errorf("per keyframe at 600ms: child plays %d, want 7", s.Col)
	}
}

func TestNestedRunsOnItsOwnClockAndOffset(t *testing.T) {
	inst, _ := oneAnim(t, filepath.Join(nestFiles(t, "", ""), "parent.anif"))
	inst.Advance(150) // child halfway through its 300ms
	if s := nested(t, inst); s.X != 115 {
		t.Errorf("at 150ms the child sprite is at x=%v, want 100 (part) + 15 (child)", s.X)
	}
	inst.Advance(300) // 450ms: the child has looped at its own length, to 150
	if s := nested(t, inst); s.X != 115 {
		t.Errorf("at 450ms x=%v, want 115 (child looped at 300ms)", s.X)
	}
	inst.Restart("a") // the parent restarts; the nested clock keeps going
	inst.Advance(0)
	if s := nested(t, inst); s.X != 115 {
		t.Errorf("after the parent restarted x=%v, want the child carrying on (115)", s.X)
	}
}

func TestNestedRotationTurnsTheChild(t *testing.T) {
	dir := nestFiles(t, "", "")
	p := filepath.Join(dir, "parent.anif")
	src, _ := os.ReadFile(p)
	// The nested part turned 90 degrees clockwise.
	os.WriteFile(p, []byte(strings.ReplaceAll(string(src), "    x = 100.0\n    z = 2.0\n", "    x = 100.0\n    z = 2.0\n    rotation_deg = 90.0\n")), 0o644)
	inst, _ := oneAnim(t, p)
	inst.Advance(150) // child at x=15 in its own space
	s := nested(t, inst)
	if d := s.X - 100; d > 0.001 || d < -0.001 || s.Y < 14.999 || s.Y > 15.001 || s.RotationDeg != 90 {
		t.Errorf("rotated child at (%v, %v) rot %v, want (100, 15) rot 90", s.X, s.Y, s.RotationDeg)
	}
}

func TestNestedPropPassthroughAndStatic(t *testing.T) {
	pass := nestFiles(t, `  [parts.nested_bindings.skin]
    passthrough_from = "skin"`, "")
	inst, l := oneAnim(t, filepath.Join(pass, "parent.anif"))
	red, err := l.LoadSheet(filepath.Join(pass, "red.sprsh"))
	if err != nil {
		t.Fatal(err)
	}
	inst.SetProp("skin", PropValue{Sheet: red})
	if s := nested(t, inst); s.Sheet != red {
		t.Errorf("passthrough: child drew %q, want the parent's swapped skin", s.Sheet.Name)
	}

	static := nestFiles(t, `  [parts.nested_bindings.skin]
    static_value = "red"`, "")
	inst, _ = oneAnim(t, filepath.Join(static, "parent.anif"))
	if s := nested(t, inst); s.Sheet.Name != "red" {
		t.Errorf("static binding: child drew %q, want red", s.Sheet.Name)
	}
}

func TestAnimationValuedPropSwapsTheNestedTrack(t *testing.T) {
	dir := nestFiles(t, `  governing_prop = "held"`, "")
	p := filepath.Join(dir, "parent.anif")
	src, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(src), "[[parts]]", "[[props]]\n  name = \"held\"\n  default = \"child.anif\"\n[[parts]]", 1)), 0o644)
	os.WriteFile(filepath.Join(dir, "lantern.anif"), []byte(`
[metadata]
  directions = 1
[[parts]]
  id = 1
  kind = "sheet"
  fixed_sheet = "s"
[directions.0]
  [[directions.0.keyframes]]
    part_id = 1
    col = 42
`), 0o644)
	inst, l := oneAnim(t, p)
	lantern, err := l.LoadTrack(filepath.Join(dir, "lantern.anif"))
	if err != nil {
		t.Fatal(err)
	}
	if s := nested(t, inst); s.Col != 4 {
		t.Errorf("default held animation drew col %d, want the child's (4, facing S)", s.Col)
	}
	inst.SetProp("held", PropValue{Track: lantern})
	if s := nested(t, inst); s.Col != 42 {
		t.Errorf("swapped held animation drew col %d, want the lantern's 42", s.Col)
	}
}

func TestSelfNestingStops(t *testing.T) {
	dir := files(t, map[string]string{"loop.anif": `
[metadata]
  directions = 1
[[parts]]
  id = 1
  kind = "sheet"
  fixed_sheet = "s"
[[parts]]
  id = 2
  kind = "nested_ani"
  nested_ani_path = "loop.anif"
[directions.0]
  [[directions.0.keyframes]]
    part_id = 1
  [[directions.0.keyframes]]
    part_id = 2
`})
	inst, _ := oneAnim(t, filepath.Join(dir, "loop.anif"))
	if s := inst.AppendSprites(nil); len(s) != maxNestDepth {
		t.Errorf("self-nesting drew %d sprites, want one per level up to %d", len(s), maxNestDepth)
	}
}

// The game reads the footprint, hitboxes and scale the animaker writes to
// an .anichar.
func TestLoadCharacterShapes(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"baby_walk.anif", "sprite.sprsh", "sprite.png"} {
		data, err := os.ReadFile(filepath.Join("../../assets/characters/baby", f))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, f), data, 0o644)
	}
	path := filepath.Join(dir, "ogre.anichar")
	os.WriteFile(path, []byte(`name = "ogre"
scale = 0.5

[footprint]
x = -20.0
y = -6.0
w = 40.0
h = 12.0

[[hitboxes]]
name = "body"
shape = "oval"
x = -30.0
y = -90.0
w = 60.0
h = 90.0

[[hitboxes]]
name = "head"
shape = "circle"
x = -10.0
y = -110.0
w = 20.0
h = 20.0

[animations.walk]
anif = "baby_walk.anif"
mode = "loop"
`), 0o644)
	c, err := NewLibrary().LoadCharacter(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Scale != 0.5 || c.Footprint == nil || *c.Footprint != (Box{-20, -6, 40, 12}) {
		t.Errorf("scale %v footprint %+v", c.Scale, c.Footprint)
	}
	if len(c.Hitboxes) != 2 || c.Hitboxes[0] != (Hitbox{"body", ShapeOval, Box{-30, -90, 60, 90}}) || c.Hitboxes[1].Shape != ShapeCircle {
		t.Errorf("hitboxes %+v", c.Hitboxes)
	}

	bad := strings.Replace(mustRead(t, path), `shape = "oval"`, `shape = "star"`, 1)
	os.WriteFile(path, []byte(bad), 0o644)
	if _, err := NewLibrary().LoadCharacter(path); err == nil {
		t.Error("an unknown hitbox shape loaded")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
