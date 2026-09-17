package ui

import (
	"animaker/pkg/editor"

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
	scale float32 // display scale for cells in this grid (independent of canvas zoom)

	dragRow, dragCol int
	dragging         bool
	lastAbsPos       fyne.Position

	// OnTileDropped fires on DragEnd with the cell that was picked up and
	// the absolute (window-relative) screen position the drag ended at.
	// The caller (app.go) is responsible for checking whether that
	// position lands inside the canvas and converting it to canvas-local
	// coordinates - this widget has no knowledge of the canvas.
	OnTileDropped func(row, col int, absPos fyne.Position)
}

func NewSheetGridWidget() *SheetGridWidget {
	g := &SheetGridWidget{scale: 2.0}
	g.ExtendBaseWidget(g)
	return g
}

// SetSheet swaps which sheet is displayed. A nil sheet renders empty.
func (g *SheetGridWidget) SetSheet(sheet *editor.SpriteSheetTemplate) {
	g.sheet = sheet
	g.Refresh()
}

func (g *SheetGridWidget) CreateRenderer() fyne.WidgetRenderer {
	return &sheetGridRenderer{widget: g}
}

func (g *SheetGridWidget) MinSize() fyne.Size {
	if g.sheet == nil {
		return fyne.NewSize(150, 100)
	}
	w := float32(g.sheet.CellW) * g.scale * float32(g.sheet.Cols())
	h := float32(g.sheet.CellH) * g.scale * float32(g.sheet.Rows())
	return fyne.NewSize(w, h)
}

// Dragged implements fyne.Draggable. The cell under the drag's start point
// is captured once, then held for the duration of the gesture. Fyne's
// DragEnd takes no event, so the last-seen absolute position is tracked
// here on every Dragged call and used when the drag finishes.
func (g *SheetGridWidget) Dragged(e *fyne.DragEvent) {
	if !g.dragging {
		g.dragging = true
		g.dragRow, g.dragCol = g.cellAt(e.Position)
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

var _ fyne.Draggable = (*SheetGridWidget)(nil)

func (g *SheetGridWidget) cellAt(pos fyne.Position) (row, col int) {
	if g.sheet == nil || g.sheet.CellW <= 0 || g.sheet.CellH <= 0 {
		return 0, 0
	}
	col = int(pos.X / (float32(g.sheet.CellW) * g.scale))
	row = int(pos.Y / (float32(g.sheet.CellH) * g.scale))
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

func (r *sheetGridRenderer) Layout(size fyne.Size) {}

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
		return []fyne.CanvasObject{canvas.NewText("No sheet selected", ColorOriginCrosshair)}
	}

	var objs []fyne.CanvasObject
	cw := float32(g.sheet.CellW) * g.scale
	ch := float32(g.sheet.CellH) * g.scale

	for row := 0; row < g.sheet.Rows(); row++ {
		for col := 0; col < g.sheet.Cols(); col++ {
			img, err := g.sheet.CellImage(row, col)
			if err != nil {
				continue
			}
			ci := canvas.NewImageFromImage(img)
			ci.ScaleMode = canvas.ImageScalePixels
			ci.FillMode = canvas.ImageFillOriginal
			ci.Resize(fyne.NewSize(cw, ch))
			ci.Move(fyne.NewPos(float32(col)*cw, float32(row)*ch))
			objs = append(objs, ci)
		}
	}
	return objs
}
