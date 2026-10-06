package main

import (
	"fmt"
	"game/client/anim"
	"game/shared"
	"os"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// lookDef is how the engine draws one .anichar in the world. These are
// facts about the game's world, not the animation, so they live here
// rather than in animaker's formats.
type lookDef struct {
	path  string  // the .anichar, relative to the assets folder
	scale float32 // screen pixels per animation pixel; the .anichar's scale, if it sets one, wins
	// origin is where the animation's (0,0) sits, in animation pixels
	// from the centre of the player's 16x16 collision box.
	origin shared.Vec2
}

// playerLook is what every player is drawn as. The baby sheet's cells are
// 160x180 with the art ~170px tall, centred on the origin; at 0.3 it
// stands ~50px over a 16px tile.
var playerLook = lookDef{path: "characters/baby/baby.anichar", scale: 0.3}

// characterLook is a loaded lookDef, shared by every player drawn with it.
type characterLook struct {
	lookDef
	char *anim.Character
}

// textureCache loads each sheet image into one texture, shared by every
// character drawing from it. Textures need the window open, so they're
// loaded on first draw rather than with the asset data. It's keyed by the
// Library's shared *Sheet, so a lookup hashes a pointer, not a path.
type textureCache map[*anim.Sheet]rl.Texture2D

func (tc textureCache) get(s *anim.Sheet) rl.Texture2D {
	if t, ok := tc[s]; ok {
		return t
	}
	t := rl.LoadTexture(s.ImagePath)
	// The art is drawn well below its authored size: mipmaps keep the
	// downscale from shimmering as it moves.
	rl.GenTextureMipmaps(&t)
	rl.SetTextureFilter(t, rl.FilterTrilinear)
	tc[s] = t
	return t
}

func (tc textureCache) unload() {
	for _, t := range tc {
		rl.UnloadTexture(t)
	}
}

// drawCharacter draws one resolved frame with the animation's origin at
// (x, y) on screen. As in the editor, each sprite's pivot lands at its
// X, Y from the origin, and it rotates about that pivot.
func (tc textureCache) drawCharacter(sprites []anim.Sprite, x, y, scale float32) {
	for _, s := range sprites {
		sh := s.Sheet
		tex := tc.get(sh)
		src := rl.NewRectangle(float32(s.Col*sh.CellW), float32(s.Row*sh.CellH), float32(sh.CellW), float32(sh.CellH))
		dst := rl.NewRectangle(x+s.X*scale, y+s.Y*scale, float32(sh.CellW)*scale, float32(sh.CellH)*scale)
		origin := rl.NewVector2(sh.PivotX*scale, sh.PivotY*scale)
		rl.DrawTexturePro(tex, src, dst, origin, s.RotationDeg, rl.White)
	}
}

// groundOffset is how far below the animation's origin the character's
// feet are, taken as the bottom of its cells with no keyframe offset - so
// it stays put while a jump moves the art up.
func groundOffset(sprites []anim.Sprite, scale float32) float32 {
	var max float32
	for _, s := range sprites {
		if d := float32(s.Sheet.CellH) - s.Sheet.PivotY; d > max {
			max = d
		}
	}
	return max * scale
}

// findAssets locates the assets folder: beside the working directory (how
// build_local.ps1 starts the client), else beside or above the executable
// (bin/client.exe run from elsewhere).
func findAssets() (string, error) {
	candidates := []string{"assets"}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, "assets"), filepath.Join(dir, "..", "assets"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("no assets folder found (looked in %v)", candidates)
}

// loadLook loads def's .anichar into lib, or returns an error the client
// reports before falling back to drawing circles.
func loadLook(lib *anim.Library, def lookDef) (*characterLook, error) {
	dir, err := findAssets()
	if err != nil {
		return nil, err
	}
	c, err := lib.LoadCharacter(filepath.Join(dir, filepath.FromSlash(def.path)))
	if err != nil {
		return nil, err
	}
	// The .anichar's own scale, which the animaker shows sizes with, wins
	// over the engine-side default.
	if c.Scale > 0 {
		def.scale = c.Scale
	}
	return &characterLook{lookDef: def, char: c}, nil
}
