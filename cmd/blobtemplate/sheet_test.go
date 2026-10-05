package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"game/shared/world"
)

// readDiagram recovers the neighbour mask a template cell shows.
func readDiagram(t *testing.T, img *image.NRGBA, i int) uint8 {
	t.Helper()
	ox, oy := cellOrigin(i)
	if c := img.NRGBAAt(ox+diagramPos(1), oy+diagramPos(1)); c != templateCentre {
		t.Fatalf("cell %d: centre square is %v", i, c)
	}
	var m uint8
	for _, n := range neighbourGrid {
		switch c := img.NRGBAAt(ox+diagramPos(n.gx), oy+diagramPos(n.gy)); c {
		case templateJoined:
			m |= n.bit
		case templateOpen:
		default:
			t.Fatalf("cell %d: neighbour square has unexpected colour %v", i, c)
		}
	}
	return m
}

func TestTemplateMatchesSolver(t *testing.T) {
	img := Template()
	if b := img.Bounds(); b.Dx() != 8*16 || b.Dy() != 6*16 {
		t.Fatalf("template is %dx%d, want 128x96", b.Dx(), b.Dy())
	}
	states := world.BlobStates()
	for i, want := range states {
		if got := readDiagram(t, img, i); got != want {
			t.Errorf("cell %d shows mask %08b, solver says %08b", i, got, want)
		}
		// And the solver maps that mask back to this cell.
		if world.StateOf(want) != i {
			t.Errorf("cell %d: StateOf(%08b) = %d", i, want, world.StateOf(want))
		}
	}
	ox, oy := cellOrigin(len(states))
	if img.NRGBAAt(ox, oy) != templateSpare {
		t.Error("spare cell is not crossed out")
	}
}

func TestPlaceholderCoverage(t *testing.T) {
	c, _ := ParseColour("4a9c3b")
	img := Placeholder(c)
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 96 {
		t.Fatalf("placeholder is %dx%d", b.Dx(), b.Dy())
	}
	alpha := func(i, px, py int) uint8 {
		ox, oy := cellOrigin(i)
		return img.NRGBAAt(ox+px, oy+py).A
	}
	isolated, full := world.StateOf(0), world.StateOf(255)
	// Isolated: only the centre is drawn.
	if alpha(isolated, 8, 8) == 0 || alpha(isolated, 0, 8) != 0 || alpha(isolated, 0, 0) != 0 {
		t.Error("isolated cell should cover the centre only")
	}
	// Full: every pixel is drawn, in the plain colour (no rim).
	for py := 0; py < 16; py++ {
		for px := 0; px < 16; px++ {
			ox, oy := cellOrigin(full)
			if img.NRGBAAt(ox+px, oy+py) != c {
				t.Fatalf("full cell pixel (%d,%d) = %v, want %v", px, py, img.NRGBAAt(ox+px, oy+py), c)
			}
		}
	}
	// N, E, S, W joined but no corners: four transparent inner-corner notches.
	cross := world.StateOf(world.MaskN | world.MaskE | world.MaskS | world.MaskW)
	if alpha(cross, 0, 0) != 0 || alpha(cross, 15, 15) != 0 || alpha(cross, 8, 0) == 0 || alpha(cross, 0, 8) == 0 {
		t.Error("N+E+S+W cell should reach every edge but leave the corners open")
	}
	// The spare cell is empty.
	if alpha(world.BlobTileCount, 8, 8) != 0 {
		t.Error("spare cell should be transparent")
	}
}

func TestFlatSheet(t *testing.T) {
	c, _ := ParseColour("#000000")
	img := FlatSheet(3, 1, c)
	if img.Bounds().Dx() != 48 || img.Bounds().Dy() != 16 {
		t.Fatalf("flat sheet is %v", img.Bounds())
	}
	if img.NRGBAAt(20, 5) != c || img.NRGBAAt(5, 5).A != 0 {
		t.Error("only the chosen cell should be filled")
	}
}

func TestParseColour(t *testing.T) {
	for _, bad := range []string{"", "fff", "zzzzzz", "#1234567"} {
		if _, err := ParseColour(bad); err == nil {
			t.Errorf("ParseColour(%q) accepted", bad)
		}
	}
	if c, err := ParseColour("#102030"); err != nil || c.R != 0x10 || c.G != 0x20 || c.B != 0x30 || c.A != 0xFF {
		t.Errorf("ParseColour(#102030) = %v, %v", c, err)
	}
}

func TestRunWritesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := run([]string{"-terrain", "grass", "-out", dir, "-scale", "4"}); err != nil {
		t.Fatal(err)
	}
	img := decode(t, filepath.Join(dir, "grass_blob47_template_x4.png"))
	if img.Bounds().Dx() != 512 || img.Bounds().Dy() != 384 {
		t.Errorf("scaled template is %v", img.Bounds())
	}

	defs := "../../world/terrains.toml"
	if err := run([]string{"-terrain", "grass", "-placeholder", "4a9c3b", "-defs", defs, "-root", dir}); err != nil {
		t.Fatal(err)
	}
	if img := decode(t, filepath.Join(dir, "assets", "tiles", "grass_blob47.png")); img.Bounds().Dx() != 128 {
		t.Errorf("grass placeholder is %v", img.Bounds())
	}
	if err := run([]string{"-terrain", "black", "-placeholder", "000000", "-defs", defs, "-root", dir}); err != nil {
		t.Fatal(err)
	}
	if img := decode(t, filepath.Join(dir, "assets", "tiles", "black_tiles.png")); img.Bounds().Dx() != 16 {
		t.Errorf("black placeholder is %v", img.Bounds())
	}

	if err := run([]string{"-terrain", "lava", "-placeholder", "ff0000", "-defs", defs, "-root", dir}); err == nil {
		t.Error("unknown terrain accepted")
	}
	if err := run([]string{"-placeholder", "ff0000", "-defs", defs, "-root", dir}); err == nil {
		t.Error("-placeholder without -terrain accepted")
	}
	if err := run([]string{"-out", dir}); err != nil {
		t.Fatal(err)
	}
	decode(t, filepath.Join(dir, "blob47_template.png"))
}

// TestCommittedPlaceholders checks the placeholder sheets in assets/tiles
// still match what this tool generates for every terrain in
// world/terrains.toml, so the art can't drift from the solver's order.
func TestCommittedPlaceholders(t *testing.T) {
	defs, err := world.LoadDefs("../../world/terrains.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range defs.Terrains {
		var sheet *world.Sheet
		if tr.Edges {
			sheet = defs.SheetByName(tr.Sheet)
		} else {
			sheet, _ = defs.SheetOf(tr.TileIndex)
		}
		path := filepath.Join("..", "..", filepath.FromSlash(sheet.Path))
		img := decode(t, path)
		if tr.Edges {
			if img.Bounds().Dx() != 128 || img.Bounds().Dy() != 96 {
				t.Errorf("%s is %v, want a 128x96 blob-47 sheet", path, img.Bounds())
			}
			// Same coverage as a fresh placeholder, whatever its colour.
			fresh := Placeholder(img.NRGBAAt(cellOrigin(world.FullState)))
			for y := 0; y < 96; y++ {
				for x := 0; x < 128; x++ {
					if (img.NRGBAAt(x, y).A == 0) != (fresh.NRGBAAt(x, y).A == 0) {
						t.Fatalf("%s differs from the generated placeholder at (%d,%d); regenerate it (see docs/LEVEL_MAKER_SPEC.md step 2)", path, x, y)
					}
				}
			}
		}
	}
}

func decode(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	n, ok := img.(*image.NRGBA)
	if !ok {
		b := img.Bounds()
		n = image.NewNRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				n.Set(x, y, img.At(x, y))
			}
		}
	}
	return n
}
