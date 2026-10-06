package world

import "testing"

func TestExactly47States(t *testing.T) {
	s := BlobStates()
	if len(s) != BlobTileCount {
		t.Fatalf("%d states, want %d", len(s), BlobTileCount)
	}
	for i := 1; i < len(s); i++ {
		if s[i] <= s[i-1] {
			t.Fatalf("states not strictly ascending at %d: %v", i, s)
		}
	}
	if s[0] != 0 || s[FullState] != 255 {
		t.Errorf("first/last state = %d/%d, want 0 (isolated) and 255 (full)", s[0], s[FullState])
	}
	for _, m := range s {
		if ReduceMask(m) != m {
			t.Errorf("state %08b is not reduced", m)
		}
	}
}

func TestReduceMaskDropsUnsupportedCorners(t *testing.T) {
	// NE with only N set: the E edge already covers that corner.
	if got := ReduceMask(MaskN | MaskNE); got != MaskN {
		t.Errorf("ReduceMask(N|NE) = %08b, want N", got)
	}
	if got := ReduceMask(MaskN | MaskE | MaskNE); got != MaskN|MaskE|MaskNE {
		t.Errorf("ReduceMask(N|E|NE) = %08b, want unchanged", got)
	}
	if StateOf(MaskNE|MaskSE|MaskSW|MaskNW) != 0 {
		t.Error("lone corners should reduce to the isolated state")
	}
}

func tileState(t *testing.T, d *Defs, l *Level, x, y int) int {
	t.Helper()
	i := l.Index(x, y)
	tr := d.Terrain(l.Ground.Terrain[i])
	return int(l.Ground.Tile[i] - tr.SheetBase)
}

func TestSingleGrassInDirtIsIsolated(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d,
		"ddd",
		"dgd",
		"ddd",
	)
	if got := tileState(t, d, l, 1, 1); got != 0 {
		t.Errorf("lone grass state = %d, want 0 (isolated)", got)
	}
	if got, want := l.Ground.Under[l.Index(1, 1)], 48+uint16(FullState); got != want {
		t.Errorf("lone grass underlay = %d, want dirt centre %d", got, want)
	}
	// Dirt around it is joined to everything (grass outranks it, void is
	// joined), so every dirt cell is the full centre with no underlay.
	for _, p := range []Point{{0, 0}, {1, 0}, {2, 2}} {
		if got := tileState(t, d, l, p.X, p.Y); got != FullState {
			t.Errorf("dirt at %v state = %d, want full", p, got)
		}
		if l.Ground.Under[l.Index(p.X, p.Y)] != 0 {
			t.Errorf("dirt at %v has an underlay", p)
		}
	}
}

func TestTwoByTwoGrassGivesOuterCorners(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d,
		"dddd",
		"dggd",
		"dggd",
		"dddd",
	)
	want := map[Point]uint8{
		{1, 1}: MaskE | MaskSE | MaskS,
		{2, 1}: MaskW | MaskSW | MaskS,
		{1, 2}: MaskN | MaskNE | MaskE,
		{2, 2}: MaskN | MaskNW | MaskW,
	}
	for p, mask := range want {
		if got := tileState(t, d, l, p.X, p.Y); got != StateOf(mask) {
			t.Errorf("grass at %v state = %d, want %d (mask %08b)", p, got, StateOf(mask), mask)
		}
	}
}

func TestThreeTerrainsMeeting(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d,
		"gdw",
	)
	if got, want := l.Ground.Under[l.Index(0, 0)], 48+uint16(FullState); got != want {
		t.Errorf("grass underlay = %d, want dirt centre %d", got, want)
	}
	if got, want := l.Ground.Under[l.Index(1, 0)], 1+uint16(FullState); got != want {
		t.Errorf("dirt underlay = %d, want water centre %d", got, want)
	}
	if l.Ground.Under[l.Index(2, 0)] != 0 {
		t.Error("water is lowest; it should have no underlay")
	}
	// Grass with both dirt and water around it stacks them: water full,
	// then dirt's own edge (open to the water on the west), then grass.
	l = solved(t, d,
		"wg",
		"dd",
	)
	i := l.Index(1, 0)
	if got, want := l.Ground.Under[i], 1+uint16(FullState); got != want {
		t.Errorf("underlay = %d, want water centre %d", got, want)
	}
	dirtMask := MaskN | MaskNE | MaskE | MaskSE | MaskS
	if got, want := l.Ground.Mid[i], 48+uint16(StateOf(dirtMask)); got != want {
		t.Errorf("mid = %d, want dirt edge %d (mask %08b)", got, want, dirtMask)
	}
}

func TestLowerEdgeCarriesOnUnderGrass(t *testing.T) {
	d := testDefs(t)
	// Grass crosses a dirt/water border: under each grass cell the lower
	// terrains continue as they would without it, rather than one of them
	// filling the cell.
	l := solved(t, d,
		"dwd",
		"ggg",
		"dwd",
	)
	// West grass: dirt above and below, water at its NE/SE corners, so a
	// dirt piece with water inner corners over water.
	i := l.Index(0, 1)
	if got, want := l.Ground.Under[i], 1+uint16(FullState); got != want {
		t.Errorf("west grass underlay = %d, want water centre %d", got, want)
	}
	dirtMask := ReduceMask(MaskN | MaskS | MaskE | MaskW | MaskNW | MaskSW)
	if got, want := l.Ground.Mid[i], 48+uint16(StateOf(dirtMask)); got != want {
		t.Errorf("west grass mid = %d, want dirt with water corners %d", got, want)
	}
	// Middle grass: only water shows, and the dirt only touches corners
	// the water edges already cover, so no mid layer.
	i = l.Index(1, 1)
	if got, want := l.Ground.Under[i], 1+uint16(FullState); got != want {
		t.Errorf("middle grass underlay = %d, want water centre %d", got, want)
	}
	if l.Ground.Mid[i] != 0 {
		t.Errorf("middle grass mid = %d, want none", l.Ground.Mid[i])
	}
}

func TestDiagonalOnlyLowerTerrainIsNotTheUnderlay(t *testing.T) {
	d := testDefs(t)
	// Each grass cell touches dirt only at a corner its open edges already
	// cover, so what shows through is the water beside it, not the dirt.
	l := solved(t, d,
		"wwwww",
		"wwwww",
		"wwdww",
		"wgwgw",
		"wwwww",
	)
	for _, p := range []Point{{1, 3}, {3, 3}} {
		if got, want := l.Ground.Under[l.Index(p.X, p.Y)], 1+uint16(FullState); got != want {
			t.Errorf("grass at %v underlay = %d, want water centre %d", p, got, want)
		}
	}
	if got, want := l.Ground.Under[l.Index(2, 2)], 1+uint16(FullState); got != want {
		t.Errorf("dirt underlay = %d, want water centre %d", got, want)
	}
	// A corner between two joined edges does show through.
	l = solved(t, d,
		"gg",
		"gd",
	)
	if got, want := l.Ground.Under[l.Index(0, 0)], 48+uint16(FullState); got != want {
		t.Errorf("grass with an inner dirt corner underlay = %d, want dirt centre %d", got, want)
	}
}

func TestVoidJoinedEmptyNot(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d, "g")
	if got := tileState(t, d, l, 0, 0); got != FullState {
		t.Errorf("grass surrounded by void state = %d, want full (void is joined)", got)
	}
	l = solved(t, d,
		"...",
		".g.",
		"...",
	)
	if got := tileState(t, d, l, 1, 1); got != 0 {
		t.Errorf("grass surrounded by empty cells state = %d, want isolated", got)
	}
	if l.Ground.Under[l.Index(1, 1)] != 0 {
		t.Error("empty neighbours should give no underlay")
	}
	if l.Ground.Tile[l.Index(0, 0)] != 0 {
		t.Error("empty cells should have no tile")
	}
}

func TestNoEdgeTerrainIsJoinedAndDrawsItsTile(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d,
		"bbb",
		"bgb",
		"bbb",
	)
	if got := tileState(t, d, l, 1, 1); got != FullState {
		t.Errorf("grass in black state = %d, want full (black is joined)", got)
	}
	if got := l.Ground.Tile[l.Index(0, 0)]; got != 142 {
		t.Errorf("black tile = %d, want its single tile 142", got)
	}
}

func TestLockedCellsUntouchedButCounted(t *testing.T) {
	d := testDefs(t)
	l := paint(t, "test", Point{},
		"ddd",
		"dgd",
		"ddd",
	)
	// Lock the centre with a hand-picked tile, and make a dirt cell's
	// tile something the solver would never pick.
	c := l.Index(1, 1)
	l.Ground.Flags[c] |= GroundLocked
	l.Ground.Tile[c] = 95 + 5
	l.Ground.Under[c] = 1 + FullState
	SolveAll(l, LevelSource{l}, d)
	if l.Ground.Tile[c] != 95+5 || l.Ground.Under[c] != 1+FullState {
		t.Errorf("locked cell rewritten: tile %d under %d", l.Ground.Tile[c], l.Ground.Under[c])
	}
	// Its neighbours still see grass there: the dirt to its left is joined
	// on the east (grass outranks dirt).
	if got := tileState(t, d, l, 0, 1); got != FullState {
		t.Errorf("dirt next to locked grass state = %d, want full", got)
	}
	// Painting over a locked cell unlocks it.
	l.SetTerrain(1, 1, tDirt)
	SolveAround(l, 1, 1, LevelSource{l}, d)
	if l.Ground.Flags[c]&GroundLocked != 0 || tileState(t, d, l, 1, 1) != FullState {
		t.Error("painting over a locked cell should unlock and re-solve it")
	}
}

func TestSolveAroundIsLocal(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d,
		"ddddd",
		"ddddd",
		"ddddd",
		"ddddd",
		"ddddd",
	)
	l.SetTerrain(0, 0, tGrass)
	SolveAround(l, 0, 0, LevelSource{l}, d)
	if got := tileState(t, d, l, 0, 0); got != StateOf(MaskN|MaskNW|MaskW|MaskNE|MaskSW) {
		t.Errorf("corner grass state = %d", got)
	}
	// Re-solving everything gives the same result as the local solve.
	before := append([]uint16(nil), l.Ground.Tile...)
	SolveAll(l, LevelSource{l}, d)
	for i := range before {
		if before[i] != l.Ground.Tile[i] {
			t.Fatalf("cell %d: local solve %d, full solve %d", i, before[i], l.Ground.Tile[i])
		}
	}
}
