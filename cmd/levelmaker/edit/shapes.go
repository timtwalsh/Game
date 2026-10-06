package edit

import "game/shared/world"

// BrushSizes are the brush widths the editor offers (D13).
var BrushSizes = []int{1, 3, 5}

// Brush returns the square of world cells a brush of the given (odd) size
// covers, centred on c.
func Brush(c world.Point, size int) []world.Point {
	r := size / 2
	out := make([]world.Point, 0, size*size)
	for y := c.Y - r; y <= c.Y+r; y++ {
		for x := c.X - r; x <= c.X+r; x++ {
			out = append(out, world.Point{X: x, Y: y})
		}
	}
	return out
}

// Rect returns every world cell in the rectangle with corners a and b,
// inclusive, in either order.
func Rect(a, b world.Point) []world.Point {
	x0, x1 := min(a.X, b.X), max(a.X, b.X)
	y0, y1 := min(a.Y, b.Y), max(a.Y, b.Y)
	out := make([]world.Point, 0, (x1-x0+1)*(y1-y0+1))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			out = append(out, world.Point{X: x, Y: y})
		}
	}
	return out
}

// Line returns the cells on a line from a to b, inclusive, so a fast mouse
// drag paints a continuous stroke instead of dots.
func Line(a, b world.Point) []world.Point {
	dx, dy := abs(b.X-a.X), -abs(b.Y-a.Y)
	sx, sy := sign(b.X-a.X), sign(b.Y-a.Y)
	err := dx + dy
	out := []world.Point{a}
	for p := a; p != b; {
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			p.X += sx
		}
		if e2 <= dx {
			err += dx
			p.Y += sy
		}
		out = append(out, p)
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// FloodCells returns the 4-connected region of cells in the current level
// with the same terrain as the world cell start (empty cells included). It
// never leaves the current level.
func (e *Editor) FloodCells(start world.Point) []world.Point {
	l := e.Current
	if l == nil || !l.ContainsWorld(start.X, start.Y) {
		return nil
	}
	want := l.TerrainAt(start.X-l.Pos.X, start.Y-l.Pos.Y)
	seen := make([]bool, l.W*l.H)
	var out []world.Point
	queue := []world.Point{{X: start.X - l.Pos.X, Y: start.Y - l.Pos.Y}}
	seen[l.Index(queue[0].X, queue[0].Y)] = true
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		out = append(out, world.Point{X: p.X + l.Pos.X, Y: p.Y + l.Pos.Y})
		for _, d := range [4]world.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			q := world.Point{X: p.X + d.X, Y: p.Y + d.Y}
			if !l.In(q.X, q.Y) || seen[l.Index(q.X, q.Y)] || l.TerrainAt(q.X, q.Y) != want {
				continue
			}
			seen[l.Index(q.X, q.Y)] = true
			queue = append(queue, q)
		}
	}
	return out
}

// FloodFill paints the region FloodCells finds, as one undo step.
func (e *Editor) FloodFill(start world.Point, terrain uint8) {
	e.Paint(e.FloodCells(start), terrain)
}
