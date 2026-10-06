package world

import "sort"

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

// SolveCell picks the ground tile and underlay for level-local (x, y).
// Locked cells (D3) are left alone; they still take part in their
// neighbours' masks through their terrain.
func SolveCell(l *Level, x, y int, src TerrainSource, defs *Defs) {
	if !l.In(x, y) {
		return
	}
	i := l.Index(x, y)
	if l.Ground.Flags[i]&GroundLocked != 0 {
		return
	}
	tile, under := solve(l.Ground.Terrain[i], l.Pos.X+x, l.Pos.Y+y, src, defs)
	l.Ground.Tile[i] = tile
	l.Ground.Under[i] = under
}

func solve(id uint8, wx, wy int, src TerrainSource, defs *Defs) (tile, under uint16) {
	t := defs.Terrain(id)
	if t == nil {
		return 0, 0 // empty, or a terrain these defs don't know
	}
	if !t.Edges {
		return t.TileIndex, 0
	}
	var mask uint8
	var below *Terrain // highest-priority neighbour that draws under t
	for _, n := range neighbours {
		nid, void := src.TerrainAt(wx+n.dx, wy+n.dy)
		if void {
			mask |= n.bit
			continue
		}
		nt := defs.Terrain(nid)
		switch {
		case nt == nil:
			// Empty: not joined, and nothing to show through.
		case !nt.Edges || nt.Priority >= t.Priority:
			mask |= n.bit
		default:
			if below == nil || nt.Priority > below.Priority {
				below = nt
			}
		}
	}
	tile = t.SheetBase + uint16(StateOf(mask))
	if below != nil {
		under = below.SheetBase + FullState
	}
	return tile, under
}

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
