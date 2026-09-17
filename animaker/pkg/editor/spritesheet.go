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
}

// NewSpriteSheetTemplate creates a template from an already-loaded image.
func NewSpriteSheetTemplate(name, filePath string, img image.Image, cellW, cellH int, pivotX, pivotY float32) *SpriteSheetTemplate {
	return &SpriteSheetTemplate{
		Name:     name,
		FilePath: filePath,
		Image:    img,
		CellW:    cellW,
		CellH:    cellH,
		PivotX:   pivotX,
		PivotY:   pivotY,
	}
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

// CellImage extracts the sub-image for a given (row, col).
func (s *SpriteSheetTemplate) CellImage(row, col int) (image.Image, error) {
	rect := s.CellRect(row, col)
	if rect == nil {
		return nil, fmt.Errorf("cell (%d,%d) out of range (sheet is %dx%d cells)", row, col, s.Cols(), s.Rows())
	}
	if sub, ok := s.Image.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(*rect), nil
	}
	dst := image.NewRGBA(*rect)
	draw.Draw(dst, *rect, s.Image, rect.Min, draw.Src)
	return dst, nil
}
