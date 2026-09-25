package editor

import (
	"fmt"
	"image"
	"image/draw"
)

// SpriteSheetTemplate is a sprite sheet imported with a fixed grid: one
// cell size and one pivot for every cell in the sheet. Different sheets
// used for the same Part can have entirely different CellW/CellH/pivot —
// that's how size variety (a giant claw vs. a tiny hand) is achieved. The
// one contract across sheets meant to be interchangeable for the same
// Part is that a given (Row, Col) means the same conceptual pose in all
// of them; that's authorial convention, not something this type enforces.
type SpriteSheetTemplate struct {
	Name           string
	FilePath       string
	Image          image.Image
	CellW, CellH   int
	PivotX, PivotY float32 // in cell-local pixel space

	// cells caches every cell, cropped once at construction. Beyond
	// avoiding a re-crop on each repaint, this is where cells get
	// normalized to a (0,0) origin — see CellImage.
	cells [][]image.Image
}

// NewSpriteSheetTemplate creates a template from an already-loaded image
// and pre-crops its cells.
func NewSpriteSheetTemplate(name, filePath string, img image.Image, cellW, cellH int, pivotX, pivotY float32) *SpriteSheetTemplate {
	s := &SpriteSheetTemplate{
		Name:     name,
		FilePath: filePath,
		Image:    img,
		CellW:    cellW,
		CellH:    cellH,
		PivotX:   pivotX,
		PivotY:   pivotY,
	}
	s.buildCells()
	return s
}

func (s *SpriteSheetTemplate) buildCells() {
	rows, cols := s.Rows(), s.Cols()
	if rows <= 0 || cols <= 0 {
		s.cells = nil
		return
	}
	s.cells = make([][]image.Image, rows)
	for row := 0; row < rows; row++ {
		s.cells[row] = make([]image.Image, cols)
		for col := 0; col < cols; col++ {
			s.cells[row][col] = s.cropCell(row, col)
		}
	}
}

// cropCell copies one cell into a fresh image whose bounds start at (0,0).
// Normalizing the origin is not cosmetic: image.SubImage returns a view
// whose Bounds().Min is its offset in the parent sheet, and Fyne's canvas
// painter draws an image from (0,0) outward, so a sub-image at Min=(160,0)
// painted nothing but transparency. That's why a 4x3 sheet rendered only
// its top-left cell while all twelve cell outlines drew correctly.
func (s *SpriteSheetTemplate) cropCell(row, col int) image.Image {
	rect := s.CellRect(row, col)
	if rect == nil {
		return nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, s.CellW, s.CellH))
	draw.Draw(dst, dst.Bounds(), s.Image, rect.Min, draw.Src)
	return dst
}

// Cols and Rows are derived from the image size, not stored — the grid is
// implied by CellW/CellH against however large the sheet image is.
func (s *SpriteSheetTemplate) Cols() int {
	if s.Image == nil || s.CellW <= 0 {
		return 0
	}
	return s.Image.Bounds().Dx() / s.CellW
}

func (s *SpriteSheetTemplate) Rows() int {
	if s.Image == nil || s.CellH <= 0 {
		return 0
	}
	return s.Image.Bounds().Dy() / s.CellH
}

// CellRect returns the pixel rectangle for a given (row, col), or nil if
// out of bounds.
func (s *SpriteSheetTemplate) CellRect(row, col int) *image.Rectangle {
	if row < 0 || col < 0 || row >= s.Rows() || col >= s.Cols() {
		return nil
	}
	origin := s.Image.Bounds().Min
	x0 := origin.X + col*s.CellW
	y0 := origin.Y + row*s.CellH
	r := image.Rect(x0, y0, x0+s.CellW, y0+s.CellH)
	return &r
}

// CellImage returns the image for a given (row, col), always with bounds
// starting at (0,0) so it can be handed straight to a renderer.
func (s *SpriteSheetTemplate) CellImage(row, col int) (image.Image, error) {
	if row < 0 || col < 0 || row >= s.Rows() || col >= s.Cols() {
		return nil, fmt.Errorf("cell (%d,%d) out of range (sheet is %dx%d cells)", row, col, s.Cols(), s.Rows())
	}
	// Served from the cache when present; cropped on demand otherwise, so
	// a template built as a bare struct literal still works.
	if row < len(s.cells) && col < len(s.cells[row]) && s.cells[row][col] != nil {
		return s.cells[row][col], nil
	}
	return s.cropCell(row, col), nil
}
