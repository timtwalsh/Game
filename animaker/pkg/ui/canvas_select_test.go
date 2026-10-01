package ui

import (
	"image"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
)

// twoOverlappingParts: "back" (Z 0) and "front" (Z 1), both 16x16 cells
// at the same spot, so front covers back entirely.
func twoOverlappingParts(t *testing.T) (*CanvasWidget, *editor.Project) {
	t.Helper()
	cw, p := testCanvas(t)
	p.LoadedSheets["s"] = editor.NewSpriteSheetTemplate("s", "s.png", image.NewRGBA(image.Rect(0, 0, 16, 16)), 16, 16, 0, 0)
	for i, name := range []string{"back", "front"} {
		part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart(name, "", "s"))
		kf := editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)
		kf.X, kf.Y, kf.Z = 10, 10, float32(i)
	}
	return cw, p
}

// drag simulates a canvas drag from anim (x,y) by (dx,dy) anim px and
// reports which part, if any, was asked to move.
func drag(cw *CanvasWidget, x, y, dx, dy float32) (moved int) {
	moved = -1
	cw.OnPartDragStart = func(idx int) { moved = idx }
	origin := cw.originScreen()
	start := fyne.NewPos(origin.X+x*cw.zoom, origin.Y+y*cw.zoom)
	d := fyne.Delta{DX: dx * cw.zoom, DY: dy * cw.zoom}
	cw.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: start.Add(d)}, Dragged: d})
	cw.DragEnd()
	return moved
}

// Requested: parts are only interactable when selected, so dragging can't
// move a part by accident.
func TestDraggingAnUnselectedPartMovesNothing(t *testing.T) {
	cw, p := twoOverlappingParts(t)
	p.Selection.PartIndex = -1
	if got := drag(cw, 15, 15, 5, 0); got != -1 {
		t.Errorf("drag with nothing selected moved part %d", got)
	}
}

func TestDraggingTheSelectedPartMovesIt(t *testing.T) {
	cw, p := twoOverlappingParts(t)
	p.Selection.PartIndex = 1 // front
	if got := drag(cw, 15, 15, 5, 0); got != 1 {
		t.Errorf("drag on the selected part moved %d, want 1", got)
	}
}

// The selection decides, not what's on top: a covered part, once selected,
// can be dragged out from under the one in front.
func TestSelectedPartCanBeDraggedFromUnderAnother(t *testing.T) {
	cw, p := twoOverlappingParts(t)
	p.Selection.PartIndex = 0 // back, fully covered by front
	if got := drag(cw, 15, 15, 5, 0); got != 0 {
		t.Errorf("drag moved %d, want the selected back part (0)", got)
	}
}

// A drag starting outside the selected part moves nothing, even over
// another part.
func TestDragOutsideTheSelectedPartMovesNothing(t *testing.T) {
	cw, p := twoOverlappingParts(t)
	p.Selection.PartIndex = 1
	if got := drag(cw, 100, 100, 5, 0); got != -1 {
		t.Errorf("drag on empty canvas moved %d", got)
	}
}

// Clicking still selects (topmost part), which is how a part becomes
// draggable in the first place.
func TestClickStillSelectsTheTopmostPart(t *testing.T) {
	cw, _ := twoOverlappingParts(t)
	origin := cw.originScreen()
	if got := cw.hitTest(fyne.NewPos(origin.X+15*cw.zoom, origin.Y+15*cw.zoom)); got != 1 {
		t.Errorf("click selects %d, want the front part (1)", got)
	}
}
