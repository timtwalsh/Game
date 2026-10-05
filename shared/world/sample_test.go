package world

import (
	"flag"
	"os"
	"reflect"
	"testing"

	"game/shared"
)

var update = flag.Bool("update", false, "rewrite the sample levels in levels/")

const (
	sampleDefs   = "../../world/terrains.toml"
	sampleLevels = "../../levels"
)

// sampleWorld builds the development levels in levels/: two adjoining
// exterior levels and one isolated interior. They're generated here, not
// hand-edited, so they always match the current format. Regenerate with
//
//	go test ./shared/world -run TestSampleLevels -update
func sampleWorld(t *testing.T, d *Defs) *World {
	t.Helper()
	id := func(name string) uint8 { return d.TerrainByName(name).ID }
	grass, dirt, water, black := id("grass"), id("dirt"), id("water"), id("black")

	fill := func(l *Level, x0, y0, x1, y1 int, terrain uint8) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				l.SetTerrain(x, y, terrain)
			}
		}
	}

	// meadow: grass with a dirt path running east into the lake level,
	// and a dirt yard in front of a house.
	meadow, err := NewLevel("meadow", Point{0, 0}, 32, 24)
	if err != nil {
		t.Fatal(err)
	}
	fill(meadow, 0, 0, 32, 24, grass)
	fill(meadow, 4, 11, 32, 13, dirt)
	fill(meadow, 18, 4, 24, 11, dirt)
	fill(meadow, 2, 18, 6, 21, water)
	spawnX, spawnY := shared.SpawnPoint.X, shared.SpawnPoint.Y
	meadow.Objects = []shared.GameObject{
		{ID: 1, Kind: "spawn", X: spawnX, Y: spawnY, CollisionType: shared.CollisionTypePassthrough},
		{ID: 2, Kind: "warp", X: 21 * shared.TileSize, Y: 5 * shared.TileSize,
			CollisionType: shared.CollisionTypePassthrough,
			Properties:    map[string]string{"destination": "house_1", "dest_x": "96.0", "dest_y": "128.0"}},
	}

	// lake: a pond ringed with a dirt shore; the path from meadow ends at it.
	lake, err := NewLevel("lake", Point{32, 0}, 24, 24)
	if err != nil {
		t.Fatal(err)
	}
	fill(lake, 0, 0, 24, 24, grass)
	fill(lake, 0, 11, 8, 13, dirt)
	fill(lake, 7, 5, 21, 19, dirt)
	fill(lake, 9, 7, 19, 17, water)

	// house_1: an interior parked well away from the overworld, with
	// blacked-out surroundings (D21).
	house, err := NewLevel("house_1", Point{5000, 5000}, 12, 10)
	if err != nil {
		t.Fatal(err)
	}
	house.Isolated = true
	fill(house, 0, 0, 12, 10, black)
	fill(house, 2, 2, 10, 9, dirt)
	house.Objects = []shared.GameObject{
		{ID: 1, Kind: "warp", X: 6 * shared.TileSize, Y: 9 * shared.TileSize,
			CollisionType: shared.CollisionTypePassthrough,
			Properties:    map[string]string{"destination": "meadow", "dest_x": "336.0", "dest_y": "104.0"}},
	}

	w := NewWorld(d)
	for _, l := range []*Level{meadow, lake, house} {
		if err := w.Add(l); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range w.Levels {
		w.SolveLevel(l)
	}
	return w
}

func TestSampleLevels(t *testing.T) {
	d, err := LoadDefs(sampleDefs)
	if err != nil {
		t.Fatal(err)
	}
	want := sampleWorld(t, d)
	if *update {
		if err := os.MkdirAll(sampleLevels, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, l := range want.Levels {
			if err := SaveLevel(sampleLevels, l); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := LoadWorld(sampleLevels, d)
	if err != nil {
		t.Fatalf("%v (regenerate with -update)", err)
	}
	if len(got.Levels) != len(want.Levels) {
		t.Fatalf("levels/ holds %d levels, want %d (regenerate with -update)", len(got.Levels), len(want.Levels))
	}
	for i, g := range got.Levels {
		w := want.Levels[i]
		g.Hash, w.Hash = "", ""
		if !reflect.DeepEqual(g, w) {
			t.Errorf("levels/%s is out of date; regenerate with -update", w.Name)
		}
	}

	// The sample world behaves as described.
	if got.Neighbours(got.Level("meadow"))[0].Name != "lake" {
		t.Error("meadow and lake should be neighbours")
	}
	sx, sy := TileOf(shared.SpawnPoint.X), TileOf(shared.SpawnPoint.Y)
	if got.Blocks(sx, sy, BlockGround) {
		t.Error("the spawn point is blocked")
	}
	if !got.Blocks(5000, 5000, BlockGround) || got.Blocks(5004, 5004, BlockGround) {
		t.Error("house_1 should be blocked in the blacked-out border and open on the floor")
	}
}
