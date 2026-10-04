package app

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// In the real layout the canvas fills its panel, opens with the origin in
// view, and gets the wheel itself - the scroll container around it only
// clips, so it mustn't consume the wheel first.
func TestCanvasInTheEditorOpensCentredAndZoomsOnTheWheel(t *testing.T) {
	a, _ := builtApp(t)
	a.Window.Resize(fyne.NewSize(1400, 900))

	size := a.canvasWidget.Size()
	if size.Width < 200 || size.Height < 200 {
		t.Fatalf("canvas is %v; it should fill its panel", size)
	}
	// The rig's sheet isn't loaded, so its parts have no size and the
	// content is just the reference box: the middle of the canvas shows
	// the middle of the box.
	local := fyne.NewPos(size.Width/2, size.Height/2)
	track := a.Project.CurrentTrack
	x, y := a.canvasWidget.LocalToAnimXY(local)
	if wx, wy := float32(track.RefBoxWidth)/2, float32(track.RefBoxHeight)/2; x != wx || y != wy {
		t.Errorf("canvas centre shows (%v,%v), want the reference box's centre (%v,%v)", x, y, wx, wy)
	}

	abs := fyne.CurrentApp().Driver().AbsolutePositionForObject(a.canvasWidget)
	before := a.canvasWidget.Zoom()
	test.Scroll(a.Window.Canvas(), abs.Add(local), 0, 10)
	if a.canvasWidget.Zoom() <= before {
		t.Errorf("wheel up over the canvas: zoom %v, want more than %v", a.canvasWidget.Zoom(), before)
	}
	test.Scroll(a.Window.Canvas(), abs.Add(local), 0, -10)
	test.Scroll(a.Window.Canvas(), abs.Add(local), 0, -10)
	if a.canvasWidget.Zoom() >= before {
		t.Errorf("wheel down over the canvas: zoom %v, want less than %v", a.canvasWidget.Zoom(), before)
	}
}
