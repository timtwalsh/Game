package world

import (
	"slices"
	"sort"
)

// Neighbour bits of an autotile mask.
const (
	MaskN  uint8 = 1
	MaskNE uint8 = 2
	MaskE  uint8 = 4
	MaskSE uint8 = 8
	MaskS  uint8 = 16
	MaskSW uint8 = 32
	MaskW  uint8 = 64
	MaskNW uint8 = 128
)

// neighbours lists the 8 offsets in bit order (N, NE, E, SE, S, SW, W, NW),
// with y growing down.
var neighbours = [8]struct {
	dx, dy int
	bit    uint8
}{
	{0, -1, MaskN}, {1, -1, MaskNE}, {1, 0, MaskE}, {1, 1, MaskSE},
	{0, 1, MaskS}, {-1, 1, MaskSW}, {-1, 0, MaskW}, {-1, -1, MaskNW},
}

// FullState is the state of a cell joined on all sides: the plain centre
// tile, which is also what an underlay draws.
const FullState = BlobTileCount - 1

var (
	blobStates  []uint8    // the 47 reduced masks, ascending; index = sheet cell
	stateOfMask [256]uint8 // raw mask -> state (sheet cell)
)

func init() {
	seen := map[uint8]bool{}
	for m := 0; m < 256; m++ {
		r := ReduceMask(uint8(m))
		if !seen[r] {
			seen[r] = true
			blobStates = append(blobStates, r)
		}
	}
	sort.Slice(blobStates, func(i, j int) bool { return blobStates[i] < blobStates[j] })
	index := map[uint8]uint8{}
	for i, r := range blobStates {
		index[r] = uint8(i)
	}
	for m := 0; m < 256; m++ {
		stateOfMask[m] = index[ReduceMask(uint8(m))]
	}
}

// ReduceMask drops each corner bit whose two adjacent edge bits aren't
// both set: that corner is already covered by an edge. This maps the 256
// raw masks onto exactly 47.
func ReduceMask(m uint8) uint8 {
	corner := func(c, a, b uint8) {
		if m&a == 0 || m&b == 0 {
			m &^= c
		}
	}
	corner(MaskNE, MaskN, MaskE)
	corner(MaskSE, MaskS, MaskE)
	corner(MaskSW, MaskS, MaskW)
	corner(MaskNW, MaskN, MaskW)
	return m
}

// BlobStates returns the 47 reduced masks in sheet order: cell i of a
// blob-47 sheet draws mask BlobStates()[i]. The template generator draws
// from this so art and solver can't disagree.
func BlobStates() []uint8 { return append([]uint8(nil), blobStates...) }

// StateOf returns the sheet cell for a raw (unreduced) neighbour mask.
func StateOf(mask uint8) int { return int(stateOfMask[mask]) }

// TerrainSource answers "what terrain is at this world tile" for the
// solver, so one implementation solves a lone level and across level edges.
// void is true for cells outside every level the source can see; void
// counts as joined (D20). terrain 0 with void false is an empty cell inside
// a level, which does not.
type TerrainSource interface {
	TerrainAt(wx, wy int) (terrain uint8, void bool)
}

// LevelSource sees one level alone: everything outside it is void.
type LevelSource struct{ L *Level }

func (s LevelSource) TerrainAt(wx, wy int) (uint8, bool) {
	l := s.L
	if !l.ContainsWorld(wx, wy) {
		return 0, true
	}
	return l.TerrainAt(wx-l.Pos.X, wy-l.Pos.Y), false
}

// SolveCell picks the ground tile and what draws beneath it for
// level-local (x, y). Locked cells (D3) are left alone; they still take
// part in their neighbours' masks through their terrain.
func SolveCell(l *Level, x, y int, src TerrainSource, defs *Defs) {
	if !l.In(x, y) {
		return
	}
	i := l.Index(x, y)
	if l.Ground.Flags[i]&GroundLocked != 0 {
		return
	}
	tile, mid, under := solve(l.Ground.Terrain[i], l.Pos.X+x, l.Pos.Y+y, src, defs)
	l.Ground.Tile[i] = tile
	l.Ground.Mid[i] = mid
	l.Ground.Under[i] = under
}

// solve returns the cell's own tile and the stack beneath it. Every lower
// terrain around the cell is drawn as it would be here, with its own edge
// tile, lowest first: under is the lowest drawn terrain's full centre, mid
// the next one's edge tile. So where grass crosses a dirt/water border the
// dirt's edge carries on under the grass instead of filling the cell.
func solve(id uint8, wx, wy int, src TerrainSource, defs *Defs) (tile, mid, under uint16) {
	t := defs.Terrain(id)
	if t == nil {
		return 0, 0, 0 // empty, or a terrain these defs don't know
	}
	if !t.Edges {
		return t.TileIndex, 0, 0
	}
	var around [8]*Terrain // nil: empty; void is voidTerrain
	var lower []*Terrain   // distinct lower terrains, highest first
	for k, n := range neighbours {
		nid, void := src.TerrainAt(wx+n.dx, wy+n.dy)
		if void {
			around[k] = &voidTerrain
			continue
		}
		nt := defs.Terrain(nid)
		around[k] = nt
		if nt != nil && nt.Edges && nt.Priority < t.Priority && !slices.Contains(lower, nt) {
			lower = append(lower, nt)
		}
	}
	slices.SortFunc(lower, func(a, b *Terrain) int { return b.Priority - a.Priority })

	// maskFor is the reduced mask of a terrain at priority p drawn here.
	maskFor := func(p int) uint8 {
		var m uint8
		for k, n := range neighbours {
			if a := around[k]; a != nil && (!a.Edges || a.Priority >= p) {
				m |= n.bit
			}
		}
		return ReduceMask(m)
	}
	above := maskFor(t.Priority)
	tile = t.SheetBase + uint16(StateOf(above))

	// A lower terrain whose mask matches the one above it is entirely
	// covered here (it only touches at a corner an edge already hides), so
	// it's skipped. Masks only grow going down, so the lowest drawn one is
	// near-full and draws as the plain centre.
	var drawn []*Terrain // top-down
	var edge uint8       // mask of drawn[0]
	for _, lt := range lower {
		m := maskFor(lt.Priority)
		if m == above {
			continue
		}
		if len(drawn) == 0 {
			edge = m
		}
		drawn = append(drawn, lt)
		above = m
	}
	switch len(drawn) {
	case 0:
	case 1:
		under = drawn[0].SheetBase + FullState
	default:
		// Only one edge layer fits between under and tile; with four or
		// more terrains around one cell, the lowest ones are dropped.
		mid = drawn[0].SheetBase + uint16(StateOf(edge))
		under = drawn[1].SheetBase + FullState
	}
	return tile, mid, under
}

// voidTerrain stands in for world void around a cell: joined to everything.
var voidTerrain = Terrain{Edges: false}

// SolveRect solves every cell in the level-local rectangle [x0,x1) x
// [y0,y1), clipped to the level. After painting a cell, solve it and its 8
// neighbours (SolveAround); re-solving is always local.
func SolveRect(l *Level, x0, y0, x1, y1 int, src TerrainSource, defs *Defs) {
	x0, y0 = max(x0, 0), max(y0, 0)
	x1, y1 = min(x1, l.W), min(y1, l.H)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			SolveCell(l, x, y, src, defs)
		}
	}
}

// SolveAround re-solves a painted cell and its 8 neighbours.
func SolveAround(l *Level, x, y int, src TerrainSource, defs *Defs) {
	SolveRect(l, x-1, y-1, x+2, y+2, src, defs)
}

// SolveAll re-solves the whole ground layer.
func SolveAll(l *Level, src TerrainSource, defs *Defs) {
	SolveRect(l, 0, 0, l.W, l.H, src, defs)
}
