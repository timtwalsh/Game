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
// playhead time, Z-sorted, around the animation's origin — drawn as a
// full-span crosshair through (0,0) with a character-sized reference box in
// its bottom-right quadrant, the way GraalShop's GANI editor does it. Parts
// are positioned relative to that origin.
//
// The canvas has no fixed working area and does no centering: its extent is
// derived from what's actually placed (see viewBounds), so art may sit at
// negative coordinates above/left of the origin — a raised sword, a
// trailing cape — and the canvas simply grows to include it. The extent is
// computed over *every* keyframe rather than just the current frame, and
// quantized, so scrubbing never resizes the canvas and a drag only does so
// when it crosses a quantum boundary. A canvas that resized continuously
// under a drag would shift the origin out from under the cursor.
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

const (
	canvasMarginPx   = 24 // anim px of breathing room around the content
	canvasQuantizePx = 32 // view extent rounds outward to this, so small moves don't resize
	nestedBoxPx      = 24 // placeholder box size for a NestedAni part, in anim px
)

// refBox returns the track's character-sized reference box dimensions.
func (cw *CanvasWidget) refBox() (w, h float32) {
	t := cw.project.CurrentTrack
	if t == nil || t.RefBoxWidth <= 0 || t.RefBoxHeight <= 0 {
		return editor.DefaultRefBoxWidth, editor.DefaultRefBoxHeight
	}
	return float32(t.RefBoxWidth), float32(t.RefBoxHeight)
}

// partExtentAnim is a part's drawn size and pivot in animation pixels,
// which is a property of its sheet and so the same for all its keyframes.
func (cw *CanvasWidget) partExtentAnim(part *editor.Part) (w, h, pivotX, pivotY float32) {
	if part.Kind == editor.PartKindNestedAni {
		return nestedBoxPx, nestedBoxPx, nestedBoxPx / 2, nestedBoxPx / 2
	}
	sheet := cw.project.ResolveActiveSheet(part)
	if sheet == nil {
		return 0, 0, 0, 0
	}
	return float32(sheet.CellW), float32(sheet.CellH), sheet.PivotX, sheet.PivotY
}

// viewBounds is the visible region in animation coordinates. It always
// contains the origin and the reference box, plus every keyframe of every
// part (not just the current frame — so scrubbing can't resize the canvas),
// padded and rounded outward to canvasQuantizePx.
func (cw *CanvasWidget) viewBounds() (minX, minY, maxX, maxY float32) {
	refW, refH := cw.refBox()
	minX, minY, maxX, maxY = 0, 0, refW, refH

	if dir := cw.project.ActiveDirection(); dir != nil {
		for _, part := range cw.project.CurrentTrack.Parts {
			w, h, px, py := cw.partExtentAnim(part)
			for _, kf := range dir.KeyframesFor(part.ID) {
				x0, y0 := kf.X-px, kf.Y-py
				minX, minY = minF(minX, x0), minF(minY, y0)
				maxX, maxY = maxF(maxX, x0+w), maxF(maxY, y0+h)
			}
		}
	}

	return floorTo(minX-canvasMarginPx, canvasQuantizePx),
		floorTo(minY-canvasMarginPx, canvasQuantizePx),
		ceilTo(maxX+canvasMarginPx, canvasQuantizePx),
		ceilTo(maxY+canvasMarginPx, canvasQuantizePx)
}

// originScreen is where animation (0,0) sits in widget-local pixels. It's
// offset from the widget's top-left by however much negative space the
// current view includes.
func (cw *CanvasWidget) originScreen() fyne.Position {
	minX, minY, _, _ := cw.viewBounds()
	return fyne.NewPos(-minX*cw.zoom, -minY*cw.zoom)
}

// LocalToAnimXY converts a position local to this widget into the
// animation's own X/Y coordinate space — the inverse of how a resolved
// transform is drawn (screen = originScreen + animXY*zoom).
func (cw *CanvasWidget) LocalToAnimXY(local fyne.Position) (x, y float32) {
	if cw.zoom == 0 {
		return 0, 0
	}
	origin := cw.originScreen()
	return (local.X - origin.X) / cw.zoom, (local.Y - origin.Y) / cw.zoom
}

func (cw *CanvasWidget) CreateRenderer() fyne.WidgetRenderer {
	return &canvasRenderer{widget: cw}
}

func (cw *CanvasWidget) MinSize() fyne.Size {
	if cw.project == nil || cw.project.CurrentTrack == nil {
		return fyne.NewSize(300, 300)
	}
	minX, minY, maxX, maxY := cw.viewBounds()
	return fyne.NewSize(maxF((maxX-minX)*cw.zoom, 100), maxF((maxY-minY)*cw.zoom, 100))
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// floorTo/ceilTo round outward to a multiple of q, including for negative
// values (where Go's integer truncation rounds toward zero, i.e. the wrong
// way for a lower bound).
func floorTo(v, q float32) float32 {
	n := float32(int(v / q))
	if v < 0 && n*q != v {
		n--
	}
	return n * q
}

func ceilTo(v, q float32) float32 {
	n := float32(int(v / q))
	if v > 0 && n*q != v {
		n++
	}
	return n * q
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
	for i, part := range cw.project.CurrentTrack.Parts {
		// A part with no keyframes in this facing isn't posed here, so it
		// isn't drawn — it still exists on the rig and in every other
		// direction's part list.
		if len(dir.KeyframesFor(part.ID)) == 0 {
			continue
		}
		var sheet *editor.SpriteSheetTemplate
		if part.Kind == editor.PartKindSheet {
			sheet = cw.project.ResolveActiveSheet(part)
		}
		d := resolvedDraw{partIdx: i, part: part, tr: dir.ValueAt(part.ID, elapsed), sheet: sheet}
		d.rect, d.size = cw.screenRectFor(d)
		draws = append(draws, d)
	}
	sort.SliceStable(draws, func(i, j int) bool { return draws[i].tr.Z < draws[j].tr.Z })
	return draws
}

func (cw *CanvasWidget) screenRectFor(d resolvedDraw) (pos fyne.Position, size fyne.Size) {
	zoom := cw.zoom
	origin := cw.originScreen()
	w, h, px, py := cw.partExtentAnim(d.part)
	return fyne.NewPos(
			origin.X+(d.tr.X-px)*zoom,
			origin.Y+(d.tr.Y-py)*zoom,
		), fyne.NewSize(w*zoom, h*zoom)
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

	origin := cw.originScreen()

	bg := canvas.NewRectangle(ColorCanvasBackground)
	bg.Resize(size)
	objs = append(objs, bg)

	if cw.showGrid {
		objs = append(objs, r.buildGrid(size, origin)...)
	}

	// Character-sized reference box, filling the crosshair's bottom-right
	// quadrant — parts are placed relative to it.
	refW, refH := cw.refBox()
	refRect := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
	refRect.StrokeColor = ColorRefBox
	refRect.StrokeWidth = 1
	refRect.Resize(fyne.NewSize(refW*cw.zoom, refH*cw.zoom))
	refRect.Move(origin)
	objs = append(objs, refRect)

	refLabel := canvas.NewText(fmt.Sprintf("%.0fx%.0f", refW, refH), ColorRefBox)
	refLabel.TextSize = 9
	refLabel.Move(fyne.NewPos(origin.X+2, origin.Y+refH*cw.zoom+2))
	objs = append(objs, refLabel)

	// Origin crosshair, spanning the whole canvas so 0,0 stays findable
	// however far the view has grown.
	hLine := canvas.NewLine(ColorOriginCrosshair)
	hLine.StrokeWidth = 1
	hLine.Position1 = fyne.NewPos(0, origin.Y)
	hLine.Position2 = fyne.NewPos(size.Width, origin.Y)
	vLine := canvas.NewLine(ColorOriginCrosshair)
	vLine.StrokeWidth = 1
	vLine.Position1 = fyne.NewPos(origin.X, 0)
	vLine.Position2 = fyne.NewPos(origin.X, size.Height)
	objs = append(objs, hLine, vLine)

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

// buildGrid draws gridlines aligned to the origin rather than the widget's
// top-left, so a gridline always falls exactly on 0,0 and the squares mean
// the same thing on both sides of each axis.
func (r *canvasRenderer) buildGrid(size fyne.Size, origin fyne.Position) []fyne.CanvasObject {
	gridSpacing := float32(16) * r.widget.zoom
	if gridSpacing < 2 {
		return nil
	}

	var objs []fyne.CanvasObject
	for x := origin.X - floorTo(origin.X, gridSpacing); x <= size.Width; x += gridSpacing {
		line := canvas.NewLine(ColorGrid)
		line.Position1 = fyne.NewPos(x, 0)
		line.Position2 = fyne.NewPos(x, size.Height)
		objs = append(objs, line)
	}
	for y := origin.Y - floorTo(origin.Y, gridSpacing); y <= size.Height; y += gridSpacing {
		line := canvas.NewLine(ColorGrid)
		line.Position1 = fyne.NewPos(0, y)
		line.Position2 = fyne.NewPos(size.Width, y)
		objs = append(objs, line)
	}
	return objs
}
