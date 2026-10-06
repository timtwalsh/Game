package main

import (
	"fmt"
	"strings"

	"game/client/render"
	"game/cmd/levelmaker/edit"
	"game/shared"
	"game/shared/world"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var (
	colGrid      = rl.Fade(rl.White, 0.12)
	colNeighbour = rl.Fade(rl.Black, 0.55)
	colCursor    = rl.White
)

// canvasRect is the editing area: everything right of the panel, above the
// status bar.
func canvasRect() (x, y, w, h int) {
	return panelW, 0, max(rl.GetScreenWidth()-panelW, 1), max(rl.GetScreenHeight()-statusH, 1)
}

// camera2D is the raylib camera for the canvas: the shared camera, centred
// in the canvas rather than the window.
func (a *app) camera2D() rl.Camera2D {
	cx, cy, cw, ch := canvasRect()
	c := a.cam.Raylib(cw, ch)
	c.Offset.X += float32(cx)
	c.Offset.Y += float32(cy)
	return c
}

// mouseWorld is the world pixel under the mouse.
func (a *app) mouseWorld() shared.Vec2 {
	cx, cy, cw, ch := canvasRect()
	m := rl.GetMousePosition()
	return a.cam.ScreenToWorld(m.X-float32(cx), m.Y-float32(cy), cw, ch)
}

func (a *app) mouseCell() world.Point {
	p := a.mouseWorld()
	return world.Point{X: world.TileOf(p.X), Y: world.TileOf(p.Y)}
}

func (a *app) mouseOnCanvas() bool {
	cx, cy, cw, ch := canvasRect()
	return mouseIn(rl.NewRectangle(float32(cx), float32(cy), float32(cw), float32(ch)))
}

// canvas handles view and painting input, then draws the level.
func (a *app) canvas() {
	if a.dialog == nil {
		a.viewInput()
		a.paintInput()
	} else if a.painting {
		a.painting = false
		a.ed.EndStroke()
	}
	a.drawCanvas()
	a.drawStatusBar()
}

func (a *app) viewInput() {
	if a.mouseOnCanvas() {
		// Zoom about the cursor: the world point under it stays put.
		if wheel := rl.GetMouseWheelMove(); wheel != 0 {
			before := a.mouseWorld()
			a.cam.ZoomBy(wheel)
			after := a.mouseWorld()
			a.cam.Target.X += before.X - after.X
			a.cam.Target.Y += before.Y - after.Y
		}
	}
	// Pan: drag with the right or middle button, or the arrow keys.
	if rl.IsMouseButtonDown(rl.MouseButtonRight) || rl.IsMouseButtonDown(rl.MouseButtonMiddle) {
		d := rl.GetMouseDelta()
		a.cam.Pan(-d.X, -d.Y)
	}
	step := float32(600) * rl.GetFrameTime()
	if rl.IsKeyDown(rl.KeyLeft) {
		a.cam.Pan(-step, 0)
	}
	if rl.IsKeyDown(rl.KeyRight) {
		a.cam.Pan(step, 0)
	}
	if rl.IsKeyDown(rl.KeyUp) {
		a.cam.Pan(0, -step)
	}
	if rl.IsKeyDown(rl.KeyDown) {
		a.cam.Pan(0, step)
	}
}

func (a *app) brushSize() int { return edit.BrushSizes[a.brush] }

func (a *app) paintInput() {
	if a.ed.Current == nil {
		return
	}
	cell := a.mouseCell()
	pressed := rl.IsMouseButtonPressed(rl.MouseButtonLeft) && a.mouseOnCanvas()
	down := rl.IsMouseButtonDown(rl.MouseButtonLeft)
	released := rl.IsMouseButtonReleased(rl.MouseButtonLeft)

	switch a.tool {
	case toolBrush:
		if pressed {
			a.ed.BeginStroke()
			a.painting = true
			a.lastCell = cell
			a.ed.Paint(edit.Brush(cell, a.brushSize()), a.paint)
		} else if a.painting && down && cell != a.lastCell {
			// Fill the gap since last frame, so a fast drag is continuous.
			for _, p := range edit.Line(a.lastCell, cell)[1:] {
				a.ed.Paint(edit.Brush(p, a.brushSize()), a.paint)
			}
			a.lastCell = cell
		}
		if a.painting && (released || !down) {
			a.painting = false
			a.ed.EndStroke()
		}
	case toolRect:
		if pressed {
			c := cell
			a.rectStart = &c
		}
		if a.rectStart != nil && (released || !down) {
			a.ed.Paint(edit.Rect(*a.rectStart, cell), a.paint)
			a.rectStart = nil
		}
	case toolFill:
		if pressed {
			a.ed.FloodFill(cell, a.paint)
		}
	}
}

// shownWorld is the current level plus its neighbours: what the canvas
// draws (D16).
func (a *app) shownWorld() *world.World {
	return &world.World{Defs: a.ed.World.Defs, Levels: append([]*world.Level{a.ed.Current}, a.ed.Neighbours()...)}
}

func levelRect(l *world.Level) render.TileRect {
	return render.TileRect{X0: l.Pos.X, Y0: l.Pos.Y, X1: l.Pos.X + l.W, Y1: l.Pos.Y + l.H}
}

func intersect(a, b render.TileRect) render.TileRect {
	return render.TileRect{X0: max(a.X0, b.X0), Y0: max(a.Y0, b.Y0), X1: min(a.X1, b.X1), Y1: min(a.Y1, b.Y1)}
}

func tileRectPixels(r render.TileRect) rl.Rectangle {
	ts := shared.TileSize
	return rl.NewRectangle(float32(r.X0)*ts, float32(r.Y0)*ts, float32(r.X1-r.X0)*ts, float32(r.Y1-r.Y0)*ts)
}

func (a *app) drawCanvas() {
	cx, cy, cw, ch := canvasRect()
	rl.BeginScissorMode(int32(cx), int32(cy), int32(cw), int32(ch))
	defer rl.EndScissorMode()
	rl.DrawRectangle(int32(cx), int32(cy), int32(cw), int32(ch), rl.Black) // void

	l := a.ed.Current
	if l == nil {
		label("No levels yet. Press N to create one.", float32(cx+20), float32(cy+20), colDim)
		return
	}
	rl.BeginMode2D(a.camera2D())
	cur := levelRect(l)
	strip := render.TileRect{X0: cur.X0 - stripTiles, Y0: cur.Y0 - stripTiles, X1: cur.X1 + stripTiles, Y1: cur.Y1 + stripTiles}
	view := intersect(a.cam.Visible(cw, ch), strip)

	r := render.Renderer{World: a.shownWorld(), Atlas: a.atlas}
	r.DrawUnder(view)
	items := r.AppendYSort(nil, view)
	render.SortItems(items)
	for _, it := range items {
		r.DrawItem(it)
	}
	r.DrawOverhead(view)

	px := 1 / a.cam.Zoom // one screen pixel, in world units
	for _, n := range a.ed.Neighbours() {
		rect := tileRectPixels(intersect(levelRect(n), strip))
		rl.DrawRectangleRec(rect, colNeighbour)
		rl.DrawRectangleLinesEx(tileRectPixels(levelRect(n)), px, colDim)
		label(n.Name, rect.X+4*px, rect.Y+4*px, colDim) // drawn in world space; scales with zoom
	}

	if a.grid && a.cam.Zoom >= 0.5 {
		g := intersect(view, cur)
		for x := g.X0; x <= g.X1; x++ {
			fx := float32(x) * shared.TileSize
			rl.DrawLineV(rl.NewVector2(fx, float32(g.Y0)*shared.TileSize), rl.NewVector2(fx, float32(g.Y1)*shared.TileSize), colGrid)
		}
		for y := g.Y0; y <= g.Y1; y++ {
			fy := float32(y) * shared.TileSize
			rl.DrawLineV(rl.NewVector2(float32(g.X0)*shared.TileSize, fy), rl.NewVector2(float32(g.X1)*shared.TileSize, fy), colGrid)
		}
	}
	rl.DrawRectangleLinesEx(tileRectPixels(cur), 2*px, colAccent)

	// What the tool would touch.
	if a.mouseOnCanvas() && a.dialog == nil {
		cell := a.mouseCell()
		var area render.TileRect
		switch {
		case a.rectStart != nil:
			s := *a.rectStart
			area = render.TileRect{X0: min(s.X, cell.X), Y0: min(s.Y, cell.Y), X1: max(s.X, cell.X) + 1, Y1: max(s.Y, cell.Y) + 1}
		case a.tool == toolBrush:
			half := a.brushSize() / 2
			area = render.TileRect{X0: cell.X - half, Y0: cell.Y - half, X1: cell.X + half + 1, Y1: cell.Y + half + 1}
		default:
			area = render.TileRect{X0: cell.X, Y0: cell.Y, X1: cell.X + 1, Y1: cell.Y + 1}
		}
		rl.DrawRectangleLinesEx(tileRectPixels(area), 1.5*px, colCursor)
	}
	rl.EndMode2D()
}

func (a *app) drawStatusBar() {
	sw, sh := rl.GetScreenWidth(), rl.GetScreenHeight()
	y := sh - statusH
	rl.DrawRectangle(panelW, int32(y), int32(sw-panelW), statusH, colPanel)
	paintName := "eraser"
	if t := a.ed.World.Defs.Terrain(a.paint); t != nil {
		paintName = t.Name
	}
	text := fmt.Sprintf("%s  %s  size %d   zoom %sx", toolNames[a.tool], paintName, a.brushSize(), trimZoom(a.cam.Zoom))
	if a.mouseOnCanvas() {
		c := a.mouseCell()
		text = fmt.Sprintf("tile %d,%d", c.X, c.Y) + a.describeCell(c) + "     " + text
	}
	label(text, panelW+pad, float32(y+4), colDim)
}

// describeCell says which level a world cell is in and what's painted there.
func (a *app) describeCell(c world.Point) string {
	l := a.ed.World.LevelAt(c.X, c.Y)
	if l == nil {
		return "  (void)"
	}
	x, y := c.X-l.Pos.X, c.Y-l.Pos.Y
	name := "empty"
	if t := a.ed.World.Defs.Terrain(l.TerrainAt(x, y)); t != nil {
		name = t.Name
	}
	where := l.Name
	if l != a.ed.Current {
		where += ", read-only"
	}
	return fmt.Sprintf("  (%s %d,%d: %s)", where, x, y, name)
}

// trimZoom formats a zoom factor briefly: 2, 0.5, 1.19.
func trimZoom(z float32) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", z), "0"), ".")
}
