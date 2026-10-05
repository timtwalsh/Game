package world

import (
	"slices"
	"testing"
)

func testWorld(t *testing.T, d *Defs, levels ...*Level) *World {
	t.Helper()
	w := NewWorld(d)
	for _, l := range levels {
		if err := w.Add(l); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range levels {
		w.SolveLevel(l)
	}
	return w
}

func TestLookupsAcrossBoundaryAndVoid(t *testing.T) {
	d := testDefs(t)
	a := paint(t, "a", Point{-2, -2}, "gg", "gg")
	b := paint(t, "b", Point{0, -2}, "ww", "ww")
	w := testWorld(t, d, a, b)

	if w.LevelAt(-1, -1) != a || w.LevelAt(0, -1) != b {
		t.Error("LevelAt picked the wrong level at the boundary")
	}
	if w.Blocks(-1, -1, BlockGround) || w.Blocks(0, -1, BlockGround) {
		t.Error("grass and water should not block walking")
	}
	if w.InteractionAt(1, -2) != 1 || w.SurfaceAt(-2, -2) != 1 {
		t.Error("interaction/surface lookups across levels are wrong")
	}
	for _, p := range []Point{{-3, -1}, {2, -1}, {0, 0}, {-1, -3}} {
		if w.LevelAt(p.X, p.Y) != nil || w.BlockingAt(p.X, p.Y) != BlockAll {
			t.Errorf("%v should be void and block everything", p)
		}
	}
}

func TestTileOfFloors(t *testing.T) {
	cases := map[float32]int{0: 0, 15.9: 0, 16: 1, -0.5: -1, -16: -1, -16.5: -2}
	for px, want := range cases {
		if got := TileOf(px); got != want {
			t.Errorf("TileOf(%v) = %d, want %d", px, got, want)
		}
	}
}

func TestOverlapRejected(t *testing.T) {
	d := testDefs(t)
	w := NewWorld(d)
	if err := w.Add(paint(t, "a", Point{0, 0}, "ggg", "ggg")); err != nil {
		t.Fatal(err)
	}
	err := w.Add(paint(t, "b", Point{2, 1}, "gg"))
	wantErr(t, err, `"b" overlaps level "a"`)
	if err := w.CheckPlacement(Point{3, 0}, 2, 2, ""); err != nil {
		t.Errorf("touching placement refused: %v", err)
	}
	wantErr(t, w.CheckPlacement(Point{1, 1}, 1, 1, ""), `overlaps level "a"`)
	if err := w.CheckPlacement(Point{1, 1}, 1, 1, "a"); err != nil {
		t.Errorf("a level may overlap its own old footprint: %v", err)
	}
	wantErr(t, w.Add(paint(t, "a", Point{50, 50}, "g")), "two levels")
}

func TestNeighboursByGeometry(t *testing.T) {
	d := testDefs(t)
	a := paint(t, "a", Point{0, 0}, "gg", "gg")
	east := paint(t, "east", Point{2, 0}, "g")
	corner := paint(t, "corner", Point{2, 2}, "g")
	far := paint(t, "far", Point{10, 10}, "g")
	inside := paint(t, "inside", Point{0, 2}, "g")
	inside.Isolated = true
	w := testWorld(t, d, a, east, corner, far, inside)

	var names []string
	for _, n := range w.Neighbours(a) {
		names = append(names, n.Name)
	}
	if !slices.Equal(names, []string{"corner", "east"}) {
		t.Errorf("neighbours of a = %v, want [corner east]", names)
	}
	if n := w.Neighbours(inside); n != nil {
		t.Errorf("isolated level has neighbours %v", n)
	}
}

func TestIsolatedLevelDoesNotTileAcrossEdges(t *testing.T) {
	d := testDefs(t)
	out := paint(t, "out", Point{0, 0}, "dd", "dd")
	in := paint(t, "in", Point{2, 0}, "gg", "gg")
	in.Isolated = true
	testWorld(t, d, out, in)
	// The interior's grass sees void to its west, not the dirt next door.
	if got := in.Ground.Tile[in.Index(0, 0)]; got != 95+FullState {
		t.Errorf("isolated grass tile = %d, want full (edge is void)", got)
	}
	if in.Ground.Under[in.Index(0, 0)] != 0 {
		t.Error("isolated grass picked up an underlay from the level next door")
	}
}

func TestSolveBordersRemovesSeam(t *testing.T) {
	d := testDefs(t)
	rowsA := []string{"gggg", "gggg", "gggg"}
	rowsB := []string{"dddd", "dwwd", "dddd"}
	a := paint(t, "a", Point{0, 0}, rowsA...)
	b := paint(t, "b", Point{4, 0}, rowsB...)
	// Each painted and solved alone, as if edited independently.
	SolveAll(a, LevelSource{a}, d)
	SolveAll(b, LevelSource{b}, d)
	w := NewWorld(d)
	w.Add(a)
	w.Add(b)

	changed := w.SolveBorders(a)
	if len(changed) != 0 {
		// b's dirt is lower than a's grass: its tiles don't change.
		t.Errorf("SolveBorders(a) changed %d neighbours, want 0", len(changed))
	}
	w.SolveBorders(b)

	// Reference: the same content as one level.
	var joined []string
	for i := range rowsA {
		joined = append(joined, rowsA[i]+rowsB[i])
	}
	ref := paint(t, "ref", Point{0, 0}, joined...)
	SolveAll(ref, LevelSource{ref}, d)
	for y := 0; y < 3; y++ {
		for x := 0; x < 8; x++ {
			l, lx := a, x
			if x >= 4 {
				l, lx = b, x-4
			}
			i, ri := l.Index(lx, y), ref.Index(x, y)
			if l.Ground.Tile[i] != ref.Ground.Tile[ri] || l.Ground.Under[i] != ref.Ground.Under[ri] {
				t.Errorf("seam at (%d,%d): tile %d/%d under %d/%d", x, y,
					l.Ground.Tile[i], ref.Ground.Tile[ri], l.Ground.Under[i], ref.Ground.Under[ri])
			}
		}
	}
}

func TestSolveBordersReportsChangedNeighbours(t *testing.T) {
	d := testDefs(t)
	a := paint(t, "a", Point{0, 0}, "dd", "dd")
	b := paint(t, "b", Point{2, 0}, "gg", "gg")
	w := testWorld(t, d, a, b)
	// Repaint a's east column to water. b's west column is still grass
	// over something lower, but its underlay must change from dirt to water.
	a.SetTerrain(1, 0, tWater)
	a.SetTerrain(1, 1, tWater)
	SolveAll(a, w.SourceFor(a), d)
	changed := w.SolveBorders(a)
	if len(changed) != 1 || changed[0] != b {
		t.Fatalf("changed = %v, want [b]", changed)
	}
	if got := b.Ground.Under[b.Index(0, 0)]; got != 1+FullState {
		t.Errorf("b's border underlay = %d, want water centre", got)
	}
}
