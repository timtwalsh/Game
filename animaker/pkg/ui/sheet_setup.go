package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// SheetSetupPreview shows a sprite sheet with the grid and pivot the import
// dialog is about to give it, redrawn as the numbers change, so a wrong
// cell size is visible before importing rather than after (it's the
// easiest mistake to make there). Clicking a cell places the pivot at that
// point of the cell - every cell shares one pivot - which reports through
// OnPivotPicked.
type SheetSetupPreview struct {
	widget.BaseWidget

	img            image.Image
	cellW, cellH   int
	pivotX, pivotY float32

	// hover highlights the cell under the mouse.
	hover *hoverBox

	// OnPivotPicked reports a click as a pivot, in cell pixels.
	OnPivotPicked func(x, y float32)
}

// Room the preview takes in the dialog; the sheet is scaled to fit.
const (
	sheetSetupMaxW = 480
	sheetSetupMaxH = 280
	// sheetSetupMaxMarks caps the pivot markers drawn, so a sheet of
	// thousands of tiny cells doesn't become thousands of objects.
	sheetSetupMaxMarks = 256
)

func NewSheetSetupPreview(img image.Image) *SheetSetupPreview {
	p := &SheetSetupPreview{img: img, hover: newHoverBox(true)}
	p.ExtendBaseWidget(p)
	return p
}

// SetGrid updates the grid and pivot drawn. Non-positive cell sizes (a
// half-typed field) draw no grid.
func (p *SheetSetupPreview) SetGrid(cellW, cellH int, pivotX, pivotY float32) {
	p.hover.hide() // the cells under it may have changed size
	p.cellW, p.cellH, p.pivotX, p.pivotY = cellW, cellH, pivotX, pivotY
	p.Refresh()
}

// scale is screen pixels per sheet pixel: the largest that fits the
// preview area, capped at 4x for small sheets.
func (p *SheetSetupPreview) scale() float32 {
	if p.img == nil {
		return 1
	}
	b := p.img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return 1
	}
	s := float32(math.Min(float64(sheetSetupMaxW)/float64(b.Dx()), float64(sheetSetupMaxH)/float64(b.Dy())))
	if s > 4 {
		s = 4
	}
	return s
}

func (p *SheetSetupPreview) MinSize() fyne.Size {
	if p.img == nil {
		return fyne.NewSize(0, 0)
	}
	b, s := p.img.Bounds(), p.scale()
	return fyne.NewSize(float32(b.Dx())*s, float32(b.Dy())*s+18)
}

// Tapped places the pivot at the clicked point of whichever cell it fell
// in, rounded to a whole pixel.
func (p *SheetSetupPreview) Tapped(e *fyne.PointEvent) {
	if p.img == nil || p.cellW <= 0 || p.cellH <= 0 || p.OnPivotPicked == nil {
		return
	}
	s := p.scale()
	sx, sy := e.Position.X/s, e.Position.Y/s
	b := p.img.Bounds()
	if sx < 0 || sy < 0 || sx >= float32(b.Dx()) || sy >= float32(b.Dy()) {
		return
	}
	x := float32(math.Round(float64(sx - float32(int(sx)/p.cellW*p.cellW))))
	y := float32(math.Round(float64(sy - float32(int(sy)/p.cellH*p.cellH))))
	p.OnPivotPicked(x, y)
}

var _ fyne.Tappable = (*SheetSetupPreview)(nil)

func (p *SheetSetupPreview) CreateRenderer() fyne.WidgetRenderer {
	r := &sheetSetupRenderer{p: p}
	r.rebuild()
	return r
}

type sheetSetupRenderer struct {
	p       *SheetSetupPreview
	objects []fyne.CanvasObject
	image   *canvas.Image
}

func (r *sheetSetupRenderer) rebuild() {
	p := r.p
	r.objects = nil
	if p.img == nil {
		return
	}
	b, s := p.img.Bounds(), p.scale()
	w, h := float32(b.Dx())*s, float32(b.Dy())*s
	if r.image == nil {
		r.image = canvas.NewImageFromImage(p.img)
		r.image.ScaleMode = canvas.ImageScalePixels
		r.image.FillMode = canvas.ImageFillStretch
	}
	r.image.Resize(fyne.NewSize(w, h))
	bg := canvas.NewRectangle(ColorCanvasBackground)
	bg.Resize(fyne.NewSize(w, h))
	r.objects = append(r.objects, bg, r.image)

	cols, rows := SheetGridSize(b.Dx(), b.Dy(), p.cellW, p.cellH)
	caption := "Cell size must be above 0"
	if cols > 0 && rows > 0 {
		caption = fmt.Sprintf("%d cols x %d rows - click a cell to place the pivot", cols, rows)
	} else if p.cellW > 0 && p.cellH > 0 {
		caption = "A cell is bigger than the image"
	}
	label := canvas.NewText(caption, ColorSectionHeader)
	label.TextSize = 11
	label.Move(fyne.NewPos(0, h+2))
	r.objects = append(r.objects, label)
	if cols == 0 || rows == 0 {
		return
	}

	gridColor := color.RGBA{R: 80, G: 200, B: 255, A: 200}
	for c := 0; c <= cols; c++ {
		x := float32(c*p.cellW) * s
		l := canvas.NewLine(gridColor)
		l.Position1, l.Position2 = fyne.NewPos(x, 0), fyne.NewPos(x, float32(rows*p.cellH)*s)
		r.objects = append(r.objects, l)
	}
	for rr := 0; rr <= rows; rr++ {
		y := float32(rr*p.cellH) * s
		l := canvas.NewLine(gridColor)
		l.Position1, l.Position2 = fyne.NewPos(0, y), fyne.NewPos(float32(cols*p.cellW)*s, y)
		r.objects = append(r.objects, l)
	}

	pivotColor := color.RGBA{R: 255, G: 220, B: 60, A: 255}
	const arm = 4
	marks := 0
	for rr := 0; rr < rows && marks < sheetSetupMaxMarks; rr++ {
		for c := 0; c < cols && marks < sheetSetupMaxMarks; c++ {
			px := (float32(c*p.cellW) + p.pivotX) * s
			py := (float32(rr*p.cellH) + p.pivotY) * s
			hl := canvas.NewLine(pivotColor)
			hl.Position1, hl.Position2 = fyne.NewPos(px-arm, py), fyne.NewPos(px+arm, py)
			vl := canvas.NewLine(pivotColor)
			vl.Position1, vl.Position2 = fyne.NewPos(px, py-arm), fyne.NewPos(px, py+arm)
			r.objects = append(r.objects, hl, vl)
			marks++
		}
	}
	r.objects = append(r.objects, p.hover.rect)
}

func (r *sheetSetupRenderer) Layout(fyne.Size)            {}
func (r *sheetSetupRenderer) MinSize() fyne.Size          { return r.p.MinSize() }
func (r *sheetSetupRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *sheetSetupRenderer) Destroy()                    {}
func (r *sheetSetupRenderer) Refresh() {
	r.rebuild()
	canvas.Refresh(r.p)
}

// SheetGridSize is how many whole cells of cellW x cellH fit an image of
// imgW x imgH - partial trailing cells are dropped, as when slicing.
func SheetGridSize(imgW, imgH, cellW, cellH int) (cols, rows int) {
	if cellW <= 0 || cellH <= 0 {
		return 0, 0
	}
	return imgW / cellW, imgH / cellH
}

// sheetCellSizeCandidates are common sprite cell sizes, most likely first.
var sheetCellSizeCandidates = []int{32, 16, 48, 64, 24, 96, 128}

// SuggestCellSize guesses a new sheet's cell size: the first common sprite
// size that divides both the image's width and height exactly, else the
// whole image as one cell. Only a starting point - the preview shows
// whether it's right.
func SuggestCellSize(imgW, imgH int) (cellW, cellH int) {
	for _, c := range sheetCellSizeCandidates {
		if imgW >= c && imgH >= c && imgW%c == 0 && imgH%c == 0 {
			return c, c
		}
	}
	return imgW, imgH
}

// DefaultPivot is a new sheet's pivot: bottom-centre of the cell, where a
// top-down character's feet are - the point it should stand on.
func DefaultPivot(cellW, cellH int) (x, y float32) {
	return float32(cellW / 2), float32(cellH)
}

// -- Hover: the cell under the mouse is highlighted, with a crosshair
// cursor, since a click there places the pivot.

var _ desktop.Hoverable = (*SheetSetupPreview)(nil)
var _ desktop.Cursorable = (*SheetSetupPreview)(nil)

func (p *SheetSetupPreview) MouseIn(e *desktop.MouseEvent) { p.MouseMoved(e) }

func (p *SheetSetupPreview) MouseMoved(e *desktop.MouseEvent) {
	if p.img == nil {
		return
	}
	b := p.img.Bounds()
	cols, rows := SheetGridSize(b.Dx(), b.Dy(), p.cellW, p.cellH)
	s := p.scale()
	col, row := int(e.Position.X/s)/max(p.cellW, 1), int(e.Position.Y/s)/max(p.cellH, 1)
	if e.Position.X < 0 || e.Position.Y < 0 || col >= cols || row >= rows {
		p.hover.hide()
		return
	}
	cw, ch := float32(p.cellW)*s, float32(p.cellH)*s
	p.hover.show(fyne.NewPos(float32(col)*cw, float32(row)*ch), fyne.NewSize(cw, ch))
}

func (p *SheetSetupPreview) MouseOut() { p.hover.hide() }

func (p *SheetSetupPreview) Cursor() desktop.Cursor { return desktop.CrosshairCursor }
