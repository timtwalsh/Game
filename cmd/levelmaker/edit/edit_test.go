package edit

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"game/shared/world"
)

// tempRoot copies the repo's sample world (definitions and levels) into a
// temporary root, so tests can edit and save freely.
func tempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyFile := func(src, dst string) {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copyFile("../../../world/terrains.toml", filepath.Join(root, "world", "terrains.toml"))
	levels, _ := filepath.Glob("../../../levels/*")
	for _, p := range levels {
		copyFile(p, filepath.Join(root, "levels", filepath.Base(p)))
	}
	return root
}

func open(t *testing.T) *Editor {
	t.Helper()
	e, err := Open(tempRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func terrainID(t *testing.T, e *Editor, name string) uint8 {
	t.Helper()
	tr := e.World.Defs.TerrainByName(name)
	if tr == nil {
		t.Fatalf("no terrain %q", name)
	}
	return tr.ID
}

func terrainAt(e *Editor, wx, wy int) uint8 {
	l := e.World.LevelAt(wx, wy)
	return l.TerrainAt(wx-l.Pos.X, wy-l.Pos.Y)
}

func snapshot(l *world.Level) world.GroundGrid {
	g := l.Ground
	return world.GroundGrid{
		Terrain: slices.Clone(g.Terrain), Tile: slices.Clone(g.Tile),
		Under: slices.Clone(g.Under), Flags: slices.Clone(g.Flags),
	}
}

func TestOpensOnTheSpawnLevel(t *testing.T) {
	// house_1 sorts first, but the editor should open where players start.
	if e := open(t); e.Current == nil || e.Current.Name != "meadow" {
		t.Errorf("opened on %v, want meadow (holds the spawn)", e.Current)
	}
}

func TestShapes(t *testing.T) {
	if got := Brush(world.Point{X: 5, Y: 5}, 1); !slices.Equal(got, []world.Point{{X: 5, Y: 5}}) {
		t.Errorf("Brush size 1 = %v", got)
	}
	b := Brush(world.Point{X: 0, Y: 0}, 3)
	if len(b) != 9 || b[0] != (world.Point{X: -1, Y: -1}) || b[8] != (world.Point{X: 1, Y: 1}) {
		t.Errorf("Brush size 3 = %v", b)
	}
	if got := len(Brush(world.Point{}, 5)); got != 25 {
		t.Errorf("Brush size 5 covers %d cells", got)
	}
	r := Rect(world.Point{X: 3, Y: 2}, world.Point{X: 1, Y: 1})
	if len(r) != 6 || r[0] != (world.Point{X: 1, Y: 1}) {
		t.Errorf("Rect = %v", r)
	}
	l := Line(world.Point{X: 0, Y: 0}, world.Point{X: 4, Y: 2})
	if l[0] != (world.Point{}) || l[len(l)-1] != (world.Point{X: 4, Y: 2}) {
		t.Errorf("Line ends = %v", l)
	}
	for i := 1; i < len(l); i++ {
		if abs(l[i].X-l[i-1].X) > 1 || abs(l[i].Y-l[i-1].Y) > 1 {
			t.Errorf("Line has a gap at %d: %v", i, l)
		}
	}
}

func TestBrushPaintsAndSolves(t *testing.T) {
	e := open(t)
	if err := e.Select("meadow"); err != nil {
		t.Fatal(err)
	}
	water := terrainID(t, e, "water")
	e.Paint(Brush(world.Point{X: 10, Y: 3}, 3), water)
	for _, p := range Brush(world.Point{X: 10, Y: 3}, 3) {
		if terrainAt(e, p.X, p.Y) != water {
			t.Fatalf("%v not painted", p)
		}
	}
	// The grass around the new pond now draws an edge over water.
	m := e.Current
	i := m.Index(10, 1) // just north of the painted block
	if m.Ground.Under[i] == 0 {
		t.Error("grass next to new water has no underlay: the stroke didn't re-solve its neighbours")
	}
	if !e.Dirty() || !slices.Equal(e.DirtyLevels(), []string{"meadow"}) {
		t.Errorf("dirty = %v, want [meadow]", e.DirtyLevels())
	}
}

func TestPaintStaysInCurrentLevel(t *testing.T) {
	e := open(t)
	e.Select("meadow") // x 0..31; lake starts at x 32
	water := terrainID(t, e, "water")
	before := terrainAt(e, 32, 3)
	e.Paint(Brush(world.Point{X: 31, Y: 3}, 3), water)
	if terrainAt(e, 32, 3) != before {
		t.Error("painting at the edge changed the neighbour level's terrain")
	}
	if terrainAt(e, 31, 3) != water {
		t.Error("the current level's edge cell wasn't painted")
	}
}

func TestStrokeAcrossEdgeResolvesNeighbourAndUndoes(t *testing.T) {
	e := open(t)
	e.Select("meadow")
	lake := e.World.Level("lake")
	lakeBefore := snapshot(lake)
	meadowBefore := snapshot(e.Current)

	// One drag stroke down meadow's east column, as water.
	water := terrainID(t, e, "water")
	e.BeginStroke()
	for _, p := range Line(world.Point{X: 31, Y: 2}, world.Point{X: 31, Y: 6}) {
		e.Paint([]world.Point{p}, water)
	}
	e.EndStroke()

	if reflect.DeepEqual(snapshot(lake).Tile, lakeBefore.Tile) && reflect.DeepEqual(snapshot(lake).Under, lakeBefore.Under) {
		t.Fatal("lake's border tiles didn't change after water was painted against them")
	}
	if !slices.Equal(snapshot(lake).Terrain, lakeBefore.Terrain) {
		t.Error("lake's terrain changed; neighbours are read-only")
	}
	if !slices.Equal(e.DirtyLevels(), []string{"lake", "meadow"}) {
		t.Errorf("dirty = %v, want both levels", e.DirtyLevels())
	}

	// The whole drag is one undo step, and it restores both levels.
	if !e.Undo() {
		t.Fatal("nothing to undo")
	}
	if !reflect.DeepEqual(snapshot(lake), lakeBefore) || !reflect.DeepEqual(snapshot(e.Current), meadowBefore) {
		t.Error("undo didn't restore both levels exactly")
	}
	if e.CanUndo() {
		t.Error("the drag left more than one undo step")
	}
	after := snapshot(e.Current)
	if !e.Redo() {
		t.Fatal("nothing to redo")
	}
	if reflect.DeepEqual(snapshot(e.Current), after) {
		t.Error("redo changed nothing")
	}
	if terrainAt(e, 31, 4) != water {
		t.Error("redo didn't repaint the stroke")
	}
}

func TestUndoRedoHistory(t *testing.T) {
	e := open(t)
	e.Select("meadow")
	water, dirt := terrainID(t, e, "water"), terrainID(t, e, "dirt")
	orig := terrainAt(e, 8, 8)
	e.Paint([]world.Point{{X: 8, Y: 8}}, water)
	e.Paint([]world.Point{{X: 8, Y: 8}}, dirt)
	e.Undo()
	if terrainAt(e, 8, 8) != water {
		t.Error("undo didn't step back one stroke")
	}
	e.Undo()
	if terrainAt(e, 8, 8) != orig {
		t.Error("second undo didn't restore the original")
	}
	if e.Undo() {
		t.Error("undo past the start reported success")
	}
	e.Redo()
	e.Paint([]world.Point{{X: 9, Y: 9}}, dirt) // a new stroke clears redo
	if e.CanRedo() {
		t.Error("a new stroke should clear the redo history")
	}
	// A stroke that changes nothing isn't an undo step.
	n := len(e.undo)
	e.Paint([]world.Point{{X: 9, Y: 9}}, dirt)
	if len(e.undo) != n {
		t.Error("a no-op stroke was added to the undo history")
	}
}

func TestFloodFill(t *testing.T) {
	e := open(t)
	e.Select("meadow")
	// The meadow pond is water at x 2..5, y 18..20: 12 cells.
	water := terrainID(t, e, "water")
	cells := e.FloodCells(world.Point{X: 3, Y: 19})
	if len(cells) != 12 {
		t.Fatalf("flood region = %d cells, want the 12-cell pond", len(cells))
	}
	dirt := terrainID(t, e, "dirt")
	e.FloodFill(world.Point{X: 3, Y: 19}, dirt)
	if terrainAt(e, 2, 18) != dirt || terrainAt(e, 5, 20) != dirt || terrainAt(e, 6, 19) == dirt {
		t.Error("flood fill didn't fill exactly the pond")
	}
	e.Undo()
	if terrainAt(e, 2, 18) != water {
		t.Error("flood fill isn't one undo step")
	}
	// Filling the grass doesn't leak into the lake level next door.
	grass := terrainID(t, e, "grass")
	region := e.FloodCells(world.Point{X: 0, Y: 0})
	for _, p := range region {
		if !e.Current.ContainsWorld(p.X, p.Y) || terrainAt(e, p.X, p.Y) != grass {
			t.Fatalf("flood region left the level or the terrain at %v", p)
		}
	}
	if e.FloodCells(world.Point{X: 500, Y: 500}) != nil {
		t.Error("flood outside the level returned cells")
	}
}

func TestEraseMakesCellsEmpty(t *testing.T) {
	e := open(t)
	e.Select("meadow")
	e.Paint([]world.Point{{X: 4, Y: 4}}, 0)
	if terrainAt(e, 4, 4) != 0 || e.Current.Ground.Tile[e.Current.Index(4, 4)] != 0 {
		t.Error("erased cell isn't empty")
	}
}

func TestNewLevel(t *testing.T) {
	e := open(t)
	if err := e.NewLevel("meadow", world.Point{X: 900, Y: 900}, 4, 4, false); err == nil {
		t.Error("duplicate name accepted")
	}
	if err := e.NewLevel("field", world.Point{X: 30, Y: 0}, 4, 4, false); err == nil {
		t.Error("overlapping level accepted")
	}
	if err := e.NewLevel("Bad Name", world.Point{X: 900, Y: 900}, 4, 4, false); err == nil {
		t.Error("invalid name accepted")
	}
	// South of meadow, sharing its bottom edge.
	if err := e.NewLevel("field", world.Point{X: 0, Y: 24}, 8, 4, false); err != nil {
		t.Fatal(err)
	}
	if e.Current.Name != "field" {
		t.Error("the new level isn't selected")
	}
	// meadow's bottom row used to face void (joined) and now faces the
	// field's empty cells (not joined), so its edges changed.
	if !slices.Equal(e.DirtyLevels(), []string{"field", "meadow"}) {
		t.Errorf("dirty = %v, want field and meadow", e.DirtyLevels())
	}
	if err := e.NewLevel("cellar", world.Point{X: 7000, Y: 7000}, 4, 4, true); err != nil || !e.Current.Isolated {
		t.Errorf("isolated level: %v, isolated=%v", err, e.Current.Isolated)
	}
}

func TestSaveThenReloadIsIdentical(t *testing.T) {
	root := tempRoot(t)
	e, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	e.Select("meadow")
	water := terrainID(t, e, "water")
	e.Paint(Rect(world.Point{X: 28, Y: 2}, world.Point{X: 31, Y: 5}), water)
	if err := e.NewLevel("field", world.Point{X: 0, Y: 24}, 6, 3, false); err != nil {
		t.Fatal(err)
	}
	e.Paint(Brush(world.Point{X: 2, Y: 25}, 3), terrainID(t, e, "dirt"))

	written, err := e.Save()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, n := range []string{"field", "lake", "meadow"} {
		tp, gp := world.LevelPaths(e.LevelsDir, n)
		want[tp], want[gp] = true, true
	}
	if len(written) != len(want) {
		t.Errorf("wrote %v, want files for field, lake (border) and meadow", written)
	}
	for _, p := range written {
		if !want[p] {
			t.Errorf("unexpected file written: %s", p)
		}
	}
	if e.Dirty() {
		t.Errorf("still dirty after save: %v", e.DirtyLevels())
	}

	again, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.World.Levels) != len(e.World.Levels) {
		t.Fatalf("reloaded %d levels, saved %d", len(again.World.Levels), len(e.World.Levels))
	}
	for i, l := range e.World.Levels {
		g := again.World.Levels[i]
		if !reflect.DeepEqual(l, g) {
			t.Errorf("level %s differs after reload", l.Name)
		}
	}
	// And the game sees the same world: the borders are seamless, i.e. a
	// full re-solve changes nothing.
	for _, l := range again.World.Levels {
		before := slices.Clone(l.Ground.Tile)
		again.World.SolveLevel(l)
		if !slices.Equal(before, l.Ground.Tile) {
			t.Errorf("level %s has stale tiles after save", l.Name)
		}
	}
}

func TestOpenNeedsDefinitions(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("opening a root without world/terrains.toml succeeded")
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "world"), 0o755)
	data, _ := os.ReadFile("../../../world/terrains.toml")
	os.WriteFile(filepath.Join(root, "world", "terrains.toml"), data, 0o644)
	e, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if e.Current != nil || len(e.World.Levels) != 0 {
		t.Error("a fresh world should open with no levels")
	}
	if _, err := os.Stat(filepath.Join(root, "levels")); err != nil {
		t.Error("levels/ wasn't created")
	}
}
