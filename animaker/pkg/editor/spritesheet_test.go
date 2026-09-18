package editor

import (
	"image"
	"image/color"
	"testing"
)

// buildTestSheetImage makes a cols x rows grid of cellW x cellH cells, each
// filled with a colour unique to its (row, col) so a mis-sliced cell is
// detectable rather than just "some pixels".
func buildTestSheetImage(cols, rows, cellW, cellH int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			c := color.RGBA{R: uint8(10 + row*40), G: uint8(10 + col*40), B: 200, A: 255}
			for y := row * cellH; y < (row+1)*cellH; y++ {
				for x := col * cellW; x < (col+1)*cellW; x++ {
					img.Set(x, y, c)
				}
			}
		}
	}
	return img
}

// A 4x3 sheet must slice into 12 distinct cells. Reported as "I imported a
// 4x3 sheet and only saw 1 sprite" — which turned out to be a display-side
// scaling problem, not slicing, but the slicing had no coverage to rule it
// out, so this pins it down.
func TestSpriteSheetSlicesEveryCell(t *testing.T) {
	const cols, rows, cellW, cellH = 4, 3, 32, 48
	sheet := NewSpriteSheetTemplate("test", "test.png",
		buildTestSheetImage(cols, rows, cellW, cellH), cellW, cellH, 16, 24)

	if sheet.Cols() != cols || sheet.Rows() != rows {
		t.Fatalf("got %dx%d cells, want %dx%d", sheet.Cols(), sheet.Rows(), cols, rows)
	}

	seen := map[color.RGBA]string{}
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cell, err := sheet.CellImage(row, col)
			if err != nil {
				t.Fatalf("CellImage(%d,%d): %v", row, col, err)
			}

			b := cell.Bounds()
			if b.Dx() != cellW || b.Dy() != cellH {
				t.Errorf("cell (%d,%d) is %dx%d, want %dx%d", row, col, b.Dx(), b.Dy(), cellW, cellH)
			}

			// Sample the cell's own top-left, which for a SubImage is at
			// Bounds().Min, not (0,0) — getting that wrong would silently
			// return the same pixel for every cell.
			got := color.RGBAModel.Convert(cell.At(b.Min.X, b.Min.Y)).(color.RGBA)
			want := color.RGBA{R: uint8(10 + row*40), G: uint8(10 + col*40), B: 200, A: 255}
			if got != want {
				t.Errorf("cell (%d,%d) top-left = %v, want %v", row, col, got, want)
			}
			if prev, dup := seen[got]; dup {
				t.Errorf("cell (%d,%d) has the same content as %s", row, col, prev)
			}
			seen[got] = "cell above"
		}
	}

	if len(seen) != cols*rows {
		t.Errorf("got %d distinct cells, want %d", len(seen), cols*rows)
	}
}

func TestSpriteSheetCellOutOfRange(t *testing.T) {
	sheet := NewSpriteSheetTemplate("test", "test.png",
		buildTestSheetImage(2, 2, 16, 16), 16, 16, 0, 0)

	for _, c := range []struct{ row, col int }{{-1, 0}, {0, -1}, {2, 0}, {0, 2}} {
		if _, err := sheet.CellImage(c.row, c.col); err == nil {
			t.Errorf("CellImage(%d,%d) succeeded, want out-of-range error", c.row, c.col)
		}
	}
}

// A sheet whose pixel size isn't an exact multiple of the cell size keeps
// only the whole cells; a partial trailing cell would slice out of bounds.
func TestSpriteSheetIgnoresPartialTrailingCells(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 70, 30)) // 2.18 x 1.875 cells of 32x16
	sheet := NewSpriteSheetTemplate("test", "test.png", img, 32, 16, 0, 0)

	if sheet.Cols() != 2 || sheet.Rows() != 1 {
		t.Errorf("got %dx%d cells, want 2x1", sheet.Cols(), sheet.Rows())
	}
}
