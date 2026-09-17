package file

import (
	"animaker/pkg/editor"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// ---- TOML structures for .anif files (a Track) ----

type tomlTrack struct {
	Metadata tomlTrackMeta `toml:"metadata"`
	Props    []tomlPropDef `toml:"props,omitempty"`
	// Directions are keyed by the string form of their int key (TOML table
	// keys must be strings) - converted at the Save/LoadTrack boundary.
	Directions map[string]tomlDirection `toml:"directions"`
}

type tomlTrackMeta struct {
	Name         string `toml:"name"`
	Version      string `toml:"version"`
	CanvasWidth  int    `toml:"canvas_width"`
	CanvasHeight int    `toml:"canvas_height"`
}

type tomlPropDef struct {
	Name    string `toml:"name"`
	Default string `toml:"default"`
}

type tomlDirection struct {
	Parts []tomlPart `toml:"parts"`
}

type tomlPart struct {
	Name           string                     `toml:"name"`
	Kind           string                     `toml:"kind"`
	GoverningProp  string                     `toml:"governing_prop,omitempty"`
	FixedSheet     string                     `toml:"fixed_sheet,omitempty"`
	NestedAniPath  string                     `toml:"nested_ani_path,omitempty"`
	NestedBindings map[string]tomlPropBinding `toml:"nested_bindings,omitempty"`
	Keyframes      []tomlKeyframe             `toml:"keyframes"`
}

type tomlPropBinding struct {
	PassthroughFrom string `toml:"passthrough_from,omitempty"`
	StaticValue     string `toml:"static_value,omitempty"`
}

type tomlKeyframe struct {
	TimeMs      uint32  `toml:"time_ms"`
	X           float32 `toml:"x"`
	Y           float32 `toml:"y"`
	Z           float32 `toml:"z"`
	RotationDeg float32 `toml:"rotation_deg"`
	Row         int     `toml:"row,omitempty"`
	Col         int     `toml:"col,omitempty"`
}

// ---- TOML structure for .sprsh files (a SpriteSheetTemplate) ----

type tomlSheetTemplate struct {
	Name     string  `toml:"name"`
	FilePath string  `toml:"file_path"`
	CellW    int     `toml:"cell_w"`
	CellH    int     `toml:"cell_h"`
	PivotX   float32 `toml:"pivot_x"`
	PivotY   float32 `toml:"pivot_y"`
}

// ---- Save/Load Track (.anif) ----

func partKindToString(k editor.PartKind) string {
	if k == editor.PartKindNestedAni {
		return "nested_ani"
	}
	return "sheet"
}

func partKindFromString(s string) editor.PartKind {
	if s == "nested_ani" {
		return editor.PartKindNestedAni
	}
	return editor.PartKindSheet
}

// SaveTrack writes a track to a .anif TOML file.
func SaveTrack(t *editor.Track, path string) error {
	tt := tomlTrack{
		Metadata: tomlTrackMeta{
			Name: t.Metadata.Name, Version: t.Metadata.Version,
			CanvasWidth: t.CanvasWidth, CanvasHeight: t.CanvasHeight,
		},
		Directions: make(map[string]tomlDirection, len(t.Directions)),
	}

	for _, prop := range t.Props {
		tt.Props = append(tt.Props, tomlPropDef{Name: prop.Name, Default: prop.Default})
	}

	for dirKey, dir := range t.Directions {
		dirName := strconv.Itoa(dirKey)
		td := tomlDirection{}
		for _, part := range dir.Parts {
			tp := tomlPart{
				Name:          part.Name,
				Kind:          partKindToString(part.Kind),
				GoverningProp: part.GoverningProp,
				FixedSheet:    part.FixedSheet,
				NestedAniPath: part.NestedAniPath,
			}
			if len(part.NestedBindings) > 0 {
				tp.NestedBindings = make(map[string]tomlPropBinding, len(part.NestedBindings))
				for k, v := range part.NestedBindings {
					tp.NestedBindings[k] = tomlPropBinding{PassthroughFrom: v.PassthroughFrom, StaticValue: v.StaticValue}
				}
			}
			for _, kf := range part.Keyframes {
				tp.Keyframes = append(tp.Keyframes, tomlKeyframe{
					TimeMs: kf.TimeMs, X: kf.X, Y: kf.Y, Z: kf.Z,
					RotationDeg: kf.RotationDeg, Row: kf.Row, Col: kf.Col,
				})
			}
			td.Parts = append(td.Parts, tp)
		}
		tt.Directions[dirName] = td
	}

	buf := &bytes.Buffer{}
	if err := toml.NewEncoder(buf).Encode(tt); err != nil {
		return fmt.Errorf("failed to encode track: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// LoadTrack reads a track from a .anif TOML file.
func LoadTrack(path string) (*editor.Track, error) {
	var tt tomlTrack
	if _, err := toml.DecodeFile(path, &tt); err != nil {
		return nil, fmt.Errorf("failed to decode track: %w", err)
	}

	t := &editor.Track{
		Metadata:     editor.TrackMetadata{Name: tt.Metadata.Name, Version: tt.Metadata.Version},
		CanvasWidth:  tt.Metadata.CanvasWidth,
		CanvasHeight: tt.Metadata.CanvasHeight,
		Directions:   make(map[int]*editor.Direction, len(tt.Directions)),
	}
	if t.CanvasWidth == 0 {
		t.CanvasWidth = editor.DefaultCanvasWidth
	}
	if t.CanvasHeight == 0 {
		t.CanvasHeight = editor.DefaultCanvasHeight
	}
	for _, p := range tt.Props {
		t.Props = append(t.Props, editor.PropDef{Name: p.Name, Default: p.Default})
	}

	for dirName, td := range tt.Directions {
		dirKey, err := strconv.Atoi(dirName)
		if err != nil {
			return nil, fmt.Errorf("direction key %q is not an integer: %w", dirName, err)
		}
		dir := &editor.Direction{}
		for _, tp := range td.Parts {
			part := &editor.Part{
				Name:          tp.Name,
				Kind:          partKindFromString(tp.Kind),
				GoverningProp: tp.GoverningProp,
				FixedSheet:    tp.FixedSheet,
				NestedAniPath: tp.NestedAniPath,
			}
			if len(tp.NestedBindings) > 0 {
				part.NestedBindings = make(map[string]editor.PropBinding, len(tp.NestedBindings))
				for k, v := range tp.NestedBindings {
					part.NestedBindings[k] = editor.PropBinding{PassthroughFrom: v.PassthroughFrom, StaticValue: v.StaticValue}
				}
			}
			for i, tkf := range tp.Keyframes {
				part.Keyframes = append(part.Keyframes, &editor.Keyframe{
					ID: i, TimeMs: tkf.TimeMs, X: tkf.X, Y: tkf.Y, Z: tkf.Z,
					RotationDeg: tkf.RotationDeg, Row: tkf.Row, Col: tkf.Col,
				})
			}
			dir.Parts = append(dir.Parts, part)
		}
		t.Directions[dirKey] = dir
	}

	if len(t.Directions) == 0 {
		t.Directions[0] = &editor.Direction{}
	}

	return t, nil
}

// ---- Save/Load SpriteSheetTemplate (.sprsh) ----

// SaveSheetTemplate writes sheet template metadata to a .sprsh TOML file.
// FilePath is stored relative to the .sprsh's own directory when possible,
// so a template and its image can be moved together.
func SaveSheetTemplate(s *editor.SpriteSheetTemplate, sprshPath string) error {
	imgPath := s.FilePath
	if rel, err := filepath.Rel(filepath.Dir(sprshPath), s.FilePath); err == nil {
		imgPath = rel
	}

	ts := tomlSheetTemplate{
		Name: s.Name, FilePath: imgPath,
		CellW: s.CellW, CellH: s.CellH,
		PivotX: s.PivotX, PivotY: s.PivotY,
	}

	buf := &bytes.Buffer{}
	if err := toml.NewEncoder(buf).Encode(ts); err != nil {
		return fmt.Errorf("failed to encode sheet template: %w", err)
	}
	return os.WriteFile(sprshPath, buf.Bytes(), 0644)
}

// LoadSheetTemplate reads a .sprsh file and loads its referenced image.
func LoadSheetTemplate(sprshPath string) (*editor.SpriteSheetTemplate, error) {
	var ts tomlSheetTemplate
	if _, err := toml.DecodeFile(sprshPath, &ts); err != nil {
		return nil, fmt.Errorf("failed to decode sheet template: %w", err)
	}

	imgPath := ts.FilePath
	if !filepath.IsAbs(imgPath) {
		imgPath = filepath.Join(filepath.Dir(sprshPath), imgPath)
	}
	img, err := LoadImage(imgPath)
	if err != nil {
		return nil, err
	}

	return editor.NewSpriteSheetTemplate(ts.Name, imgPath, img, ts.CellW, ts.CellH, ts.PivotX, ts.PivotY), nil
}
