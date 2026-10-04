package ui

import (
	"image"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// walkTorchCanvas: a parent with torch.anif nested at (10,20); the torch is
// one 16x16 flame (pivot 8,8) at its own (0,-6) at 0ms and (4,-6) at 300ms.
func walkTorchCanvas(t *testing.T) (*CanvasWidget, *editor.Project, *editor.Part) {
	t.Helper()
	cw, p := testCanvas(t)
	fire := editor.NewSpriteSheetTemplate("fire", "fire.png", image.NewRGBA(image.Rect(0, 0, 32, 16)), 16, 16, 8, 8)

	torch := editor.NewTrack("torch")
	flame := editor.AddPart(torch, editor.NewSheetPart("flame", "", "fire"))
	editor.AddKeyframe(torch.Directions[0], flame.ID, 0).Y = -6
	b := editor.AddKeyframe(torch.Directions[0], flame.ID, 300)
	b.X, b.Y, b.Col = 4, -6, 1
	anim := &editor.NestedAnim{Path: editor.AnimKey("torch.anif"), Track: torch,
		Sheets: map[string]*editor.SpriteSheetTemplate{"fire": fire}}
	p.LoadedAnims[anim.Path] = anim

	part := editor.AddPart(p.CurrentTrack, editor.NewNestedAniPart("torch_1", anim.Path))
	kf := editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)
	kf.X, kf.Y = 10, 20
	return cw, p, part
}

// Requested: render the nested animation live rather than a placeholder.
func TestNestedPartIsDrawnAsItsAnimation(t *testing.T) {
	cw, p, _ := walkTorchCanvas(t)
	r := &canvasRenderer{widget: cw}

	images := func() []*canvas.Image {
		var out []*canvas.Image
		for _, o := range r.buildObjects() {
			if img, ok := o.(*canvas.Image); ok {
				out = append(out, img)
			}
		}
		return out
	}
	origin := cw.originScreen()
	want := func(animX, animY float32) fyne.Position {
		return fyne.NewPos(origin.X+animX*cw.zoom, origin.Y+animY*cw.zoom)
	}

	imgs := images()
	if len(imgs) != 1 {
		t.Fatalf("%d images drawn, want the torch's one flame", len(imgs))
	}
	// Part at (10,20) + flame at (0,-6) - pivot (8,8) = (2,6).
	if got := imgs[0].Position(); got != want(2, 6) {
		t.Errorf("flame drawn at %v, want %v", got, want(2, 6))
	}

	before := cw.originScreen()
	p.Playback.NestedClockMs = 150 // halfway: x lerps to 2
	imgs = images()
	if got := imgs[0].Position(); got != want(4, 6) {
		t.Errorf("at 150ms flame drawn at %v, want %v", got, want(4, 6))
	}
	if cw.originScreen() != before {
		t.Error("the origin moved as the nested animation played")
	}
}

func TestClickingANestedSpriteSelectsItsPart(t *testing.T) {
	cw, _, _ := walkTorchCanvas(t)
	origin := cw.originScreen()
	// Inside the flame: anim (2..18, 6..22) at time 0.
	at := fyne.NewPos(origin.X+5*cw.zoom, origin.Y+10*cw.zoom)
	if got := cw.hitTest(at); got != 0 {
		t.Errorf("hitTest on the flame = %d, want part 0 (the torch)", got)
	}
}

// A nested part's rotation turns its whole animation about the part: at
// 180 the flame, (0,-6) above the torch's origin, hangs (0,6) below it,
// and that's where it's drawn and clicked.
func TestRotatedNestedPartTurnsItsAnimation(t *testing.T) {
	cw, p, part := walkTorchCanvas(t)
	p.ActiveDirection().KeyframesFor(part.ID)[0].RotationDeg = 180
	origin := cw.originScreen()
	at := func(x, y float32) fyne.Position { return fyne.NewPos(origin.X+x*cw.zoom, origin.Y+y*cw.zoom) }

	// The flame's 16x16 cell, turned about its pivot at (10,26): (2,18) to (18,34).
	var flame *canvas.Image
	for _, o := range (&canvasRenderer{widget: cw}).buildObjects() {
		if img, ok := o.(*canvas.Image); ok {
			flame = img
		}
	}
	if flame == nil || flame.Position() != at(2, 18) || flame.Size() != fyne.NewSize(16*cw.zoom, 16*cw.zoom) {
		t.Fatalf("flame drawn at %v size %v, want %v size 16x16 anim px", flame.Position(), flame.Size(), at(2, 18))
	}
	if got := cw.hitTest(at(10, 30)); got != 0 {
		t.Errorf("click on the turned flame hit %d, want 0", got)
	}
	if got := cw.hitTest(at(10, 8)); got != -1 {
		t.Errorf("click where the flame was before turning hit %d, want -1", got)
	}
}
