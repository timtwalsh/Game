// Package world is the level/world model shared by the client, the server
// and the level maker: definitions (terrains, tile sheets, cell property
// types), levels and their file format, the blob-47 autotile solver, the
// property compiler, and a World that answers queries across level edges.
//
// It is pure Go with no raylib dependency, so the server can import it.
// The design is docs/LEVEL_MAKER_SPEC.md; decision numbers (D3, D30...) in
// comments refer to that document's decision table.
package world

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Blocking flags (D29). A cell's compiled blocking value is the OR of
// everything in it that blocks (D30). 9 bits are spare; adding a flag needs
// no format change.
const (
	BlockGround     uint16 = 1 << 0
	BlockProjectile uint16 = 1 << 1
	BlockFlight     uint16 = 1 << 2
	BlockJump       uint16 = 1 << 3
	BlockRoll       uint16 = 1 << 4
	BlockEthereal   uint16 = 1 << 5
	BlockMagic      uint16 = 1 << 6

	// BlockAll is what void and empty ground compile to.
	BlockAll uint16 = 0xFFFF
)

var blockNames = map[string]uint16{
	"ground":     BlockGround,
	"projectile": BlockProjectile,
	"flight":     BlockFlight,
	"jump":       BlockJump,
	"roll":       BlockRoll,
	"ethereal":   BlockEthereal,
	"magic":      BlockMagic,
}

// ParseBlocks turns a list of flag names into a bit mask.
func ParseBlocks(names []string) (uint16, error) {
	var mask uint16
	for _, n := range names {
		bit, ok := blockNames[n]
		if !ok {
			return 0, fmt.Errorf("unknown blocking flag %q", n)
		}
		mask |= bit
	}
	return mask, nil
}

// BlobTileCount is how many cells a terrain's blob-47 sheet must have.
const BlobTileCount = 47

// Sheet is a PNG cut into 16x16 cells, numbered left-to-right,
// top-to-bottom. Cell c has the global tile index Base+c.
type Sheet struct {
	Name      string `toml:"name"`
	Path      string `toml:"path"`
	Base      uint16 `toml:"base"`
	Cells     int    `toml:"cells"`     // how many cells are in use; reserves Base..Base+Cells-1
	Flippable bool   `toml:"flippable"` // D4: default for every cell on this sheet
}

// Terrain is a ground-layer material (D9, D23).
type Terrain struct {
	ID          uint8    `toml:"id"`
	Name        string   `toml:"name"`
	Priority    int      `toml:"priority"` // higher draws over lower at a border
	Sheet       string   `toml:"sheet"`    // blob-47 sheet, for terrains with edges
	Tile        string   `toml:"tile"`     // "sheet:cell", for edges = false
	EdgesOpt    *bool    `toml:"edges"`    // default true
	BlocksNames []string `toml:"blocks"`
	Surface     string   `toml:"surface"`
	Interaction string   `toml:"interaction"`

	// Resolved by LoadDefs.
	Edges         bool   `toml:"-"`
	Blocks        uint16 `toml:"-"`
	SurfaceID     uint8  `toml:"-"`
	InteractionID uint8  `toml:"-"`
	SheetBase     uint16 `toml:"-"` // first tile of the blob sheet (Edges)
	TileIndex     uint16 `toml:"-"` // the single tile (!Edges)
}

// Surface drives footstep sound and effects. Client-only (D32).
type Surface struct {
	ID          uint8  `toml:"id"`
	Name        string `toml:"name"`
	FootstepSFX string `toml:"footstep_sfx"`
	Effect      string `toml:"effect"`
}

// Interaction says how movement behaves on a cell. Server-authoritative
// (D32): client prediction and the server's speed check must apply the same
// SpeedMultiplier.
type Interaction struct {
	ID              uint8   `toml:"id"`
	Name            string  `toml:"name"`
	SpeedMultiplier float32 `toml:"speed_multiplier"`
}

// tomlTileProps is one [[tile_props]] entry: defaults for some cells of a
// sheet (D25).
type tomlTileProps struct {
	Sheet       string   `toml:"sheet"`
	Cells       []int    `toml:"cells"`
	Blocks      []string `toml:"blocks"`
	Surface     string   `toml:"surface"`
	Interaction string   `toml:"interaction"`
	Flippable   *bool    `toml:"flippable"` // overrides the sheet's default (D4)
}

// TileProps is what a placed tile contributes to its cell's properties.
type TileProps struct {
	Blocks         uint16
	HasSurface     bool
	Surface        uint8
	HasInteraction bool
	Interaction    uint8
}

type tomlDefs struct {
	Sheets       []Sheet         `toml:"sheets"`
	Terrains     []Terrain       `toml:"terrains"`
	Surfaces     []Surface       `toml:"surfaces"`
	Interactions []Interaction   `toml:"interactions"`
	TileProps    []tomlTileProps `toml:"tile_props"`
}

// Defs is world/terrains.toml, validated and indexed. It is shared by every
// level, because autotiling across a level edge only works if both sides
// agree on what each terrain is.
type Defs struct {
	Sheets       []Sheet
	Terrains     []Terrain
	Surfaces     []Surface
	Interactions []Interaction

	terrainByID     [256]*Terrain
	terrainByName   map[string]*Terrain
	sheetByName     map[string]*Sheet
	surfaceByID     [256]*Surface
	surfaceByName   map[string]uint8
	interactionByID [256]*Interaction
	interactByName  map[string]uint8
	tileProps       map[uint16]TileProps
	flipOverride    map[uint16]bool
}

// LoadDefs reads and validates a terrains.toml file.
func LoadDefs(path string) (*Defs, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d, err := ParseDefs(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return d, nil
}

// ParseDefs decodes and validates terrains.toml content.
func ParseDefs(data []byte) (*Defs, error) {
	var raw tomlDefs
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, err
	}
	if und := md.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("unknown key %q", und[0].String())
	}
	d := &Defs{
		Sheets:         raw.Sheets,
		Terrains:       raw.Terrains,
		Surfaces:       raw.Surfaces,
		Interactions:   raw.Interactions,
		terrainByName:  map[string]*Terrain{},
		sheetByName:    map[string]*Sheet{},
		surfaceByName:  map[string]uint8{},
		interactByName: map[string]uint8{},
		tileProps:      map[uint16]TileProps{},
		flipOverride:   map[uint16]bool{},
	}
	if err := d.indexSheets(); err != nil {
		return nil, err
	}
	if err := d.indexProperties(); err != nil {
		return nil, err
	}
	if err := d.indexTerrains(); err != nil {
		return nil, err
	}
	if err := d.indexTileProps(raw.TileProps); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Defs) indexSheets() error {
	for i := range d.Sheets {
		s := &d.Sheets[i]
		if s.Name == "" {
			return fmt.Errorf("sheet %d has no name", i)
		}
		if _, dup := d.sheetByName[s.Name]; dup {
			return fmt.Errorf("duplicate sheet %q", s.Name)
		}
		if s.Base == 0 {
			return fmt.Errorf("sheet %q: base must be at least 1 (tile 0 means no tile)", s.Name)
		}
		if s.Cells <= 0 {
			return fmt.Errorf("sheet %q: cells must be positive", s.Name)
		}
		if int(s.Base)+s.Cells-1 > 0xFFFF {
			return fmt.Errorf("sheet %q: tiles run past 65535", s.Name)
		}
		for _, o := range d.sheetByName {
			if int(s.Base) <= o.last() && int(o.Base) <= s.last() {
				return fmt.Errorf("sheets %q and %q have overlapping tile ranges", o.Name, s.Name)
			}
		}
		d.sheetByName[s.Name] = s
	}
	return nil
}

func (s *Sheet) last() int { return int(s.Base) + s.Cells - 1 }

func (d *Defs) indexProperties() error {
	for i := range d.Surfaces {
		s := &d.Surfaces[i]
		if d.surfaceByID[s.ID] != nil {
			return fmt.Errorf("duplicate surface id %d", s.ID)
		}
		if _, dup := d.surfaceByName[s.Name]; dup || s.Name == "" {
			return fmt.Errorf("surface %d: name %q is empty or duplicated", s.ID, s.Name)
		}
		d.surfaceByID[s.ID] = s
		d.surfaceByName[s.Name] = s.ID
	}
	for i := range d.Interactions {
		it := &d.Interactions[i]
		if d.interactionByID[it.ID] != nil {
			return fmt.Errorf("duplicate interaction id %d", it.ID)
		}
		if _, dup := d.interactByName[it.Name]; dup || it.Name == "" {
			return fmt.Errorf("interaction %d: name %q is empty or duplicated", it.ID, it.Name)
		}
		if it.SpeedMultiplier <= 0 {
			return fmt.Errorf("interaction %q: speed_multiplier must be positive", it.Name)
		}
		d.interactionByID[it.ID] = it
		d.interactByName[it.Name] = it.ID
	}
	// Id 0 is what a cell compiles to when nothing declares a value.
	if d.surfaceByID[0] == nil {
		return fmt.Errorf("surface id 0 (the default, usually \"none\") is missing")
	}
	if d.interactionByID[0] == nil {
		return fmt.Errorf("interaction id 0 (the default, usually \"normal\") is missing")
	}
	return nil
}

func (d *Defs) indexTerrains() error {
	priorities := map[int]string{}
	for i := range d.Terrains {
		t := &d.Terrains[i]
		if t.ID == 0 {
			return fmt.Errorf("terrain %q: id 0 is reserved for empty", t.Name)
		}
		if d.terrainByID[t.ID] != nil {
			return fmt.Errorf("duplicate terrain id %d", t.ID)
		}
		if _, dup := d.terrainByName[t.Name]; dup || t.Name == "" {
			return fmt.Errorf("terrain %d: name %q is empty or duplicated", t.ID, t.Name)
		}
		t.Edges = t.EdgesOpt == nil || *t.EdgesOpt
		var err error
		if t.Blocks, err = ParseBlocks(t.BlocksNames); err != nil {
			return fmt.Errorf("terrain %q: %w", t.Name, err)
		}
		if t.SurfaceID, err = d.SurfaceID(t.Surface); err != nil {
			return fmt.Errorf("terrain %q: %w", t.Name, err)
		}
		if t.InteractionID, err = d.InteractionID(t.Interaction); err != nil {
			return fmt.Errorf("terrain %q: %w", t.Name, err)
		}
		if t.Edges {
			s := d.sheetByName[t.Sheet]
			if s == nil {
				return fmt.Errorf("terrain %q: unknown sheet %q", t.Name, t.Sheet)
			}
			if s.Cells < BlobTileCount {
				return fmt.Errorf("terrain %q: sheet %q has %d cells, a blob-47 sheet needs %d", t.Name, s.Name, s.Cells, BlobTileCount)
			}
			t.SheetBase = s.Base
			// Equal priorities would join both ways and draw no border at
			// all, which is never what "who draws on top" means.
			if other, dup := priorities[t.Priority]; dup {
				return fmt.Errorf("terrains %q and %q share priority %d", other, t.Name, t.Priority)
			}
			priorities[t.Priority] = t.Name
		} else {
			if t.Tile == "" {
				return fmt.Errorf("terrain %q: edges = false needs a tile", t.Name)
			}
			if t.TileIndex, err = d.ResolveTile(t.Tile); err != nil {
				return fmt.Errorf("terrain %q: %w", t.Name, err)
			}
		}
		d.terrainByID[t.ID] = t
		d.terrainByName[t.Name] = t
	}
	return nil
}

func (d *Defs) indexTileProps(entries []tomlTileProps) error {
	for i, e := range entries {
		s := d.sheetByName[e.Sheet]
		if s == nil {
			return fmt.Errorf("tile_props %d: unknown sheet %q", i, e.Sheet)
		}
		var p TileProps
		var err error
		if p.Blocks, err = ParseBlocks(e.Blocks); err != nil {
			return fmt.Errorf("tile_props %d: %w", i, err)
		}
		if e.Surface != "" {
			p.HasSurface = true
			if p.Surface, err = d.SurfaceID(e.Surface); err != nil {
				return fmt.Errorf("tile_props %d: %w", i, err)
			}
		}
		if e.Interaction != "" {
			p.HasInteraction = true
			if p.Interaction, err = d.InteractionID(e.Interaction); err != nil {
				return fmt.Errorf("tile_props %d: %w", i, err)
			}
		}
		for _, c := range e.Cells {
			if c < 0 || c >= s.Cells {
				return fmt.Errorf("tile_props %d: sheet %q has no cell %d", i, s.Name, c)
			}
			idx := s.Base + uint16(c)
			if _, dup := d.tileProps[idx]; dup {
				return fmt.Errorf("tile_props: %s:%d is listed twice", s.Name, c)
			}
			d.tileProps[idx] = p
			if e.Flippable != nil {
				d.flipOverride[idx] = *e.Flippable
			}
		}
	}
	return nil
}

// Terrain returns the terrain with this id, or nil (including for 0, empty).
func (d *Defs) Terrain(id uint8) *Terrain { return d.terrainByID[id] }

// TerrainByName returns the named terrain, or nil.
func (d *Defs) TerrainByName(name string) *Terrain { return d.terrainByName[name] }

// SheetByName returns the named sheet, or nil.
func (d *Defs) SheetByName(name string) *Sheet { return d.sheetByName[name] }

// Surface returns the surface with this id, or nil.
func (d *Defs) Surface(id uint8) *Surface { return d.surfaceByID[id] }

// Interaction returns the interaction with this id, or nil.
func (d *Defs) Interaction(id uint8) *Interaction { return d.interactionByID[id] }

// SurfaceID resolves a surface name; "" is id 0.
func (d *Defs) SurfaceID(name string) (uint8, error) {
	if name == "" {
		return 0, nil
	}
	id, ok := d.surfaceByName[name]
	if !ok {
		return 0, fmt.Errorf("unknown surface %q", name)
	}
	return id, nil
}

// InteractionID resolves an interaction name; "" is id 0.
func (d *Defs) InteractionID(name string) (uint8, error) {
	if name == "" {
		return 0, nil
	}
	id, ok := d.interactByName[name]
	if !ok {
		return 0, fmt.Errorf("unknown interaction %q", name)
	}
	return id, nil
}

// TileProps returns the property defaults a tile declares (D25); the zero
// value if it declares nothing.
func (d *Defs) TileProps(tile uint16) TileProps { return d.tileProps[tile] }

// SheetOf returns the sheet a global tile index belongs to and its cell on
// that sheet, or nil if no sheet covers it.
func (d *Defs) SheetOf(tile uint16) (*Sheet, int) {
	for i := range d.Sheets {
		s := &d.Sheets[i]
		if int(tile) >= int(s.Base) && int(tile) <= s.last() {
			return s, int(tile - s.Base)
		}
	}
	return nil, 0
}

// MayFlip reports whether a tile may be drawn flipped (D4).
func (d *Defs) MayFlip(tile uint16) bool {
	if v, ok := d.flipOverride[tile]; ok {
		return v
	}
	s, _ := d.SheetOf(tile)
	return s != nil && s.Flippable
}

// ResolveTile turns "sheet:cell" into a global tile index.
func (d *Defs) ResolveTile(ref string) (uint16, error) {
	name, cellStr, ok := strings.Cut(ref, ":")
	if !ok {
		return 0, fmt.Errorf("tile %q is not sheet:cell", ref)
	}
	s := d.sheetByName[name]
	if s == nil {
		return 0, fmt.Errorf("tile %q: unknown sheet %q", ref, name)
	}
	cell, err := strconv.Atoi(cellStr)
	if err != nil || cell < 0 || cell >= s.Cells {
		return 0, fmt.Errorf("tile %q: sheet %q has no cell %q", ref, name, cellStr)
	}
	return s.Base + uint16(cell), nil
}
