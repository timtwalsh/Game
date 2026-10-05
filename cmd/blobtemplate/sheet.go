package main

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"game/shared"
	"game/shared/world"
)

// Blob-47 sheets are 8 cells wide and 6 tall: the 47 states in solver order
// (world.BlobStates) plus one spare cell at the end.
const (
	sheetCols = 8
	sheetRows = 6
	cell      = int(shared.TileSize)
)

// The template's mini-diagram: a 3x3 grid of 4 px squares with 1 px gaps,
// inset 1 px from the cell edge. Square (gx, gy) starts at diagramPos(gx).
const (
	diagSquare = 4
	diagStep   = 5
)

func diagramPos(g int) int { return 1 + g*diagStep }

// neighbourGrid maps each neighbour bit to its square in the 3x3 diagram.
var neighbourGrid = []struct {
	bit    uint8
	gx, gy int
}{
	{world.MaskN, 1, 0}, {world.MaskNE, 2, 0}, {world.MaskE, 2, 1}, {world.MaskSE, 2, 2},
	{world.MaskS, 1, 2}, {world.MaskSW, 0, 2}, {world.MaskW, 0, 1}, {world.MaskNW, 0, 0},
}

var (
	templateBG     = [2]color.NRGBA{{0xF2, 0xF2, 0xF2, 0xFF}, {0xE0, 0xE0, 0xE6, 0xFF}} // checkered cells
	templateCentre = color.NRGBA{0x30, 0x30, 0x30, 0xFF}
	templateJoined = color.NRGBA{0x3C, 0x8C, 0x3C, 0xFF}
	templateOpen   = color.NRGBA{0xC8, 0xC8, 0xC8, 0xFF}
	templateSpare  = color.NRGBA{0xC0, 0x40, 0x40, 0xFF}
)

// cellOrigin is the top-left pixel of sheet cell i, unscaled.
func cellOrigin(i int) (int, int) { return (i % sheetCols) * cell, (i / sheetCols) * cell }

// Template draws the labelled 8x6 template sheet for artists. Each used
// cell shows which neighbours are joined for its state: the dark centre is
// the cell itself, green squares are joined neighbours, grey ones are not.
// The spare cell is crossed out. The diagrams come straight from the
// solver's own state table, so the template can't drift from it.
func Template() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, sheetCols*cell, sheetRows*cell))
	states := world.BlobStates()
	for i := 0; i < sheetCols*sheetRows; i++ {
		ox, oy := cellOrigin(i)
		bg := templateBG[(i%sheetCols+i/sheetCols)%2]
		fill(img, ox, oy, cell, cell, bg)
		if i >= len(states) {
			for d := 0; d < cell; d++ {
				img.SetNRGBA(ox+d, oy+d, templateSpare)
				img.SetNRGBA(ox+cell-1-d, oy+d, templateSpare)
			}
			continue
		}
		mask := states[i]
		fill(img, ox+diagramPos(1), oy+diagramPos(1), diagSquare, diagSquare, templateCentre)
		for _, n := range neighbourGrid {
			c := templateOpen
			if mask&n.bit != 0 {
				c = templateJoined
			}
			fill(img, ox+diagramPos(n.gx), oy+diagramPos(n.gy), diagSquare, diagSquare, c)
		}
	}
	return img
}

// placeholderEdge is how far a terrain's placeholder art stops short of an
// unjoined side, leaving transparency for the underlay to show through.
const placeholderEdge = 4

// covered reports whether pixel (px, py) of a cell with reduced mask m is
// part of the terrain. The cell is split into a 3x3 of zones: the centre is
// always covered, an edge zone when that neighbour is joined, a corner zone
// when both its edges and the corner itself are joined.
func covered(m uint8, px, py int) bool {
	zone := func(p int) int {
		switch {
		case p < placeholderEdge:
			return 0
		case p >= cell-placeholderEdge:
			return 2
		}
		return 1
	}
	zx, zy := zone(px), zone(py)
	need := uint8(0)
	for _, n := range neighbourGrid {
		if n.gx == zx && n.gy == zy {
			need = n.bit
		}
	}
	if need == 0 {
		return true // centre zone
	}
	switch need {
	case world.MaskNE:
		need |= world.MaskN | world.MaskE
	case world.MaskSE:
		need |= world.MaskS | world.MaskE
	case world.MaskSW:
		need |= world.MaskS | world.MaskW
	case world.MaskNW:
		need |= world.MaskN | world.MaskW
	}
	return m&need == need
}

// Placeholder draws a usable flat-colour blob-47 sheet: the terrain colour
// over transparency, with a darker 1 px rim where it meets the
// transparent part, so the game and editor show real-looking transitions
// before any art exists.
func Placeholder(c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, sheetCols*cell, sheetRows*cell))
	rim := darken(c)
	for i, m := range world.BlobStates() {
		ox, oy := cellOrigin(i)
		for py := 0; py < cell; py++ {
			for px := 0; px < cell; px++ {
				if !covered(m, px, py) {
					continue
				}
				col := c
				if touchesGap(m, px, py) {
					col = rim
				}
				img.SetNRGBA(ox+px, oy+py, col)
			}
		}
	}
	return img
}

// touchesGap reports whether a covered pixel has an uncovered pixel of the
// same cell next to it.
func touchesGap(m uint8, px, py int) bool {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		x, y := px+d[0], py+d[1]
		if x >= 0 && y >= 0 && x < cell && y < cell && !covered(m, x, y) {
			return true
		}
	}
	return false
}

// FlatSheet draws a sheet of n cells (n wide, one tall) where cell fillCell
// is solid colour and the rest are transparent: placeholder art for
// terrains with edges = false, such as black.
func FlatSheet(n, fillCell int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n*cell, cell))
	fill(img, fillCell*cell, 0, cell, cell, c)
	return img
}

func darken(c color.NRGBA) color.NRGBA {
	return color.NRGBA{c.R * 3 / 5, c.G * 3 / 5, c.B * 3 / 5, c.A}
}

func fill(img *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			img.SetNRGBA(px, py, c)
		}
	}
}

// Scale enlarges an image by an integer factor, nearest-neighbour.
func Scale(src *image.NRGBA, k int) *image.NRGBA {
	if k <= 1 {
		return src
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dst.SetNRGBA(x, y, src.NRGBAAt(b.Min.X+x/k, b.Min.Y+y/k))
		}
	}
	return dst
}

// ParseColour reads "rrggbb" or "#rrggbb" (opaque).
func ParseColour(s string) (color.NRGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.NRGBA{}, fmt.Errorf("colour %q: want rrggbb", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("colour %q: want rrggbb", s)
	}
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xFF}, nil
}
