// Package render draws levels: a tile atlas built from the world's sheets,
// the shared camera (D33), and culled per-layer drawing, with the ysort
// layer emitted as items the caller sorts together with players. It is
// used by the client and, later, the level maker, so both look the same.
//
// See docs/LEVEL_MAKER_SPEC.md, build-order step 3.
package render

import (
	"math"

	"game/shared"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// CameraMode picks how the camera moves (D33).
type CameraMode int

const (
	// Follow tracks a target (the local player), within the player's zoom
	// limits. Zoom steps are whole numbers, so tiles stay pixel-exact.
	Follow CameraMode = iota
	// Free flies anywhere, ignoring walls, and zooms out further than a
	// player can: the level maker's view, and later a spectator's.
	Free
)

// Camera is the one camera shared by the game and the tools. Target is the
// world pixel shown at the centre of the screen.
type Camera struct {
	Mode    CameraMode
	Target  shared.Vec2
	Zoom    float32
	MinZoom float32
	MaxZoom float32
}

// NewFollowCamera is the game's player camera.
func NewFollowCamera() Camera {
	return Camera{Mode: Follow, Zoom: 2, MinZoom: 1, MaxZoom: 4}
}

// NewFreeCamera is the level maker's camera.
func NewFreeCamera() Camera {
	return Camera{Mode: Free, Zoom: 1, MinZoom: 0.125, MaxZoom: 8}
}

// CentreOn puts p at the centre of the screen. Call it each frame with the
// followed target (the local player in Follow mode; any chosen target, or
// a jump-to spot, in Free mode).
func (c *Camera) CentreOn(p shared.Vec2) { c.Target = p }

// Pan moves the camera by a screen-space distance (e.g. a mouse drag).
func (c *Camera) Pan(dxScreen, dyScreen float32) {
	c.Target.X += dxScreen / c.Zoom
	c.Target.Y += dyScreen / c.Zoom
}

// ZoomBy changes zoom by wheel steps. Follow mode moves in whole steps
// (1x, 2x, 3x...) so tiles land on whole screen pixels; Free mode scales
// smoothly by 2^(steps/4).
func (c *Camera) ZoomBy(steps float32) {
	if steps == 0 {
		return
	}
	if c.Mode == Follow {
		if steps > 0 {
			c.Zoom = float32(math.Floor(float64(c.Zoom))) + 1
		} else {
			c.Zoom = float32(math.Ceil(float64(c.Zoom))) - 1
		}
	} else {
		c.Zoom *= float32(math.Pow(2, float64(steps)/4))
	}
	c.Zoom = min(max(c.Zoom, c.MinZoom), c.MaxZoom)
}

// Raylib returns the raylib camera for a screen of the given size. The
// target is snapped to whole screen pixels, so tiles don't shimmer or
// open seams as the camera moves.
func (c *Camera) Raylib(screenW, screenH int) rl.Camera2D {
	snap := func(v float32) float32 { return float32(math.Round(float64(v*c.Zoom))) / c.Zoom }
	return rl.Camera2D{
		Offset: rl.NewVector2(float32(screenW/2), float32(screenH/2)),
		Target: rl.NewVector2(snap(c.Target.X), snap(c.Target.Y)),
		Zoom:   c.Zoom,
	}
}

// ScreenToWorld converts a screen position to world pixels.
func (c *Camera) ScreenToWorld(sx, sy float32, screenW, screenH int) shared.Vec2 {
	return shared.Vec2{
		X: c.Target.X + (sx-float32(screenW)/2)/c.Zoom,
		Y: c.Target.Y + (sy-float32(screenH)/2)/c.Zoom,
	}
}

// TileRect is a half-open rectangle of world tiles: [X0, X1) x [Y0, Y1).
type TileRect struct{ X0, Y0, X1, Y1 int }

// Visible is the world tile rectangle the camera can see on a screen of
// the given size, plus a margin of one tile so tall ysort art that starts
// just off-screen still draws.
func (c *Camera) Visible(screenW, screenH int) TileRect {
	tl := c.ScreenToWorld(0, 0, screenW, screenH)
	br := c.ScreenToWorld(float32(screenW), float32(screenH), screenW, screenH)
	floor := func(v float32) int { return int(math.Floor(float64(v / shared.TileSize))) }
	return TileRect{floor(tl.X) - 1, floor(tl.Y) - 1, floor(br.X) + 2, floor(br.Y) + 2}
}
