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
	w.Resize(cw.MinSize())

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
