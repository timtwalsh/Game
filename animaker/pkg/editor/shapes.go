package editor

import (
	"fmt"
	"slices"
)

// A character's footprint and hitboxes are simple shapes fixed to the
// animation's origin, in animation pixels (the same space as keyframe X/Y,
// so what the artist sees on the canvas is what's saved). They belong to
// the character, not to any one animation: every animation the character
// plays shares them, and art is expected to stay within them rather than
// carry them along (docs/ANI_MAKER_SPEC.md, "Footprint and hitboxes").
//
//   - The footprint is the one rectangle that blocks movement: what the
//     character occupies on the ground.
//   - Hitboxes are one or more named rects, ovals or circles: where the
//     character can be hit. A big creature has a big presence without a
//     big footprint.

// ShapeKind is a hitbox's shape.
type ShapeKind int

const (
	ShapeRect ShapeKind = iota
	ShapeOval
	ShapeCircle
)

// ShapeKinds lists every kind, in menu order.
var ShapeKinds = []ShapeKind{ShapeRect, ShapeOval, ShapeCircle}

func (k ShapeKind) String() string {
	switch k {
	case ShapeOval:
		return "oval"
	case ShapeCircle:
		return "circle"
	}
	return "rect"
}

// ParseShapeKind reads a kind as written in an .anichar; "" is rect.
func ParseShapeKind(s string) (ShapeKind, error) {
	for _, k := range ShapeKinds {
		if s == k.String() {
			return k, nil
		}
	}
	if s == "" {
		return ShapeRect, nil
	}
	return ShapeRect, fmt.Errorf("unknown shape %q (want rect, oval or circle)", s)
}

// Box is an axis-aligned rectangle relative to the origin: X, Y its
// top-left, W, H its size, in animation pixels. An oval or circle is the
// one inscribed in its box; a circle's box is square.
type Box struct {
	X, Y, W, H float32
}

// minShapeSize is the smallest a shape may be resized to, in animation
// pixels, so it can't vanish under a drag.
const minShapeSize = 1

// Valid reports why a box can't be used, or nil.
func (b Box) Valid() error {
	if b.W <= 0 || b.H <= 0 {
		return fmt.Errorf("size %gx%g must be positive", b.W, b.H)
	}
	return nil
}

// Moved is the box shifted by dx, dy.
func (b Box) Moved(dx, dy float32) Box {
	b.X += dx
	b.Y += dy
	return b
}

// Handle is a resize handle: which edges a drag moves.
type Handle int

const (
	HandleNone Handle = iota
	HandleTopLeft
	HandleTopRight
	HandleBottomLeft
	HandleBottomRight
)

// Handles lists the corner handles.
var Handles = []Handle{HandleTopLeft, HandleTopRight, HandleBottomLeft, HandleBottomRight}

// Corner is where a handle sits on the box.
func (b Box) Corner(h Handle) (x, y float32) {
	switch h {
	case HandleTopLeft:
		return b.X, b.Y
	case HandleTopRight:
		return b.X + b.W, b.Y
	case HandleBottomLeft:
		return b.X, b.Y + b.H
	}
	return b.X + b.W, b.Y + b.H
}

// Resized is the box with handle h dragged by dx, dy; the opposite corner
// stays put. A box can't shrink below minShapeSize or turn inside out.
// square keeps it square (a circle), sized by the larger movement.
func (b Box) Resized(h Handle, dx, dy float32, square bool) Box {
	left := h == HandleTopLeft || h == HandleBottomLeft
	top := h == HandleTopLeft || h == HandleTopRight
	// Signed growth of each dimension.
	gw, gh := dx, dy
	if left {
		gw = -dx
	}
	if top {
		gh = -dy
	}
	w, hh := max(b.W+gw, minShapeSize), max(b.H+gh, minShapeSize)
	if square {
		// Follow whichever direction the drag moved more.
		s := w
		if abs32(gh) > abs32(gw) {
			s = hh
		}
		w, hh = s, s
	}
	out := Box{X: b.X, Y: b.Y, W: w, H: hh}
	if left {
		out.X = b.X + b.W - w
	}
	if top {
		out.Y = b.Y + b.H - hh
	}
	return out
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Contains reports whether animation point (x, y) is inside the shape.
func (b Box) Contains(kind ShapeKind, x, y float32) bool {
	if x < b.X || y < b.Y || x > b.X+b.W || y > b.Y+b.H {
		return false
	}
	if kind == ShapeRect {
		return true
	}
	// Inside the inscribed ellipse.
	rx, ry := b.W/2, b.H/2
	nx, ny := (x-b.X-rx)/rx, (y-b.Y-ry)/ry
	return nx*nx+ny*ny <= 1
}

// Hitbox is one named shape where the character can be hit.
type Hitbox struct {
	Name string
	Kind ShapeKind
	Box  Box
}

// DefaultFootprint is the footprint a character starts with when the
// artist adds one: a small rect centred under the origin.
var DefaultFootprint = Box{X: -16, Y: -8, W: 32, H: 16}

// DefaultHitbox is a new hitbox's shape: roughly a body around the origin.
var DefaultHitbox = Box{X: -24, Y: -48, W: 48, H: 64}

// SetFootprint sets the character's footprint.
func (c *Character) SetFootprint(b Box) error {
	if err := b.Valid(); err != nil {
		return fmt.Errorf("footprint: %w", err)
	}
	c.Footprint = &b
	return nil
}

// ClearFootprint removes the footprint.
func (c *Character) ClearFootprint() { c.Footprint = nil }

// FindHitbox returns the index of the named hitbox, or -1.
func (c *Character) FindHitbox(name string) int {
	return slices.IndexFunc(c.Hitboxes, func(h Hitbox) bool { return h.Name == name })
}

// AddHitbox adds a rect hitbox with a free name based on "hitbox" and
// returns its index.
func (c *Character) AddHitbox() int {
	name := "hitbox"
	for n := 2; c.FindHitbox(name) >= 0; n++ {
		name = fmt.Sprintf("hitbox_%d", n)
	}
	c.Hitboxes = append(c.Hitboxes, Hitbox{Name: name, Kind: ShapeRect, Box: DefaultHitbox})
	return len(c.Hitboxes) - 1
}

// SetHitbox replaces hitbox i, checking its name and box. A circle's box is
// made square (sized by its width).
func (c *Character) SetHitbox(i int, h Hitbox) error {
	if i < 0 || i >= len(c.Hitboxes) {
		return fmt.Errorf("no hitbox %d", i)
	}
	if err := ValidAnimName(h.Name); err != nil {
		return fmt.Errorf("hitbox %w", err)
	}
	if j := c.FindHitbox(h.Name); j >= 0 && j != i {
		return fmt.Errorf("the character already has a hitbox called %q", h.Name)
	}
	if h.Kind == ShapeCircle {
		h.Box.H = h.Box.W
	}
	if err := h.Box.Valid(); err != nil {
		return fmt.Errorf("hitbox %q: %w", h.Name, err)
	}
	c.Hitboxes[i] = h
	return nil
}

// RemoveHitbox deletes hitbox i.
func (c *Character) RemoveHitbox(i int) {
	if i >= 0 && i < len(c.Hitboxes) {
		c.Hitboxes = slices.Delete(c.Hitboxes, i, i+1)
	}
}

// GameSize converts animation pixels to game pixels with the character's
// scale; ok is false when no scale is set.
func (c *Character) GameSize(animPx float32) (gamePx float32, ok bool) {
	if c.Scale <= 0 {
		return 0, false
	}
	return animPx * c.Scale, true
}
