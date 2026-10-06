package editor

import "testing"

func TestShapeKindRoundTrip(t *testing.T) {
	for _, k := range ShapeKinds {
		if got, err := ParseShapeKind(k.String()); err != nil || got != k {
			t.Errorf("ParseShapeKind(%q) = %v, %v", k, got, err)
		}
	}
	if k, err := ParseShapeKind(""); err != nil || k != ShapeRect {
		t.Errorf(`"" should read as rect, got %v, %v`, k, err)
	}
	if _, err := ParseShapeKind("triangle"); err == nil {
		t.Error("an unknown shape was accepted")
	}
}

func TestBoxResizeKeepsOppositeCorner(t *testing.T) {
	b := Box{X: 0, Y: 0, W: 10, H: 10}
	cases := []struct {
		h      Handle
		dx, dy float32
		want   Box
	}{
		{HandleBottomRight, 5, 2, Box{0, 0, 15, 12}},
		{HandleTopLeft, 2, 3, Box{2, 3, 8, 7}},
		{HandleTopRight, 4, -4, Box{0, -4, 14, 14}},
		{HandleBottomLeft, -6, 1, Box{-6, 0, 16, 11}},
		// Can't turn inside out: clamps at minShapeSize, anchored.
		{HandleBottomRight, -50, -50, Box{0, 0, 1, 1}},
		{HandleTopLeft, 50, 50, Box{9, 9, 1, 1}},
	}
	for _, c := range cases {
		if got := b.Resized(c.h, c.dx, c.dy, false); got != c.want {
			t.Errorf("Resized(%v, %g, %g) = %+v, want %+v", c.h, c.dx, c.dy, got, c.want)
		}
	}
}

func TestBoxResizeSquare(t *testing.T) {
	b := Box{X: 0, Y: 0, W: 10, H: 10}
	if got := b.Resized(HandleBottomRight, 6, 2, true); got != (Box{0, 0, 16, 16}) {
		t.Errorf("square resize = %+v, want 16x16 following the larger move", got)
	}
	if got := b.Resized(HandleTopLeft, 1, 4, true); got != (Box{4, 4, 6, 6}) {
		t.Errorf("square resize from top-left = %+v, want 6x6 anchored at the bottom-right", got)
	}
}

func TestBoxContains(t *testing.T) {
	b := Box{X: 0, Y: 0, W: 20, H: 10}
	if !b.Contains(ShapeRect, 1, 1) || b.Contains(ShapeRect, 21, 5) {
		t.Error("rect containment wrong")
	}
	if b.Contains(ShapeOval, 1, 1) {
		t.Error("an oval doesn't reach its box's corner")
	}
	if !b.Contains(ShapeOval, 10, 5) || !b.Contains(ShapeOval, 19, 5) {
		t.Error("oval should contain its centre and the middle of its sides")
	}
}

func TestHitboxes(t *testing.T) {
	c := NewCharacter("ogre")
	i := c.AddHitbox()
	j := c.AddHitbox()
	if c.Hitboxes[i].Name != "hitbox" || c.Hitboxes[j].Name != "hitbox_2" {
		t.Errorf("names = %q, %q", c.Hitboxes[i].Name, c.Hitboxes[j].Name)
	}
	if err := c.SetHitbox(j, Hitbox{Name: "hitbox", Kind: ShapeRect, Box: DefaultHitbox}); err == nil {
		t.Error("a duplicate name was accepted")
	}
	if err := c.SetHitbox(j, Hitbox{Name: "head band", Box: DefaultHitbox}); err == nil {
		t.Error("an invalid name was accepted")
	}
	if err := c.SetHitbox(j, Hitbox{Name: "head", Kind: ShapeCircle, Box: Box{0, 0, 30, 10}}); err != nil {
		t.Fatal(err)
	}
	if c.Hitboxes[j].Box.H != 30 {
		t.Errorf("a circle's box wasn't made square: %+v", c.Hitboxes[j].Box)
	}
	if err := c.SetHitbox(i, Hitbox{Name: "body", Box: Box{0, 0, 0, 5}}); err == nil {
		t.Error("a zero-size box was accepted")
	}
	c.RemoveHitbox(i)
	if len(c.Hitboxes) != 1 || c.Hitboxes[0].Name != "head" {
		t.Errorf("after remove: %+v", c.Hitboxes)
	}
}

func TestFootprintAndScale(t *testing.T) {
	c := NewCharacter("baby")
	if err := c.SetFootprint(Box{0, 0, -1, 4}); err == nil {
		t.Error("an invalid footprint was accepted")
	}
	if err := c.SetFootprint(DefaultFootprint); err != nil || c.Footprint == nil {
		t.Fatalf("SetFootprint: %v", err)
	}
	c.ClearFootprint()
	if c.Footprint != nil {
		t.Error("ClearFootprint left a footprint")
	}
	if _, ok := c.GameSize(10); ok {
		t.Error("GameSize without a scale reported a size")
	}
	c.Scale = 0.3
	if g, ok := c.GameSize(40); !ok || g != 12 {
		t.Errorf("GameSize(40) at 0.3 = %v, %v; want 12", g, ok)
	}
}
