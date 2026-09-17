package file

import (
	"animaker/pkg/editor"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// ---- TOML structures for .anif files ----

type tomlAnimation struct {
	Metadata  tomlMetadata    `toml:"metadata"`
	Animation tomlAnimConfig  `toml:"animation"`
	KeyFrames []tomlKeyFrame  `toml:"keyframes"`
	Nested    []tomlNested    `toml:"nested,omitempty"`
}

type tomlMetadata struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description,omitempty"`
	Author      string `toml:"author,omitempty"`
}

type tomlAnimConfig struct {
	Loop          bool    `toml:"loop"`
	DefaultSpeed  float32 `toml:"default_speed"`
	CharacterSize string  `toml:"character_size"`
	RootAnchor    string  `toml:"root_anchor"`
}

type tomlKeyFrame struct {
	ID            int            `toml:"id"`
	DurationMs    uint32         `toml:"duration_ms"`
	Sprite        string         `toml:"sprite"`
	SpeedModifier float32        `toml:"speed_modifier,omitempty"`
	HitBox        *tomlBox       `toml:"hitbox,omitempty"`
	AttackHitBox  *tomlBox       `toml:"attack_hitbox,omitempty"`
	Sound         string         `toml:"sound,omitempty"`
	SoundPitch    float32        `toml:"sound_pitch,omitempty"`
	Particles     []tomlParticle `toml:"particles,omitempty"`
	Shake         *tomlShake     `toml:"shake,omitempty"`
	Flash         *tomlFlash     `toml:"flash,omitempty"`
}

type tomlBox struct {
	X int `toml:"x"`
	Y int `toml:"y"`
	W int `toml:"w"`
	H int `toml:"h"`
}

type tomlParticle struct {
	Type     string   `toml:"type"`
	X        int      `toml:"x"`
	Y        int      `toml:"y"`
	Rotation *float32 `toml:"rotation,omitempty"`
	Scale    *float32 `toml:"scale,omitempty"`
}

type tomlShake struct {
	DurationMs uint32  `toml:"duration_ms"`
	Intensity  float32 `toml:"intensity"`
}

type tomlFlash struct {
	Color      string  `toml:"color"`
	DurationMs uint32  `toml:"duration_ms"`
	Opacity    float32 `toml:"opacity"`
}

type tomlNested struct {
	KeyFrame  int      `toml:"keyframe"`
	Animation string   `toml:"animation"`
	Offset    tomlXY   `toml:"offset"`
	Scale     float32  `toml:"scale"`
	Opacity   float32  `toml:"opacity"`
}

type tomlXY struct {
	X int `toml:"x"`
	Y int `toml:"y"`
}

// ---- TOML structures for .sprsh files ----

type tomlSpriteSheet struct {
	Sheet   tomlSheetInfo   `toml:"sheet"`
	Grid    tomlGridConfig  `toml:"grid"`
	Sprites []tomlSpriteInfo `toml:"sprites,omitempty"`
}

type tomlSheetInfo struct {
	Name   string `toml:"name"`
	File   string `toml:"file"`
	Width  int    `toml:"width"`
	Height int    `toml:"height"`
}

type tomlGridConfig struct {
	Cols      int `toml:"cols"`
	Rows      int `toml:"rows"`
	TileWidth int `toml:"tile_width"`
	TileHeight int `toml:"tile_height"`
}

type tomlSpriteInfo struct {
	Index int    `toml:"index"`
	X     int    `toml:"x"`
	Y     int    `toml:"y"`
	W     int    `toml:"w"`
	H     int    `toml:"h"`
	Label string `toml:"label,omitempty"`
}

// ---- Save/Load Animation (.anif) ----

// SaveAnimation writes an animation to a .anif TOML file.
func SaveAnimation(anim *editor.Animation, path string) error {
	ta := tomlAnimation{
		Metadata: tomlMetadata{
			Name:        anim.Metadata.Name,
			Version:     anim.Metadata.Version,
			Description: anim.Metadata.Description,
			Author:      anim.Metadata.Author,
		},
		Animation: tomlAnimConfig{
			Loop:          anim.Config.Loop,
			DefaultSpeed:  anim.Config.DefaultSpeed,
			CharacterSize: anim.Config.CharacterSize,
			RootAnchor:    anim.Config.RootAnchor,
		},
	}

	// Serialize keyframes
	ta.KeyFrames = make([]tomlKeyFrame, len(anim.KeyFrames))
	for i, kf := range anim.KeyFrames {
		tkf := tomlKeyFrame{
			ID:            kf.ID,
			DurationMs:    kf.Duration,
			Sprite:        formatSpriteRef(kf.Sprite),
			SpeedModifier: kf.Speed,
		}

		if kf.HitBox != nil {
			tkf.HitBox = &tomlBox{X: kf.HitBox.X, Y: kf.HitBox.Y, W: kf.HitBox.W, H: kf.HitBox.H}
		}
		if kf.AttackHitBox != nil {
			tkf.AttackHitBox = &tomlBox{X: kf.AttackHitBox.X, Y: kf.AttackHitBox.Y, W: kf.AttackHitBox.W, H: kf.AttackHitBox.H}
		}

		// Serialize events
		for _, e := range kf.Events {
			switch v := e.(type) {
			case *editor.SoundEvent:
				tkf.Sound = v.FilePath
				tkf.SoundPitch = v.Pitch
			case *editor.ParticleEvent:
				tkf.Particles = append(tkf.Particles, tomlParticle{
					Type: v.Type, X: v.X, Y: v.Y,
					Rotation: v.Rotation, Scale: v.Scale,
				})
			case *editor.ShakeEvent:
				tkf.Shake = &tomlShake{DurationMs: v.DurationMs, Intensity: v.Intensity}
			case *editor.FlashEvent:
				tkf.Flash = &tomlFlash{Color: v.Color, DurationMs: v.DurationMs, Opacity: v.Opacity}
			}
		}

		ta.KeyFrames[i] = tkf
	}

	// Serialize nested animations
	for _, n := range anim.Nested {
		ta.Nested = append(ta.Nested, tomlNested{
			KeyFrame:  n.KeyFrameID,
			Animation: n.AnimationPath,
			Offset:    tomlXY{X: n.Offset.X, Y: n.Offset.Y},
			Scale:     n.Scale,
			Opacity:   n.Opacity,
		})
	}

	buf := &bytes.Buffer{}
	enc := toml.NewEncoder(buf)
	if err := enc.Encode(ta); err != nil {
		return fmt.Errorf("failed to encode animation: %w", err)
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// LoadAnimation reads an animation from a .anif TOML file.
func LoadAnimation(path string) (*editor.Animation, error) {
	var ta tomlAnimation
	if _, err := toml.DecodeFile(path, &ta); err != nil {
		return nil, fmt.Errorf("failed to decode animation: %w", err)
	}

	anim := &editor.Animation{
		Metadata: editor.AnimationMetadata{
			Name:        ta.Metadata.Name,
			Version:     ta.Metadata.Version,
			Description: ta.Metadata.Description,
			Author:      ta.Metadata.Author,
		},
		Config: editor.AnimationConfig{
			Loop:          ta.Animation.Loop,
			DefaultSpeed:  ta.Animation.DefaultSpeed,
			CharacterSize: ta.Animation.CharacterSize,
			RootAnchor:    ta.Animation.RootAnchor,
		},
	}

	// Parse keyframes
	for _, tkf := range ta.KeyFrames {
		kf := &editor.KeyFrame{
			ID:       tkf.ID,
			Duration: tkf.DurationMs,
			Sprite:   parseSpriteRef(tkf.Sprite),
			Speed:    tkf.SpeedModifier,
			Events:   []editor.Event{},
		}

		if kf.Speed == 0 {
			kf.Speed = 1.0
		}

		if tkf.HitBox != nil {
			kf.HitBox = &editor.Box{X: tkf.HitBox.X, Y: tkf.HitBox.Y, W: tkf.HitBox.W, H: tkf.HitBox.H}
		}
		if tkf.AttackHitBox != nil {
			kf.AttackHitBox = &editor.Box{X: tkf.AttackHitBox.X, Y: tkf.AttackHitBox.Y, W: tkf.AttackHitBox.W, H: tkf.AttackHitBox.H}
		}

		// Parse events
		if tkf.Sound != "" {
			pitch := tkf.SoundPitch
			if pitch == 0 {
				pitch = 1.0
			}
			kf.Events = append(kf.Events, &editor.SoundEvent{FilePath: tkf.Sound, Pitch: pitch})
		}
		for _, tp := range tkf.Particles {
			kf.Events = append(kf.Events, &editor.ParticleEvent{
				Type: tp.Type, X: tp.X, Y: tp.Y,
				Rotation: tp.Rotation, Scale: tp.Scale,
			})
		}
		if tkf.Shake != nil {
			kf.Events = append(kf.Events, &editor.ShakeEvent{
				DurationMs: tkf.Shake.DurationMs, Intensity: tkf.Shake.Intensity,
			})
		}
		if tkf.Flash != nil {
			kf.Events = append(kf.Events, &editor.FlashEvent{
				Color: tkf.Flash.Color, DurationMs: tkf.Flash.DurationMs, Opacity: tkf.Flash.Opacity,
			})
		}

		anim.KeyFrames = append(anim.KeyFrames, kf)
	}

	// Parse nested
	for _, tn := range ta.Nested {
		anim.Nested = append(anim.Nested, &editor.NestedAnimation{
			KeyFrameID:    tn.KeyFrame,
			AnimationPath: tn.Animation,
			Offset:        editor.Point{X: tn.Offset.X, Y: tn.Offset.Y},
			Scale:         tn.Scale,
			Opacity:       tn.Opacity,
		})
	}

	return anim, nil
}

// ---- Save/Load Sprite Sheet Metadata (.sprsh) ----

// SaveSpriteSheetMeta writes sprite sheet metadata to a .sprsh TOML file.
func SaveSpriteSheetMeta(sheet *editor.SpriteSheet, path string) error {
	bounds := sheet.Image.Bounds()
	ts := tomlSpriteSheet{
		Sheet: tomlSheetInfo{
			Name:   sheet.Name,
			File:   sheet.FilePath,
			Width:  bounds.Dx(),
			Height: bounds.Dy(),
		},
		Grid: tomlGridConfig{
			Cols:       sheet.GridConfig.Cols,
			Rows:       sheet.GridConfig.Rows,
			TileWidth:  sheet.GridConfig.TileW,
			TileHeight: sheet.GridConfig.TileH,
		},
	}

	for _, s := range sheet.Sprites {
		ts.Sprites = append(ts.Sprites, tomlSpriteInfo{
			Index: s.Index, X: s.X, Y: s.Y, W: s.W, H: s.H, Label: s.Label,
		})
	}

	buf := &bytes.Buffer{}
	enc := toml.NewEncoder(buf)
	if err := enc.Encode(ts); err != nil {
		return fmt.Errorf("failed to encode sprite sheet metadata: %w", err)
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

// LoadSpriteSheetMeta reads sprite sheet metadata from a .sprsh TOML file.
func LoadSpriteSheetMeta(path string) (*editor.GridConfig, []editor.SpriteInfo, string, error) {
	var ts tomlSpriteSheet
	if _, err := toml.DecodeFile(path, &ts); err != nil {
		return nil, nil, "", fmt.Errorf("failed to decode sprite sheet metadata: %w", err)
	}

	cfg := &editor.GridConfig{
		Cols:  ts.Grid.Cols,
		Rows:  ts.Grid.Rows,
		TileW: ts.Grid.TileWidth,
		TileH: ts.Grid.TileHeight,
	}

	sprites := make([]editor.SpriteInfo, len(ts.Sprites))
	for i, s := range ts.Sprites {
		sprites[i] = editor.SpriteInfo{
			Index: s.Index, X: s.X, Y: s.Y, W: s.W, H: s.H, Label: s.Label,
		}
	}

	return cfg, sprites, ts.Sheet.File, nil
}

// ---- Helpers ----

func formatSpriteRef(ref editor.SpriteReference) string {
	if ref.SheetName != "" {
		return ref.SheetName + ":" + strconv.Itoa(ref.Index)
	}
	if ref.Absolute != nil {
		return ref.Absolute.FilePath
	}
	return ""
}

func parseSpriteRef(s string) editor.SpriteReference {
	if idx := strings.Index(s, ":"); idx >= 0 {
		name := s[:idx]
		index, _ := strconv.Atoi(s[idx+1:])
		return editor.SpriteReference{SheetName: name, Index: index}
	}
	return editor.SpriteReference{}
}
