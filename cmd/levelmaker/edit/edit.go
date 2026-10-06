// Package edit is the level maker's editing logic with no graphics: opening
// a world, creating levels, terrain painting with local re-solving (across
// into neighbouring levels), undo/redo per stroke, and saving. The raylib
// side (cmd/levelmaker) only draws and routes input, so everything here is
// testable in CI. See docs/LEVEL_MAKER_SPEC.md, build-order step 5.
package edit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"game/shared/world"
)

// Cell is everything a terrain stroke can change in one ground cell.
type Cell struct {
	Terrain uint8
	Tile    uint16
	Under   uint16
	Flags   uint8
}

func cellAt(l *world.Level, i int) Cell {
	g := &l.Ground
	return Cell{g.Terrain[i], g.Tile[i], g.Under[i], g.Flags[i]}
}

func setCell(l *world.Level, i int, c Cell) {
	g := &l.Ground
	g.Terrain[i], g.Tile[i], g.Under[i], g.Flags[i] = c.Terrain, c.Tile, c.Under, c.Flags
}

// change is one cell's before and after across a stroke.
type change struct {
	level         *world.Level
	i             int
	before, after Cell
}

type cellKey struct {
	level *world.Level
	i     int
}

// Editor is one editing session over a world.
type Editor struct {
	World     *world.World
	LevelsDir string
	// Current is the level being edited; its neighbours are shown
	// read-only, though re-solving may update their border tiles.
	Current *world.Level

	dirty map[*world.Level]bool
	undo  [][]change
	redo  [][]change

	stroke      []change // open stroke, in first-touched order
	strokeIndex map[cellKey]int
}

// Open loads the world under root: root/world/terrains.toml must exist;
// root/levels/ is created if missing, so a fresh world can be started.
func Open(root string) (*Editor, error) {
	defs, err := world.LoadDefs(filepath.Join(root, "world", "terrains.toml"))
	if err != nil {
		return nil, err
	}
	levelsDir := filepath.Join(root, "levels")
	if err := os.MkdirAll(levelsDir, 0o755); err != nil {
		return nil, err
	}
	w, err := world.LoadWorld(levelsDir, defs)
	if err != nil {
		return nil, err
	}
	e := &Editor{World: w, LevelsDir: levelsDir, dirty: map[*world.Level]bool{}}
	e.Current = startLevel(w)
	return e, nil
}

// startLevel picks the level to open on: the one holding the spawn, else
// the first exterior level, else any (interiors sort first by name, but
// are rarely where you want to start).
func startLevel(w *world.World) *world.Level {
	if p, ok := w.Spawn(); ok {
		if l := w.LevelAt(world.TileOf(p.X), world.TileOf(p.Y)); l != nil {
			return l
		}
	}
	for _, l := range w.Levels {
		if !l.Isolated {
			return l
		}
	}
	if len(w.Levels) > 0 {
		return w.Levels[0]
	}
	return nil
}

// Select makes the named level the one being edited. An open stroke is
// ended first.
func (e *Editor) Select(name string) error {
	l := e.World.Level(name)
	if l == nil {
		return fmt.Errorf("no level named %q", name)
	}
	e.EndStroke()
	e.Current = l
	return nil
}

// NewLevel creates an empty level (every cell empty, so it blocks until
// painted), adds it to the world and selects it. It is refused, with a
// message naming the problem, if the name is invalid or taken or the area
// overlaps another level. A new level is unsaved until Save. Creating a
// level is not undoable; the strokes before it stay on the undo stack.
func (e *Editor) NewLevel(name string, pos world.Point, w, h int, isolated bool) error {
	if e.World.Level(name) != nil {
		return fmt.Errorf("a level named %q already exists", name)
	}
	if err := e.World.CheckPlacement(pos, w, h, ""); err != nil {
		return err
	}
	l, err := world.NewLevel(name, pos, w, h)
	if err != nil {
		return err
	}
	l.Isolated = isolated
	if err := e.World.Add(l); err != nil {
		return err
	}
	e.EndStroke()
	e.Current = l
	e.dirty[l] = true
	// Neighbours' border cells used to face void (joined) and now face
	// empty cells (not joined), so their edges change.
	for _, n := range e.World.SolveBorders(l) {
		e.dirty[n] = true
	}
	return nil
}

// Neighbours are the levels shown around the current one (D16).
func (e *Editor) Neighbours() []*world.Level {
	if e.Current == nil {
		return nil
	}
	return e.World.Neighbours(e.Current)
}

// BeginStroke starts a new undo step. Paint calls until EndStroke join it.
func (e *Editor) BeginStroke() {
	e.EndStroke()
	e.stroke = nil
	e.strokeIndex = map[cellKey]int{}
}

// InStroke reports whether a stroke is open.
func (e *Editor) InStroke() bool { return e.strokeIndex != nil }

// touch records a cell's state before the stroke first changes it.
func (e *Editor) touch(l *world.Level, i int) {
	k := cellKey{l, i}
	if _, ok := e.strokeIndex[k]; ok {
		return
	}
	e.strokeIndex[k] = len(e.stroke)
	e.stroke = append(e.stroke, change{level: l, i: i, before: cellAt(l, i)})
}

// viewLevel reports which level a world cell belongs to, as far as this
// session's view goes: the current level, or a neighbour (whose tiles may
// be re-solved but whose terrain is never painted).
func (e *Editor) viewLevel(wx, wy int) *world.Level {
	if e.Current.ContainsWorld(wx, wy) {
		return e.Current
	}
	if e.Current.Isolated {
		return nil
	}
	l := e.World.LevelAt(wx, wy)
	if l == nil || l.Isolated {
		return nil
	}
	return l
}

// Paint sets terrain on the given world cells, keeping only those inside
// the current level, and re-solves each painted cell and its 8 neighbours,
// including cells across the edge in neighbouring levels. Terrain 0
// erases. Outside a stroke it is its own undo step.
func (e *Editor) Paint(cells []world.Point, terrain uint8) {
	if e.Current == nil {
		return
	}
	own := !e.InStroke()
	if own {
		e.BeginStroke()
	}
	l := e.Current
	var painted []world.Point
	for _, p := range cells {
		if l.ContainsWorld(p.X, p.Y) {
			painted = append(painted, p)
		}
	}
	// Record every cell this can change before changing any.
	type target struct {
		l    *world.Level
		x, y int
	}
	var solve []target
	seen := map[cellKey]bool{}
	for _, p := range painted {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				tl := e.viewLevel(p.X+dx, p.Y+dy)
				if tl == nil {
					continue
				}
				x, y := p.X+dx-tl.Pos.X, p.Y+dy-tl.Pos.Y
				i := tl.Index(x, y)
				if seen[cellKey{tl, i}] {
					continue
				}
				seen[cellKey{tl, i}] = true
				e.touch(tl, i)
				solve = append(solve, target{tl, x, y})
			}
		}
	}
	for _, p := range painted {
		l.SetTerrain(p.X-l.Pos.X, p.Y-l.Pos.Y, terrain)
	}
	for _, t := range solve {
		world.SolveCell(t.l, t.x, t.y, e.World.SourceFor(t.l), e.World.Defs)
	}
	if own {
		e.EndStroke()
	}
}

// EndStroke closes the open stroke. Cells whose state ended where it
// started are dropped; if anything changed, the stroke becomes one undo
// step and the redo history is cleared.
func (e *Editor) EndStroke() {
	if !e.InStroke() {
		return
	}
	var step []change
	for _, c := range e.stroke {
		c.after = cellAt(c.level, c.i)
		if c.after != c.before {
			step = append(step, c)
		}
	}
	e.stroke, e.strokeIndex = nil, nil
	if len(step) == 0 {
		return
	}
	e.undo = append(e.undo, step)
	e.redo = nil
	e.markDirty(step)
}

func (e *Editor) markDirty(step []change) {
	for _, c := range step {
		e.dirty[c.level] = true
	}
}

// CanUndo and CanRedo report whether there is a step to undo or redo.
func (e *Editor) CanUndo() bool { return len(e.undo) > 0 }
func (e *Editor) CanRedo() bool { return len(e.redo) > 0 }

// Undo reverts the last stroke, in whichever levels it touched.
func (e *Editor) Undo() bool {
	e.EndStroke()
	if len(e.undo) == 0 {
		return false
	}
	step := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for _, c := range slices.Backward(step) {
		setCell(c.level, c.i, c.before)
	}
	e.redo = append(e.redo, step)
	e.markDirty(step)
	return true
}

// Redo re-applies the last undone stroke.
func (e *Editor) Redo() bool {
	e.EndStroke()
	if len(e.redo) == 0 {
		return false
	}
	step := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, c := range step {
		setCell(c.level, c.i, c.after)
	}
	e.undo = append(e.undo, step)
	e.markDirty(step)
	return true
}

// Dirty reports whether anything is unsaved.
func (e *Editor) Dirty() bool { return len(e.dirty) > 0 }

// DirtyLevels names the levels with unsaved changes, sorted.
func (e *Editor) DirtyLevels() []string {
	var names []string
	for l := range e.dirty {
		names = append(names, l.Name)
	}
	slices.Sort(names)
	return names
}

// Save re-solves the borders of every changed level, compiles cell
// properties, and writes every level with unsaved changes, including
// neighbours whose border tiles changed. It returns the paths written, so
// the artist can see when a neighbour's files were rewritten (open
// question 2's default).
func (e *Editor) Save() ([]string, error) {
	e.EndStroke()
	changed := make([]*world.Level, 0, len(e.dirty))
	for l := range e.dirty {
		changed = append(changed, l)
	}
	for _, l := range changed {
		for _, n := range e.World.SolveBorders(l) {
			e.dirty[n] = true
		}
	}
	var written []string
	var errs []error
	for _, name := range e.DirtyLevels() {
		l := e.World.Level(name)
		world.Compile(l, e.World.Defs)
		if err := world.SaveLevel(e.LevelsDir, l); err != nil {
			errs = append(errs, fmt.Errorf("saving %s: %w", name, err))
			continue
		}
		delete(e.dirty, l)
		t, g := world.LevelPaths(e.LevelsDir, name)
		written = append(written, t, g)
	}
	return written, errors.Join(errs...)
}
