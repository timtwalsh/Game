package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestSuggestCellSizeDividesTheImage(t *testing.T) {
	cases := []struct{ w, h, cw, ch int }{
		{128, 96, 32, 32}, // 32 fits both: the usual case
		{48, 96, 16, 16},  // 32 doesn't divide 48; 16 does
		{192, 144, 16, 16},
		{144, 96, 16, 16}, // 32 doesn't divide 144
		{50, 30, 50, 30},  // nothing common fits: one cell
	}
	for _, c := range cases {
		cw, ch := SuggestCellSize(c.w, c.h)
		if cw <= 0 || c.w%cw != 0 || c.h%ch != 0 {
			t.Errorf("SuggestCellSize(%d, %d) = %dx%d, doesn't divide the image", c.w, c.h, cw, ch)
		}
	}
	if cw, ch := SuggestCellSize(128, 96); cw != 32 || ch != 32 {
		t.Errorf("128x96: %dx%d, want 32x32", cw, ch)
	}
	if cw, ch := SuggestCellSize(50, 30); cw != 50 || ch != 30 {
		t.Errorf("50x30: %dx%d, want the whole image", cw, ch)
	}
}

func TestDefaultPivotIsBottomCentre(t *testing.T) {
	if x, y := DefaultPivot(32, 48); x != 16 || y != 48 {
		t.Errorf("DefaultPivot(32, 48) = %v,%v, want 16,48", x, y)
	}
}

func TestSheetGridSizeDropsPartialCells(t *testing.T) {
	if c, r := SheetGridSize(100, 70, 32, 32); c != 3 || r != 2 {
		t.Errorf("100x70 in 32px cells: %dx%d, want 3x2", c, r)
	}
	if c, r := SheetGridSize(100, 70, 0, 32); c != 0 || r != 0 {
		t.Errorf("zero cell width: %dx%d, want 0x0", c, r)
	}
}

// Clicking a cell in the preview places the pivot at that point of the
// cell - whichever cell, since all share one pivot.
func TestSheetSetupPreviewClickPlacesThePivot(t *testing.T) {
	test.NewTempApp(t)
	p := NewSheetSetupPreview(image.NewRGBA(image.Rect(0, 0, 64, 32)))
	p.SetGrid(32, 32, 16, 32)
	w := test.NewWindow(p)
	t.Cleanup(w.Close)

	var gotX, gotY float32
	p.OnPivotPicked = func(x, y float32) { gotX, gotY = x, y }
	s := p.scale()
	// Point (40, 30) of the sheet is (8, 30) inside the second cell.
	test.TapAt(p, fyne.NewPos(40*s, 30*s))
	if gotX != 8 || gotY != 30 {
		t.Errorf("pivot picked at %v,%v, want 8,30", gotX, gotY)
	}
}
