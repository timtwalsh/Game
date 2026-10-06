// Command levelmaker is the level editor (docs/LEVEL_MAKER_SPEC.md, editor
// v1): terrain painting with autotile on the ground layer of one level at a
// time, with read-only strips of its neighbours. It renders through
// client/render so levels look exactly as they do in the game; the editing
// logic lives in cmd/levelmaker/edit.
//
//	levelmaker [-root folder]   # folder holding world/ and levels/ (default ".")
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"game/client/render"
	"game/cmd/levelmaker/edit"
	"game/shared"
	"game/shared/world"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type tool int

const (
	toolBrush tool = iota
	toolRect
	toolFill
)

var toolNames = []string{"Brush", "Rect", "Fill"}

const (
	panelW     = 250
	statusH    = 24
	stripTiles = 12 // how much of each neighbour is shown around the current level
)

type app struct {
	ed     *edit.Editor
	root   string
	log    *sessionLog
	atlas  *render.Atlas
	cam    render.Camera
	tool   tool
	paint  uint8 // terrain id to paint; 0 is the eraser
	brush  int   // index into edit.BrushSizes
	grid   bool
	status string
	isErr  bool

	painting  bool
	lastCell  world.Point
	rectStart *world.Point
	scroll    float32

	dialog *dialog
	done   bool
}

func main() {
	root := flag.String("root", ".", "folder holding world/ and levels/")
	flag.Parse()

	slog, err := startLog(logDir())
	if err != nil {
		fmt.Fprintln(os.Stderr, "can't start the session log:", err)
		os.Exit(1)
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Crashed(r)
			panic(r)
		}
		slog.Close()
	}()

	rl.SetConfigFlags(rl.FlagWindowResizable)
	rl.InitWindow(1280, 800, "Level maker")
	defer rl.CloseWindow()
	rl.SetWindowMinSize(800, 500)
	rl.SetTargetFPS(60)
	rl.SetExitKey(rl.KeyNull) // Esc closes dialogs; quitting asks about unsaved work
	loadFonts()
	defer unloadFonts()

	ed, err := edit.Open(*root)
	if err != nil {
		showFatal(slog.Errorf("can't open the world in %s: %v", *root, err))
		return
	}
	a := &app{ed: ed, root: *root, log: slog, cam: render.NewFreeCamera(), paint: firstTerrain(ed), brush: 0, grid: true}
	a.cam.Zoom = 2
	atlas, problems := render.NewAtlas(ed.World.Defs, *root)
	defer atlas.Unload()
	a.atlas = atlas
	for _, p := range problems {
		a.setError(slog.Errorf("%v", p))
	}
	if slog.PreviousCrash != "" {
		a.setError(fmt.Errorf("the last session crashed; its log is %s", slog.PreviousCrash))
	}
	a.centreOnLevel()
	if ed.Current == nil {
		a.status = "No levels yet: press N to create one."
	}

	for !a.done {
		if rl.WindowShouldClose() {
			a.requestQuit()
		}
		rl.BeginDrawing()
		rl.ClearBackground(colPanel)
		a.frame()
		rl.EndDrawing()
	}
}

// showFatal keeps the window open with an error until it's closed, so a
// world that won't load says why instead of vanishing.
func showFatal(err error) {
	rl.SetExitKey(rl.KeyEscape)
	for !rl.WindowShouldClose() {
		rl.BeginDrawing()
		rl.ClearBackground(colPanel)
		y := float32(40)
		for _, l := range wrap(err.Error(), float32(rl.GetScreenWidth()-80)) {
			y += label(l, 40, y, colError)
		}
		label("Close the window or press Esc to quit.", 40, y+20, colDim)
		rl.EndDrawing()
	}
}

func firstTerrain(ed *edit.Editor) uint8 {
	if len(ed.World.Defs.Terrains) > 0 {
		return ed.World.Defs.Terrains[0].ID
	}
	return 0
}

func (a *app) setStatus(s string) { a.status, a.isErr = s, false }
func (a *app) setError(err error) { a.status, a.isErr = err.Error(), true }

func (a *app) frame() {
	keyFromDialog := -1
	if a.dialog != nil {
		keyFromDialog = a.dialog.update()
	} else {
		a.shortcuts()
	}
	a.canvas()
	a.panel()
	if d := a.dialog; d != nil {
		clicked := d.draw()
		if clicked < 0 {
			clicked = keyFromDialog
		}
		if clicked >= 0 && d.onButton(d, clicked) && a.dialog == d {
			a.dialog = nil
		}
	}
}

func ctrl() bool  { return rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) }
func shift() bool { return rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) }

func (a *app) shortcuts() {
	switch {
	case ctrl() && rl.IsKeyPressed(rl.KeyZ) && shift(), ctrl() && rl.IsKeyPressed(rl.KeyY):
		a.redo()
	case ctrl() && rl.IsKeyPressed(rl.KeyZ):
		a.undo()
	case ctrl() && rl.IsKeyPressed(rl.KeyS):
		a.save()
	case ctrl():
	case rl.IsKeyPressed(rl.KeyB):
		a.tool = toolBrush
	case rl.IsKeyPressed(rl.KeyR):
		a.tool = toolRect
	case rl.IsKeyPressed(rl.KeyF):
		a.tool = toolFill
	case rl.IsKeyPressed(rl.KeyE):
		a.paint = 0
	case rl.IsKeyPressed(rl.KeyG):
		a.grid = !a.grid
	case rl.IsKeyPressed(rl.KeyN):
		a.newLevelDialog()
	case rl.IsKeyPressed(rl.KeyJ):
		a.jumpDialog()
	case rl.IsKeyPressed(rl.KeyHome):
		a.centreOnLevel()
	case rl.IsKeyPressed(rl.KeyLeftBracket):
		a.brush = max(a.brush-1, 0)
	case rl.IsKeyPressed(rl.KeyRightBracket):
		a.brush = min(a.brush+1, len(edit.BrushSizes)-1)
	}
	// 1-9 pick the terrain in palette order.
	for i, t := range a.ed.World.Defs.Terrains {
		if i < 9 && !ctrl() && rl.IsKeyPressed(int32(rl.KeyOne)+int32(i)) {
			a.paint = t.ID
		}
	}
}

func (a *app) undo() {
	if a.ed.Undo() {
		a.setStatus("Undone.")
	}
}

func (a *app) redo() {
	if a.ed.Redo() {
		a.setStatus("Redone.")
	}
}

// save writes every changed level and says which files it wrote, so a
// rewritten neighbour is never a surprise (spec open question 2).
func (a *app) save() bool {
	if !a.ed.Dirty() {
		a.setStatus("Nothing to save.")
		return true
	}
	current := ""
	if a.ed.Current != nil {
		current = a.ed.Current.Name
	}
	written, err := a.ed.Save()
	if err != nil {
		a.setError(a.log.Errorf("%v", err))
		return false
	}
	var names, others []string
	for _, p := range written {
		names = append(names, filepath.Base(p))
		if n := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(p), world.LevelSuffix), world.GridSuffix); n != current && !contains(others, n) {
			others = append(others, n)
		}
	}
	msg := "Saved " + strings.Join(names, ", ") + "."
	if len(others) > 0 {
		msg += " Also rewrote " + strings.Join(others, ", ") + " (changed tiles at the border)."
	}
	log.Print(msg)
	a.setStatus(msg)
	return true
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func (a *app) requestQuit() {
	if !a.ed.Dirty() {
		a.done = true
		return
	}
	if a.dialog != nil && a.dialog.title == "Unsaved changes" {
		return
	}
	a.dialog = &dialog{
		title:   "Unsaved changes",
		message: "These levels have unsaved changes: " + strings.Join(a.ed.DirtyLevels(), ", ") + ".",
		buttons: []string{"Save and quit", "Quit without saving", "Cancel"},
		onButton: func(d *dialog, i int) bool {
			switch i {
			case 0:
				if !a.save() {
					d.err = a.status
					return false
				}
				a.done = true
			case 1:
				a.done = true
			}
			return true
		},
	}
}

// centreOnLevel jumps the camera to the middle of the current level.
func (a *app) centreOnLevel() {
	l := a.ed.Current
	if l == nil {
		return
	}
	a.cam.CentreOn(shared.Vec2{
		X: (float32(l.Pos.X) + float32(l.W)/2) * shared.TileSize,
		Y: (float32(l.Pos.Y) + float32(l.H)/2) * shared.TileSize,
	})
}

func intField(label string, v int) *textField {
	return &textField{Label: label, Text: strconv.Itoa(v), MaxLen: 7, Allow: intRune}
}

func (a *app) newLevelDialog() {
	pos, size := world.Point{}, world.Point{X: 32, Y: 24}
	if l := a.ed.Current; l != nil {
		pos = world.Point{X: l.Pos.X + l.W, Y: l.Pos.Y} // east of the current level
	}
	name := &textField{Label: "Name", MaxLen: 40, Allow: levelNameRune}
	x, y := intField("X (tiles)", pos.X), intField("Y (tiles)", pos.Y)
	w, h := intField("Width", size.X), intField("Height", size.Y)
	a.dialog = &dialog{
		title:    "New level",
		message:  "Position is the world tile of its top-left corner. It may not overlap another level.",
		fields:   []*textField{name, x, y, w, h},
		checkbox: "Isolated (interior)",
		buttons:  []string{"Create", "Cancel"},
		onButton: func(d *dialog, i int) bool {
			if i != 0 {
				return true
			}
			nums := make([]int, 4)
			for j, f := range []*textField{x, y, w, h} {
				v, err := strconv.Atoi(f.Text)
				if err != nil {
					d.err = f.Label + " must be a whole number."
					return false
				}
				nums[j] = v
			}
			if err := a.ed.NewLevel(name.Text, world.Point{X: nums[0], Y: nums[1]}, nums[2], nums[3], d.checked); err != nil {
				d.err = err.Error()
				return false
			}
			a.centreOnLevel()
			a.setStatus(fmt.Sprintf("Created level %s (unsaved). Paint it, then Ctrl+S.", name.Text))
			return true
		},
	}
}

func (a *app) jumpDialog() {
	c := a.cam.Target
	x, y := intField("X (tiles)", world.TileOf(c.X)), intField("Y (tiles)", world.TileOf(c.Y))
	a.dialog = &dialog{
		title:   "Jump to",
		message: "Centre the view on a world tile.",
		fields:  []*textField{x, y},
		buttons: []string{"Jump", "Cancel"},
		onButton: func(d *dialog, i int) bool {
			if i != 0 {
				return true
			}
			tx, err1 := strconv.Atoi(x.Text)
			ty, err2 := strconv.Atoi(y.Text)
			if err1 != nil || err2 != nil {
				d.err = "Both must be whole numbers."
				return false
			}
			a.cam.CentreOn(shared.Vec2{X: (float32(tx) + 0.5) * shared.TileSize, Y: (float32(ty) + 0.5) * shared.TileSize})
			return true
		},
	}
}
