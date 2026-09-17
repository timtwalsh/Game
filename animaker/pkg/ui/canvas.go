package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// CanvasWidget renders every Part of the active direction at the current
// playhead time, Z-sorted, within the track's defined 0,0-to-(W,H) working
// area (drawn as a bordered rect). Origin is the top-left of that area, not
// the widget's center — this lets the widget's MinSize equal the working
// area at the current zoom, so wrapping it in a Scroll container gives
// scrollbars exactly when the zoomed content overflows the viewport.
//
// Rotation is stored and saved but not visually applied here — Fyne has no
// simple rotated-image primitive, and the real consumer of rotation is the
// game's own (raylib) renderer, not this preview. That's a known,
// deliberate gap for this pass.
type CanvasWidget struct {
	widget.BaseWidget

	project  *editor.Project
	zoom     float32
	showGrid bool

	draggingPartIdx int // -1 = not dragging

	// OnPartTapped fires with the part index under the click, or -1 if the
	// click landed on empty space (a deselect).
	OnPartTapped func(partIdx int)
	// OnPartDragStart/Dragged/DragEnd fire while an existing part is being
	// repositioned directly on the canvas. Dragged's x/y are already
	// converted to the animation's own coordinate space.
	OnPartDragStart func(partIdx int)
	OnPartDragged   func(partIdx int, animX, animY float32)
	OnPartDragEnd   func()
}

func NewCanvasWidget(project *editor.Project) *CanvasWidget {
	cw := &CanvasWidget{project: project, zoom: 4.0, showGrid: true, draggingPartIdx: -1}
	cw.ExtendBaseWidget(cw)
	return cw
}

func (cw *CanvasWidget) SetProject(project *editor.Project) {
	cw.project = project
	cw.Refresh()
}

func (cw *CanvasWidget) SetZoom(z float32) {
	cw.zoom = z
	cw.Refresh()
}

func (cw *CanvasWidget) ToggleGrid() {
	cw.showGrid = !cw.showGrid
	cw.Refresh()
}

// LocalToAnimXY converts a position local to this widget into the
// animation's own X/Y coordinate space - the inverse of how a resolved
// transform is drawn (screen = origin(0,0) + animXY*zoom).
func (cw *CanvasWidget) LocalToAnimXY(local fyne.Position) (x, y float32) {
	if cw.zoom == 0 {
		return 0, 0
	}
	return local.X / cw.zoom, local.Y / cw.zoom
}

func (cw *CanvasWidget) CreateRenderer() fyne.WidgetRenderer {
	return &canvasRenderer{widget: cw}
}

func (cw *CanvasWidget) MinSize() fyne.Size {
	if cw.project == nil || cw.project.CurrentTrack == nil {
		return fyne.NewSize(300, 300)
	}
	w := float32(cw.project.CurrentTrack.CanvasWidth) * cw.zoom
	h := float32(cw.project.CurrentTrack.CanvasHeight) * cw.zoom
	if w < 100 {
		w = 100
	}
	if h < 100 {
		h = 100
	}
	return fyne.NewSize(w, h)
}

// resolvedDraw is one part's fully-resolved placement for a single frame,
// plus the screen rect it occupies (for hit-testing) once computed.
type resolvedDraw struct {
	partIdx int
	part    *editor.Part
	tr      editor.ResolvedTransform
	sheet   *editor.SpriteSheetTemplate
	rect    fyne.Position // top-left
	size    fyne.Size
}

// resolvedDraws computes every part's resolved transform and screen rect
// at the current playhead, Z-sorted back-to-front. Shared between the
// renderer (drawing) and this widget's own hit-testing, so both always
// agree on where a part actually is.
func (cw *CanvasWidget) resolvedDraws() []resolvedDraw {
	dir := cw.project.ActiveDirection()
	if dir == nil {
		return nil
	}
	elapsed := cw.project.Playback.ElapsedMs
	var draws []resolvedDraw
	for i, part := range dir.Parts {
		tr := part.ValueAt(elapsed)
		var sheet *editor.SpriteSheetTemplate
		if part.Kind == editor.PartKindSheet {
			sheet = cw.project.ResolveActiveSheet(part)
		}
		d := resolvedDraw{partIdx: i, part: part, tr: tr, sheet: sheet}
		d.rect, d.size = cw.screenRectFor(d)
		draws = append(draws, d)
	}
	sort.SliceStable(draws, func(i, j int) bool { return draws[i].tr.Z < draws[j].tr.Z })
	return draws
}

func (cw *CanvasWidget) screenRectFor(d resolvedDraw) (pos fyne.Position, size fyne.Size) {
	zoom := cw.zoom
	if d.part.Kind == editor.PartKindNestedAni {
		w, h := float32(24)*zoom/4, float32(24)*zoom/4
		return fyne.NewPos(d.tr.X*zoom-w/2, d.tr.Y*zoom-h/2), fyne.NewSize(w, h)
	}
	if d.sheet == nil {
		return fyne.NewPos(d.tr.X*zoom, d.tr.Y*zoom), fyne.NewSize(0, 0)
	}
	w := float32(d.sheet.CellW) * zoom
	h := float32(d.sheet.CellH) * zoom
	x := d.tr.X*zoom - d.sheet.PivotX*zoom
	y := d.tr.Y*zoom - d.sheet.PivotY*zoom
	return fyne.NewPos(x, y), fyne.NewSize(w, h)
}

func posInRect(p fyne.Position, rectPos fyne.Position, rectSize fyne.Size) bool {
	return p.X >= rectPos.X && p.X <= rectPos.X+rectSize.Width &&
		p.Y >= rectPos.Y && p.Y <= rectPos.Y+rectSize.Height
}

// hitTest returns the topmost (highest Z) part under pos, or -1.
func (cw *CanvasWidget) hitTest(pos fyne.Position) int {
	draws := cw.resolvedDraws()
	for i := len(draws) - 1; i >= 0; i-- {
		if posInRect(pos, draws[i].rect, draws[i].size) {
			return draws[i].partIdx
		}
	}
	return -1
}

// -- Gestures --

var _ fyne.Tappable = (*CanvasWidget)(nil)
var _ fyne.Draggable = (*CanvasWidget)(nil)

func (cw *CanvasWidget) Tapped(e *fyne.PointEvent) {
	if cw.OnPartTapped != nil {
		cw.OnPartTapped(cw.hitTest(e.Position))
	}
}

func (cw *CanvasWidget) Dragged(e *fyne.DragEvent) {
	if cw.draggingPartIdx < 0 {
		cw.draggingPartIdx = cw.hitTest(e.Position)
		if cw.draggingPartIdx < 0 {
			return
		}
		if cw.OnPartDragStart != nil {
			cw.OnPartDragStart(cw.draggingPartIdx)
		}
	}
	if cw.draggingPartIdx < 0 {
		return
	}
	x, y := cw.LocalToAnimXY(e.Position)
	if cw.OnPartDragged != nil {
		cw.OnPartDragged(cw.draggingPartIdx, x, y)
	}
}

func (cw *CanvasWidget) DragEnd() {
	if cw.draggingPartIdx >= 0 && cw.OnPartDragEnd != nil {
		cw.OnPartDragEnd()
	}
	cw.draggingPartIdx = -1
}

// -- Renderer --

type canvasRenderer struct {
	widget  *CanvasWidget
	objects []fyne.CanvasObject
}

func (r *canvasRenderer) Layout(size fyne.Size) {}

func (r *canvasRenderer) MinSize() fyne.Size { return r.widget.MinSize() }

func (r *canvasRenderer) Refresh() {
	r.objects = r.buildObjects()
	canvas.Refresh(r.widget)
}

func (r *canvasRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = r.buildObjects()
	}
	return r.objects
}

func (r *canvasRenderer) Destroy() {}

func (r *canvasRenderer) buildObjects() []fyne.CanvasObject {
	cw := r.widget
	size := cw.MinSize()
	objs := []fyne.CanvasObject{}

	bg := canvas.NewRectangle(ColorCanvasBackground)
	bg.Resize(size)
	objs = append(objs, bg)

	// Working-area bounds: 0,0 to (CanvasWidth, CanvasHeight).
	bounds := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
	bounds.StrokeColor = ColorOriginCrosshair
	bounds.StrokeWidth = 1
	bounds.Resize(size)
	bounds.Move(fyne.NewPos(0, 0))
	objs = append(objs, bounds)

	if cw.showGrid {
		objs = append(objs, r.buildGrid(size)...)
	}

	// Origin marker at (0,0).
	crossLen := float32(10)
	hLine := canvas.NewLine(ColorOriginCrosshair)
	hLine.Position1 = fyne.NewPos(0, 0)
	hLine.Position2 = fyne.NewPos(crossLen, 0)
	objs = append(objs, hLine)
	vLine := canvas.NewLine(ColorOriginCrosshair)
	vLine.Position1 = fyne.NewPos(0, 0)
	vLine.Position2 = fyne.NewPos(0, crossLen)
	objs = append(objs, vLine)

	if cw.project.ActiveDirection() == nil {
		return objs
	}

	draws := cw.resolvedDraws()
	sel := cw.project.Selection
	for _, d := range draws {
		selected := sel != nil && sel.PartIndex == d.partIdx
		objs = append(objs, r.drawPart(d, selected)...)
	}

	return objs
}

func (r *canvasRenderer) drawPart(d resolvedDraw, selected bool) []fyne.CanvasObject {
	if d.part.Kind == editor.PartKindNestedAni {
		box := canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 0})
		box.StrokeColor = ColorOriginCrosshair
		box.StrokeWidth = 1
		box.Resize(d.size)
		box.Move(d.rect)
		label := canvas.NewText(fmt.Sprintf("%s -> %s", d.part.Name, d.part.NestedAniPath), ColorOriginCrosshair)
		label.TextSize = 9
		label.Move(fyne.NewPos(d.rect.X, d.rect.Y+d.size.Height+2))
		objs := []fyne.CanvasObject{box, label}
		if selected {
			objs = append(objs, selectionOutline(d.rect, d.size))
		}
		return objs
	}

	if d.sheet == nil || d.sheet.Image == nil {
		return nil
	}
	cellImg, err := d.sheet.CellImage(d.tr.Row, d.tr.Col)
	if err != nil {
		return nil
	}

	img := canvas.NewImageFromImage(cellImg)
	img.ScaleMode = canvas.ImageScalePixels
	img.FillMode = canvas.ImageFillOriginal
	img.Resize(d.size)
	img.Move(d.rect)
	objs := []fyne.CanvasObject{img}
	if selected {
		objs = append(objs, selectionOutline(d.rect, d.size))
	}
	return objs
}

func selectionOutline(pos fyne.Position, size fyne.Size) fyne.CanvasObject {
	r := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
	r.StrokeColor = color.RGBA{R: 255, G: 220, B: 60, A: 255}
	r.StrokeWidth = 2
	r.Resize(size)
	r.Move(pos)
	return r
}

func (r *canvasRenderer) buildGrid(size fyne.Size) []fyne.CanvasObject {
	zoom := r.widget.zoom
	gridSpacing := float32(16) * zoom
	if gridSpacing < 2 {
		return nil
	}

	var objs []fyne.CanvasObject
	for x := float32(0); x <= size.Width; x += gridSpacing {
		line := canvas.NewLine(ColorGrid)
		line.Position1 = fyne.NewPos(x, 0)
		line.Position2 = fyne.NewPos(x, size.Height)
		objs = append(objs, line)
	}
	for y := float32(0); y <= size.Height; y += gridSpacing {
		line := canvas.NewLine(ColorGrid)
		line.Position1 = fyne.NewPos(0, y)
		line.Position2 = fyne.NewPos(size.Width, y)
		objs = append(objs, line)
	}
	return objs
}
