package world

import (
	"testing"

	"game/shared"
)

var (
	_ Collider = (*World)(nil)
	_ Collider = (*Grid)(nil)
)

func TestWorldSpeedMultiplier(t *testing.T) {
	d := testDefs(t)
	w := testWorld(t, d, paint(t, "a", Point{}, "gw"))
	if got := w.SpeedMultiplier(0, 0); got != 1 {
		t.Errorf("grass speed = %v, want 1", got)
	}
	if got := w.SpeedMultiplier(1, 0); got != 0.5 {
		t.Errorf("water speed = %v, want 0.5", got)
	}
	if got := w.SpeedMultiplier(9, 9); got != 1 {
		t.Errorf("void speed = %v, want 1", got)
	}
}

func TestGrid(t *testing.T) {
	g := NewGrid(4, 3)
	if g.Blocks(0, 0, BlockGround) || g.Blocks(3, 2, BlockGround) {
		t.Error("a new grid should be open inside")
	}
	for _, p := range []Point{{-1, 0}, {0, -1}, {4, 0}, {0, 3}} {
		if !g.Blocks(p.X, p.Y, BlockGround) {
			t.Errorf("%v is outside and should block", p)
		}
	}
	g.SetBlocking(1, 1, BlockGround|BlockRoll)
	if !g.Blocks(1, 1, BlockGround) || g.Blocks(1, 1, BlockJump) {
		t.Error("SetBlocking flags not honoured per bit")
	}
	g.SetSpeed(2, 2, 0.5)
	if g.SpeedMultiplier(2, 2) != 0.5 || g.SpeedMultiplier(0, 0) != 1 {
		t.Error("SetSpeed not honoured")
	}
}

func TestSpawn(t *testing.T) {
	d := testDefs(t)
	inside := paint(t, "inside", Point{100, 100}, "g")
	inside.Isolated = true
	inside.Objects = []shared.GameObject{{Kind: "spawn", X: 1, Y: 1}}
	out := paint(t, "out", Point{2, 3}, "gg")
	out.Objects = []shared.GameObject{{Kind: "warp"}, {Kind: "spawn", X: 4, Y: 5}}
	w := testWorld(t, d, inside, out)
	pos, ok := w.Spawn()
	if !ok || pos != (shared.Vec2{X: 2*16 + 4, Y: 3*16 + 5}) {
		t.Errorf("Spawn() = %v, %v; want the exterior level's spawn in world pixels", pos, ok)
	}
	if _, ok := testWorld(t, d, paint(t, "b", Point{}, "g")).Spawn(); ok {
		t.Error("a world with no spawn object reported one")
	}
}

func TestLoadMap(t *testing.T) {
	m, err := LoadMap(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if m.World != nil || m.Spawn != shared.SpawnPoint || m.Collider.Blocks(FallbackSize-1, 0, BlockGround) {
		t.Errorf("an empty root should give the open fallback grid at shared.SpawnPoint, got %+v", m)
	}

	m, err = LoadMap("../..")
	if err != nil {
		t.Fatal(err)
	}
	if m.World == nil || len(m.World.Levels) != 3 {
		t.Fatalf("repo root should load the sample world, got %+v", m)
	}
	if m.Collider.Blocks(TileOf(m.Spawn.X), TileOf(m.Spawn.Y), BlockGround) {
		t.Error("the sample world's spawn is blocked")
	}
}
