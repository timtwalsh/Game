package app

import (
	"animaker/pkg/editor"
	"image"
	"testing"

	"fyne.io/fyne/v2"
)

// builtApp is the real editor UI in a test window, with a two-part rig
// posed in direction 0: body keyed at 0 and 200ms, head at 100ms.
func builtApp(t *testing.T) (*Application, *editor.Part) {
	t.Helper()
	a := testApp(t)
	a.build()
	track := a.Project.CurrentTrack
	dir := a.Project.ActiveDirection()
	body := editor.AddPart(track, editor.NewSheetPart("body", "", "sheet"))
	head := editor.AddPart(track, editor.NewSheetPart("head", "", "sheet"))
	editor.AddKeyframe(dir, body.ID, 0).Z = 1
	editor.AddKeyframe(dir, body.ID, 200).Z = 1
	editor.AddKeyframe(dir, head.ID, 100).Z = 5
	a.refreshAll()
	return a, body
}

func typeKey(a *Application, k fyne.KeyName) { a.onTypedKey(&fyne.KeyEvent{Name: k}) }

func TestKeysStepAndNudge(t *testing.T) {
	a, body := builtApp(t)
	dir := a.Project.ActiveDirection()

	typeKey(a, fyne.KeyPeriod)
	if a.Project.Playback.ElapsedMs != 100 {
		t.Fatalf(". went to %dms, want the next keyframe at 100ms", a.Project.Playback.ElapsedMs)
	}

	// Nothing selected: arrows do nothing.
	typeKey(a, fyne.KeyRight)
	if a.Project.Dirty {
		t.Error("arrow with nothing selected changed the track")
	}

	a.properties.SelectPart(0)
	typeKey(a, fyne.KeyRight)
	typeKey(a, fyne.KeyUp)
	kfs := dir.KeyframesFor(body.ID)
	if len(kfs) != 3 || kfs[1].TimeMs != 100 || kfs[1].X != 1 || kfs[1].Y != -1 {
		t.Fatalf("body keyframes %d; want a new one at 100ms nudged to (1,-1)", len(kfs))
	}
	a.onUndo()
	a.onUndo()
	if got := len(a.Project.ActiveDirection().KeyframesFor(body.ID)); got != 2 {
		t.Errorf("two undos left %d keyframes, want the original 2 (one undo per nudge)", got)
	}
}

func TestDeleteKeyRemovesTheKeyframeAtThePlayhead(t *testing.T) {
	a, body := builtApp(t)
	a.properties.SelectPart(0)
	a.Project.Scrub(200)
	typeKey(a, fyne.KeyDelete)
	if got := len(a.Project.ActiveDirection().KeyframesFor(body.ID)); got != 1 {
		t.Errorf("body has %d keyframes after Delete at 200ms, want 1", got)
	}
}

func TestSpaceAndNumberKeys(t *testing.T) {
	a, _ := builtApp(t)
	typeKey(a, fyne.KeySpace)
	if !a.Project.Playback.IsPlaying {
		t.Error("Space didn't play")
	}
	typeKey(a, fyne.KeySpace)
	if a.Project.Playback.IsPlaying {
		t.Error("second Space didn't pause")
	}

	editor.AddStandardDirections(a.Project.CurrentTrack)
	typeKey(a, fyne.Key3)
	if a.Project.Playback.ActiveDirection != 2 {
		t.Errorf("3 switched to direction %d, want 2 (down)", a.Project.Playback.ActiveDirection)
	}
}

func TestZOrderShortcuts(t *testing.T) {
	a, _ := builtApp(t)
	a.properties.SelectPart(0)
	a.Project.Seek(100)
	a.onZOrder(true, true)
	kf := a.Project.EditTarget()
	if kf.TimeMs != 100 || kf.Z != 6 {
		t.Errorf("bring to front: keyframe at %dms Z %v, want Z 6 at 100ms (in front of head's 5)", kf.TimeMs, kf.Z)
	}
}

// A drop that adds a part asks for its name; one that keys the selected
// part doesn't.
func TestDropNewPartAsksForItsName(t *testing.T) {
	a, _ := builtApp(t)
	a.Project.LoadedSheets["sheet"] = editor.NewSpriteSheetTemplate("sheet", "s.png", image.NewRGBA(image.Rect(0, 0, 8, 8)), 8, 8, 0, 0)
	a.Window.Resize(fyne.NewSize(1200, 800))
	inCanvas := fyne.CurrentApp().Driver().AbsolutePositionForObject(a.canvasWidget).Add(fyne.NewPos(5, 5))
	overlays := a.Window.Canvas().Overlays()

	a.properties.SelectPart(-1)
	a.onTileDropped("sheet", 0, 0, inCanvas)
	if len(a.Project.CurrentTrack.Parts) != 3 {
		t.Fatalf("%d parts after the drop, want 3", len(a.Project.CurrentTrack.Parts))
	}
	top := overlays.Top()
	if top == nil {
		t.Fatal("no rename dialog after a drop made a new part")
	}
	overlays.Remove(top)

	a.onTileDropped("sheet", 0, 1, inCanvas) // the new part is selected: keys it
	if overlays.Top() != nil {
		t.Error("rename dialog after a drop that only keyed the selected part")
	}
}
