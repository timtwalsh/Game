package world

import (
	"strings"
	"testing"
)

// testDefsTOML is a small, self-contained world for unit tests. Terrain ids:
// 1 water (10), 2 dirt (20), 3 grass (30), 4 black (no edges), 5 chasm (5,
// blocks ground).
const testDefsTOML = `
[[sheets]]
name = "water"
path = "water.png"
base = 1
cells = 47

[[sheets]]
name = "dirt"
path = "dirt.png"
base = 48
cells = 47

[[sheets]]
name = "grass"
path = "grass.png"
base = 95
cells = 47

[[sheets]]
name = "black"
path = "black.png"
base = 142
cells = 1

[[sheets]]
name = "chasm"
path = "chasm.png"
base = 143
cells = 47

[[sheets]]
name = "fences"
path = "fences.png"
base = 200
cells = 16
flippable = true

[[sheets]]
name = "bridges"
path = "bridges.png"
base = 300
cells = 4

[[terrains]]
id = 1
name = "water"
priority = 10
sheet = "water"
surface = "water"
interaction = "swimming"

[[terrains]]
id = 2
name = "dirt"
priority = 20
sheet = "dirt"
surface = "dirt"

[[terrains]]
id = 3
name = "grass"
priority = 30
sheet = "grass"
surface = "grass"

[[terrains]]
id = 4
name = "black"
tile = "black:0"
edges = false
blocks = ["ground", "projectile", "flight", "jump", "roll", "ethereal", "magic"]

[[terrains]]
id = 5
name = "chasm"
priority = 5
sheet = "chasm"
blocks = ["ground", "roll"]

[[surfaces]]
id = 0
name = "none"

[[surfaces]]
id = 1
name = "grass"

[[surfaces]]
id = 2
name = "water"

[[surfaces]]
id = 3
name = "dirt"

[[surfaces]]
id = 4
name = "wood"

[[interactions]]
id = 0
name = "normal"
speed_multiplier = 1.0

[[interactions]]
id = 1
name = "swimming"
speed_multiplier = 0.5

[[tile_props]]
sheet = "fences"
cells = [0, 1, 2, 3, 8, 9, 10, 11]
blocks = ["ground", "roll"]

[[tile_props]]
sheet = "fences"
cells = [4]
flippable = false

[[tile_props]]
sheet = "bridges"
cells = [0, 1, 2]
surface = "wood"
interaction = "normal"
`

const (
	tWater uint8 = 1
	tDirt  uint8 = 2
	tGrass uint8 = 3
	tBlack uint8 = 4
	tChasm uint8 = 5
)

func testDefs(t *testing.T) *Defs {
	t.Helper()
	d, err := ParseDefs([]byte(testDefsTOML))
	if err != nil {
		t.Fatalf("ParseDefs: %v", err)
	}
	return d
}

// paint builds a level from rows of terrain letters: '.' empty, w water,
// d dirt, g grass, b black, c chasm. It does not solve.
func paint(t *testing.T, name string, pos Point, rows ...string) *Level {
	t.Helper()
	l, err := NewLevel(name, pos, len(rows[0]), len(rows))
	if err != nil {
		t.Fatalf("NewLevel: %v", err)
	}
	ids := map[byte]uint8{'.': 0, 'w': tWater, 'd': tDirt, 'g': tGrass, 'b': tBlack, 'c': tChasm}
	for y, row := range rows {
		if len(row) != l.W {
			t.Fatalf("row %d has %d cells, want %d", y, len(row), l.W)
		}
		for x := range row {
			id, ok := ids[row[x]]
			if !ok {
				t.Fatalf("unknown terrain letter %q", row[x])
			}
			l.SetTerrain(x, y, id)
		}
	}
	return l
}

// solved paints and solves a lone level (void all round) and compiles it.
func solved(t *testing.T, d *Defs, rows ...string) *Level {
	t.Helper()
	l := paint(t, "test", Point{}, rows...)
	SolveAll(l, LevelSource{l}, d)
	Compile(l, d)
	return l
}

func wantErr(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want one containing %q", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("error %q does not contain %q", err, substr)
	}
}
