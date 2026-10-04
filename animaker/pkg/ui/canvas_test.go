package ui

import (
	"animaker/pkg/editor"
	"math"
	"testing"

	"fyne.io/fyne/v2"
)

func testCanvas(t *testing.T) (*CanvasWidget, *editor.Project) {
	t.Helper()
	p := editor.NewProject("test")
	return &CanvasWidget{project: p, zoom: 4.0, draggingPartIdx: -1}, p
}

// The view centres on the origin and the whole reference box even with
// nothing placed, so a new track opens with 0,0 and the character guide in
// sight.
func TestContentBoundsContainOriginAndRefBox(t *testing.T) {
	cw, p := testCanvas(t)
	minX, minY, maxX, maxY := cw.contentBounds(cw.partExtents())

	if minX > 0 || minY > 0 {
		t.Errorf("content min (%v,%v) excludes the origin", minX, minY)
	}
	if maxX < float32(p.CurrentTrack.RefBoxWidth) || maxY < float32(p.CurrentTrack.RefBoxHeight) {
		t.Errorf("content max (%v,%v) excludes the %dx%d reference box",
			maxX, maxY, p.CurrentTrack.RefBoxWidth, p.CurrentTrack.RefBoxHeight)
	}
}

// Art above/left of the origin (a raised sword, a trailing cape) is part
// of what the view centres on, from every keyframe and not just the
// current frame.
func TestContentBoundsIncludeNegativeAndOffFrameKeyframes(t *testing.T) {
	cw, p := testCanvas(t)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("Sword", "", "sword"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0).X = 0
	kf := editor.AddKeyframe(p.ActiveDirection(), part.ID, 500)
	kf.X, kf.Y = -120, -90

	p.Seek(0)
	minX, minY, _, _ := cw.contentBounds(cw.partExtents())
	if minX > -120 || minY > -90 {
		t.Errorf("content min (%v,%v), must reach the keyframe at (-120,-90)", minX, minY)
	}
}

// Centring puts the middle of the content in the middle of the canvas.
func TestCenterViewCentresTheContent(t *testing.T) {
	cw, _ := testCanvas(t)
	size := fyne.NewSize(400, 300)
	cw.centerView(size)

	minX, minY, maxX, maxY := cw.contentBounds(cw.partExtents())
	x, y := cw.LocalToAnimXY(fyne.NewPos(size.Width/2, size.Height/2))
	if x != (minX+maxX)/2 || y != (minY+maxY)/2 {
		t.Errorf("canvas centre shows (%v,%v), want the content centre (%v,%v)",
			x, y, (minX+maxX)/2, (minY+maxY)/2)
	}
}

// Resizing the canvas (the window, or a split) keeps what was in the
// middle in the middle rather than sliding the art toward a corner.
func TestResizeKeepsTheViewCentre(t *testing.T) {
	cw, _ := testCanvas(t)
	cw.layoutView(fyne.NewSize(400, 300))
	cw.panBy(fyne.NewDelta(37, -12)) // anywhere but the content centre
	x, y := cw.LocalToAnimXY(fyne.NewPos(200, 150))

	cw.layoutView(fyne.NewSize(700, 500))
	if gx, gy := cw.LocalToAnimXY(fyne.NewPos(350, 250)); gx != x || gy != y {
		t.Errorf("after resize the centre shows (%v,%v), want (%v,%v)", gx, gy, x, y)
	}
}

// A drop lands where it was released: converting a widget-local position to
// animation space and back must round-trip, including left of / above the
// origin where the coordinates go negative.
func TestLocalToAnimXYRoundTrips(t *testing.T) {
	cw, _ := testCanvas(t)
	cw.panBy(fyne.NewDelta(150, 90))
	origin := cw.originScreen()

	for _, want := range []fyne.Position{
		fyne.NewPos(0, 0),
		fyne.NewPos(10, 20),
		fyne.NewPos(-32, -48),
	} {
		local := fyne.NewPos(origin.X+want.X*cw.zoom, origin.Y+want.Y*cw.zoom)
		gotX, gotY := cw.LocalToAnimXY(local)
		if gotX != want.X || gotY != want.Y {
			t.Errorf("round trip of (%v,%v) gave (%v,%v)", want.X, want.Y, gotX, gotY)
		}
	}
}

func wheel(cw *CanvasWidget, at fyne.Position, dy float32) {
	cw.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: at}, Scrolled: fyne.NewDelta(0, dy)})
}

// Requested: wheel up zooms in, wheel down zooms out - about the cursor,
// so the point under it stays put.
func TestWheelZoomsAboutTheCursor(t *testing.T) {
	cw, _ := testCanvas(t)
	cw.panBy(fyne.NewDelta(100, 80))
	at := fyne.NewPos(230, 170)
	x, y := cw.LocalToAnimXY(at)

	wheel(cw, at, 10)
	if want := float32(4.0 * wheelZoomStep); cw.zoom != want {
		t.Errorf("wheel up: zoom %v, want %v", cw.zoom, want)
	}
	if gx, gy := cw.LocalToAnimXY(at); math.Abs(float64(gx-x)) > 1e-3 || math.Abs(float64(gy-y)) > 1e-3 {
		t.Errorf("wheel up moved the point under the cursor: (%v,%v) -> (%v,%v)", x, y, gx, gy)
	}

	wheel(cw, at, -10)
	wheel(cw, at, -10)
	if want := float32(4.0 / wheelZoomStep); math.Abs(float64(cw.zoom-want)) > 1e-4 {
		t.Errorf("wheel down twice: zoom %v, want %v", cw.zoom, want)
	}
	if gx, gy := cw.LocalToAnimXY(at); math.Abs(float64(gx-x)) > 1e-3 || math.Abs(float64(gy-y)) > 1e-3 {
		t.Errorf("wheel down moved the point under the cursor: (%v,%v) -> (%v,%v)", x, y, gx, gy)
	}
}

// Requested: zoom stops at 500% in and 10% out, however far the wheel
// turns or whatever the menu asks for.
func TestZoomIsClampedTo10And500Percent(t *testing.T) {
	cw, _ := testCanvas(t)
	at := fyne.NewPos(50, 50)
	for range 40 {
		wheel(cw, at, 10)
	}
	if cw.zoom != maxZoom || maxZoom != 5.0 {
		t.Errorf("zoomed all the way in: %v, want 5 (500%%)", cw.zoom)
	}
	for range 80 {
		wheel(cw, at, -10)
	}
	if cw.zoom != minZoom || minZoom != float32(0.1) {
		t.Errorf("zoomed all the way out: %v, want 0.1 (10%%)", cw.zoom)
	}
	cw.SetZoom(8)
	if cw.zoom != maxZoom {
		t.Errorf("SetZoom(8): %v, want clamped to %v", cw.zoom, maxZoom)
	}
}

func TestFloorToRoundsDown(t *testing.T) {
	tests := []struct{ v, q, want float32 }{
		{0, 32, 0},
		{1, 32, 0},
		{32, 32, 32},
		{33, 32, 32},
		{-1, 32, -32},
		{-32, 32, -32},
		{-33, 32, -64},
	}
	for _, tt := range tests {
		if got := floorTo(tt.v, tt.q); got != tt.want {
			t.Errorf("floorTo(%v, %v) = %v, want %v", tt.v, tt.q, got, tt.want)
		}
	}
}

// The rotation tick points up at 0 and turns clockwise on screen, the way
// the game's renderer rotates.
func TestRotationTickEndTurnsClockwise(t *testing.T) {
	p := fyne.NewPos(100, 100)
	near := func(a, b fyne.Position) bool {
		return math.Abs(float64(a.X-b.X)) < 0.01 && math.Abs(float64(a.Y-b.Y)) < 0.01
	}
	for deg, want := range map[float32]fyne.Position{
		0: {X: 100, Y: 90}, 90: {X: 110, Y: 100}, 180: {X: 100, Y: 110}, -90: {X: 90, Y: 100},
	} {
		if got := rotationTickEnd(p, 10, deg); !near(got, want) {
			t.Errorf("%v deg: end %v, want %v", deg, got, want)
		}
	}
}
