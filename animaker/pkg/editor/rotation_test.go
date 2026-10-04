package editor

import (
	"math"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

// Clockwise on screen, y down: "up" turns right at 90, down at 180.
func TestRotatePointTurnsClockwiseOnScreen(t *testing.T) {
	for deg, want := range map[float32][2]float32{
		0: {0, -1}, 90: {1, 0}, 180: {0, 1}, 270: {-1, 0}, -90: {-1, 0},
	} {
		x, y := RotatePoint(0, -1, deg)
		if !near(x, want[0]) || !near(y, want[1]) {
			t.Errorf("up at %v deg = (%v,%v), want (%v,%v)", deg, x, y, want[0], want[1])
		}
	}
}

func TestRotatedCellBox(t *testing.T) {
	// A 16x8 cell pivoted at its centre-bottom (8,8).
	tests := []struct {
		deg                    float32
		minX, minY, maxX, maxY float32
	}{
		{0, -8, -8, 8, 0},
		{90, 0, -8, 8, 8},   // lies along +x, spanning y -8..8
		{180, -8, 0, 8, 8},  // hangs below the pivot
		{270, -8, -8, 0, 8}, // lies along -x
		// Leans right; 16.97 wide and high, rounded up to 17 from the
		// exact top-left.
		{45, -5.657, -11.314, -5.657 + 17, -11.314 + 17},
	}
	for _, tt := range tests {
		a, b, c, d := RotatedCellBox(16, 8, 8, 8, tt.deg)
		if !near3(a, tt.minX) || !near3(b, tt.minY) || !near3(c, tt.maxX) || !near3(d, tt.maxY) {
			t.Errorf("%v deg: box (%v,%v)-(%v,%v), want (%v,%v)-(%v,%v)",
				tt.deg, a, b, c, d, tt.minX, tt.minY, tt.maxX, tt.maxY)
		}
	}
}

// A quarter turn about a pivot between pixels is still exact: no pixel is
// added to the box.
func TestRotatedCellBoxQuarterTurnAboutAHalfPixelPivot(t *testing.T) {
	minX, minY, maxX, maxY := RotatedCellBox(3, 1, 0.5, 0.5, 90)
	if !near3(minX, -0.5) || !near3(minY, -0.5) || !near3(maxX, 0.5) || !near3(maxY, 2.5) {
		t.Errorf("box (%v,%v)-(%v,%v), want (-0.5,-0.5)-(0.5,2.5)", minX, minY, maxX, maxY)
	}
}

func near3(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

// A nested part's rotation turns its whole animation about the part's
// origin: the flame at (1,-4) swings round to (4,1) at 90, and the flame's
// cell turns with it, on top of its own rotation.
func TestNestedPartRotationTurnsItsAnimation(t *testing.T) {
	p, part := walkWithTorch(t)
	p.ActiveDirection().KeyframesFor(part.ID)[0].RotationDeg = 90
	torch := p.LoadedAnims[AnimKey("torch.anif")].Track
	for _, kf := range torch.Directions[0].KeyframesFor(torch.Parts[0].ID) {
		kf.RotationDeg = 10
	}

	s := p.FlattenNested(part)[0]
	if !near(s.X, 4) || !near(s.Y, 1) {
		t.Errorf("flame at (%v,%v), want (4,1)", s.X, s.Y)
	}
	if !near(s.RotationDeg, 100) {
		t.Errorf("flame rotation %v, want its own 10 + the part's 90", s.RotationDeg)
	}
}

// Through two levels: the inner part's rotation turns its sprites first,
// then the outer one turns the lot.
func TestRotationComposesThroughNesting(t *testing.T) {
	p, part := walkWithTorch(t)
	torch := p.LoadedAnims[AnimKey("torch.anif")]
	// Wrap the torch in a holder that holds it at (0,-10), turned 90.
	holder := NewTrack("holder")
	held := AddPart(holder, NewNestedAniPart("torch", torch.Path))
	kf := AddKeyframe(holder.Directions[0], held.ID, 0)
	kf.X, kf.Y, kf.RotationDeg = 0, -10, 90
	p.LoadedAnims[AnimKey("holder.anif")] = &NestedAnim{Path: AnimKey("holder.anif"), Track: holder}
	part.NestedAniPath = AnimKey("holder.anif")
	p.ActiveDirection().KeyframesFor(part.ID)[0].RotationDeg = 90

	s := p.FlattenNested(part)
	if len(s) != 1 {
		t.Fatalf("%d sprites, want 1", len(s))
	}
	// Inner: (1,-4) turned 90 = (4,1), plus (0,-10) = (4,-9). Outer: turned
	// 90 again = (9,4).
	if !near(s[0].X, 9) || !near(s[0].Y, 4) || !near(s[0].RotationDeg, 180) {
		t.Errorf("sprite (%v,%v) at %v deg, want (9,4) at 180", s[0].X, s[0].Y, s[0].RotationDeg)
	}
}

// The extent covers the nested animation's sprites as they're turned by
// their own rotation (the part's own rotation is the canvas's to apply).
func TestNestedExtentCoversRotatedSprites(t *testing.T) {
	p, part := walkWithTorch(t)
	torch := p.LoadedAnims[AnimKey("torch.anif")].Track
	for _, d := range []int{0, 2} {
		for _, kf := range torch.Directions[d].KeyframesFor(torch.Parts[0].ID) {
			kf.RotationDeg = 180
		}
	}
	// The 16x16 flame, pivot (3,4), at (1,-4): upright it covers (-2,-8) to
	// (14,8); turned about its pivot it covers (-12,-16) to (4,0).
	minX, minY, maxX, maxY, ok := p.NestedExtent(part)
	if !ok || !near3(minX, -12) || !near3(minY, -16) || !near3(maxX, 4) || !near3(maxY, 0) {
		t.Errorf("extent (%v,%v)-(%v,%v), want (-12,-16)-(4,0)", minX, minY, maxX, maxY)
	}
}
