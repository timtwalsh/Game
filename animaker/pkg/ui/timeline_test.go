package ui

import (
	"animaker/pkg/editor"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// Ticks must stay legible at every zoom: never closer than the minimum
// spacing, and never coarser than they need to be.
func TestNiceTickStepRespectsMinimumSpacing(t *testing.T) {
	for _, msPerPx := range []float32{timelineMinMsPerPx, 0.5, 1, 2, 5, 13, timelineMaxMsPerPx} {
		step := niceTickStep(msPerPx, timelineMinTickPx, 0)
		if px := float32(step) / msPerPx; px < timelineMinTickPx {
			t.Errorf("at %v ms/px, tick step %dms is %vpx apart, want >= %v", msPerPx, step, px, timelineMinTickPx)
		}
		// The next finer step must have been too tight, or it would have
		// been chosen instead.
		for i, s := range timelineTickSteps {
			if s == step && i > 0 {
				if finer := float32(timelineTickSteps[i-1]) / msPerPx; finer >= timelineMinTickPx {
					t.Errorf("at %v ms/px, chose %dms but %dms (%vpx) also fits", msPerPx, step, timelineTickSteps[i-1], finer)
				}
			}
		}
	}
}

// Every label must sit on a tick, at every zoom level.
func TestLabelStepIsAMultipleOfTickStep(t *testing.T) {
	for _, msPerPx := range []float32{timelineMinMsPerPx, 0.3, 0.75, 1, 2, 3, 7, 20, timelineMaxMsPerPx} {
		tick := niceTickStep(msPerPx, timelineMinTickPx, 0)
		label := niceTickStep(msPerPx, timelineMinLabelPx, tick)
		if label%tick != 0 {
			t.Errorf("at %v ms/px, label step %dms is not a multiple of tick step %dms", msPerPx, label, tick)
		}
		if px := float32(label) / msPerPx; px < timelineMinLabelPx && label != timelineTickSteps[len(timelineTickSteps)-1] {
			t.Errorf("at %v ms/px, labels %vpx apart, want >= %v", msPerPx, px, timelineMinLabelPx)
		}
	}
}

func TestClampMsPerPx(t *testing.T) {
	if got := clampMsPerPx(0.001); got != timelineMinMsPerPx {
		t.Errorf("clamp below range = %v, want %v", got, timelineMinMsPerPx)
	}
	if got := clampMsPerPx(1e6); got != timelineMaxMsPerPx {
		t.Errorf("clamp above range = %v, want %v", got, timelineMaxMsPerPx)
	}
	if got := clampMsPerPx(3); got != 3 {
		t.Errorf("clamp in range = %v, want 3", got)
	}
}

// Zooming keeps whatever time is under the cursor under the cursor. This
// is the whole point of anchoring; without it the view slides to the left
// edge on every zoom step.
func TestZoomAnchorOffsetKeepsTheAnchoredTimeOnScreen(t *testing.T) {
	cases := []struct {
		name                     string
		contentX, offsetX, oldMs float32
		newMs                    float32
	}{
		{"zoom in mid-view", 520, 100, 2, 2 / timelineZoomStep},
		{"zoom out mid-view", 520, 100, 2, 2 * timelineZoomStep},
		{"zoom in near the right of a scrolled view", 2400, 1800, 4, 1},
		{"big jump", 700, 0, timelineMaxMsPerPx, timelineMinMsPerPx},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			viewportX := c.contentX - c.offsetX
			timeBefore := (c.contentX - timelineLabelWidth) * c.oldMs

			newOffset := zoomAnchorOffset(c.contentX, c.offsetX, c.oldMs, c.newMs)
			timeAfter := (newOffset + viewportX - timelineLabelWidth) * c.newMs

			if math.Abs(float64(timeAfter-timeBefore)) > 0.01 {
				t.Errorf("time under the cursor moved from %vms to %vms", timeBefore, timeAfter)
			}
		})
	}
}

// An anchor in the label column (left of time 0) pins time 0 rather than
// extrapolating to negative time.
func TestZoomAnchorOffsetInTheLabelColumnPinsTimeZero(t *testing.T) {
	off := zoomAnchorOffset(50, 0, 2, 1)
	// Time 0 sits at timelineLabelWidth in content space; with the anchor
	// at viewport x 50, offset = labelWidth - 50.
	if want := float32(timelineLabelWidth - 50); off != want {
		t.Errorf("offset = %v, want %v", off, want)
	}
}

// Coordinate conversion has to round-trip at every zoom, or a dragged
// marker would land somewhere other than where it was dropped.
func TestTimeForXRoundTripsAtEveryZoom(t *testing.T) {
	s := &scrubArea{project: editor.NewProject("test")}
	for _, msPerPx := range []float32{timelineMinMsPerPx, 0.5, 1, 2, 3.2, 10, timelineMaxMsPerPx} {
		s.msPerPixel = msPerPx
		for _, ms := range []uint32{0, 1, 17, 250, 999, 4321} {
			got := s.timeForX(s.xForTime(ms))
			// Zoomed out, one pixel covers several ms; round-trip is exact
			// only to within half a pixel's worth of time.
			tol := uint32(msPerPx/2) + 1
			diff := int64(got) - int64(ms)
			if diff < 0 {
				diff = -diff
			}
			if uint32(diff) > tol {
				t.Errorf("at %v ms/px, %dms -> x -> %dms (off by %d, tolerance %d)", msPerPx, ms, got, diff, tol)
			}
		}
	}
}

func TestFormatTimelineMs(t *testing.T) {
	cases := map[uint32]string{0: "0ms", 250: "250ms", 1000: "1s", 1500: "1500ms", 3000: "3s"}
	for in, want := range cases {
		if got := formatTimelineMs(in); got != want {
			t.Errorf("formatTimelineMs(%d) = %q, want %q", in, got, want)
		}
	}
}

// Requested: a delete x at the left of each timeline row, beside (not
// over) the double-click-to-rename name.
func TestTimelineRowHasDeleteBesideName(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	editor.AddPart(p.CurrentTrack, editor.NewSheetPart("head", "", "sheet"))
	editor.AddPart(p.CurrentTrack, editor.NewSheetPart("legs", "", "sheet"))

	s := newScrubArea(p)
	deleted := -1
	s.OnPartDelete = func(idx int) { deleted = idx }
	w := test.NewWindow(s)
	defer w.Close()
	w.Resize(fyne.NewSize(600, 200))
	s.Refresh()

	del, label := s.rowDelete(1), s.rowLabel(1)
	if del.Position().X != 0 || del.Size().Width != timelineDeleteWidth {
		t.Errorf("delete at x=%v width %v, want x=0 width %v", del.Position().X, del.Size().Width, timelineDeleteWidth)
	}
	if label.Position().X < del.Position().X+del.Size().Width {
		t.Errorf("name starts at x=%v, overlapping the delete button", label.Position().X)
	}
	if label.Position().Y != del.Position().Y {
		t.Errorf("delete (y=%v) and name (y=%v) aren't on the same row", del.Position().Y, label.Position().Y)
	}

	test.Tap(del)
	if deleted != 1 {
		t.Errorf("tapping row 1's x asked to delete %d, want 1", deleted)
	}
}

// Switching project shows the new project's loop and speed, not the old.
func TestTimelineShowsNewProjectsPlaybackSettings(t *testing.T) {
	test.NewTempApp(t)
	tw := NewTimelineWidget(editor.NewProject("a"))
	w := test.NewWindow(tw.Build())
	t.Cleanup(w.Close)

	p := editor.NewProject("b")
	p.Playback.LoopEnabled, p.Playback.SpeedFactor = false, 2
	tw.SetProject(p)
	if tw.loopCheck.Checked || tw.speed.Selected != "200%" {
		t.Errorf("loop=%v speed=%q, want off and 200%%", tw.loopCheck.Checked, tw.speed.Selected)
	}
	if p.Playback.LoopEnabled || p.Playback.SpeedFactor != 2 {
		t.Error("syncing the widgets changed the project's settings")
	}
}

// Dragging a marker snaps onto another part's keyframe when close, else
// onto the ruler's tick grid; a click near a keyframe scrubs exactly to it.
func TestTimelineDragsSnap(t *testing.T) {
	test.NewTempApp(t)
	p := editor.NewProject("t")
	dir := p.ActiveDirection()
	body := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("body", "", "s"))
	head := editor.AddPart(p.CurrentTrack, editor.NewSheetPart("head", "", "s"))
	editor.AddKeyframe(dir, body.ID, 0)
	editor.AddKeyframe(dir, body.ID, 200)
	editor.AddKeyframe(dir, head.ID, 600)

	s := newScrubArea(p) // 2ms per pixel: snaps within 12ms, 20ms tick grid
	var got []uint32
	s.OnRetime = func(_ int, _ *editor.Keyframe, ms uint32) { got = append(got, ms) }

	// Grab body's 200ms marker and drag it to ~593ms, then to ~331ms.
	rowY := float32(timelineRulerHeight + timelineRowHeight/2)
	start := fyne.NewPos(s.xForTime(200), rowY)
	to := func(ms float32) {
		pos := fyne.NewPos(start.X+(ms-200)/s.msPerPixel, rowY)
		s.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: pos}, Dragged: fyne.NewDelta(pos.X-start.X, 0)})
	}
	to(593)
	if s.snapMarkMs != 600 {
		t.Errorf("snap guide at %d, want 600 (head's key)", s.snapMarkMs)
	}
	to(331)
	s.DragEnd()
	if len(got) != 2 || got[0] != 600 || got[1] != 340 {
		t.Errorf("retimed to %v, want [600 340]", got)
	}

	var scrubbed uint32
	s.OnScrub = func(ms uint32) { scrubbed = ms }
	s.scrubTo(s.xForTime(604))
	if scrubbed != 600 {
		t.Errorf("click near 600ms scrubbed to %d, want 600", scrubbed)
	}
}
