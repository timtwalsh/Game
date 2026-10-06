package world

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"game/shared"
)

// World is a set of non-overlapping levels on one plane (D14), answering
// queries in world tile coordinates across level edges. Coordinates are
// signed: negative positions are normal. Cells outside every level are void,
// which blocks everything.
type World struct {
	Defs   *Defs
	Levels []*Level // sorted by name
}

// NewWorld makes an empty world.
func NewWorld(defs *Defs) *World { return &World{Defs: defs} }

// LoadWorld loads every *.level.toml in dir. There is no central index:
// adding a level means adding its two files (D14). Each level's property
// grids are recompiled from its terrain, tiles and overrides with these
// defs, so the stored compiled grids are never trusted (D32) and always
// reflect the current definitions.
func LoadWorld(dir string, defs *Defs) (*World, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*"+LevelSuffix))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	w := NewWorld(defs)
	for _, p := range paths {
		l, err := LoadLevel(p)
		if err != nil {
			return nil, err
		}
		if err := w.Add(l); err != nil {
			return nil, err
		}
		Compile(l, defs)
	}
	return w, nil
}

// LoadDir loads the defs and every level of a world laid out the standard
// way: <root>/world/terrains.toml and <root>/levels/. ok is false (with no
// error) when root has no world/ or levels/ at all, so callers can fall
// back to their placeholder grid.
func LoadDir(root string) (w *World, ok bool, err error) {
	defsPath := filepath.Join(root, "world", "terrains.toml")
	levelsDir := filepath.Join(root, "levels")
	if _, err := os.Stat(defsPath); os.IsNotExist(err) {
		return nil, false, nil
	}
	if _, err := os.Stat(levelsDir); os.IsNotExist(err) {
		return nil, false, nil
	}
	defs, err := LoadDefs(defsPath)
	if err != nil {
		return nil, false, err
	}
	w, err = LoadWorld(levelsDir, defs)
	if err != nil {
		return nil, false, err
	}
	return w, true, nil
}

// Add puts a level into the world. It refuses a duplicate name or a level
// that overlaps another, naming both.
func (w *World) Add(l *Level) error {
	r := l.worldRect()
	for _, o := range w.Levels {
		if o.Name == l.Name {
			return fmt.Errorf("two levels are named %q", l.Name)
		}
		if r.overlaps(o.worldRect()) {
			return fmt.Errorf("level %q overlaps level %q", l.Name, o.Name)
		}
	}
	w.Levels = append(w.Levels, l)
	sort.Slice(w.Levels, func(i, j int) bool { return w.Levels[i].Name < w.Levels[j].Name })
	return nil
}

// CheckPlacement reports whether a level could be placed at pos with the
// given size without overlapping any level other than the one named
// ignore. The editor uses it before creating or moving a level.
func (w *World) CheckPlacement(pos Point, width, height int, ignore string) error {
	r := rect{pos.X, pos.Y, pos.X + width, pos.Y + height}
	for _, o := range w.Levels {
		if o.Name != ignore && r.overlaps(o.worldRect()) {
			return fmt.Errorf("that area overlaps level %q", o.Name)
		}
	}
	return nil
}

// Level returns the named level, or nil.
func (w *World) Level(name string) *Level {
	for _, l := range w.Levels {
		if l.Name == name {
			return l
		}
	}
	return nil
}

// LevelAt returns the level containing world tile (wx, wy), or nil for void.
func (w *World) LevelAt(wx, wy int) *Level {
	for _, l := range w.Levels {
		if l.ContainsWorld(wx, wy) {
			return l
		}
	}
	return nil
}

// Neighbours returns the levels whose rectangles share an edge or a corner
// with l's. Isolated levels have none and are nobody's neighbour (D22).
func (w *World) Neighbours(l *Level) []*Level {
	if l.Isolated {
		return nil
	}
	var out []*Level
	r := l.worldRect()
	for _, o := range w.Levels {
		if o != l && !o.Isolated && r.touches(o.worldRect()) {
			out = append(out, o)
		}
	}
	return out
}

// BlockingAt is the compiled blocking flags at a world tile; void blocks
// everything.
func (w *World) BlockingAt(wx, wy int) uint16 {
	l := w.LevelAt(wx, wy)
	if l == nil {
		return BlockAll
	}
	return l.Props.Blocking[l.Index(wx-l.Pos.X, wy-l.Pos.Y)]
}

// Blocks reports whether any of flag's bits are blocked at a world tile.
// Walking tests BlockGround.
func (w *World) Blocks(wx, wy int, flag uint16) bool {
	return w.BlockingAt(wx, wy)&flag != 0
}

// SurfaceAt is the compiled surface id at a world tile; 0 for void.
func (w *World) SurfaceAt(wx, wy int) uint8 {
	l := w.LevelAt(wx, wy)
	if l == nil {
		return 0
	}
	return l.Props.Surface[l.Index(wx-l.Pos.X, wy-l.Pos.Y)]
}

// InteractionAt is the compiled interaction id at a world tile; 0 for void.
func (w *World) InteractionAt(wx, wy int) uint8 {
	l := w.LevelAt(wx, wy)
	if l == nil {
		return 0
	}
	return l.Props.Interaction[l.Index(wx-l.Pos.X, wy-l.Pos.Y)]
}

// TileOf converts a world pixel coordinate to a world tile coordinate. It
// floors, so -0.5 px is tile -1 (a uint32 cast would wrap instead).
func TileOf(px float32) int {
	return int(math.Floor(float64(px / shared.TileSize)))
}

// SourceFor returns the autotile view from level l: l itself plus its
// neighbours (D16). Anything else, including every cell outside an isolated
// level, is void (D22).
func (w *World) SourceFor(l *Level) TerrainSource {
	return worldSource{w: w, from: l}
}

type worldSource struct {
	w    *World
	from *Level
}

func (s worldSource) TerrainAt(wx, wy int) (uint8, bool) {
	if s.from.ContainsWorld(wx, wy) {
		return s.from.TerrainAt(wx-s.from.Pos.X, wy-s.from.Pos.Y), false
	}
	if s.from.Isolated {
		return 0, true
	}
	o := s.w.LevelAt(wx, wy)
	if o == nil || o.Isolated {
		return 0, true
	}
	return o.TerrainAt(wx-o.Pos.X, wy-o.Pos.Y), false
}

// SolveBorders re-solves the cells on both sides of every edge between l
// and its neighbours: l's outer ring, and each neighbour's cells within one
// tile of l. It returns the neighbours whose ground tiles changed, which
// the editor must rewrite on save (D16). Changed neighbours are recompiled.
func (w *World) SolveBorders(l *Level) []*Level {
	defs := w.Defs
	src := w.SourceFor(l)
	SolveRect(l, 0, 0, l.W, 1, src, defs)
	SolveRect(l, 0, l.H-1, l.W, l.H, src, defs)
	SolveRect(l, 0, 0, 1, l.H, src, defs)
	SolveRect(l, l.W-1, 0, l.W, l.H, src, defs)

	var changed []*Level
	for _, n := range w.Neighbours(l) {
		before := append([]uint16(nil), n.Ground.Tile...)
		beforeUnder := append([]uint16(nil), n.Ground.Under...)
		// l's footprint grown by one, in n's local coordinates.
		x0, y0 := l.Pos.X-1-n.Pos.X, l.Pos.Y-1-n.Pos.Y
		SolveRect(n, x0, y0, x0+l.W+2, y0+l.H+2, w.SourceFor(n), defs)
		if !slices.Equal(before, n.Ground.Tile) || !slices.Equal(beforeUnder, n.Ground.Under) {
			Compile(n, defs)
			changed = append(changed, n)
		}
	}
	return changed
}

// SolveLevel re-solves all of l, including across its edges, and
// recompiles it.
func (w *World) SolveLevel(l *Level) {
	SolveAll(l, w.SourceFor(l), w.Defs)
	Compile(l, w.Defs)
}
