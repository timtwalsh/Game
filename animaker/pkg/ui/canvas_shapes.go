package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// The character's footprint and hitboxes (editor/shapes.go) on the canvas.
// They're drawn whenever the open track is one of the open character's
// animations: the footprint behind the art, in place of the track's
// reference box, and the hitboxes as outlines over it. The shape selected in
// the character panel can be dragged to move it, or by a corner handle to
// resize it; that takes priority over dragging the selected part.

// ShapeRef names one of the character's shapes: its footprint, or hitbox
// Hitbox (an index). NoShape names none.
type ShapeRef struct {
	Footprint bool
	Hitbox    int
}

// NoShape is no shape selected.
var NoShape = ShapeRef{Hitbox: -1}

// IsNone reports whether r names no shape.
func (r ShapeRef) IsNone() bool { return !r.Footprint && r.Hitbox < 0 }

// FootprintRef and HitboxRef name a shape.
func FootprintRef() ShapeRef   { return ShapeRef{Footprint: true, Hitbox: -1} }
func HitboxRef(i int) ShapeRef { return ShapeRef{Hitbox: i} }
func (r ShapeRef) String() string {
	switch {
	case r.Footprint:
		return "footprint"
	case r.Hitbox >= 0:
		return fmt.Sprintf("hitbox %d", r.Hitbox)
	}
	return "none"
}

// NRGBA, not RGBA: these are translucent, and color.RGBA is
// alpha-premultiplied, so {255, 170, 40, 210} isn't a valid RGBA and draws
// the wrong colour (text came out green).
var (
	ColorFootprintFill   = color.NRGBA{R: 0, G: 120, B: 255, A: 70}
	ColorFootprintBorder = color.NRGBA{R: 70, G: 165, B: 255, A: 235}
	ColorHitbox          = color.NRGBA{R: 255, G: 175, B: 40, A: 230}
	ColorShapeSelected   = color.NRGBA{R: 255, G: 255, B: 255, A: 235}
)

const (
	shapeHandlePx = 7  // corner handle size, screen pixels
	ovalSegments  = 40 // line segments an oval is drawn with
)

// shapeCharacter is the character whose shapes are shown, or nil.
func (cw *CanvasWidget) shapeCharacter() *editor.Character {
	if cw.Character == nil {
		return nil
	}
	return cw.Character()
}

// shape returns the box and kind of a shape of c; ok is false if c has no
// such shape.
func shapeOf(c *editor.Character, r ShapeRef) (b editor.Box, kind editor.ShapeKind, ok bool) {
	switch {
	case c == nil:
	case r.Footprint && c.Footprint != nil:
		return *c.Footprint, editor.ShapeRect, true
	case r.Hitbox >= 0 && r.Hitbox < len(c.Hitboxes):
		h := c.Hitboxes[r.Hitbox]
		return h.Box, h.Kind, true
	}
	return editor.Box{}, editor.ShapeRect, false
}

// boxScreen is a box's top-left and size on screen.
func (cw *CanvasWidget) boxScreen(b editor.Box, origin fyne.Position) (fyne.Position, fyne.Size) {
	return fyne.NewPos(origin.X+b.X*cw.zoom, origin.Y+b.Y*cw.zoom), fyne.NewSize(b.W*cw.zoom, b.H*cw.zoom)
}

// shapeGrab is what a press at pos would drag: a handle of the selected
// shape, or (HandleNone) its body. ok is false when it grabs nothing.
func (cw *CanvasWidget) shapeGrab(pos fyne.Position) (h editor.Handle, ok bool) {
	b, kind, found := shapeOf(cw.shapeCharacter(), cw.SelectedShape)
	if !found || cw.zoom == 0 {
		return editor.HandleNone, false
	}
	origin := cw.originScreen()
	for _, hd := range editor.Handles {
		x, y := b.Corner(hd)
		sx, sy := origin.X+x*cw.zoom, origin.Y+y*cw.zoom
		if abs(pos.X-sx) <= shapeHandlePx && abs(pos.Y-sy) <= shapeHandlePx {
			return hd, true
		}
	}
	ax, ay := cw.LocalToAnimXY(pos)
	// The body grabs by its box for rects, and by the shape itself for
	// ovals and circles, so a press just outside an oval's curve can still
	// reach a part behind it.
	if b.Contains(kind, ax, ay) {
		return editor.HandleNone, true
	}
	return editor.HandleNone, false
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// startShapeDrag begins dragging the selected shape if the press at start
// grabs it.
func (cw *CanvasWidget) startShapeDrag(start fyne.Position) bool {
	h, ok := cw.shapeGrab(start)
	if !ok {
		return false
	}
	b, _, _ := shapeOf(cw.shapeCharacter(), cw.SelectedShape)
	cw.shapeDrag = &shapeDrag{ref: cw.SelectedShape, handle: h, startBox: b, startLocal: start}
	return true
}

// dragShape applies a drag of the selected shape; pos is the cursor,
// widget-local.
func (cw *CanvasWidget) dragShape(pos fyne.Position) {
	d := cw.shapeDrag
	dx := (pos.X - d.startLocal.X) / cw.zoom
	dy := (pos.Y - d.startLocal.Y) / cw.zoom
	_, kind, _ := shapeOf(cw.shapeCharacter(), d.ref)
	var b editor.Box
	if d.handle == editor.HandleNone {
		b = d.startBox.Moved(dx, dy)
	} else {
		b = d.startBox.Resized(d.handle, dx, dy, kind == editor.ShapeCircle)
	}
	// Whole animation pixels, like keyframe positions placed by hand.
	b = editor.Box{X: round(b.X), Y: round(b.Y), W: max(round(b.W), 1), H: max(round(b.H), 1)}
	if cw.OnShapeEdited != nil {
		cw.OnShapeEdited(d.ref, b)
	}
}

func round(v float32) float32 { return float32(math.Round(float64(v))) }

type shapeDrag struct {
	ref        ShapeRef
	handle     editor.Handle
	startBox   editor.Box
	startLocal fyne.Position
}

// shapeBounds widens content bounds to include the character's shapes, so
// centring the view shows them.
func (cw *CanvasWidget) shapeBounds(minX, minY, maxX, maxY float32) (float32, float32, float32, float32) {
	c := cw.shapeCharacter()
	if c == nil {
		return minX, minY, maxX, maxY
	}
	grow := func(b editor.Box) {
		minX, minY = minF(minX, b.X), minF(minY, b.Y)
		maxX, maxY = maxF(maxX, b.X+b.W), maxF(maxY, b.Y+b.H)
	}
	if c.Footprint != nil {
		grow(*c.Footprint)
	}
	for _, h := range c.Hitboxes {
		grow(h.Box)
	}
	return minX, minY, maxX, maxY
}

// sizeLabel describes a box's size: animation pixels, plus game pixels when
// the character has a scale.
func sizeLabel(c *editor.Character, b editor.Box) string {
	s := fmt.Sprintf("%gx%g", b.W, b.H)
	if gw, ok := c.GameSize(b.W); ok {
		gh, _ := c.GameSize(b.H)
		s += fmt.Sprintf(" (%.3gx%.3g game px)", gw, gh)
	}
	return s
}

// buildFootprint draws the character's footprint, or, when the open track
// isn't a character's or the character has none, the track's reference box.
func (r *canvasRenderer) buildFootprint(origin fyne.Position) []fyne.CanvasObject {
	cw := r.widget
	c := cw.shapeCharacter()
	if c == nil || c.Footprint == nil {
		return r.buildRefBox(origin)
	}
	b := *c.Footprint
	pos, size := cw.boxScreen(b, origin)
	rect := canvas.NewRectangle(ColorFootprintFill)
	rect.StrokeColor = ColorFootprintBorder
	rect.StrokeWidth = 1
	rect.Move(pos)
	rect.Resize(size)
	label := canvas.NewText("footprint "+sizeLabel(c, b), ColorFootprintBorder)
	label.TextSize = 9
	label.Move(fyne.NewPos(pos.X+2, pos.Y+size.Height+2))
	return []fyne.CanvasObject{rect, label}
}

// buildRefBox is the track's character-sized reference box, filling the
// crosshair's bottom-right quadrant: a placement guide only.
func (r *canvasRenderer) buildRefBox(origin fyne.Position) []fyne.CanvasObject {
	cw := r.widget
	refW, refH := cw.refBox()
	refRect := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
	refRect.StrokeColor = ColorRefBox
	refRect.StrokeWidth = 1
	refRect.Resize(fyne.NewSize(refW*cw.zoom, refH*cw.zoom))
	refRect.Move(origin)
	refLabel := canvas.NewText(fmt.Sprintf("%.0fx%.0f", refW, refH), ColorRefBox)
	refLabel.TextSize = 9
	refLabel.Move(fyne.NewPos(origin.X+2, origin.Y+refH*cw.zoom+2))
	return []fyne.CanvasObject{refRect, refLabel}
}

// buildHitboxes outlines the character's hitboxes, drawn over the art,
// and marks the selected shape there too: the footprint is drawn behind
// the art, but its handles must stay visible to be grabbed.
func (r *canvasRenderer) buildHitboxes(origin fyne.Position) []fyne.CanvasObject {
	cw := r.widget
	c := cw.shapeCharacter()
	if c == nil {
		return nil
	}
	var objs []fyne.CanvasObject
	if cw.SelectedShape.Footprint && c.Footprint != nil {
		objs = append(objs, r.selection(*c.Footprint, editor.ShapeRect, origin)...)
	}
	for i, h := range c.Hitboxes {
		objs = append(objs, r.outline(h.Box, h.Kind, origin, ColorHitbox)...)
		pos, _ := cw.boxScreen(h.Box, origin)
		label := canvas.NewText(h.Name, ColorHitbox)
		label.TextSize = 9
		label.Move(fyne.NewPos(pos.X+2, pos.Y-12))
		objs = append(objs, label)
		if cw.SelectedShape.Hitbox == i {
			objs = append(objs, r.selection(h.Box, h.Kind, origin)...)
		}
	}
	return objs
}

// outline draws a shape's outline.
func (r *canvasRenderer) outline(b editor.Box, kind editor.ShapeKind, origin fyne.Position, col color.Color) []fyne.CanvasObject {
	cw := r.widget
	pos, size := cw.boxScreen(b, origin)
	switch kind {
	case editor.ShapeCircle:
		c := canvas.NewCircle(color.Transparent)
		c.StrokeColor = col
		c.StrokeWidth = 1.5
		c.Move(pos)
		c.Resize(size)
		return []fyne.CanvasObject{c}
	case editor.ShapeOval:
		// Fyne's circle in a non-square box is a rounded rectangle, not an
		// ellipse, so an oval is drawn as segments.
		cx, cy := pos.X+size.Width/2, pos.Y+size.Height/2
		rx, ry := size.Width/2, size.Height/2
		at := func(i int) fyne.Position {
			a := 2 * math.Pi * float64(i) / ovalSegments
			return fyne.NewPos(cx+rx*float32(math.Cos(a)), cy+ry*float32(math.Sin(a)))
		}
		objs := make([]fyne.CanvasObject, 0, ovalSegments)
		for i := 0; i < ovalSegments; i++ {
			l := canvas.NewLine(col)
			l.StrokeWidth = 1.5
			l.Position1, l.Position2 = at(i), at(i+1)
			objs = append(objs, l)
		}
		return objs
	}
	rect := canvas.NewRectangle(color.Transparent)
	rect.StrokeColor = col
	rect.StrokeWidth = 1.5
	rect.Move(pos)
	rect.Resize(size)
	return []fyne.CanvasObject{rect}
}

// selection marks the selected shape: its box, dashed by being drawn thin
// over the outline, and a handle at each corner.
func (r *canvasRenderer) selection(b editor.Box, kind editor.ShapeKind, origin fyne.Position) []fyne.CanvasObject {
	cw := r.widget
	pos, size := cw.boxScreen(b, origin)
	frame := canvas.NewRectangle(color.Transparent)
	frame.StrokeColor = ColorShapeSelected
	frame.StrokeWidth = 1
	frame.Move(pos)
	frame.Resize(size)
	objs := []fyne.CanvasObject{frame}
	for _, h := range editor.Handles {
		x, y := b.Corner(h)
		hd := canvas.NewRectangle(ColorShapeSelected)
		hd.Move(fyne.NewPos(origin.X+x*cw.zoom-shapeHandlePx/2, origin.Y+y*cw.zoom-shapeHandlePx/2))
		hd.Resize(fyne.NewSize(shapeHandlePx, shapeHandlePx))
		objs = append(objs, hd)
	}
	return objs
}
