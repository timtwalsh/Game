package ui

import (
	"animaker/pkg/editor"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// SheetGridWidget renders every cell of a SpriteSheetTemplate in its actual
// grid layout and lets the artist drag a cell out onto another widget (the
// canvas) to place it. There's no floating "ghost" preview during the drag
// (a deliberate v1 simplification) — the cell is picked up at drag-start
// and placed at wherever the drag ends.
type SheetGridWidget struct {
	widget.BaseWidget

	sheet *editor.SpriteSheetTemplate

	// viewWidth is the width the layout last gave this widget. The grid
	// scales its cells to fit that width (see displayScale) rather than
	// drawing at a fixed zoom: at a fixed 2x, a sheet of large cells
	// overflowed the narrow palette column and only the first cell or two
	// were ever visible, which read as "the import only produced one
	// sprite". Fitting to width means every column is always on screen and
	// the grid scrolls vertically if it's tall.
	viewWidth float32

	dragRow, dragCol int
	dragging         bool
	lastAbsPos       fyne.Position

	// OnDragStart fires once when a tile is picked up, before any
	// OnTileDropped. app.go uses it to freeze the canvas view for the
	// duration of the gesture.
	OnDragStart func()

	// OnTileTapped fires on a plain click (no drag). Clicking re-assigns
	// the selected keyframe's cell, where dragging creates a whole new
	// part — two different jobs, so two different gestures.
	OnTileTapped func(row, col int)

	// OnTileDropped fires on DragEnd with the cell that was picked up and
	// the absolute (window-relative) screen position the drag ended at.
	// The caller (app.go) is responsible for checking whether that
	// position lands inside the canvas and converting it to canvas-local
	// coordinates - this widget has no knowledge of the canvas.
	OnTileDropped func(row, col int, absPos fyne.Position)
}

// sheetGridMinScale/MaxScale bound the fit: a huge sheet still stays
// legible rather than shrinking to nothing, and a tiny 8x8 sheet doesn't
// blow up to fill the whole column.
const (
	sheetGridMinScale = 0.25
	sheetGridMaxScale = 4.0
)

func NewSheetGridWidget() *SheetGridWidget {
	g := &SheetGridWidget{}
	g.ExtendBaseWidget(g)
	return g
}

// SetSheet swaps which sheet is displayed. A nil sheet renders empty.
func (g *SheetGridWidget) SetSheet(sheet *editor.SpriteSheetTemplate) {
	g.sheet = sheet
	g.Refresh()
}

// displayScale is how many screen pixels one sheet pixel occupies: enough
// to fit all the sheet's columns across the width the layout gave us.
func (g *SheetGridWidget) displayScale() float32 {
	if g.sheet == nil || g.sheet.Cols() <= 0 || g.sheet.CellW <= 0 {
		return 1
	}
	if g.viewWidth <= 0 {
		return 1 // not laid out yet; 1:1 until Layout tells us the width
	}
	scale := g.viewWidth / (float32(g.sheet.CellW) * float32(g.sheet.Cols()))
	if scale < sheetGridMinScale {
		return sheetGridMinScale
	}
	if scale > sheetGridMaxScale {
		return sheetGridMaxScale
	}
	return scale
}

func (g *SheetGridWidget) CreateRenderer() fyne.WidgetRenderer {
	return &sheetGridRenderer{widget: g}
}

// MinSize deliberately reports a small width: the grid fits itself to
// whatever width it's given, so demanding its natural width here would
// force a horizontal scrollbar and defeat the fit. Height is the real
// height at the fitted scale, so tall sheets scroll vertically.
func (g *SheetGridWidget) MinSize() fyne.Size {
	if g.sheet == nil {
		return fyne.NewSize(80, 60)
	}
	h := float32(g.sheet.CellH) * g.displayScale() * float32(g.sheet.Rows())
	return fyne.NewSize(80, h)
}

// Dragged implements fyne.Draggable. The cell under the drag's start point
// is captured once, then held for the duration of the gesture. Fyne's
// DragEnd takes no event, so the last-seen absolute position is tracked
// here on every Dragged call and used when the drag finishes.
func (g *SheetGridWidget) Dragged(e *fyne.DragEvent) {
	if !g.dragging {
		g.dragging = true
		g.dragRow, g.dragCol = g.cellAt(e.Position)
		if g.OnDragStart != nil {
			g.OnDragStart()
		}
	}
	g.lastAbsPos = e.AbsolutePosition
}

// DragEnd implements fyne.Draggable.
func (g *SheetGridWidget) DragEnd() {
	g.dragging = false
	if g.OnTileDropped != nil {
		g.OnTileDropped(g.dragRow, g.dragCol, g.lastAbsPos)
	}
}

// Tapped implements fyne.Tappable. Fyne delivers a tap only when the
// pointer didn't drag, so this can't fire for a drag gesture.
func (g *SheetGridWidget) Tapped(e *fyne.PointEvent) {
	if g.sheet == nil || g.OnTileTapped == nil {
		return
	}
	row, col := g.cellAt(e.Position)
	g.OnTileTapped(row, col)
}

var _ fyne.Draggable = (*SheetGridWidget)(nil)
var _ fyne.Tappable = (*SheetGridWidget)(nil)

func (g *SheetGridWidget) cellAt(pos fyne.Position) (row, col int) {
	if g.sheet == nil || g.sheet.CellW <= 0 || g.sheet.CellH <= 0 {
		return 0, 0
	}
	scale := g.displayScale()
	col = int(pos.X / (float32(g.sheet.CellW) * scale))
	row = int(pos.Y / (float32(g.sheet.CellH) * scale))
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	if col >= g.sheet.Cols() {
		col = g.sheet.Cols() - 1
	}
	if row >= g.sheet.Rows() {
		row = g.sheet.Rows() - 1
	}
	return row, col
}

type sheetGridRenderer struct {
	widget  *SheetGridWidget
	objects []fyne.CanvasObject
}

// Layout records the width we've been given and re-lays the cells at the
// scale that fits it. It rebuilds r.objects directly rather than calling
// Refresh, which would re-enter layout.
func (r *sheetGridRenderer) Layout(size fyne.Size) {
	if size.Width == r.widget.viewWidth && r.objects != nil {
		return
	}
	r.widget.viewWidth = size.Width
	r.objects = r.buildObjects()
}

func (r *sheetGridRenderer) MinSize() fyne.Size { return r.widget.MinSize() }

func (r *sheetGridRenderer) Refresh() {
	r.objects = r.buildObjects()
	canvas.Refresh(r.widget)
}

func (r *sheetGridRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = r.buildObjects()
	}
	return r.objects
}

func (r *sheetGridRenderer) Destroy() {}

func (r *sheetGridRenderer) buildObjects() []fyne.CanvasObject {
	g := r.widget
	if g.sheet == nil || g.sheet.Image == nil {
		return []fyne.CanvasObject{canvas.NewText("No tiles to show", ColorOriginCrosshair)}
	}

	scale := g.displayScale()
	cw := float32(g.sheet.CellW) * scale
	ch := float32(g.sheet.CellH) * scale

	var objs []fyne.CanvasObject
	for row := 0; row < g.sheet.Rows(); row++ {
		for col := 0; col < g.sheet.Cols(); col++ {
			img, err := g.sheet.CellImage(row, col)
			if err != nil {
				continue
			}
			pos := fyne.NewPos(float32(col)*cw, float32(row)*ch)
			ci := canvas.NewImageFromImage(img)
			ci.ScaleMode = canvas.ImageScalePixels
			// FillStretch, not FillOriginal: the cell is drawn at the fitted
			// scale, not at its own pixel size.
			ci.FillMode = canvas.ImageFillStretch
			ci.Resize(fyne.NewSize(cw, ch))
			ci.Move(pos)

			// One outline per cell, so the slicing the artist entered is
			// visible and wrong cell dimensions are obvious at a glance.
			outline := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
			outline.StrokeColor = ColorGrid
			outline.StrokeWidth = 1
			outline.Resize(fyne.NewSize(cw, ch))
			outline.Move(pos)

			objs = append(objs, ci, outline)
		}
	}
	return objs
}
