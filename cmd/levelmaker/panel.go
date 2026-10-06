package main

import (
	"fmt"

	"game/cmd/levelmaker/edit"
	"game/shared/world"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// panel draws the left-hand panel: levels, terrain palette, tools, and the
// status message. It scrolls with the wheel when it doesn't fit.
func (a *app) panel() {
	sh := float32(rl.GetScreenHeight())
	rl.DrawRectangle(0, 0, panelW, int32(sh), colPanel)
	interactive := a.dialog == nil
	if interactive && rl.GetMousePosition().X < panelW {
		a.scroll -= rl.GetMouseWheelMove() * 3 * rowH
	}
	rl.BeginScissorMode(0, 0, panelW, int32(sh))
	defer rl.EndScissorMode()

	x, w := float32(pad), float32(panelW-2*pad)
	y := pad - a.scroll
	btn := func(label string, active bool) bool {
		r := rl.NewRectangle(x, y, w, rowH)
		y += rowH + 4
		return button(r, label, active, interactive)
	}
	row := func(labels []string, active int, enabled []bool) int {
		bw := (w - 4*float32(len(labels)-1)) / float32(len(labels))
		clicked := -1
		for i, l := range labels {
			r := rl.NewRectangle(x+float32(i)*(bw+4), y, bw, rowH)
			if button(r, l, i == active, interactive && (enabled == nil || enabled[i])) {
				clicked = i
			}
		}
		y += rowH + 4
		return clicked
	}
	section := func(s string) { y += 6; y += heading(s, x, y, colDim) }

	y += heading("Level maker", x, y, colAccent)
	if l := a.ed.Current; l != nil {
		name := l.Name
		if contains(a.ed.DirtyLevels(), l.Name) {
			name += "  (unsaved)"
		}
		y += label(name, x, y, colText)
		info := fmt.Sprintf("at %d,%d  size %dx%d", l.Pos.X, l.Pos.Y, l.W, l.H)
		if l.Isolated {
			info += "  isolated"
		}
		for _, line := range wrap(info, w) {
			y += label(line, x, y, colDim)
		}
	}

	section("Levels")
	for _, l := range a.ed.World.Levels {
		text := l.Name
		if contains(a.ed.DirtyLevels(), l.Name) {
			text += " *"
		}
		if btn(text, l == a.ed.Current) && l != a.ed.Current {
			a.ed.Select(l.Name)
			a.centreOnLevel()
		}
	}
	if btn("New level...  (N)", false) {
		a.newLevelDialog()
	}

	section("Terrain  (1-9, E erases)")
	for i, t := range a.ed.World.Defs.Terrains {
		key := ""
		if i < 9 {
			key = fmt.Sprintf("%d  ", i+1)
		}
		r := rl.NewRectangle(x, y, w, rowH)
		if button(r, "", a.paint == t.ID, interactive) {
			a.paint = t.ID
		}
		swatch := t.TileIndex
		if t.Edges {
			swatch = t.SheetBase + world.FullState
		}
		a.atlas.DrawRect(swatch, 0, rl.NewRectangle(x+4, y+3, rowH-6, rowH-6))
		label(key+t.Name, x+rowH+6, y+(rowH-fontSize)/2, colText)
		y += rowH + 4
	}
	if btn("Eraser  (E)", a.paint == 0) {
		a.paint = 0
	}

	section("Tool  (B / R / F)")
	if i := row(toolNames, int(a.tool), nil); i >= 0 {
		a.tool = tool(i)
	}
	section("Brush size  ([ / ])")
	sizes := make([]string, len(edit.BrushSizes))
	for i, s := range edit.BrushSizes {
		sizes[i] = fmt.Sprint(s)
	}
	if i := row(sizes, a.brush, nil); i >= 0 {
		a.brush = i
	}

	section("View")
	if btn("Grid  (G)", a.grid) {
		a.grid = !a.grid
	}
	if i := row([]string{"Centre (Home)", "Jump to (J)"}, -1, nil); i == 0 {
		a.centreOnLevel()
	} else if i == 1 {
		a.jumpDialog()
	}

	section("Edit")
	switch row([]string{"Undo", "Redo"}, -1, []bool{a.ed.CanUndo(), a.ed.CanRedo()}) {
	case 0:
		a.undo()
	case 1:
		a.redo()
	}
	if btn("Save  (Ctrl+S)", false) {
		a.save()
	}
	if btn("Quit", false) {
		a.requestQuit()
	}

	if a.status != "" {
		y += 8
		col := colText
		if a.isErr {
			col = colError
		}
		for _, l := range wrap(a.status, w) {
			y += label(l, x, y, col)
		}
	}
	y += pad

	// Clamp scrolling to the content.
	content := y + a.scroll
	a.scroll = min(max(a.scroll, 0), max(content-sh, 0))
}
