package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// CanvasWidget renders every Part of the active direction at the current
// playhead time, Z-sorted, around the animation's origin — drawn as a
// full-span crosshair through (0,0) with a character-sized reference box in
// its bottom-right quadrant, the way GraalShop's GANI editor does it. Parts
// are positioned relative to that origin.
//
// The canvas is a view onto unbounded animation space: it fills the panel
// it sits in, and where the origin sits in it (pan) and at what scale
// (zoom) are the view's own state, not derived from what's placed. Middle
// mouse drag pans, from anywhere and whatever is selected; the wheel zooms
// about the cursor, 10% to 500%. Art may sit at negative coordinates
// above/left of the origin — a raised sword, a trailing cape — and is
// simply panned to. Because the origin only moves when the view does, a
// drag or an edit can never shift it out from under the cursor.
//
// The view is centred on the content (contentBounds) when the canvas is
// first laid out, when a track is opened, and on View > Center View.
//
// Parts are drawn rotated about their pivot (clockwise, as the game's
// raylib renderer does). Fyne can't rotate an image, so rotate.go turns the
// pixels itself; a nested part's rotation turns its whole animation
// (editor.FlattenNested).
type CanvasWidget struct {
	widget.BaseWidget

	project  *editor.Project
	zoom     float32
	showGrid bool
	// showOnion draws the selected part's neighbouring keyframe poses
	// faintly behind it (ToggleOnion).
	showOnion bool
	// preview is a palette tile being dragged over the canvas, or nil.
	preview *dropPreview
	// hover outlines the part under the mouse (hoverPartIdx, -1 for none).
	hover        *hoverBox
	hoverPartIdx int

	draggingPartIdx int // -1 = not dragging
	// dragStartLocal is where the press began, widget-local. Drags report
	// movement relative to it rather than the cursor's absolute position,
	// so a sprite moves with the point it was grabbed by instead of
	// snapping its pivot to the cursor — which for a 160px cell grabbed by
	// a corner was a jump of half its size on the first frame.
	dragStartLocal fyne.Position

	// pan is where animation (0,0) sits, widget-local. viewPlaced is false
	// until the view has been centred at a real size (layoutView);
	// viewSize is the size it was last laid out at.
	pan        fyne.Position
	viewPlaced bool
	viewSize   fyne.Size
	// panning is set by a middle-button press: the drag that follows moves
	// the view rather than a part.
	panning bool

	// OnPartTapped fires with the part index under the click, or -1 if the
	// click landed on empty space (a deselect).
	OnPartTapped func(partIdx int)
	// OnPartDragStart/Dragged/DragEnd fire while a placed part is being
	// repositioned directly on the canvas. Dragged reports dx/dy in the
	// animation's own coordinate space, measured from where the drag
	// started — the caller adds them to the keyframe's position as it was
	// at OnPartDragStart.
	OnPartDragStart func(partIdx int)
	OnPartDragged   func(partIdx int, dx, dy float32)
	OnPartDragEnd   func()
}

func NewCanvasWidget(project *editor.Project) *CanvasWidget {
	cw := &CanvasWidget{project: project, zoom: 4.0, showGrid: true, draggingPartIdx: -1,
		hoverPartIdx: -1}
	cw.ExtendBaseWidget(cw)
	return cw
}

// SetProject shows another project, centred on its content.
func (cw *CanvasWidget) SetProject(project *editor.Project) {
	cw.project = project
	cw.viewPlaced = false
	cw.layoutView(cw.Size())
	cw.Refresh()
}

// SetZoom sets the zoom (clamped to minZoom..maxZoom), keeping whatever is
// at the middle of the canvas where it is.
func (cw *CanvasWidget) SetZoom(z float32) {
	size := cw.Size()
	cw.zoomAround(fyne.NewPos(size.Width/2, size.Height/2), z)
}

// Zoom is the current zoom factor (1 = 100%).
func (cw *CanvasWidget) Zoom() float32 { return cw.zoom }

// CenterView pans so the content (the reference box and every keyframe
// of the active direction) is centred, keeping the zoom.
func (cw *CanvasWidget) CenterView() {
	cw.centerView(cw.Size())
	cw.Refresh()
}

func (cw *CanvasWidget) ToggleGrid() {
	cw.showGrid = !cw.showGrid
	cw.Refresh()
}

const (
	minZoom       = 0.1  // 10%
	maxZoom       = 5.0  // 500%
	wheelZoomStep = 1.25 // zoom factor per wheel notch
	nestedBoxPx   = 24   // placeholder box for a NestedAni part whose animation isn't loaded, in anim px
)

func clampZoom(z float32) float32 {
	return minF(maxF(z, minZoom), maxZoom)
}

// zoomAround changes the zoom to z (clamped), keeping the animation point
// under the widget-local position at at that same position.
func (cw *CanvasWidget) zoomAround(at fyne.Position, z float32) {
	z = clampZoom(z)
	if z == cw.zoom {
		return
	}
	ax, ay := cw.LocalToAnimXY(at)
	cw.zoom = z
	cw.pan = fyne.NewPos(at.X-ax*z, at.Y-ay*z)
	cw.Refresh()
}

// panBy moves the view by d widget pixels.
func (cw *CanvasWidget) panBy(d fyne.Delta) {
	cw.pan = cw.pan.Add(d)
	cw.Refresh()
}

// layoutView keeps the view placed as the canvas is resized: centred on
// the content the first time it has a real size, and after that keeping
// whatever was in the middle in the middle.
func (cw *CanvasWidget) layoutView(size fyne.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	if !cw.viewPlaced {
		cw.centerView(size)
	} else if size != cw.viewSize {
		cw.pan = cw.pan.Add(fyne.NewDelta((size.Width-cw.viewSize.Width)/2, (size.Height-cw.viewSize.Height)/2))
	}
	cw.viewSize = size
}

// centerView pans so contentBounds is centred in a canvas of this size.
func (cw *CanvasWidget) centerView(size fyne.Size) {
	if cw.project == nil || cw.project.CurrentTrack == nil {
		return
	}
	minX, minY, maxX, maxY := cw.contentBounds(cw.partExtents())
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	cw.pan = fyne.NewPos(size.Width/2-cx*cw.zoom, size.Height/2-cy*cw.zoom)
	cw.viewPlaced = size.Width > 0 && size.Height > 0
	cw.viewSize = size
}

// refBox returns the track's character-sized reference box dimensions.
func (cw *CanvasWidget) refBox() (w, h float32) {
	t := cw.project.CurrentTrack
	if t == nil || t.RefBoxWidth <= 0 || t.RefBoxHeight <= 0 {
		return editor.DefaultRefBoxWidth, editor.DefaultRefBoxHeight
	}
	return float32(t.RefBoxWidth), float32(t.RefBoxHeight)
}

// partExtentAnim is a part's drawn size and pivot in animation pixels,
// the same for all its keyframes. For a sheet part that's its cell; for a
// nested part it's the box covering every pose of the animation it plays
// (Project.NestedExtent), "pivoted" at the nested animation's own origin -
// so its hit box and the content the view centres on don't change as it
// plays.
func (cw *CanvasWidget) partExtentAnim(part *editor.Part) (w, h, pivotX, pivotY float32) {
	if part.Kind == editor.PartKindNestedAni {
		if minX, minY, maxX, maxY, ok := cw.project.NestedExtent(part); ok {
			return maxX - minX, maxY - minY, -minX, -minY
		}
		return nestedBoxPx, nestedBoxPx, nestedBoxPx / 2, nestedBoxPx / 2
	}
	sheet := cw.project.ResolveActiveSheet(part)
	if sheet == nil {
		return 0, 0, 0, 0
	}
	return float32(sheet.CellW), float32(sheet.CellH), sheet.PivotX, sheet.PivotY
}

// partExtent is partExtentAnim's result for one part.
type partExtent struct{ w, h, pivotX, pivotY float32 }

// partExtents measures every part once. Measuring a nested part flattens
// its animation at each of its keyframe times, so this is done once per
// redraw and shared, not once per part per lookup.
func (cw *CanvasWidget) partExtents() map[*editor.Part]partExtent {
	ext := make(map[*editor.Part]partExtent, len(cw.project.CurrentTrack.Parts))
	for _, part := range cw.project.CurrentTrack.Parts {
		w, h, px, py := cw.partExtentAnim(part)
		ext[part] = partExtent{w, h, px, py}
	}
	return ext
}

// contentBounds is what the view centres on, in animation coordinates: the
// origin and the reference box, plus every keyframe of every part in the
// active direction (not just the current frame, so it doesn't matter where
// the playhead is when the view is centred).
func (cw *CanvasWidget) contentBounds(ext map[*editor.Part]partExtent) (minX, minY, maxX, maxY float32) {
	refW, refH := cw.refBox()
	minX, minY, maxX, maxY = 0, 0, refW, refH

	if dir := cw.project.ActiveDirection(); dir != nil {
		for _, part := range cw.project.CurrentTrack.Parts {
			e := ext[part]
			w, h, px, py := e.w, e.h, e.pivotX, e.pivotY
			for _, kf := range dir.KeyframesFor(part.ID) {
				x0, y0 := kf.X-px, kf.Y-py
				minX, minY = minF(minX, x0), minF(minY, y0)
				maxX, maxY = maxF(maxX, x0+w), maxF(maxY, y0+h)
			}
		}
	}

	return minX, minY, maxX, maxY
}

// originScreen is where animation (0,0) sits in widget-local pixels: the
// view's pan.
func (cw *CanvasWidget) originScreen() fyne.Position {
	return cw.pan
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

// MinSize is small and fixed: the canvas fills whatever room it's given,
// and what it shows is decided by pan and zoom, not by its size.
func (cw *CanvasWidget) MinSize() fyne.Size {
	return fyne.NewSize(100, 100)
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

// floorTo rounds down to a multiple of q, including for negative values
// (where Go's integer truncation rounds toward zero, i.e. up).
func floorTo(v, q float32) float32 {
	n := float32(int(v / q))
	if v < 0 && n*q != v {
		n--
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
	// sprites is a nested part's animation, flattened for this frame and
	// positioned relative to the part (nil for sheet parts, or a nested
	// part whose animation isn't loaded).
	sprites []editor.FlatSprite
	// rot is the part's rotation as drawn (drawnRotation; 0 for none).
	// rect and size are the box covering it as rotated, so hit-testing
	// matches what's drawn.
	rot  float32
	rect fyne.Position // top-left
	size fyne.Size
}

// resolvedDraws computes every part's resolved transform and screen rect
// at the current playhead, Z-sorted back-to-front. Shared between the
// renderer (drawing) and this widget's own hit-testing, so both always
// agree on where a part actually is.
func (cw *CanvasWidget) resolvedDraws() []resolvedDraw {
	return cw.resolvedDrawsAt(cw.partExtents(), cw.originScreen())
}

// resolvedDrawsAt is resolvedDraws with the parts already measured and the
// origin already placed, so a redraw does each once.
func (cw *CanvasWidget) resolvedDrawsAt(ext map[*editor.Part]partExtent, origin fyne.Position) []resolvedDraw {
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
		if part.Kind == editor.PartKindNestedAni {
			d.sprites = cw.project.FlattenNested(part)
		}
		e := ext[part]
		d.rot = drawnRotation(d.tr.RotationDeg)
		// A nested part's extent is "pivoted" at its own origin, which is
		// what it turns about, so one box rule serves both kinds.
		minX, minY, maxX, maxY := editor.RotatedCellBox(e.w, e.h, e.pivotX, e.pivotY, d.rot)
		d.rect = fyne.NewPos(origin.X+(d.tr.X+minX)*cw.zoom, origin.Y+(d.tr.Y+minY)*cw.zoom)
		d.size = fyne.NewSize((maxX-minX)*cw.zoom, (maxY-minY)*cw.zoom)
		draws = append(draws, d)
	}
	sort.SliceStable(draws, func(i, j int) bool { return draws[i].tr.Z < draws[j].tr.Z })
	return draws
}

func posInRect(p fyne.Position, rectPos fyne.Position, rectSize fyne.Size) bool {
	return p.X >= rectPos.X && p.X <= rectPos.X+rectSize.Width &&
		p.Y >= rectPos.Y && p.Y <= rectPos.Y+rectSize.Height
}

// dragTarget is the part a drag starting at pos would move: the selected
// part, if pos is inside it, else none (-1). Only the selected part can be
// moved on the canvas - requested so that parts can't be nudged by
// accident, e.g. a background piece grabbed while reaching for the one in
// front. Click to select, then drag. Because the selection decides rather
// than what's on top, a part covered by another can still be dragged once
// it's selected (from the part list or the timeline).
func (cw *CanvasWidget) dragTarget(pos fyne.Position) int {
	sel := cw.project.Selection
	if sel == nil || sel.PartIndex < 0 {
		return -1
	}
	for _, d := range cw.resolvedDraws() {
		if d.partIdx == sel.PartIndex && posInRect(pos, d.rect, d.size) {
			return d.partIdx
		}
	}
	return -1
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

var _ fyne.Scrollable = (*CanvasWidget)(nil)
var _ desktop.Mouseable = (*CanvasWidget)(nil)

// Scrolled zooms about the cursor: wheel up zooms in, down zooms out, a
// fixed step per notch. Not while a part is held: its drag is measured in
// the view it started in.
func (cw *CanvasWidget) Scrolled(e *fyne.ScrollEvent) {
	if cw.draggingPartIdx >= 0 {
		return
	}
	switch {
	case e.Scrolled.DY > 0:
		cw.zoomAround(e.Position, cw.zoom*wheelZoomStep)
	case e.Scrolled.DY < 0:
		cw.zoomAround(e.Position, cw.zoom/wheelZoomStep)
	}
}

// MouseDown notes whether the drag that may follow is a pan: Fyne starts a
// drag for the middle button as for the left, but DragEvent doesn't say
// which button is held. Any other press clears it, so a middle click
// released off the canvas (no MouseUp here) can't leave it set.
func (cw *CanvasWidget) MouseDown(e *desktop.MouseEvent) {
	cw.panning = e.Button == desktop.MouseButtonTertiary
}

func (cw *CanvasWidget) MouseUp(e *desktop.MouseEvent) {
	if e.Button == desktop.MouseButtonTertiary {
		cw.panning = false
	}
}

// Dragged pans the view for a middle-button drag, from anywhere on the
// canvas and whatever is selected; a left-button drag moves the selected
// part.
func (cw *CanvasWidget) Dragged(e *fyne.DragEvent) {
	if cw.panning && cw.draggingPartIdx < 0 {
		cw.panBy(e.Dragged)
		return
	}
	if cw.draggingPartIdx < 0 {
		// Fyne's first Dragged event already carries the first movement,
		// so the press point is Position minus Dragged. Hit-testing
		// Position instead would test a point a few pixels off, and miss a
		// small part grabbed near its edge.
		start := e.Position.Subtract(fyne.NewPos(e.Dragged.DX, e.Dragged.DY))
		cw.draggingPartIdx = cw.dragTarget(start)
		if cw.draggingPartIdx < 0 {
			return
		}
		cw.dragStartLocal = start
		if cw.OnPartDragStart != nil {
			cw.OnPartDragStart(cw.draggingPartIdx)
		}
	}
	if cw.draggingPartIdx < 0 || cw.zoom == 0 {
		return
	}
	// The origin only moves when the view is panned or zoomed, neither of
	// which can happen mid-drag, so a delta in widget pixels maps to
	// animation pixels by zoom alone.
	dx := (e.Position.X - cw.dragStartLocal.X) / cw.zoom
	dy := (e.Position.Y - cw.dragStartLocal.Y) / cw.zoom
	if cw.OnPartDragged != nil {
		cw.OnPartDragged(cw.draggingPartIdx, dx, dy)
	}
}

func (cw *CanvasWidget) DragEnd() {
	if cw.draggingPartIdx >= 0 && cw.OnPartDragEnd != nil {
		cw.OnPartDragEnd()
	}
	cw.draggingPartIdx = -1
	cw.panning = false
}

// -- Renderer --

type canvasRenderer struct {
	widget  *CanvasWidget
	objects []fyne.CanvasObject

	// images keeps the canvas.Image drawn for each sheet cell from one
	// redraw to the next (a cell shown twice in a frame gets two), so a
	// playing animation moves the same objects around rather than creating
	// new ones every frame. Fyne caches a GPU texture per object for a
	// minute, so fresh objects each frame meant a texture upload per sprite
	// per frame and up to a minute of stale ones held at once.
	images    map[image.Image][]*canvas.Image
	imageUsed map[image.Image]int

	// Rotated cells are fresh pixels whenever the angle changes (every
	// frame, while a rotation plays), so they aren't pooled by image like
	// the above - that pool would only grow. Instead rotImages are reused
	// in draw order, each given the frame's pixels and refreshed (which
	// re-uploads its texture) only when they change.
	rotations rotationCache
	rotImages []*canvas.Image
	rotUsed   int
}

// cellImage returns a canvas.Image showing img, reusing one from an
// earlier redraw when there is one not yet used in this one.
// drawCell draws a sheet cell with its pivot at animation (x, y), turned
// deg (a drawnRotation) about it. translucency above 0 gives it an image
// of its own, since a pooled one would carry the translucency over to its
// next, solid, use.
func (r *canvasRenderer) drawCell(cell image.Image, sheet *editor.SpriteSheetTemplate, x, y, deg float32,
	origin fyne.Position, translucency float64) *canvas.Image {
	zoom := r.widget.zoom
	w, h := float32(sheet.CellW), float32(sheet.CellH)
	minX, minY, maxX, maxY := editor.RotatedCellBox(w, h, sheet.PivotX, sheet.PivotY, deg)
	pix := cell
	if deg != 0 {
		pix = r.rotations.get(cell, sheet.PivotX, sheet.PivotY, deg)
	}

	var img *canvas.Image
	switch {
	case translucency > 0:
		img = canvas.NewImageFromImage(pix)
		img.ScaleMode = canvas.ImageScalePixels
		img.FillMode = canvas.ImageFillOriginal
		img.Translucency = translucency
	case deg != 0:
		img = r.rotatedImage(pix)
	default:
		img = r.cellImage(pix)
	}
	img.Resize(fyne.NewSize((maxX-minX)*zoom, (maxY-minY)*zoom))
	img.Move(fyne.NewPos(origin.X+(x+minX)*zoom, origin.Y+(y+minY)*zoom))
	return img
}

// rotatedImage is the next of rotImages, showing pix.
func (r *canvasRenderer) rotatedImage(pix image.Image) *canvas.Image {
	if r.rotUsed < len(r.rotImages) {
		img := r.rotImages[r.rotUsed]
		r.rotUsed++
		if img.Image != pix {
			img.Image = pix
			img.Refresh()
		}
		return img
	}
	img := canvas.NewImageFromImage(pix)
	img.ScaleMode = canvas.ImageScalePixels
	img.FillMode = canvas.ImageFillOriginal
	r.rotImages = append(r.rotImages, img)
	r.rotUsed++
	return img
}

func (r *canvasRenderer) cellImage(img image.Image) *canvas.Image {
	if r.images == nil {
		r.images = map[image.Image][]*canvas.Image{}
	}
	n := r.imageUsed[img]
	r.imageUsed[img] = n + 1
	if pool := r.images[img]; n < len(pool) {
		return pool[n]
	}
	ci := canvas.NewImageFromImage(img)
	ci.ScaleMode = canvas.ImageScalePixels
	ci.FillMode = canvas.ImageFillOriginal
	r.images[img] = append(r.images[img], ci)
	return ci
}

// Layout keeps the view placed (layoutView) and redraws: the background,
// grid and crosshair fill the widget, so they change with its size.
func (r *canvasRenderer) Layout(size fyne.Size) {
	r.widget.layoutView(size)
	r.objects = r.buildObjects()
}

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

// buildObjects is the scene with the zoom readout over it, in the
// bottom-left corner.
func (r *canvasRenderer) buildObjects() []fyne.CanvasObject {
	cw := r.widget
	objs := r.buildScene()
	label := canvas.NewText(fmt.Sprintf("%.0f%%", cw.zoom*100), ColorRefBox)
	label.TextSize = 10
	label.Move(fyne.NewPos(4, cw.Size().Height-label.MinSize().Height-2))
	return append(objs, label)
}

func (r *canvasRenderer) buildScene() []fyne.CanvasObject {
	cw := r.widget
	r.imageUsed = map[image.Image]int{}
	r.rotUsed = 0
	ext := cw.partExtents()
	size := cw.Size()
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
	// while it's anywhere in view.
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

	draws := cw.resolvedDrawsAt(ext, origin)
	sel := cw.project.Selection
	// Onion skin first, so the real poses draw over it.
	if cw.showOnion && !cw.project.Playback.IsPlaying && sel != nil {
		for _, d := range draws {
			if d.partIdx == sel.PartIndex {
				objs = append(objs, r.onionGhosts(d, origin)...)
			}
		}
	}
	for _, d := range draws {
		selected := sel != nil && sel.PartIndex == d.partIdx
		objs = append(objs, r.drawPart(d, origin, selected)...)
	}
	// The hovered part's outline follows it as the animation plays.
	h := cw.hoverOutline()
	h.on = false
	h.rect.Hide()
	for _, d := range draws {
		if d.partIdx == cw.hoverPartIdx {
			h.on = true
			h.rect.Move(d.rect)
			h.rect.Resize(d.size)
			h.rect.Show()
		}
	}
	objs = append(objs, h.rect)
	// A tile being dragged in from the palette, on top of everything.
	if pv := r.dropPreviewImage(origin); pv != nil {
		objs = append(objs, pv)
	}

	return objs
}

func (r *canvasRenderer) drawPart(d resolvedDraw, origin fyne.Position, selected bool) []fyne.CanvasObject {
	if d.part.Kind == editor.PartKindNestedAni {
		return r.drawNested(d, origin, selected)
	}

	if d.sheet == nil || d.sheet.Image == nil {
		return nil
	}
	cellImg, err := d.sheet.CellImage(d.tr.Row, d.tr.Col)
	if err != nil {
		return nil
	}

	img := r.drawCell(cellImg, d.sheet, d.tr.X, d.tr.Y, d.rot, origin, 0)
	objs := []fyne.CanvasObject{img}
	if selected {
		objs = append(objs, selectionOutline(d.rect, d.size))
		if tick := rotationTick(d, origin, r.widget.zoom); tick != nil {
			objs = append(objs, tick)
		}
	}
	return objs
}

// drawNested draws a nested part's animation, live: every sprite of the
// flattened frame, offset by the part's own position. One whose animation
// isn't loaded is drawn as a labelled placeholder box instead of nothing,
// so it can still be seen, selected and moved.
func (r *canvasRenderer) drawNested(d resolvedDraw, origin fyne.Position, selected bool) []fyne.CanvasObject {
	cw := r.widget
	var objs []fyne.CanvasObject
	if len(d.sprites) == 0 {
		box := canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 0})
		box.StrokeColor = ColorOriginCrosshair
		box.StrokeWidth = 1
		box.Resize(d.size)
		box.Move(d.rect)
		name := filepath.Base(cw.project.ResolveNestedAnimPath(d.part))
		label := canvas.NewText(fmt.Sprintf("%s: %s not loaded", d.part.Name, name), ColorOriginCrosshair)
		label.TextSize = 9
		label.Move(fyne.NewPos(d.rect.X, d.rect.Y+d.size.Height+2))
		objs = append(objs, box, label)
	}

	for _, s := range d.sprites {
		cellImg, err := s.Sheet.CellImage(s.Row, s.Col)
		if err != nil {
			continue
		}
		objs = append(objs, r.drawCell(cellImg, s.Sheet, d.tr.X+s.X, d.tr.Y+s.Y, drawnRotation(s.RotationDeg), origin, 0))
	}
	if selected {
		objs = append(objs, selectionOutline(d.rect, d.size))
		if tick := rotationTick(d, origin, cw.zoom); tick != nil {
			objs = append(objs, tick)
		}
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

// rotationTick marks a selected part's rotation: a line from its pivot
// pointing the way the part's "up" now faces, which shows the pivot and
// the angle even where the art itself is hard to read. nil for no
// rotation.
func rotationTick(d resolvedDraw, origin fyne.Position, zoom float32) fyne.CanvasObject {
	if d.tr.RotationDeg == 0 {
		return nil
	}
	pivot := fyne.NewPos(origin.X+d.tr.X*zoom, origin.Y+d.tr.Y*zoom)
	length := d.size.Height / 2
	if d.size.Width/2 > length {
		length = d.size.Width / 2
	}
	line := canvas.NewLine(color.RGBA{R: 255, G: 220, B: 60, A: 255})
	line.StrokeWidth = 2
	line.Position1 = pivot
	line.Position2 = rotationTickEnd(pivot, length, d.tr.RotationDeg)
	return line
}

// rotationTickEnd is where a tick of the given length from pivot ends for
// a rotation of deg degrees, clockwise on screen (as raylib, the game's
// renderer, rotates): 0 points straight up, 90 to the right.
func rotationTickEnd(pivot fyne.Position, length, deg float32) fyne.Position {
	rad := float64(deg) * math.Pi / 180
	return fyne.NewPos(pivot.X+length*float32(math.Sin(rad)), pivot.Y-length*float32(math.Cos(rad)))
}

// onionTranslucency is how faint an onion-skin ghost is (1 = invisible).
const onionTranslucency = 0.7

// onionGhosts draws the selected part's poses at its keyframes either side
// of the playhead, faintly, so the pose between them can be judged
// without scrubbing back and forth. Sheet parts only; ghosts are drawn,
// never hit-tested, so they can't be clicked or dragged.
func (r *canvasRenderer) onionGhosts(d resolvedDraw, origin fyne.Position) []fyne.CanvasObject {
	if d.part.Kind != editor.PartKindSheet || d.sheet == nil {
		return nil
	}
	cw := r.widget
	prev, next := cw.project.ActiveDirection().NeighbourKeyframes(d.part.ID, cw.project.Playback.ElapsedMs)
	var objs []fyne.CanvasObject
	for _, kf := range []*editor.Keyframe{prev, next} {
		if kf == nil {
			continue
		}
		cell, err := d.sheet.CellImage(kf.Row, kf.Col)
		if err != nil {
			continue
		}
		objs = append(objs, r.drawCell(cell, d.sheet, kf.X, kf.Y, drawnRotation(kf.RotationDeg), origin, onionTranslucency))
	}
	return objs
}

// ToggleOnion turns the onion skin on or off.
func (cw *CanvasWidget) ToggleOnion() {
	cw.showOnion = !cw.showOnion
	cw.Refresh()
}

// dropPreview is a palette tile being dragged over the canvas: which cell,
// and the animation X/Y it would land at.
type dropPreview struct {
	sheet    *editor.SpriteSheetTemplate
	row, col int
	x, y     float32
	deg      float32 // the rotation the drop keeps (the selected part's)
}

// dropPreviewTranslucency is how faint the dragged tile's preview is.
const dropPreviewTranslucency = 0.4

// SetDropPreview shows where a dragged palette tile would land: the cell,
// drawn faintly by its pivot at animation (x, y), turned deg, exactly as a
// drop there would place it.
func (cw *CanvasWidget) SetDropPreview(sheet *editor.SpriteSheetTemplate, row, col int, x, y, deg float32) {
	cw.preview = &dropPreview{sheet: sheet, row: row, col: col, x: x, y: y, deg: deg}
	cw.Refresh()
}

// ClearDropPreview removes the drag preview (the drag ended or left the
// canvas).
func (cw *CanvasWidget) ClearDropPreview() {
	if cw.preview == nil {
		return
	}
	cw.preview = nil
	cw.Refresh()
}

// dropPreviewImage draws the drag preview, or nil without one.
func (r *canvasRenderer) dropPreviewImage(origin fyne.Position) fyne.CanvasObject {
	cw := r.widget
	pv := cw.preview
	if pv == nil || pv.sheet == nil {
		return nil
	}
	cell, err := pv.sheet.CellImage(pv.row, pv.col)
	if err != nil {
		return nil
	}
	return r.drawCell(cell, pv.sheet, pv.x, pv.y, drawnRotation(pv.deg), origin, dropPreviewTranslucency)
}

// -- Hover: the part under the mouse is outlined, with a hand cursor, so
// it's clear what a click would select (and, once selected, drag).

var _ desktop.Hoverable = (*CanvasWidget)(nil)
var _ desktop.Cursorable = (*CanvasWidget)(nil)

func (cw *CanvasWidget) MouseIn(e *desktop.MouseEvent) { cw.MouseMoved(e) }

func (cw *CanvasWidget) MouseMoved(e *desktop.MouseEvent) {
	idx := -1
	if cw.draggingPartIdx < 0 { // mid-drag, the part is plainly the one held
		idx = cw.hitTest(e.Position)
	}
	if idx == cw.hoverPartIdx {
		return
	}
	cw.hoverPartIdx = idx
	cw.placeHover()
}

func (cw *CanvasWidget) MouseOut() {
	cw.hoverPartIdx = -1
	cw.hoverOutline().hide()
}

func (cw *CanvasWidget) Cursor() desktop.Cursor {
	if cw.hoverPartIdx >= 0 {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

// placeHover moves the hover outline onto hoverPartIdx without redrawing
// the rest of the canvas.
func (cw *CanvasWidget) placeHover() {
	for _, d := range cw.resolvedDraws() {
		if d.partIdx == cw.hoverPartIdx {
			cw.hoverOutline().show(d.rect, d.size)
			return
		}
	}
	cw.hoverOutline().hide()
}

// hoverOutline is the hover box, made on first use.
func (cw *CanvasWidget) hoverOutline() *hoverBox {
	if cw.hover == nil {
		cw.hover = newHoverBox(false)
	}
	return cw.hover
}
