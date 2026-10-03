package ui

import (
	"animaker/pkg/editor"
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func mouseAt(x, y float32) *desktop.MouseEvent {
	return &desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)}}
}

// Requested: hover states on the clickable parts of the UI.

func TestPaletteHighlightsTheTileUnderTheMouse(t *testing.T) {
	test.NewTempApp(t)
	g := NewSheetGridWidget()
	g.SetSheet(editor.NewSpriteSheetTemplate("s", "s.png", image.NewRGBA(image.Rect(0, 0, 64, 32)), 32, 32, 0, 0))
	w := test.NewWindow(g)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(128, 80))
	cell := 32 * g.displayScale()

	g.MouseIn(mouseAt(cell+3, 3)) // second tile
	if !g.hover.on || g.hover.rect.Position() != fyne.NewPos(cell, 0) || g.Cursor() != desktop.PointerCursor {
		t.Errorf("hover on %v at %v, cursor %v; want on the second tile with a hand", g.hover.on, g.hover.rect.Position(), g.Cursor())
	}
	g.MouseMoved(mouseAt(3, cell+3)) // below the sheet
	if g.hover.on {
		t.Error("highlight shown below the tiles")
	}
	g.MouseMoved(mouseAt(3, 3))
	g.MouseOut()
	if g.hover.on {
		t.Error("highlight left on after the mouse went out")
	}
}

func TestCanvasOutlinesThePartUnderTheMouse(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	p.LoadedSheets["s"] = editor.NewSpriteSheetTemplate("s", "s.png", image.NewRGBA(image.Rect(0, 0, 8, 8)), 8, 8, 0, 0)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("box", "", "s"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)
	cw := NewCanvasWidget(p)
	cw.SetZoom(2)
	w := test.NewWindow(cw)
	t.Cleanup(w.Close)

	o := cw.originScreen()
	cw.MouseMoved(mouseAt(o.X+4, o.Y+4))
	if cw.hoverPartIdx != 0 || !cw.hoverOutline().on || cw.Cursor() != desktop.PointerCursor {
		t.Errorf("over the part: hover %d (shown %v), cursor %v", cw.hoverPartIdx, cw.hoverOutline().on, cw.Cursor())
	}
	cw.MouseMoved(mouseAt(o.X+40, o.Y+40))
	if cw.hoverPartIdx != -1 || cw.hoverOutline().on || cw.Cursor() != desktop.DefaultCursor {
		t.Error("hover kept over empty canvas")
	}
}

func TestTimelineMarkerAndRowControlsReactToHover(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("body", "", "s"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 200)
	s := newScrubArea(p)
	w := test.NewWindow(s)
	t.Cleanup(w.Close)

	rowY := float32(timelineRulerHeight + timelineRowHeight/2)
	s.MouseMoved(mouseAt(s.xForTime(200), rowY))
	if s.hoverPart != 0 || s.hoverKf != 0 || s.Cursor() != desktop.HResizeCursor {
		t.Errorf("over the marker: hover (%d,%d), cursor %v; want (0,0) and a resize cursor", s.hoverPart, s.hoverKf, s.Cursor())
	}
	s.timingLocked = true
	if s.Cursor() != desktop.PointerCursor {
		t.Error("timing locked: want a hand, since a drag scrubs")
	}
	s.MouseOut()
	if s.hoverPart != -1 {
		t.Error("marker still hovered after the mouse left")
	}

	label := s.rowLabel(0)
	label.MouseIn(mouseAt(1, 1))
	if label.bg.FillColor != ColorHoverFill {
		t.Error("part name doesn't light up under the mouse")
	}
	label.MouseOut()
	if label.bg.FillColor != color.Transparent {
		t.Error("part name stays lit after the mouse left")
	}
	del := s.rowDelete(0)
	del.MouseIn(mouseAt(1, 1))
	if del.bg.FillColor != ColorDeleteHover || del.Cursor() != desktop.PointerCursor {
		t.Error("delete x doesn't turn red with a hand cursor under the mouse")
	}
}
