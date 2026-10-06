package world

import (
	"fmt"
	"regexp"

	"game/shared"
)

// WorldBound is how far from the origin, in tiles, a level may reach.
// Positions are float32 world pixels on the wire, which keeps 1/32 px
// precision out to about 16,000 tiles.
const WorldBound = 16000

// Ground cell flags (grid file, ground layer).
const (
	GroundLocked uint8 = 1 << 0 // D3: the solver leaves tile and under alone
)

// Upper-layer cell flags.
const (
	TileFlipH uint8 = 1 << 0
	TileFlipV uint8 = 1 << 1
)

// Override mask bits: which of a cell's three property maps the artist set
// by hand (D29).
const (
	OverrideBlocking    uint8 = 1 << 0
	OverrideSurface     uint8 = 1 << 1
	OverrideInteraction uint8 = 1 << 2
)

// LayerInfo is one entry of a level's ordered layer list (D5).
type LayerInfo struct {
	Name string
	ZMin uint32
	ZMax uint32
}

// DefaultLayers is the layer set a new level starts with. Ground is always
// first (D23).
func DefaultLayers() []LayerInfo {
	return []LayerInfo{
		{Name: "ground", ZMin: 0, ZMax: 5},
		{Name: "decor", ZMin: 6, ZMax: 10},
		{Name: "ysort", ZMin: 11, ZMax: 50},
		{Name: "overhead", ZMin: 51, ZMax: 70},
	}
}

// GroundGrid is the ground layer: the only layer with terrain (D23).
type GroundGrid struct {
	Terrain []uint8  // 0 = empty
	Tile    []uint16 // 0 = none
	Under   []uint16 // what shows through Tile's transparent parts; 0 = none
	Flags   []uint8  // GroundLocked
}

// TileGrid is any layer above ground: hand-placed tiles only.
type TileGrid struct {
	Tile  []uint16
	Flags []uint8 // TileFlipH, TileFlipV
}

// Overrides are the artist's per-cell property overrides (D29). A value is
// only meaningful where its bit is set in Mask.
type Overrides struct {
	Mask        []uint8
	Blocking    []uint16
	Surface     []uint8
	Interaction []uint8
}

// Props are the compiled per-cell properties (D30). They are derived data:
// Compile rebuilds them from terrain, tiles and overrides.
type Props struct {
	Blocking    []uint16
	Surface     []uint8
	Interaction []uint8
}

// Point is a position in tiles.
type Point struct{ X, Y int }

// Level is one rectangle of the world plane.
type Level struct {
	Name     string
	Pos      Point // world tile coordinates of the top-left cell
	W, H     int
	Isolated bool // interiors (D22): no neighbours, no cross-edge autotile

	Layers    []LayerInfo // Layers[0] is ground
	Ground    GroundGrid
	Upper     []TileGrid // Upper[i] is Layers[i+1]
	Overrides Overrides
	Props     Props

	Objects []shared.GameObject // x/y in level pixels

	// Hash is the version hash (SHA-256 over the TOML bytes then the grid
	// bytes, hex), set by LoadLevel and SaveLevel. It is the streaming cache
	// key (D15).
	Hash string
}

var levelNameRE = regexp.MustCompile(`^[a-z0-9_]+(-[a-z0-9_]+)*$`)

// ValidName reports whether name can be a level name. Names become file
// names, so they're limited to lower-case letters, digits, '_' and '-'.
func ValidName(name string) bool { return levelNameRE.MatchString(name) }

// NewLevel makes an empty level with the default layers. Every ground cell
// is empty, so until painted the whole level blocks everything.
func NewLevel(name string, pos Point, w, h int) (*Level, error) {
	l := &Level{Name: name, Pos: pos, W: w, H: h, Layers: DefaultLayers()}
	if err := l.validateShape(); err != nil {
		return nil, err
	}
	l.allocate()
	return l, nil
}

func (l *Level) validateShape() error {
	if !ValidName(l.Name) {
		return fmt.Errorf("level name %q: use lower-case letters, digits, '_' and '-'", l.Name)
	}
	if l.W <= 0 || l.H <= 0 {
		return fmt.Errorf("level %q: size %dx%d must be positive", l.Name, l.W, l.H)
	}
	if l.Pos.X < -WorldBound || l.Pos.Y < -WorldBound ||
		l.Pos.X+l.W > WorldBound || l.Pos.Y+l.H > WorldBound {
		return fmt.Errorf("level %q: must lie within ±%d tiles of the origin", l.Name, WorldBound)
	}
	if len(l.Layers) == 0 || l.Layers[0].Name != "ground" {
		return fmt.Errorf("level %q: the first layer must be ground", l.Name)
	}
	if len(l.Layers) > 255 {
		return fmt.Errorf("level %q: at most 255 layers", l.Name)
	}
	seen := map[string]bool{}
	for _, ly := range l.Layers {
		if ly.Name == "" || seen[ly.Name] {
			return fmt.Errorf("level %q: layer name %q is empty or duplicated", l.Name, ly.Name)
		}
		if ly.ZMin > ly.ZMax {
			return fmt.Errorf("level %q: layer %q has z_min > z_max", l.Name, ly.Name)
		}
		seen[ly.Name] = true
	}
	return nil
}

// allocate sizes every grid for W x H and len(Layers).
func (l *Level) allocate() {
	n := l.W * l.H
	l.Ground = GroundGrid{
		Terrain: make([]uint8, n),
		Tile:    make([]uint16, n),
		Under:   make([]uint16, n),
		Flags:   make([]uint8, n),
	}
	l.Upper = make([]TileGrid, len(l.Layers)-1)
	for i := range l.Upper {
		l.Upper[i] = TileGrid{Tile: make([]uint16, n), Flags: make([]uint8, n)}
	}
	l.Overrides = Overrides{
		Mask:        make([]uint8, n),
		Blocking:    make([]uint16, n),
		Surface:     make([]uint8, n),
		Interaction: make([]uint8, n),
	}
	l.Props = Props{
		Blocking:    make([]uint16, n),
		Surface:     make([]uint8, n),
		Interaction: make([]uint8, n),
	}
	for i := range l.Props.Blocking {
		l.Props.Blocking[i] = BlockAll
	}
}

// In reports whether level-local (x, y) is inside the level.
func (l *Level) In(x, y int) bool { return x >= 0 && y >= 0 && x < l.W && y < l.H }

// Index is the grid index of level-local (x, y). The caller checks In.
func (l *Level) Index(x, y int) int { return y*l.W + x }

// ContainsWorld reports whether world tile (wx, wy) is inside the level.
func (l *Level) ContainsWorld(wx, wy int) bool { return l.In(wx-l.Pos.X, wy-l.Pos.Y) }

// TerrainAt is the ground terrain at level-local (x, y); 0 outside.
func (l *Level) TerrainAt(x, y int) uint8 {
	if !l.In(x, y) {
		return 0
	}
	return l.Ground.Terrain[l.Index(x, y)]
}

// SetTerrain paints terrain at level-local (x, y). Painting over a locked
// cell unlocks it (D3). It does not re-solve; call SolveRect afterwards.
func (l *Level) SetTerrain(x, y int, terrain uint8) {
	if !l.In(x, y) {
		return
	}
	i := l.Index(x, y)
	l.Ground.Terrain[i] = terrain
	l.Ground.Flags[i] &^= GroundLocked
}

// LayerIndex returns the position of the named layer, or -1.
func (l *Level) LayerIndex(name string) int {
	for i, ly := range l.Layers {
		if ly.Name == name {
			return i
		}
	}
	return -1
}

// rect is a level's footprint in world tiles, half-open.
type rect struct{ x0, y0, x1, y1 int }

func (l *Level) worldRect() rect {
	return rect{l.Pos.X, l.Pos.Y, l.Pos.X + l.W, l.Pos.Y + l.H}
}

func (a rect) overlaps(b rect) bool {
	return a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1
}

// touches reports whether two non-overlapping rects share an edge or a
// corner.
func (a rect) touches(b rect) bool {
	grown := rect{a.x0 - 1, a.y0 - 1, a.x1 + 1, a.y1 + 1}
	return grown.overlaps(b) && !a.overlaps(b)
}
