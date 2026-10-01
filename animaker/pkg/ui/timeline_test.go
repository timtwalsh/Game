package ui

import (
	"animaker/pkg/editor"
	"math"
	"testing"
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
