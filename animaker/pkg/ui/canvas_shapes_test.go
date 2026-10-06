package ui

import (
	"animaker/pkg/editor"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// shapeCanvas is a canvas at zoom 4 with the origin at (100, 100), showing
// a character with a footprint and one hitbox.
func shapeCanvas(t *testing.T) (*CanvasWidget, *editor.Character) {
	t.Helper()
	cw, _ := testCanvas(t)
	cw.SelectedShape = NoShape
	cw.pan = fyne.NewPos(100, 100)
	c := editor.NewCharacter("ogre")
	c.SetFootprint(editor.Box{X: -10, Y: -5, W: 20, H: 10})
	c.AddHitbox()
	c.SetHitbox(0, editor.Hitbox{Name: "body", Kind: editor.ShapeOval, Box: editor.Box{X: -20, Y: -60, W: 40, H: 60}})
	cw.Character = func() *editor.Character { return c }
	cw.OnShapeEdited = func(r ShapeRef, b editor.Box) {
		if r.Footprint {
			c.SetFootprint(b)
			return
		}
		h := c.Hitboxes[r.Hitbox]
		h.Box = b
		c.SetHitbox(r.Hitbox, h)
	}
	return cw, c
}

// dragScreen simulates a left drag from one widget-local point to another.
func dragScreen(cw *CanvasWidget, from, to fyne.Position) {
	mid := fyne.NewPos((from.X+to.X)/2, (from.Y+to.Y)/2)
	cw.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: mid}, Dragged: fyne.NewDelta(mid.X-from.X, mid.Y-from.Y)})
	cw.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: to}, Dragged: fyne.NewDelta(to.X-mid.X, to.Y-mid.Y)})
	cw.DragEnd()
}

func TestDragMovesSelectedFootprint(t *testing.T) {
	cw, c := shapeCanvas(t)
	ended := false
	cw.OnShapeEditEnd = func() { ended = true }
	cw.SelectedShape = FootprintRef()
	// The footprint's centre is anim (0,0) = screen (100,100); drag 40px
	// right and 20px down = 10 and 5 anim px at zoom 4.
	dragScreen(cw, fyne.NewPos(100, 100), fyne.NewPos(140, 120))
	if *c.Footprint != (editor.Box{X: 0, Y: 0, W: 20, H: 10}) {
		t.Errorf("footprint = %+v, want moved by (10, 5)", *c.Footprint)
	}
	if !ended {
		t.Error("OnShapeEditEnd didn't fire")
	}
}

func TestDragCornerResizesSelectedHitbox(t *testing.T) {
	cw, c := shapeCanvas(t)
	cw.SelectedShape = HitboxRef(0)
	// Bottom-right corner: anim (20, 0) = screen (180, 100). Drag it 20px
	// right and 40px down = 5, 10 anim px.
	dragScreen(cw, fyne.NewPos(180, 100), fyne.NewPos(200, 140))
	if got := c.Hitboxes[0].Box; got != (editor.Box{X: -20, Y: -60, W: 45, H: 70}) {
		t.Errorf("hitbox = %+v, want resized from its bottom-right corner", got)
	}
}

func TestUnselectedShapeIsNotDragged(t *testing.T) {
	cw, c := shapeCanvas(t)
	before := *c.Footprint
	dragScreen(cw, fyne.NewPos(100, 100), fyne.NewPos(140, 120))
	if *c.Footprint != before {
		t.Error("an unselected footprint moved")
	}
}

func TestOvalGrabsByItsShape(t *testing.T) {
	cw, _ := shapeCanvas(t)
	cw.SelectedShape = HitboxRef(0)
	// Inside the oval's box near its top-left corner, but outside the oval
	// and beyond the corner handle's reach: grabs nothing.
	if _, ok := cw.shapeGrab(fyne.NewPos(100-80+12, 100-240+12)); ok {
		t.Error("a press in the oval's box corner, outside the oval, grabbed it")
	}
	if h, ok := cw.shapeGrab(fyne.NewPos(100, 100-120)); !ok || h != editor.HandleNone {
		t.Error("a press at the oval's centre didn't grab its body")
	}
	if h, ok := cw.shapeGrab(fyne.NewPos(100-80, 100-240)); !ok || h != editor.HandleTopLeft {
		t.Errorf("a press on the top-left corner gave handle %v, %v", h, ok)
	}
}

func TestCircleResizeStaysSquare(t *testing.T) {
	cw, c := shapeCanvas(t)
	c.SetHitbox(0, editor.Hitbox{Name: "head", Kind: editor.ShapeCircle, Box: editor.Box{X: 0, Y: 0, W: 10, H: 10}})
	cw.SelectedShape = HitboxRef(0)
	dragScreen(cw, fyne.NewPos(140, 140), fyne.NewPos(180, 148)) // corner (10,10) by (10, 2) anim px
	if b := c.Hitboxes[0].Box; b.W != b.H || b.W != 20 {
		t.Errorf("circle box = %+v, want 20x20", b)
	}
}

// With a footprint the canvas draws it instead of the track's reference
// box; without a character it draws the reference box as before.
func TestFootprintReplacesRefBox(t *testing.T) {
	cw, _ := shapeCanvas(t)
	r := &canvasRenderer{widget: cw}
	objs := r.buildFootprint(cw.originScreen())
	if !hasText(objs, "footprint 20x10") {
		t.Errorf("footprint label missing: %v", texts(objs))
	}
	cw.Character = func() *editor.Character { return nil }
	objs = r.buildFootprint(cw.originScreen())
	if !hasText(objs, "48x64") {
		t.Errorf("reference box label missing without a character: %v", texts(objs))
	}
}

func TestFootprintLabelShowsGameSize(t *testing.T) {
	cw, c := shapeCanvas(t)
	c.Scale = 0.5
	r := &canvasRenderer{widget: cw}
	if objs := r.buildFootprint(cw.originScreen()); !hasText(objs, "footprint 20x10 (10x5 game px)") {
		t.Errorf("labels = %v", texts(objs))
	}
}

func TestHitboxesDrawnWithNamesAndSelection(t *testing.T) {
	cw, _ := shapeCanvas(t)
	r := &canvasRenderer{widget: cw}
	objs := r.buildHitboxes(cw.originScreen())
	if !hasText(objs, "body") {
		t.Errorf("hitbox name missing: %v", texts(objs))
	}
	n := len(objs)
	cw.SelectedShape = HitboxRef(0)
	if got := len(r.buildHitboxes(cw.originScreen())); got != n+5 {
		t.Errorf("selected hitbox drew %d objects, want %d (frame + 4 handles)", got, n+5)
	}
}

func TestContentBoundsIncludeShapes(t *testing.T) {
	cw, _ := shapeCanvas(t)
	minX, minY, maxX, _ := cw.contentBounds(cw.partExtents())
	if minX > -20 || minY > -60 || maxX < 20 {
		t.Errorf("bounds (%v,%v)-(%v,_) don't cover the hitbox", minX, minY, maxX)
	}
}

func texts(objs []fyne.CanvasObject) []string {
	var out []string
	for _, o := range objs {
		if t, ok := o.(*canvas.Text); ok {
			out = append(out, t.Text)
		}
	}
	return out
}

func hasText(objs []fyne.CanvasObject, s string) bool {
	for _, t := range texts(objs) {
		if t == s {
			return true
		}
	}
	return false
}

// The selected footprint's handles are drawn in the layer over the art,
// not behind it with the footprint.
func TestSelectedFootprintHandlesDrawOverArt(t *testing.T) {
	cw, _ := shapeCanvas(t)
	r := &canvasRenderer{widget: cw}
	under := len(r.buildFootprint(cw.originScreen()))
	over := len(r.buildHitboxes(cw.originScreen()))
	cw.SelectedShape = FootprintRef()
	if got := len(r.buildFootprint(cw.originScreen())); got != under {
		t.Errorf("selecting the footprint changed the layer behind the art: %d objects, want %d", got, under)
	}
	if got := len(r.buildHitboxes(cw.originScreen())); got != over+5 {
		t.Errorf("over-art layer has %d objects, want %d (frame + 4 handles)", got, over+5)
	}
}
