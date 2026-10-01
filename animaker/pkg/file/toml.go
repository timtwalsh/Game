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
	// Parts are track-level: the rig's slot list, shared by every
	// direction. Each direction then stores keyframes referencing a part_id.
	Parts []tomlPart `toml:"parts,omitempty"`
	// Directions are keyed by the string form of their int key (TOML table
	// keys must be strings) - converted at the Save/LoadTrack boundary.
	Directions map[string]tomlDirection `toml:"directions"`

	// Sheets is an editor hint: where the editor found each sheet the
	// last time this track was saved. Parts and props refer to sheets by
	// name only, which stays the real reference (the game resolves names
	// against its own art library); this just lets the editor reopen a
	// track with its art instead of with nothing loaded.
	Sheets []tomlSheetRef `toml:"sheets,omitempty"`
}

type tomlTrackMeta struct {
	Name         string `toml:"name"`
	Version      string `toml:"version"`
	RefBoxWidth  int    `toml:"ref_box_width"`
	RefBoxHeight int    `toml:"ref_box_height"`
}

type tomlPropDef struct {
	Name    string `toml:"name"`
	Default string `toml:"default"`
}

type tomlDirection struct {
	Keyframes []tomlKeyframe `toml:"keyframes,omitempty"`
}

type tomlPart struct {
	ID             int                        `toml:"id"`
	Name           string                     `toml:"name"`
	Kind           string                     `toml:"kind"`
	GoverningProp  string                     `toml:"governing_prop,omitempty"`
	FixedSheet     string                     `toml:"fixed_sheet,omitempty"`
	NestedAniPath  string                     `toml:"nested_ani_path,omitempty"`
	NestedBindings map[string]tomlPropBinding `toml:"nested_bindings,omitempty"`
}

type tomlPropBinding struct {
	PassthroughFrom string `toml:"passthrough_from,omitempty"`
	StaticValue     string `toml:"static_value,omitempty"`
}

type tomlKeyframe struct {
	PartID      int     `toml:"part_id"`
	TimeMs      uint32  `toml:"time_ms"`
	X           float32 `toml:"x"`
	Y           float32 `toml:"y"`
	Z           float32 `toml:"z"`
	RotationDeg float32 `toml:"rotation_deg"`
	Row         int     `toml:"row,omitempty"`
	Col         int     `toml:"col,omitempty"`
}

type tomlSheetRef struct {
	Name string `toml:"name"`
	Path string `toml:"path"` // the .sprsh, relative to the .anif when possible
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

// SheetRef locates a sheet's .sprsh. SprshPath is absolute in memory;
// SaveTrack writes it relative to the .anif.
type SheetRef struct {
	Name      string
	SprshPath string
}

// SaveTrack writes a track to a .anif TOML file, along with where to find
// the given sheets again (see tomlTrack.Sheets).
func SaveTrack(t *editor.Track, path string, sheets []SheetRef) error {
	tt := tomlTrack{
		Metadata: tomlTrackMeta{
			Name: t.Metadata.Name, Version: t.Metadata.Version,
			RefBoxWidth: t.RefBoxWidth, RefBoxHeight: t.RefBoxHeight,
		},
		Directions: make(map[string]tomlDirection, len(t.Directions)),
	}

	for _, prop := range t.Props {
		def := prop.Default
		if prop.IsAnimProp() {
			def = relAnimPath(path, def)
		}
		tt.Props = append(tt.Props, tomlPropDef{Name: prop.Name, Default: def})
	}

	for _, ref := range sheets {
		p := ref.SprshPath
		if rel, err := filepath.Rel(filepath.Dir(path), p); err == nil {
			p = filepath.ToSlash(rel)
		}
		tt.Sheets = append(tt.Sheets, tomlSheetRef{Name: ref.Name, Path: p})
	}

	for _, part := range t.Parts {
		tp := tomlPart{
			ID:            part.ID,
			Name:          part.Name,
			Kind:          partKindToString(part.Kind),
			GoverningProp: part.GoverningProp,
			FixedSheet:    part.FixedSheet,
			NestedAniPath: relAnimPath(path, part.NestedAniPath),
		}
		if len(part.NestedBindings) > 0 {
			tp.NestedBindings = make(map[string]tomlPropBinding, len(part.NestedBindings))
			for k, v := range part.NestedBindings {
				tp.NestedBindings[k] = tomlPropBinding{PassthroughFrom: v.PassthroughFrom, StaticValue: v.StaticValue}
			}
		}
		tt.Parts = append(tt.Parts, tp)
	}

	for dirKey, dir := range t.Directions {
		td := tomlDirection{}
		// Written in the track's part order, then by time, so the file is
		// stable across saves rather than reordering with Go's map iteration.
		for _, part := range t.Parts {
			for _, kf := range dir.KeyframesFor(part.ID) {
				td.Keyframes = append(td.Keyframes, tomlKeyframe{
					PartID: part.ID, TimeMs: kf.TimeMs, X: kf.X, Y: kf.Y, Z: kf.Z,
					RotationDeg: kf.RotationDeg, Row: kf.Row, Col: kf.Col,
				})
			}
		}
		tt.Directions[strconv.Itoa(dirKey)] = td
	}

	buf := &bytes.Buffer{}
	if err := toml.NewEncoder(buf).Encode(tt); err != nil {
		return fmt.Errorf("failed to encode track: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// LoadTrack reads a track from a .anif TOML file, plus where it says its
// sheets are (absolute paths; possibly stale - see LoadSheetsForTrack).
func LoadTrack(path string) (*editor.Track, []SheetRef, error) {
	var tt tomlTrack
	if _, err := toml.DecodeFile(path, &tt); err != nil {
		return nil, nil, fmt.Errorf("failed to decode track: %w", err)
	}
	var refs []SheetRef
	for _, r := range tt.Sheets {
		p := filepath.FromSlash(r.Path)
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(path), p)
		}
		refs = append(refs, SheetRef{Name: r.Name, SprshPath: p})
	}

	t := &editor.Track{
		Metadata:     editor.TrackMetadata{Name: tt.Metadata.Name, Version: tt.Metadata.Version},
		RefBoxWidth:  tt.Metadata.RefBoxWidth,
		RefBoxHeight: tt.Metadata.RefBoxHeight,
		Directions:   make(map[int]*editor.Direction, len(tt.Directions)),
	}
	// Also catches tracks written before these keys existed (or under their
	// old canvas_width/canvas_height names), which decode as zero.
	if t.RefBoxWidth <= 0 {
		t.RefBoxWidth = editor.DefaultRefBoxWidth
	}
	if t.RefBoxHeight <= 0 {
		t.RefBoxHeight = editor.DefaultRefBoxHeight
	}
	for _, p := range tt.Props {
		def := p.Default
		if editor.IsAnimValue(def) {
			def = absAnimPath(path, def)
		}
		t.Props = append(t.Props, editor.PropDef{Name: p.Name, Default: def})
	}

	knownPart := make(map[int]bool, len(tt.Parts))
	for _, tp := range tt.Parts {
		part := &editor.Part{
			ID:            tp.ID,
			Name:          tp.Name,
			Kind:          partKindFromString(tp.Kind),
			GoverningProp: tp.GoverningProp,
			FixedSheet:    tp.FixedSheet,
			NestedAniPath: absAnimPath(path, tp.NestedAniPath),
		}
		if len(tp.NestedBindings) > 0 {
			part.NestedBindings = make(map[string]editor.PropBinding, len(tp.NestedBindings))
			for k, v := range tp.NestedBindings {
				part.NestedBindings[k] = editor.PropBinding{PassthroughFrom: v.PassthroughFrom, StaticValue: v.StaticValue}
			}
		}
		if knownPart[part.ID] {
			return nil, nil, fmt.Errorf("duplicate part id %d (%q)", part.ID, part.Name)
		}
		knownPart[part.ID] = true
		t.Parts = append(t.Parts, part)
	}

	for dirName, td := range tt.Directions {
		dirKey, err := strconv.Atoi(dirName)
		if err != nil {
			return nil, nil, fmt.Errorf("direction key %q is not an integer: %w", dirName, err)
		}
		dir := editor.NewDirection()
		for _, tkf := range td.Keyframes {
			if !knownPart[tkf.PartID] {
				return nil, nil, fmt.Errorf("direction %s has a keyframe for unknown part id %d", dirName, tkf.PartID)
			}
			dir.Keyframes[tkf.PartID] = append(dir.Keyframes[tkf.PartID], &editor.Keyframe{
				ID: len(dir.Keyframes[tkf.PartID]), TimeMs: tkf.TimeMs,
				X: tkf.X, Y: tkf.Y, Z: tkf.Z,
				RotationDeg: tkf.RotationDeg, Row: tkf.Row, Col: tkf.Col,
			})
		}
		t.Directions[dirKey] = dir
	}

	if len(t.Directions) == 0 {
		t.Directions[0] = editor.NewDirection()
	}

	return t, refs, nil
}

// relAnimPath writes a nested .anif path relative to the .anif containing
// it, so a folder of tracks can be moved as a whole. In memory such paths
// are absolute (absAnimPath), which is what Project.LoadedAnims is keyed by.
func relAnimPath(anifPath, p string) string {
	if p == "" {
		return ""
	}
	if rel, err := filepath.Rel(filepath.Dir(anifPath), p); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(p)
}

// absAnimPath resolves a nested .anif path read from a file against that
// file's folder.
func absAnimPath(anifPath, p string) string {
	if p == "" {
		return ""
	}
	p = filepath.FromSlash(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(filepath.Dir(anifPath), p)
	}
	return editor.AnimKey(p)
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

	s := editor.NewSpriteSheetTemplate(ts.Name, imgPath, img, ts.CellW, ts.CellH, ts.PivotX, ts.PivotY)
	s.SprshPath = sprshPath
	return s, nil
}
