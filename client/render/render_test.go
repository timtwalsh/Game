package render

import (
	"testing"

	"game/shared"
	"game/shared/world"
)

func TestFollowZoomStepsAreWholeAndClamped(t *testing.T) {
	c := NewFollowCamera()
	c.ZoomBy(1)
	if c.Zoom != 3 {
		t.Errorf("zoom in from 2 = %v, want 3", c.Zoom)
	}
	c.ZoomBy(0.3) // a trackpad's fractional wheel step still moves one whole step
	c.ZoomBy(5)
	if c.Zoom != c.MaxZoom {
		t.Errorf("zoom = %v, want clamped to %v", c.Zoom, c.MaxZoom)
	}
	for i := 0; i < 10; i++ {
		c.ZoomBy(-1)
	}
	if c.Zoom != c.MinZoom {
		t.Errorf("zoom = %v, want clamped to %v", c.Zoom, c.MinZoom)
	}
}

func TestFreeZoomGoesFurtherOut(t *testing.T) {
	c := NewFreeCamera()
	for i := 0; i < 40; i++ {
		c.ZoomBy(-1)
	}
	if c.Zoom != c.MinZoom || c.MinZoom >= NewFollowCamera().MinZoom {
		t.Errorf("free camera min zoom %v should be below the player's %v", c.Zoom, NewFollowCamera().MinZoom)
	}
}

func TestVisibleCoversTheScreen(t *testing.T) {
	c := NewFollowCamera() // zoom 2: an 800x600 screen shows 400x300 world px
	c.CentreOn(shared.Vec2{X: 0, Y: 0})
	v := c.Visible(800, 600)
	// -200..200 px is tiles -12.5..12.5, plus a tile of margin each way.
	if v.X0 > -13 || v.X1 < 13 || v.Y0 > -10 || v.Y1 < 10 {
		t.Errorf("visible = %+v, too small", v)
	}
	if v.X1-v.X0 > 30 || v.Y1-v.Y0 > 24 {
		t.Errorf("visible = %+v, far larger than the screen", v)
	}
}

func TestScreenToWorld(t *testing.T) {
	c := NewFollowCamera()
	c.CentreOn(shared.Vec2{X: 100, Y: 50})
	if p := c.ScreenToWorld(400, 300, 800, 600); p != (shared.Vec2{X: 100, Y: 50}) {
		t.Errorf("screen centre = %v, want the target", p)
	}
	if p := c.ScreenToWorld(0, 0, 800, 600); p != (shared.Vec2{X: -100, Y: -100}) {
		t.Errorf("top-left = %v, want (-100,-100) at zoom 2", p)
	}
}

func TestRaylibCameraSnapsToScreenPixels(t *testing.T) {
	c := NewFollowCamera()
	c.CentreOn(shared.Vec2{X: 10.3, Y: 7.74})
	rc := c.Raylib(800, 600)
	if rc.Target.X != 10.5 || rc.Target.Y != 7.5 {
		t.Errorf("target = (%v, %v), want snapped to half pixels at zoom 2", rc.Target.X, rc.Target.Y)
	}
	if rc.Offset.X != 400 || rc.Offset.Y != 300 {
		t.Errorf("offset = %v, want the screen centre", rc.Offset)
	}
}

func TestLayerPass(t *testing.T) {
	cases := map[string]Pass{"ground": PassUnder, "decor": PassUnder, "ysort": PassYSort, "overhead": PassOverhead}
	for _, l := range world.DefaultLayers() {
		if got := LayerPass(l); got != cases[l.Name] {
			t.Errorf("LayerPass(%s) = %v, want %v", l.Name, got, cases[l.Name])
		}
	}
}

func TestSourceRect(t *testing.T) {
	r := SourceRect(9, 8, 0) // row 1, col 1 of an 8-wide sheet
	if r.X != 16 || r.Y != 16 || r.Width != 16 || r.Height != 16 {
		t.Errorf("SourceRect(9) = %+v", r)
	}
	r = SourceRect(0, 8, world.TileFlipH|world.TileFlipV)
	if r.Width != -16 || r.Height != -16 {
		t.Errorf("flipped SourceRect = %+v, want negative size", r)
	}
}

func TestYSortItemsSortWithPlayers(t *testing.T) {
	defs, err := world.LoadDefs("../../world/terrains.toml")
	if err != nil {
		t.Fatal(err)
	}
	l, err := world.NewLevel("a", world.Point{X: -2, Y: 0}, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	ysort := l.LayerIndex("ysort") - 1
	overhead := l.LayerIndex("overhead") - 1
	l.Upper[ysort].Tile[l.Index(1, 2)] = 1 // a "fence" at world tile (-1, 2)
	l.Upper[overhead].Tile[l.Index(0, 0)] = 1
	w := world.NewWorld(defs)
	if err := w.Add(l); err != nil {
		t.Fatal(err)
	}
	r := &Renderer{World: w}

	items := r.AppendYSort(nil, TileRect{-10, -10, 10, 10})
	if len(items) != 1 {
		t.Fatalf("got %d ysort items, want 1 (overhead tiles aren't sorted)", len(items))
	}
	if it := items[0]; it.X != -16 || it.Y != 32 || it.Key != 48 || it.Player != -1 {
		t.Errorf("fence item = %+v, want top-left (-16, 32), key 48", it)
	}
	// A player whose feet are above the fence's bottom edge is behind it;
	// one whose feet are below is in front.
	items = append(items, Item{Key: 40, Player: 0}, Item{Key: 60, Player: 1})
	SortItems(items)
	if items[0].Player != 0 || items[1].Player != -1 || items[2].Player != 1 {
		t.Errorf("order = %+v, want player 0, fence, player 1", items)
	}
	// Culling: a view that misses the level yields nothing.
	if got := r.AppendYSort(nil, TileRect{5, 5, 9, 9}); len(got) != 0 {
		t.Errorf("off-screen level gave %d items", len(got))
	}
}
