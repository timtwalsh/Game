package ui

import (
	"animaker/pkg/editor"
	"testing"

	"fyne.io/fyne/v2"
)

func testCanvas(t *testing.T) (*CanvasWidget, *editor.Project) {
	t.Helper()
	p := editor.NewProject("test")
	return &CanvasWidget{project: p, zoom: 4.0, draggingPartIdx: -1}, p
}

// The view always contains the origin and the whole reference box, so 0,0
// and the character guide can never be scrolled out of existence.
func TestViewBoundsAlwaysContainsOriginAndRefBox(t *testing.T) {
	cw, p := testCanvas(t)
	minX, minY, maxX, maxY := cw.viewBounds()

	if minX > 0 || minY > 0 {
		t.Errorf("view min (%v,%v) excludes the origin", minX, minY)
	}
	if maxX < float32(p.CurrentTrack.RefBoxWidth) || maxY < float32(p.CurrentTrack.RefBoxHeight) {
		t.Errorf("view max (%v,%v) excludes the %dx%d reference box",
			maxX, maxY, p.CurrentTrack.RefBoxWidth, p.CurrentTrack.RefBoxHeight)
	}
}

// Art placed above/left of the origin is legitimate (a raised sword, a
// trailing cape) and the canvas must grow to include it rather than clip.
func TestViewBoundsGrowsToIncludeNegativeCoordinates(t *testing.T) {
	cw, p := testCanvas(t)
	_, _, beforeMaxX, _ := cw.viewBounds()

	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("Sword", "", "sword"))
	kf := editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)
	kf.X, kf.Y = -120, -90

	minX, minY, maxX, _ := cw.viewBounds()
	if minX > -120 {
		t.Errorf("view minX = %v, must reach the keyframe at x=-120", minX)
	}
	if minY > -90 {
		t.Errorf("view minY = %v, must reach the keyframe at y=-90", minY)
	}
	if maxX < beforeMaxX {
		t.Errorf("view shrank on the right (%v -> %v) when content was added to the left", beforeMaxX, maxX)
	}
}

// Scrubbing must never resize the canvas: the view is computed over every
// keyframe, not the current frame, so the origin can't shift underfoot.
func TestViewBoundsIsStableAcrossScrubbing(t *testing.T) {
	cw, p := testCanvas(t)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("Body", "", "body"))
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 0).X = 0
	editor.AddKeyframe(p.ActiveDirection(), part.ID, 500).X = 300

	minX, minY, maxX, maxY := cw.viewBounds()
	for _, ms := range []uint32{0, 125, 250, 375, 500} {
		p.Seek(ms)
		a, b, c, d := cw.viewBounds()
		if a != minX || b != minY || c != maxX || d != maxY {
			t.Fatalf("view changed at %dms: (%v,%v,%v,%v) != (%v,%v,%v,%v)",
				ms, a, b, c, d, minX, minY, maxX, maxY)
		}
	}
}

// A drop lands where it was released: converting a widget-local position to
// animation space and back must round-trip, including left of / above the
// origin where the coordinates go negative.
func TestLocalToAnimXYRoundTrips(t *testing.T) {
	cw, p := testCanvas(t)
	part := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("Body", "", "body"))
	kf := editor.AddKeyframe(p.ActiveDirection(), part.ID, 0)
	kf.X, kf.Y = -64, -64 // force negative space into the view

	origin := cw.originScreen()
	if origin.X <= 0 || origin.Y <= 0 {
		t.Fatalf("origin at %v should be inset once negative content exists", origin)
	}

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

// Even with nothing placed, the origin is inset by the margin rather than
// pinned to the widget's top-left, so there's always a little negative
// space visible to drop into before the canvas has to grow.
func TestOriginIsInsetByTheMarginWhenEmpty(t *testing.T) {
	cw, _ := testCanvas(t)
	origin := cw.originScreen()
	if origin.X <= 0 || origin.Y <= 0 {
		t.Errorf("origin = %v, want a positive inset leaving negative space visible", origin)
	}
	minX, minY, _, _ := cw.viewBounds()
	if want := -minX * cw.zoom; origin.X != want {
		t.Errorf("origin.X = %v, want %v (= -minX * zoom)", origin.X, want)
	}
	if want := -minY * cw.zoom; origin.Y != want {
		t.Errorf("origin.Y = %v, want %v (= -minY * zoom)", origin.Y, want)
	}
}

func TestFloorToCeilToRoundOutward(t *testing.T) {
	tests := []struct {
		v, q, wantFloor, wantCeil float32
	}{
		{0, 32, 0, 0},
		{1, 32, 0, 32},
		{32, 32, 32, 32},
		{33, 32, 32, 64},
		{-1, 32, -32, 0},
		{-32, 32, -32, -32},
		{-33, 32, -64, -32},
	}
	for _, tt := range tests {
		if got := floorTo(tt.v, tt.q); got != tt.wantFloor {
			t.Errorf("floorTo(%v, %v) = %v, want %v", tt.v, tt.q, got, tt.wantFloor)
		}
		if got := ceilTo(tt.v, tt.q); got != tt.wantCeil {
			t.Errorf("ceilTo(%v, %v) = %v, want %v", tt.v, tt.q, got, tt.wantCeil)
		}
	}
}
