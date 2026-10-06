package world

import (
	"strings"
	"testing"
)

func TestParseDefsResolves(t *testing.T) {
	d := testDefs(t)
	water := d.TerrainByName("water")
	if water == nil || water.SheetBase != 1 || !water.Edges {
		t.Fatalf("water = %+v", water)
	}
	if water.SurfaceID != 2 || water.InteractionID != 1 {
		t.Errorf("water surface/interaction = %d/%d, want 2/1", water.SurfaceID, water.InteractionID)
	}
	black := d.Terrain(tBlack)
	if black.Edges || black.TileIndex != 142 {
		t.Errorf("black = edges %v tile %d, want false 142", black.Edges, black.TileIndex)
	}
	if black.Blocks&BlockGround == 0 || black.Blocks&BlockMagic == 0 {
		t.Errorf("black blocks = %b, want all seven flags", black.Blocks)
	}
	if d.Terrain(0) != nil {
		t.Error("terrain 0 should be empty (nil)")
	}
	if p := d.TileProps(200 + 9); p.Blocks != BlockGround|BlockRoll {
		t.Errorf("fence cell 9 blocks = %b", p.Blocks)
	}
	if p := d.TileProps(300); !p.HasSurface || p.Surface != 4 || !p.HasInteraction {
		t.Errorf("bridge props = %+v", p)
	}
	if p := d.TileProps(200 + 5); p != (TileProps{}) {
		t.Errorf("undeclared tile props = %+v, want zero", p)
	}
}

func TestMayFlip(t *testing.T) {
	d := testDefs(t)
	cases := map[uint16]bool{
		200: true,  // fences sheet is flippable
		204: false, // per-tile override
		1:   false, // blob sheets default to not flippable
		999: false, // no sheet
	}
	for tile, want := range cases {
		if got := d.MayFlip(tile); got != want {
			t.Errorf("MayFlip(%d) = %v, want %v", tile, got, want)
		}
	}
}

func TestSheetOf(t *testing.T) {
	d := testDefs(t)
	s, cell := d.SheetOf(95 + 46)
	if s == nil || s.Name != "grass" || cell != 46 {
		t.Errorf("SheetOf(141) = %v, %d", s, cell)
	}
	if s, _ := d.SheetOf(0); s != nil {
		t.Errorf("SheetOf(0) = %v, want nil", s.Name)
	}
}

func TestParseDefsErrors(t *testing.T) {
	base := testDefsTOML
	cases := []struct {
		name, edit, want string
	}{
		{"duplicate terrain id", "\n[[terrains]]\nid = 1\nname = \"lava\"\nsheet = \"water\"\npriority = 99\n", "duplicate terrain id"},
		{"terrain id 0", "\n[[terrains]]\nid = 0\nname = \"x\"\nsheet = \"water\"\npriority = 99\n", "reserved"},
		{"unknown sheet", "\n[[terrains]]\nid = 9\nname = \"x\"\nsheet = \"nope\"\npriority = 99\n", "unknown sheet"},
		{"no tile without edges", "\n[[terrains]]\nid = 9\nname = \"x\"\nedges = false\n", "needs a tile"},
		{"small blob sheet", "\n[[terrains]]\nid = 9\nname = \"x\"\nsheet = \"bridges\"\npriority = 99\n", "blob-47 sheet needs"},
		{"shared priority", "\n[[terrains]]\nid = 9\nname = \"x\"\nsheet = \"water\"\npriority = 10\n", "share priority"},
		{"overlapping sheets", "\n[[sheets]]\nname = \"x\"\npath = \"x.png\"\nbase = 40\ncells = 10\n", "overlapping tile ranges"},
		{"base 0", "\n[[sheets]]\nname = \"x\"\npath = \"x.png\"\nbase = 0\ncells = 1\n", "base must be at least 1"},
		{"unknown surface", "\n[[terrains]]\nid = 9\nname = \"x\"\nsheet = \"water\"\npriority = 99\nsurface = \"lava\"\n", "unknown surface"},
		{"unknown flag", "\n[[terrains]]\nid = 9\nname = \"x\"\nsheet = \"water\"\npriority = 99\nblocks = [\"sight\"]\n", "unknown blocking flag"},
		{"bad tile_props cell", "\n[[tile_props]]\nsheet = \"bridges\"\ncells = [4]\n", "has no cell 4"},
		{"tile listed twice", "\n[[tile_props]]\nsheet = \"bridges\"\ncells = [0]\n", "listed twice"},
		{"unknown key", "\nflavour = 1\n", "unknown key"},
		{"zero speed", "\n[[interactions]]\nid = 7\nname = \"stuck\"\nspeed_multiplier = 0.0\n", "must be positive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := base + c.edit
			if c.name == "unknown key" {
				src = c.edit + base
			}
			_, err := ParseDefs([]byte(src))
			wantErr(t, err, c.want)
		})
	}
}

func TestParseDefsNeedsDefaultIDs(t *testing.T) {
	src := strings.Replace(testDefsTOML, "id = 0\nname = \"none\"", "id = 9\nname = \"none\"", 1)
	_, err := ParseDefs([]byte(src))
	wantErr(t, err, "surface id 0")
}

func TestSampleDefsLoad(t *testing.T) {
	d, err := LoadDefs("../../world/terrains.toml")
	if err != nil {
		t.Fatalf("world/terrains.toml: %v", err)
	}
	for _, name := range []string{"water", "dirt", "grass", "black"} {
		if d.TerrainByName(name) == nil {
			t.Errorf("sample defs lack terrain %q", name)
		}
	}
}
