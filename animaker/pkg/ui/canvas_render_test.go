package ui

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// The canvas reuses its image objects between redraws; a sprite must
// still be drawn where the animation has moved it, and not where it was.
func TestReusedSpriteImagesFollowTheAnimation(t *testing.T) {
	test.NewTempApp(t)
	red := image.NewRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(red, red.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)
	p := editor.NewProject("t")
	p.LoadedSheets["r"] = editor.NewSpriteSheetTemplate("r", "r.png", red, 8, 8, 0, 0)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("box", "", "r"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0).X = 0
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 100).X = 40

	cw := NewCanvasWidget(p)
	cw.SetZoom(1)
	cw.showGrid = false
	w := test.NewWindow(cw)
	t.Cleanup(w.Close)
	w.SetPadded(false)
	w.Resize(fyne.NewSize(200, 200))

	isRed := func(local fyne.Position) bool {
		origin := cw.originScreen()
		px := w.Canvas().Capture().At(int(origin.X+local.X), int(origin.Y+local.Y))
		r, g, b, _ := px.RGBA()
		return r > 0xc000 && g < 0x4000 && b < 0x4000
	}
	at := func(x float32) fyne.Position { return fyne.NewPos(x+4, 4) }

	p.Seek(0)
	cw.Refresh()
	if !isRed(at(0)) {
		t.Fatal("sprite not drawn at x=0")
	}
	p.Seek(100)
	cw.Refresh()
	if !isRed(at(40)) {
		t.Error("sprite not drawn at x=40 after the playhead moved")
	}
	if isRed(at(0)) {
		t.Error("sprite still drawn at its old place")
	}
}

// With the onion skin on, the selected part's neighbouring keyframe poses
// draw faintly behind it; off, or while playing, they don't.
func TestOnionSkinDrawsNeighbouringPoses(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	p.LoadedSheets["r"] = editor.NewSpriteSheetTemplate("r", "r.png", image.NewRGBA(image.Rect(0, 0, 8, 8)), 8, 8, 0, 0)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("box", "", "r"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0).X = 0
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 200).X = 40
	p.Selection.PartIndex = 0
	p.Seek(100)

	cw := NewCanvasWidget(p)
	w := test.NewWindow(cw)
	t.Cleanup(w.Close)
	ghosts := func() (n int) {
		for _, o := range test.WidgetRenderer(cw).Objects() {
			if img, ok := o.(*canvas.Image); ok && img.Translucency == onionTranslucency {
				n++
			}
		}
		return n
	}

	if ghosts() != 0 {
		t.Error("ghosts drawn with the onion skin off")
	}
	cw.ToggleOnion()
	if got := ghosts(); got != 2 {
		t.Errorf("%d ghosts, want 2 (the keyframes at 0 and 200ms)", got)
	}
	p.Playback.IsPlaying = true
	cw.Refresh()
	if got := ghosts(); got != 0 {
		t.Errorf("%d ghosts while playing, want 0", got)
	}
}

// A palette tile dragged over the canvas shows faintly where it would
// land, placed by its pivot; clearing the preview removes it.
func TestDropPreviewShowsWhereATileWouldLand(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	sheet := editor.NewSpriteSheetTemplate("r", "r.png", image.NewRGBA(image.Rect(0, 0, 16, 8)), 8, 8, 4, 8)
	cw := NewCanvasWidget(p)
	cw.SetZoom(2)
	w := test.NewWindow(cw)
	t.Cleanup(w.Close)
	preview := func() *canvas.Image {
		for _, o := range test.WidgetRenderer(cw).Objects() {
			if img, ok := o.(*canvas.Image); ok && img.Translucency == dropPreviewTranslucency {
				return img
			}
		}
		return nil
	}

	cw.SetDropPreview(sheet, 0, 1, 10, 20, 0)
	img := preview()
	if img == nil {
		t.Fatal("no preview drawn")
	}
	origin := cw.originScreen()
	if want := fyne.NewPos(origin.X+(10-4)*2, origin.Y+(20-8)*2); img.Position() != want {
		t.Errorf("preview at %v, want %v (pivot on the drop point)", img.Position(), want)
	}
	cw.ClearDropPreview()
	if preview() != nil {
		t.Error("preview still drawn after ClearDropPreview")
	}
}
