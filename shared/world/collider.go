package world

import "game/shared"

// Collider is what movement code needs from the map: client prediction and
// the server's movement validation both take one, so they can't disagree
// about where a player may go or how fast. Coordinates are signed world
// tiles (TileOf converts from pixels).
type Collider interface {
	// Blocks reports whether any of flag's bits are blocked at a tile.
	// Walking tests BlockGround.
	Blocks(tx, ty int, flag uint16) bool
	// SpeedMultiplier is the tile's interaction speed multiplier (1 for
	// normal ground, 0.5 for swimming...). It must be honoured by client
	// prediction and the server's speed check alike.
	SpeedMultiplier(tx, ty int) float32
}

// SpeedMultiplier implements Collider: the speed multiplier of the cell's
// compiled interaction. Void is 1 (it's blocked anyway).
func (w *World) SpeedMultiplier(tx, ty int) float32 {
	if w.LevelAt(tx, ty) == nil {
		return 1
	}
	if it := w.Defs.Interaction(w.InteractionAt(tx, ty)); it != nil {
		return it.SpeedMultiplier
	}
	return 1
}

// Grid is a minimal Collider: an open W x H area at the origin, blocked
// outside, with per-tile blocking and speed that can be set by hand. It is
// the fallback when there are no level files (the old blank 100x100 grid),
// and a test helper.
type Grid struct {
	W, H     int
	blocking []uint16
	speed    []float32
}

// NewGrid makes an open grid: nothing blocks inside, speed 1 everywhere.
func NewGrid(w, h int) *Grid {
	g := &Grid{W: w, H: h, blocking: make([]uint16, w*h), speed: make([]float32, w*h)}
	for i := range g.speed {
		g.speed[i] = 1
	}
	return g
}

func (g *Grid) in(tx, ty int) bool { return tx >= 0 && ty >= 0 && tx < g.W && ty < g.H }

// SetBlocking sets a tile's blocking flags.
func (g *Grid) SetBlocking(tx, ty int, flags uint16) {
	if g.in(tx, ty) {
		g.blocking[ty*g.W+tx] = flags
	}
}

// SetSpeed sets a tile's speed multiplier.
func (g *Grid) SetSpeed(tx, ty int, mult float32) {
	if g.in(tx, ty) {
		g.speed[ty*g.W+tx] = mult
	}
}

func (g *Grid) Blocks(tx, ty int, flag uint16) bool {
	if !g.in(tx, ty) {
		return true
	}
	return g.blocking[ty*g.W+tx]&flag != 0
}

func (g *Grid) SpeedMultiplier(tx, ty int) float32 {
	if !g.in(tx, ty) {
		return 1
	}
	return g.speed[ty*g.W+tx]
}

// Spawn returns where new players start, in world pixels: the first
// `spawn` object in the exterior levels (by level name, then object
// order). ok is false when there is none, and callers use
// shared.SpawnPoint. The server creates players here and validates their
// first move from it, and the client starts its prediction here, so both
// must read it from the same files.
func (w *World) Spawn() (pos shared.Vec2, ok bool) {
	for _, l := range w.Levels {
		if l.Isolated {
			continue
		}
		for _, o := range l.Objects {
			if o.Kind == "spawn" {
				return shared.Vec2{
					X: float32(l.Pos.X)*shared.TileSize + o.X,
					Y: float32(l.Pos.Y)*shared.TileSize + o.Y,
				}, true
			}
		}
	}
	return shared.Vec2{}, false
}

// Map is what the client and server load at startup: the world's collider
// and spawn point, plus the World itself when level files exist.
type Map struct {
	World    *World // nil when running on the fallback grid
	Collider Collider
	Spawn    shared.Vec2
}

// FallbackSize is the blank grid used when there are no level files.
const FallbackSize = 100

// LoadMap loads the world under root (root/world/terrains.toml and
// root/levels/). With neither present it returns the open fallback grid
// and shared.SpawnPoint, so the game runs before any level exists. A world
// that is present but broken is an error: running the client and server
// on different geometry would flag honest players.
func LoadMap(root string) (*Map, error) {
	w, ok, err := LoadDir(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &Map{Collider: NewGrid(FallbackSize, FallbackSize), Spawn: shared.SpawnPoint}, nil
	}
	spawn, found := w.Spawn()
	if !found {
		spawn = shared.SpawnPoint
	}
	return &Map{World: w, Collider: w, Spawn: spawn}, nil
}
