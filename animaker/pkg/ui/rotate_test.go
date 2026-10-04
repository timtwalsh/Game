package ui

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

var red = color.RGBA{255, 0, 0, 255}

func TestDrawnRotationRoundsAndNormalises(t *testing.T) {
	for in, want := range map[float32]float32{
		0: 0, 90: 90, 90.2: 90, 90.3: 90.5, -90: 270, 360: 0, 359.9: 0, 725: 5,
	} {
		if got := drawnRotation(in); got != want {
			t.Errorf("drawnRotation(%v) = %v, want %v", in, got, want)
		}
	}
}

// Turning a cell a quarter turn moves each pixel exactly - nothing is
// smeared or lost - and the result covers editor.RotatedCellBox.
func TestRotateCellQuarterTurnIsExact(t *testing.T) {
	// 3x1: red, green, blue left to right, pivot at the left pixel's centre.
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.Set(0, 0, red)
	src.Set(1, 0, color.RGBA{0, 255, 0, 255})
	src.Set(2, 0, color.RGBA{0, 0, 255, 255})

	got := rotateCell(src, 0.5, 0.5, 90) // clockwise: left-to-right becomes top-to-bottom
	if b := got.Bounds(); b.Dx() != 1 || b.Dy() != 3 {
		t.Fatalf("rotated to %v, want 1x3", b)
	}
	for y, want := range []color.RGBA{red, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		if c := got.RGBAAt(0, y); c != want {
			t.Errorf("pixel (0,%d) = %v, want %v", y, c, want)
		}
	}
	minX, minY, maxX, maxY := editor.RotatedCellBox(3, 1, 0.5, 0.5, 90)
	if int(maxX-minX) != 1 || int(maxY-minY) != 3 {
		t.Errorf("RotatedCellBox %vx%v doesn't match the image", maxX-minX, maxY-minY)
	}
}

// barCanvas: a part showing an 8x2 red bar pivoted at its left end, so it
// points right unrotated and down at 90 degrees.
func barCanvas(t *testing.T) (*CanvasWidget, *editor.Project, *editor.Keyframe, fyne.Window) {
	t.Helper()
	test.NewTempApp(t)
	bar := image.NewRGBA(image.Rect(0, 0, 8, 2))
	draw.Draw(bar, bar.Bounds(), &image.Uniform{red}, image.Point{}, draw.Src)
	p := editor.NewProject("t")
	p.LoadedSheets["bar"] = editor.NewSpriteSheetTemplate("bar", "bar.png", bar, 8, 2, 0, 1)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("bar", "", "bar"))
	kf := editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)

	cw := NewCanvasWidget(p)
	cw.showGrid = false
	w := test.NewWindow(cw)
	t.Cleanup(w.Close)
	w.SetPadded(false)
	w.Resize(fyne.NewSize(300, 300))
	return cw, p, kf, w
}

// isRedAt reports whether animation point (x, y) is drawn red.
func isRedAt(cw *CanvasWidget, w fyne.Window, x, y float32) bool {
	o := cw.originScreen()
	c := w.Canvas().Capture().At(int(o.X+x*cw.zoom), int(o.Y+y*cw.zoom))
	r, g, b, _ := c.RGBA()
	return r > 0xc000 && g < 0x4000 && b < 0x4000
}

// Requested: rotation is previewed - the part is drawn turned about its
// pivot, and clicks find it where it's drawn.
func TestRotatedPartIsDrawnAndHitTurned(t *testing.T) {
	cw, _, kf, w := barCanvas(t)
	if !isRedAt(cw, w, 6, 0) || isRedAt(cw, w, 0.5, 6) {
		t.Fatal("unrotated, the bar should point right from its pivot")
	}

	kf.RotationDeg = 90
	cw.Refresh()
	if !isRedAt(cw, w, 0, 6) {
		t.Error("at 90 degrees the bar should point down from its pivot")
	}
	if isRedAt(cw, w, 6, 0) {
		t.Error("at 90 degrees the bar is still drawn pointing right")
	}

	o := cw.originScreen()
	at := func(x, y float32) fyne.Position { return fyne.NewPos(o.X+x*cw.zoom, o.Y+y*cw.zoom) }
	if got := cw.hitTest(at(0, 6)); got != 0 {
		t.Errorf("click on the turned bar hit %d, want 0", got)
	}
	if got := cw.hitTest(at(6, 0)); got != -1 {
		t.Errorf("click where the bar was before turning hit %d, want -1", got)
	}
}

// A rotation that plays reuses its image objects frame to frame instead
// of making new ones (each new one is a texture Fyne holds for a minute).
func TestPlayingRotationReusesItsImage(t *testing.T) {
	cw, p, kf, _ := barCanvas(t)
	end := editor.AddKeyframe(p.ActiveDirection(), p.CurrentTrack.Parts[0].ID, 1000)
	kf.RotationDeg, end.RotationDeg = 0, 180
	r := test.WidgetRenderer(cw).(*canvasRenderer)
	for ms := uint32(100); ms < 1000; ms += 100 {
		p.Seek(ms)
		cw.Refresh()
	}
	if n := len(r.rotImages); n != 1 {
		t.Errorf("%d rotated image objects after nine angles, want 1 reused", n)
	}
}
