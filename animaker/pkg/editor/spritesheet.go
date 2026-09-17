package editor

import (
	"fmt"
	"image"
	"image/draw"
)

// SpriteSheet represents an imported sprite sheet with grid metadata.
type SpriteSheet struct {
	Name       string
	FilePath   string
	Image      image.Image
	GridConfig GridConfig
	Sprites    []SpriteInfo
}

// GridConfig defines how a sprite sheet is divided into tiles.
type GridConfig struct {
	Cols  int // sprites horizontally
	Rows  int // sprites vertically
	TileW int // width of each sprite in pixels
	TileH int // height of each sprite in pixels
}

// SpriteInfo describes a single sprite within a sheet.
type SpriteInfo struct {
	Index int
	X     int    // pixel position in sheet
	Y     int
	W     int    // width
	H     int    // height
	Label string // optional: "walk_0", "attack_1", etc.
}

// ComputeGridSprites generates SpriteInfo entries from grid config.
func ComputeGridSprites(cfg GridConfig) []SpriteInfo {
	sprites := make([]SpriteInfo, 0, cfg.Cols*cfg.Rows)
	for row := 0; row < cfg.Rows; row++ {
		for col := 0; col < cfg.Cols; col++ {
			idx := row*cfg.Cols + col
			sprites = append(sprites, SpriteInfo{
				Index: idx,
				X:     col * cfg.TileW,
				Y:     row * cfg.TileH,
				W:     cfg.TileW,
				H:     cfg.TileH,
				Label: fmt.Sprintf("sprite_%d", idx),
			})
		}
	}
	return sprites
}

// NewSpriteSheet creates a SpriteSheet from an image and grid config.
func NewSpriteSheet(name, filePath string, img image.Image, cfg GridConfig) *SpriteSheet {
	return &SpriteSheet{
		Name:       name,
		FilePath:   filePath,
		Image:      img,
		GridConfig: cfg,
		Sprites:    ComputeGridSprites(cfg),
	}
}

// GetSpriteRect returns the rectangle for the sprite at the given index.
func (ss *SpriteSheet) GetSpriteRect(index int) *image.Rectangle {
	if index < 0 || index >= len(ss.Sprites) {
		return nil
	}
	s := ss.Sprites[index]
	r := image.Rect(s.X, s.Y, s.X+s.W, s.Y+s.H)
	return &r
}

// GetSpriteImage extracts a single sprite as a sub-image.
func (ss *SpriteSheet) GetSpriteImage(index int) (image.Image, error) {
	rect := ss.GetSpriteRect(index)
	if rect == nil {
		return nil, fmt.Errorf("sprite index %d out of range (sheet has %d sprites)", index, len(ss.Sprites))
	}

	// Try SubImage if supported
	if sub, ok := ss.Image.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(*rect), nil
	}

	// Fallback: manual crop
	dst := image.NewRGBA(*rect)
	draw.Draw(dst, *rect, ss.Image, rect.Min, draw.Src)
	return dst, nil
}

// SpriteCount returns the total number of sprites in the sheet.
func (ss *SpriteSheet) SpriteCount() int {
	return len(ss.Sprites)
}
