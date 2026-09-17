package shared

import "testing"

func TestVec2DistanceTo(t *testing.T) {
	a := Vec2{X: 0, Y: 0}
	b := Vec2{X: 3, Y: 4}
	if got := a.DistanceTo(b); got != 5 {
		t.Errorf("DistanceTo() = %v, want 5", got)
	}
}

func TestTileTypeIsPassable(t *testing.T) {
	cases := map[TileType]bool{
		TileTypeBlocked:    false,
		TileTypeWalkable:   true,
		TileTypeFlyable:    true,
		TileTypeJumpable:   true,
		TileTypeCrouchable: true,
		TileTypeSwimable:   true,
		TileTypeLava:       false,
	}
	for tile, want := range cases {
		if got := tile.IsPassable(); got != want {
			t.Errorf("TileType(%d).IsPassable() = %v, want %v", tile, got, want)
		}
	}
}

func TestNewCollisionLayerAllWalkable(t *testing.T) {
	c := NewCollisionLayer(4, 4)
	for y := uint32(0); y < 4; y++ {
		for x := uint32(0); x < 4; x++ {
			if c.IsBlocked(x, y) {
				t.Errorf("tile (%d,%d) should be walkable by default", x, y)
			}
		}
	}
}

func TestCollisionLayerSetGet(t *testing.T) {
	c := NewCollisionLayer(4, 4)
	c.Set(2, 1, TileTypeBlocked)

	if got := c.Get(2, 1); got != TileTypeBlocked {
		t.Errorf("Get(2,1) = %v, want TileTypeBlocked", got)
	}
	if !c.IsBlocked(2, 1) {
		t.Error("IsBlocked(2,1) = false, want true after Set to Blocked")
	}
	// Neighboring tile untouched.
	if c.IsBlocked(1, 1) {
		t.Error("IsBlocked(1,1) = true, want false (untouched tile)")
	}
}

func TestCollisionLayerOutOfBoundsIsBlocked(t *testing.T) {
	c := NewCollisionLayer(4, 4)
	if !c.IsBlocked(10, 10) {
		t.Error("out-of-bounds tile should report Blocked, got walkable")
	}
	if got := c.Get(10, 10); got != TileTypeBlocked {
		t.Errorf("Get out-of-bounds = %v, want TileTypeBlocked", got)
	}
}

func TestCollisionLayerSetOutOfBoundsIsNoop(t *testing.T) {
	c := NewCollisionLayer(4, 4)
	// Must not panic, and must not corrupt in-bounds tiles.
	c.Set(100, 100, TileTypeBlocked)
	if c.IsBlocked(0, 0) {
		t.Error("out-of-bounds Set corrupted an in-bounds tile")
	}
}
