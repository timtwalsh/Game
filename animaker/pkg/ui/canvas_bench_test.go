package ui

import (
	"fmt"
	"image"
	"testing"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2/test"
)

// benchProject is a rig of 10 sheet parts, each keyed 8 times, plus two
// nested parts playing a 4-part animation - a mid-sized character.
func benchProject() *editor.Project {
	p := editor.NewProject("bench")
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	p.LoadedSheets["s"] = editor.NewSpriteSheetTemplate("s", "s.png", img, 32, 32, 16, 16)
	dir := p.ActiveDirection()
	for i := 0; i < 10; i++ {
		part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart(fmt.Sprintf("p%d", i), "", "s"))
		for k := 0; k < 8; k++ {
			kf := editor.AddKeyframe(dir, part.ID, uint32(k*100))
			kf.X, kf.Y, kf.Z = float32(i*3+k), float32(k), float32(i)
		}
	}
	child := editor.NewTrack("child")
	for i := 0; i < 4; i++ {
		part := editor.AddPart(child, editor.NewSheetPart(fmt.Sprintf("c%d", i), "", "s"))
		for k := 0; k < 6; k++ {
			editor.AddKeyframe(child.Directions[0], part.ID, uint32(k*80)).X = float32(k)
		}
	}
	path := editor.AnimKey("child.anif")
	p.LoadedAnims[path] = &editor.NestedAnim{Path: path, Track: child, Sheets: p.LoadedSheets}
	for i := 0; i < 2; i++ {
		part := editor.AddPart(p.CurrentTrack, editor.NewNestedAniPart(fmt.Sprintf("n%d", i), path))
		editor.AddKeyframe(dir, part.ID, 0).Z = float32(20 + i)
	}
	return p
}

// BenchmarkCanvasFrame is one playback frame of the canvas: advance and
// rebuild everything drawn.
func BenchmarkCanvasFrame(b *testing.B) {
	test.NewTempApp(b)
	p := benchProject()
	cw := NewCanvasWidget(p)
	r := cw.CreateRenderer().(*canvasRenderer)
	p.Playback.IsPlaying = true
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.AdvancePlayback(16)
		r.Refresh()
	}
}
