package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// BoxType identifies which hitbox is being interacted with.
type BoxType int

const (
	BoxNone BoxType = iota
	BoxCollision
	BoxAttack
)

// DragHandle identifies which part of a hitbox is being dragged.
type DragHandle int

const (
	HandleNone DragHandle = iota
	HandleCenter
	HandleCornerTL
	HandleCornerTR
	HandleCornerBL
	HandleCornerBR
)

// CanvasWidget is a custom widget for displaying sprites and editing hitboxes.
type CanvasWidget struct {
	widget.BaseWidget

	project      *editor.Project
	spriteImage  image.Image
	zoom         float32
	showGrid     bool
	showHitboxes bool

	// Hitbox editing state
	selectedBox BoxType
	dragHandle  DragHandle
	lastMouseX  float32
	lastMouseY  float32
	isDragging  bool

	// Callbacks
	OnBoxChanged func()

	// Cached canvas objects
	background *canvas.Rectangle
	objects    []fyne.CanvasObject
}

// NewCanvasWidget creates a new canvas widget.
func NewCanvasWidget(project *editor.Project) *CanvasWidget {
	cw := &CanvasWidget{
		project:      project,
		zoom:         4.0, // Start at 4x zoom for pixel art
		showGrid:     true,
		showHitboxes: true,
		selectedBox:  BoxNone,
		dragHandle:   HandleNone,
	}
	cw.ExtendBaseWidget(cw)
	return cw
}

// SetProject updates the project reference.
func (cw *CanvasWidget) SetProject(project *editor.Project) {
	cw.project = project
	cw.Refresh()
}

// SetSpriteImage sets the current sprite to display.
func (cw *CanvasWidget) SetSpriteImage(img image.Image) {
	cw.spriteImage = img
	cw.Refresh()
}

// SetZoom sets the zoom level.
func (cw *CanvasWidget) SetZoom(z float32) {
	cw.zoom = z
	cw.Refresh()
}

// ToggleGrid toggles grid overlay visibility.
func (cw *CanvasWidget) ToggleGrid() {
	cw.showGrid = !cw.showGrid
	cw.Refresh()
}

// ToggleHitboxes toggles hitbox overlay visibility.
func (cw *CanvasWidget) ToggleHitboxes() {
	cw.showHitboxes = !cw.showHitboxes
	cw.Refresh()
}

// CreateRenderer implements fyne.Widget.
func (cw *CanvasWidget) CreateRenderer() fyne.WidgetRenderer {
	return &canvasRenderer{widget: cw}
}

// MinSize returns the minimum size of the canvas.
func (cw *CanvasWidget) MinSize() fyne.Size {
	return fyne.NewSize(300, 300)
}

// -- Mouse interaction --

// Tappable implements secondary tap for context menu
func (cw *CanvasWidget) TappedSecondary(e *fyne.PointEvent) {
	// Context menu for add/remove hitboxes
	kf := cw.project.GetCurrentKeyFrame()
	if kf == nil {
		return
	}

	items := []*fyne.MenuItem{}
	if kf.HitBox == nil {
		items = append(items, fyne.NewMenuItem("Add Collision Box", func() {
			kf.HitBox = &editor.Box{X: 4, Y: 4, W: 24, H: 24}
			cw.project.RecordUndo()
			if cw.OnBoxChanged != nil {
				cw.OnBoxChanged()
			}
			cw.Refresh()
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Remove Collision Box", func() {
			kf.HitBox = nil
			cw.project.RecordUndo()
			if cw.OnBoxChanged != nil {
				cw.OnBoxChanged()
			}
			cw.Refresh()
		}))
	}

	if kf.AttackHitBox == nil {
		items = append(items, fyne.NewMenuItem("Add Attack Box", func() {
			kf.AttackHitBox = &editor.Box{X: 8, Y: 2, W: 20, H: 28}
			cw.project.RecordUndo()
			if cw.OnBoxChanged != nil {
				cw.OnBoxChanged()
			}
			cw.Refresh()
		}))
	} else {
		items = append(items, fyne.NewMenuItem("Remove Attack Box", func() {
			kf.AttackHitBox = nil
			cw.project.RecordUndo()
			if cw.OnBoxChanged != nil {
				cw.OnBoxChanged()
			}
			cw.Refresh()
		}))
	}

	menu := fyne.NewMenu("", items...)
	c := fyne.CurrentApp().Driver().CanvasForObject(cw)
	if c != nil {
		widget.ShowPopUpMenuAtPosition(menu, c, e.AbsolutePosition)
	}
}

// Implement desktop.Hoverable for cursor changes
var _ desktop.Hoverable = (*CanvasWidget)(nil)

func (cw *CanvasWidget) MouseIn(*desktop.MouseEvent)    {}
func (cw *CanvasWidget) MouseMoved(*desktop.MouseEvent)  {}
func (cw *CanvasWidget) MouseOut()                       {}

// Draggable implements fyne.Draggable for hitbox editing
func (cw *CanvasWidget) Dragged(e *fyne.DragEvent) {
	if !cw.isDragging {
		// Check if starting a drag on a hitbox handle
		kf := cw.project.GetCurrentKeyFrame()
		if kf == nil {
			return
		}

		pos := e.Position
		cw.selectedBox, cw.dragHandle = cw.hitTestHandles(kf, pos)
		if cw.dragHandle == HandleNone {
			return
		}
		cw.isDragging = true
		cw.lastMouseX = pos.X
		cw.lastMouseY = pos.Y
	}

	if cw.dragHandle == HandleNone {
		return
	}

	kf := cw.project.GetCurrentKeyFrame()
	if kf == nil {
		return
	}

	var box *editor.Box
	switch cw.selectedBox {
	case BoxCollision:
		box = kf.HitBox
	case BoxAttack:
		box = kf.AttackHitBox
	}
	if box == nil {
		return
	}

	dx := int((e.Position.X - cw.lastMouseX) / cw.zoom)
	dy := int((e.Position.Y - cw.lastMouseY) / cw.zoom)

	switch cw.dragHandle {
	case HandleCenter:
		box.X += dx
		box.Y += dy
	case HandleCornerTL:
		box.X += dx
		box.Y += dy
		box.W -= dx
		box.H -= dy
	case HandleCornerBR:
		box.W += dx
		box.H += dy
	case HandleCornerTR:
		box.W += dx
		box.Y += dy
		box.H -= dy
	case HandleCornerBL:
		box.X += dx
		box.H += dy
		box.W -= dx
	}

	// Clamp minimum size
	if box.W < 2 {
		box.W = 2
	}
	if box.H < 2 {
		box.H = 2
	}

	cw.lastMouseX = e.Position.X
	cw.lastMouseY = e.Position.Y

	if cw.OnBoxChanged != nil {
		cw.OnBoxChanged()
	}
	cw.Refresh()
}

func (cw *CanvasWidget) DragEnd() {
	if cw.isDragging {
		cw.project.RecordUndo()
	}
	cw.isDragging = false
	cw.dragHandle = HandleNone
}

// hitTestHandles checks if a point is near a hitbox handle.
func (cw *CanvasWidget) hitTestHandles(kf *editor.KeyFrame, pos fyne.Position) (BoxType, DragHandle) {
	handleSize := float32(8)
	canvasSize := cw.Size()
	offsetX := canvasSize.Width/2
	offsetY := canvasSize.Height/2

	// Test attack box first (drawn on top)
	if kf.AttackHitBox != nil && cw.showHitboxes {
		bt, dh := cw.testBoxHandles(kf.AttackHitBox, pos, offsetX, offsetY, handleSize)
		if dh != HandleNone {
			return BoxAttack, dh
		}
		_ = bt
	}

	// Test collision box
	if kf.HitBox != nil && cw.showHitboxes {
		bt, dh := cw.testBoxHandles(kf.HitBox, pos, offsetX, offsetY, handleSize)
		if dh != HandleNone {
			return BoxCollision, dh
		}
		_ = bt
	}

	return BoxNone, HandleNone
}

func (cw *CanvasWidget) testBoxHandles(box *editor.Box, pos fyne.Position, offX, offY, hs float32) (BoxType, DragHandle) {
	bx := offX + float32(box.X)*cw.zoom
	by := offY + float32(box.Y)*cw.zoom
	bw := float32(box.W) * cw.zoom
	bh := float32(box.H) * cw.zoom

	// Corner handles
	if nearPoint(pos.X, pos.Y, bx, by, hs) {
		return BoxCollision, HandleCornerTL
	}
	if nearPoint(pos.X, pos.Y, bx+bw, by, hs) {
		return BoxCollision, HandleCornerTR
	}
	if nearPoint(pos.X, pos.Y, bx, by+bh, hs) {
		return BoxCollision, HandleCornerBL
	}
	if nearPoint(pos.X, pos.Y, bx+bw, by+bh, hs) {
		return BoxCollision, HandleCornerBR
	}

	// Center handle
	cx := bx + bw/2
	cy := by + bh/2
	if nearPoint(pos.X, pos.Y, cx, cy, hs*2) {
		return BoxCollision, HandleCenter
	}

	return BoxNone, HandleNone
}

func nearPoint(px, py, tx, ty, threshold float32) bool {
	dx := px - tx
	dy := py - ty
	return dx*dx+dy*dy < threshold*threshold
}

// -- Canvas Renderer --

type canvasRenderer struct {
	widget  *CanvasWidget
	objects []fyne.CanvasObject
}

func (r *canvasRenderer) Layout(size fyne.Size) {
	for _, obj := range r.objects {
		obj.Resize(size)
		obj.Move(fyne.NewPos(0, 0))
	}
}

func (r *canvasRenderer) MinSize() fyne.Size {
	return fyne.NewSize(300, 300)
}

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
	size := cw.Size()
	objs := []fyne.CanvasObject{}

	// Background
	bg := canvas.NewRectangle(ColorCanvasBackground)
	bg.Resize(size)
	objs = append(objs, bg)

	kf := cw.project.GetCurrentKeyFrame()

	// Calculate center offset for sprite
	centerX := size.Width / 2
	centerY := size.Height / 2

	// Draw sprite if available
	if cw.spriteImage != nil {
		bounds := cw.spriteImage.Bounds()
		spriteW := float32(bounds.Dx()) * cw.zoom
		spriteH := float32(bounds.Dy()) * cw.zoom

		spriteX := centerX - spriteW/2
		spriteY := centerY - spriteH/2

		img := canvas.NewImageFromImage(cw.spriteImage)
		img.ScaleMode = canvas.ImageScalePixels
		img.FillMode = canvas.ImageFillOriginal
		img.Resize(fyne.NewSize(spriteW, spriteH))
		img.Move(fyne.NewPos(spriteX, spriteY))
		objs = append(objs, img)
	}

	// Draw grid overlay
	if cw.showGrid {
		gridSpacing := float32(16) * cw.zoom
		gridArea := float32(256) * cw.zoom // Show grid over a reasonable area

		startX := centerX - gridArea/2
		startY := centerY - gridArea/2

		for x := startX; x <= centerX+gridArea/2; x += gridSpacing {
			line := canvas.NewLine(ColorGrid)
			line.StrokeWidth = 1
			line.Position1 = fyne.NewPos(x, startY)
			line.Position2 = fyne.NewPos(x, centerY+gridArea/2)
			objs = append(objs, line)
		}
		for y := startY; y <= centerY+gridArea/2; y += gridSpacing {
			line := canvas.NewLine(ColorGrid)
			line.StrokeWidth = 1
			line.Position1 = fyne.NewPos(startX, y)
			line.Position2 = fyne.NewPos(centerX+gridArea/2, y)
			objs = append(objs, line)
		}
	}

	// Draw origin crosshair
	crossLen := float32(20)
	hLine := canvas.NewLine(ColorOriginCrosshair)
	hLine.StrokeWidth = 1
	hLine.Position1 = fyne.NewPos(centerX-crossLen, centerY)
	hLine.Position2 = fyne.NewPos(centerX+crossLen, centerY)
	objs = append(objs, hLine)

	vLine := canvas.NewLine(ColorOriginCrosshair)
	vLine.StrokeWidth = 1
	vLine.Position1 = fyne.NewPos(centerX, centerY-crossLen)
	vLine.Position2 = fyne.NewPos(centerX, centerY+crossLen)
	objs = append(objs, vLine)

	if kf == nil || !cw.showHitboxes {
		return objs
	}

	// Draw collision hitbox (blue)
	if kf.HitBox != nil {
		objs = append(objs, r.drawBox(kf.HitBox, centerX, centerY, ColorCollisionBox, ColorCollisionBoxBorder, cw.zoom)...)
	}

	// Draw attack hitbox (red)
	if kf.AttackHitBox != nil {
		objs = append(objs, r.drawBox(kf.AttackHitBox, centerX, centerY, ColorAttackBox, ColorAttackBoxBorder, cw.zoom)...)
	}

	// Draw info text
	if kf != nil {
		info := fmt.Sprintf("Frame %d  |  %dms  |  Zoom: %.0f%%", kf.ID, kf.Duration, cw.zoom*100)
		text := canvas.NewText(info, color.RGBA{R: 160, G: 160, B: 170, A: 200})
		text.TextSize = 11
		text.Move(fyne.NewPos(8, size.Height-20))
		objs = append(objs, text)
	}

	return objs
}

func (r *canvasRenderer) drawBox(box *editor.Box, cx, cy float32, fill, stroke color.Color, zoom float32) []fyne.CanvasObject {
	objs := []fyne.CanvasObject{}

	bx := cx + float32(box.X)*zoom
	by := cy + float32(box.Y)*zoom
	bw := float32(box.W) * zoom
	bh := float32(box.H) * zoom

	// Fill
	fillRect := canvas.NewRectangle(fill)
	fillRect.Move(fyne.NewPos(bx, by))
	fillRect.Resize(fyne.NewSize(bw, bh))
	objs = append(objs, fillRect)

	// Border lines
	top := canvas.NewLine(stroke)
	top.StrokeWidth = 2
	top.Position1 = fyne.NewPos(bx, by)
	top.Position2 = fyne.NewPos(bx+bw, by)
	objs = append(objs, top)

	bottom := canvas.NewLine(stroke)
	bottom.StrokeWidth = 2
	bottom.Position1 = fyne.NewPos(bx, by+bh)
	bottom.Position2 = fyne.NewPos(bx+bw, by+bh)
	objs = append(objs, bottom)

	left := canvas.NewLine(stroke)
	left.StrokeWidth = 2
	left.Position1 = fyne.NewPos(bx, by)
	left.Position2 = fyne.NewPos(bx, by+bh)
	objs = append(objs, left)

	right := canvas.NewLine(stroke)
	right.StrokeWidth = 2
	right.Position1 = fyne.NewPos(bx+bw, by)
	right.Position2 = fyne.NewPos(bx+bw, by+bh)
	objs = append(objs, right)

	// Corner handles
	handleSize := float32(6)
	corners := []fyne.Position{
		{X: bx - handleSize/2, Y: by - handleSize/2},
		{X: bx + bw - handleSize/2, Y: by - handleSize/2},
		{X: bx - handleSize/2, Y: by + bh - handleSize/2},
		{X: bx + bw - handleSize/2, Y: by + bh - handleSize/2},
	}
	for _, pos := range corners {
		handle := canvas.NewRectangle(ColorHandlePoint)
		handle.Move(pos)
		handle.Resize(fyne.NewSize(handleSize, handleSize))
		objs = append(objs, handle)
	}

	// Box dimensions label
	label := fmt.Sprintf("(%d,%d) %dx%d", box.X, box.Y, box.W, box.H)
	text := canvas.NewText(label, stroke)
	text.TextSize = 10
	text.Move(fyne.NewPos(bx, by-14))
	objs = append(objs, text)

	return objs
}
