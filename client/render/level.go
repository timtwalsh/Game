package render

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"

	"game/shared"
	"game/shared/world"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Pass is when a layer is drawn relative to players, decided by its
// z-range (docs/LEVEL_MAKER_SPEC.md, "Layers").
type Pass int

const (
	PassUnder    Pass = iota // ground, decor (z < 11): under everything
	PassYSort                // z 11-50: sorted with players by foot position
	PassOverhead             // z > 50: over everything
)

// LayerPass classifies a layer by where its z-range starts.
func LayerPass(l world.LayerInfo) Pass {
	switch {
	case l.ZMin < 11:
		return PassUnder
	case l.ZMin <= 50:
		return PassYSort
	}
	return PassOverhead
}

// Atlas holds one texture per tile sheet and finds the source rectangle
// for a global tile index.
type Atlas struct {
	defs *world.Defs
	tex  map[string]rl.Texture2D // by sheet name; missing if it failed to load
}

// NewAtlas loads every sheet in defs, with paths relative to root. It needs
// the window open. A sheet that won't load is reported and its tiles draw
// as magenta squares, so a missing PNG is obvious but not fatal.
func NewAtlas(defs *world.Defs, root string) (*Atlas, []error) {
	a := &Atlas{defs: defs, tex: map[string]rl.Texture2D{}}
	var problems []error
	for _, s := range defs.Sheets {
		path := filepath.Join(root, filepath.FromSlash(s.Path))
		t := rl.LoadTexture(path)
		if !rl.IsTextureValid(t) {
			problems = append(problems, fmt.Errorf("tile sheet %q: can't load %s", s.Name, path))
			continue
		}
		rl.SetTextureFilter(t, rl.FilterPoint) // pixel art: never smooth
		a.tex[s.Name] = t
	}
	return a, problems
}

// Unload frees every texture.
func (a *Atlas) Unload() {
	for _, t := range a.tex {
		rl.UnloadTexture(t)
	}
}

// SourceRect is the rectangle of a sheet cell, in sheet pixels, for a sheet
// cols cells wide. Flip flags mirror it the way raylib does: a negative
// width or height.
func SourceRect(cell, cols int, flags uint8) rl.Rectangle {
	ts := shared.TileSize
	r := rl.NewRectangle(float32(cell%cols)*ts, float32(cell/cols)*ts, ts, ts)
	if flags&world.TileFlipH != 0 {
		r.Width = -r.Width
	}
	if flags&world.TileFlipV != 0 {
		r.Height = -r.Height
	}
	return r
}

var missingTile = rl.NewColor(255, 0, 255, 255)

// Draw draws one tile with its top-left at world pixel (x, y). Flips are
// ignored for tiles that may not flip (D4).
func (a *Atlas) Draw(tile uint16, flags uint8, x, y float32) {
	a.DrawRect(tile, flags, rl.NewRectangle(x, y, shared.TileSize, shared.TileSize))
}

// DrawRect draws one tile stretched over dst, e.g. a palette swatch.
func (a *Atlas) DrawRect(tile uint16, flags uint8, dst rl.Rectangle) {
	if tile == 0 {
		return
	}
	s, cell := a.defs.SheetOf(tile)
	if s == nil {
		rl.DrawRectangleRec(dst, missingTile)
		return
	}
	t, ok := a.tex[s.Name]
	if !ok {
		rl.DrawRectangleRec(dst, missingTile)
		return
	}
	if !a.defs.MayFlip(tile) {
		flags &^= world.TileFlipH | world.TileFlipV
	}
	cols := max(1, int(t.Width)/int(shared.TileSize))
	rl.DrawTexturePro(t, SourceRect(cell, cols, flags), dst, rl.Vector2{}, 0, rl.White)
}

// Item is one thing in the y-sorted pass: a ysort-layer tile or a player.
// Items are drawn in order of Key, the world y of their bottom edge (a
// tile's cell bottom, a player's feet), so whatever is lower on screen is
// in front.
type Item struct {
	Key float32
	// Player is the caller's index for a player item, or -1 for a tile.
	Player int
	Tile   uint16
	Flags  uint8
	X, Y   float32 // a tile's top-left, world pixels
}

// SortItems orders items back to front. The sort is stable, so for equal
// keys a tile appended before a player stays behind it.
func SortItems(items []Item) {
	slices.SortStableFunc(items, func(a, b Item) int { return cmp.Compare(a.Key, b.Key) })
}

// Renderer draws a world's levels through an atlas.
type Renderer struct {
	World *world.World
	Atlas *Atlas
}

// eachCell calls fn for every cell of every level inside view, with its
// level, grid index and world pixel position.
func (r *Renderer) eachCell(view TileRect, fn func(l *world.Level, i int, x, y float32)) {
	for _, l := range r.World.Levels {
		x0, y0 := max(view.X0, l.Pos.X), max(view.Y0, l.Pos.Y)
		x1, y1 := min(view.X1, l.Pos.X+l.W), min(view.Y1, l.Pos.Y+l.H)
		for wy := y0; wy < y1; wy++ {
			for wx := x0; wx < x1; wx++ {
				i := l.Index(wx-l.Pos.X, wy-l.Pos.Y)
				fn(l, i, float32(wx)*shared.TileSize, float32(wy)*shared.TileSize)
			}
		}
	}
}

// drawUpper draws every upper layer in the given pass, in layer order.
func (r *Renderer) drawUpper(view TileRect, pass Pass) {
	r.eachCell(view, func(l *world.Level, i int, x, y float32) {
		for j, g := range l.Upper {
			if LayerPass(l.Layers[j+1]) == pass {
				r.Atlas.Draw(g.Tile[i], g.Flags[i], x, y)
			}
		}
	})
}

// DrawUnder draws the ground (underlay, mid, then tile) and every layer below
// the ysort range, such as decor.
func (r *Renderer) DrawUnder(view TileRect) {
	r.eachCell(view, func(l *world.Level, i int, x, y float32) {
		r.Atlas.Draw(l.Ground.Under[i], 0, x, y)
		r.Atlas.Draw(l.Ground.Mid[i], 0, x, y)
		r.Atlas.Draw(l.Ground.Tile[i], 0, x, y)
	})
	r.drawUpper(view, PassUnder)
}

// AppendYSort appends an item for every visible tile on a ysort layer.
func (r *Renderer) AppendYSort(dst []Item, view TileRect) []Item {
	r.eachCell(view, func(l *world.Level, i int, x, y float32) {
		for j, g := range l.Upper {
			if g.Tile[i] != 0 && LayerPass(l.Layers[j+1]) == PassYSort {
				dst = append(dst, Item{Key: y + shared.TileSize, Player: -1, Tile: g.Tile[i], Flags: g.Flags[i], X: x, Y: y})
			}
		}
	})
	return dst
}

// DrawItem draws a tile item; player items are the caller's to draw.
func (r *Renderer) DrawItem(it Item) {
	r.Atlas.Draw(it.Tile, it.Flags, it.X, it.Y)
}

// DrawOverhead draws every layer above the ysort range: canopies, roofs.
func (r *Renderer) DrawOverhead(view TileRect) {
	r.drawUpper(view, PassOverhead)
}
