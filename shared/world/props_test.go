package world

import "testing"

func props(l *Level, x, y int) (uint16, uint8, uint8) {
	i := l.Index(x, y)
	return l.Props.Blocking[i], l.Props.Surface[i], l.Props.Interaction[i]
}

func TestCompileTerrainDefaults(t *testing.T) {
	d := testDefs(t)
	l := solved(t, d, "gw.")
	if b, s, i := props(l, 0, 0); b != 0 || s != 1 || i != 0 {
		t.Errorf("grass = %b/%d/%d, want open/grass/normal", b, s, i)
	}
	if b, s, i := props(l, 1, 0); b != 0 || s != 2 || i != 1 {
		t.Errorf("water = %b/%d/%d, want open/water/swimming", b, s, i)
	}
	if b, _, _ := props(l, 2, 0); b != BlockAll {
		t.Errorf("empty ground blocking = %b, want everything", b)
	}
}

func TestFenceBlocksGroundNotJump(t *testing.T) {
	d := testDefs(t)
	l := paint(t, "test", Point{}, "g")
	l.Upper[l.LayerIndex("ysort")-1].Tile[0] = 200 // fence post
	Compile(l, d)
	b, _, _ := props(l, 0, 0)
	if b&BlockGround == 0 || b&BlockRoll == 0 {
		t.Errorf("fence blocking = %b, want ground and roll", b)
	}
	if b&BlockJump != 0 || b&BlockProjectile != 0 {
		t.Errorf("fence blocking = %b, should not block jump or projectiles", b)
	}
}

func TestBridgeOverWater(t *testing.T) {
	d := testDefs(t)
	l := paint(t, "test", Point{}, "w")
	l.Upper[l.LayerIndex("decor")-1].Tile[0] = 300 // bridge plank
	Compile(l, d)
	if b, s, i := props(l, 0, 0); b != 0 || s != 4 || i != 0 {
		t.Errorf("bridge over water = %b/%d/%d, want open/wood/normal", b, s, i)
	}
}

func TestBridgeOverChasmNeedsOverride(t *testing.T) {
	d := testDefs(t)
	l := paint(t, "test", Point{}, "c")
	l.Upper[0].Tile[0] = 300
	Compile(l, d)
	if b, _, _ := props(l, 0, 0); b&BlockGround == 0 {
		t.Fatalf("bridge over chasm blocking = %b; strictest-wins means a tile can't unblock", b)
	}
	l.SetOverride(0, 0, OverrideBlocking, 0)
	Compile(l, d)
	if b, s, _ := props(l, 0, 0); b != 0 || s != 4 {
		t.Errorf("with override = %b/%d, want open/wood", b, s)
	}
}

func TestOverrideOnlyTouchesItsMap(t *testing.T) {
	d := testDefs(t)
	l := paint(t, "test", Point{}, "w")
	l.SetOverride(0, 0, OverrideSurface, 3) // dirt
	Compile(l, d)
	if b, s, i := props(l, 0, 0); b != 0 || s != 3 || i != 1 {
		t.Errorf("= %b/%d/%d, want open/dirt (overridden)/swimming (untouched)", b, s, i)
	}
	l.ClearOverride(0, 0, OverrideSurface)
	Compile(l, d)
	if _, s, _ := props(l, 0, 0); s != 2 {
		t.Errorf("after clearing, surface = %d, want water", s)
	}
}

func TestTopmostTileWins(t *testing.T) {
	d := testDefs(t)
	l := paint(t, "test", Point{}, "g")
	l.Upper[0].Tile[0] = 300 // decor: bridge, wood surface
	l.Upper[2].Tile[0] = 200 // overhead: fence, declares no surface
	Compile(l, d)
	if b, s, _ := props(l, 0, 0); s != 4 || b&BlockGround == 0 {
		t.Errorf("= %b/%d, want blocked (fence) and wood (topmost tile that declares one)", b, s)
	}
}
