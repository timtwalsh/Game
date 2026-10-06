// Package anim is the game's runtime for animaker assets: it loads .anif
// tracks, .sprsh sheets and .anichar characters, and plays them. It has no
// raylib dependency, so it is testable headless; client/ draws what an
// Instance resolves to.
//
// It reads the same files the animaker writes but never imports animaker
// (a separate module) - the formats are the contract, see
// docs/ANI_MAKER_SPEC.md.
package anim

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// On-disk shapes. Only what the runtime uses is decoded.

type tomlTrack struct {
	Metadata struct {
		Name       string `toml:"name"`
		Directions int    `toml:"directions"`
	} `toml:"metadata"`
	Props      []tomlProp               `toml:"props"`
	Parts      []tomlPart               `toml:"parts"`
	Directions map[string]tomlDirection `toml:"directions"`
	Sheets     []tomlSheetRef           `toml:"sheets"`
}

type tomlProp struct {
	Name    string `toml:"name"`
	Default string `toml:"default"`
}

type tomlPart struct {
	ID              int                    `toml:"id"`
	Name            string                 `toml:"name"`
	Kind            string                 `toml:"kind"`
	GoverningProp   string                 `toml:"governing_prop"`
	FixedSheet      string                 `toml:"fixed_sheet"`
	NestedAniPath   string                 `toml:"nested_ani_path"`
	NestedBindings  map[string]tomlBinding `toml:"nested_bindings"`
	DirectionMode   string                 `toml:"direction_mode"`
	StaticDirection int                    `toml:"static_direction"`
}

type tomlBinding struct {
	PassthroughFrom string `toml:"passthrough_from"`
	StaticValue     string `toml:"static_value"`
}

type tomlDirection struct {
	Keyframes []tomlKeyframe `toml:"keyframes"`
}

type tomlKeyframe struct {
	PartID      int     `toml:"part_id"`
	TimeMs      uint32  `toml:"time_ms"`
	X           float32 `toml:"x"`
	Y           float32 `toml:"y"`
	Z           float32 `toml:"z"`
	RotationDeg float32 `toml:"rotation_deg"`
	Row         int     `toml:"row"`
	Col         int     `toml:"col"`
	Direction   int     `toml:"direction"`
}

type tomlSheetRef struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type tomlSheet struct {
	Name     string  `toml:"name"`
	FilePath string  `toml:"file_path"`
	CellW    int     `toml:"cell_w"`
	CellH    int     `toml:"cell_h"`
	PivotX   float32 `toml:"pivot_x"`
	PivotY   float32 `toml:"pivot_y"`
}

type tomlCharacter struct {
	Name       string                       `toml:"name"`
	Controller string                       `toml:"controller"`
	Scale      float32                      `toml:"scale"`
	Footprint  *tomlBox                     `toml:"footprint"`
	Hitboxes   []tomlHitbox                 `toml:"hitboxes"`
	Animations map[string]tomlCharAnimation `toml:"animations"`
}

type tomlBox struct {
	X float32 `toml:"x"`
	Y float32 `toml:"y"`
	W float32 `toml:"w"`
	H float32 `toml:"h"`
}

type tomlHitbox struct {
	Name  string  `toml:"name"`
	Shape string  `toml:"shape"`
	X     float32 `toml:"x"`
	Y     float32 `toml:"y"`
	W     float32 `toml:"w"`
	H     float32 `toml:"h"`
}

type tomlCharAnimation struct {
	Anif string `toml:"anif"`
	Mode string `toml:"mode"`
	// Markers maps a name to one time (hit = 300) or several
	// (footstep = [0, 250]).
	Markers map[string]any `toml:"markers"`
}

// Library loads assets once and shares them by path, so every instance
// (and every character) using a track or sheet holds the same one. The
// client owns one for its lifetime. It isn't safe for concurrent use: load
// on one goroutine (today, at startup on the main one).
type Library struct {
	tracks map[string]*Track
	sheets map[string]*Sheet // by absolute .sprsh path
	// Problems are what loading couldn't find but could carry on without
	// - a sheet or nested .anif that's missing draws nothing, as in the
	// editor. Callers should report them.
	Problems []string
}

func NewLibrary() *Library {
	return &Library{tracks: map[string]*Track{}, sheets: map[string]*Sheet{}}
}

func (l *Library) problem(format string, args ...any) {
	l.Problems = append(l.Problems, fmt.Sprintf(format, args...))
}

// LoadCharacter reads an .anichar and every .anif it names.
func (l *Library) LoadCharacter(path string) (*Character, error) {
	var tc tomlCharacter
	if _, err := toml.DecodeFile(path, &tc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c := &Character{Name: tc.Name, Controller: tc.Controller, Scale: tc.Scale, Animations: map[string]*Animation{}}
	if tc.Scale < 0 {
		return nil, fmt.Errorf("%s: scale %g must be positive", path, tc.Scale)
	}
	if b := tc.Footprint; b != nil {
		if b.W <= 0 || b.H <= 0 {
			return nil, fmt.Errorf("%s: footprint size %gx%g must be positive", path, b.W, b.H)
		}
		c.Footprint = &Box{X: b.X, Y: b.Y, W: b.W, H: b.H}
	}
	for _, th := range tc.Hitboxes {
		kind, err := parseShape(th.Shape)
		if err != nil {
			return nil, fmt.Errorf("%s: hitbox %q: %w", path, th.Name, err)
		}
		if th.W <= 0 || th.H <= 0 {
			return nil, fmt.Errorf("%s: hitbox %q: size %gx%g must be positive", path, th.Name, th.W, th.H)
		}
		c.Hitboxes = append(c.Hitboxes, Hitbox{Name: th.Name, Shape: kind, Box: Box{X: th.X, Y: th.Y, W: th.W, H: th.H}})
	}
	for name, ta := range tc.Animations {
		mode, err := parseMode(ta.Mode)
		if err != nil {
			return nil, fmt.Errorf("%s: animation %q: %w", path, name, err)
		}
		t, err := l.LoadTrack(resolve(path, ta.Anif))
		if err != nil {
			return nil, fmt.Errorf("%s: animation %q: %w", path, name, err)
		}
		a := &Animation{Name: name, Track: t, Mode: mode}
		for mn, v := range ta.Markers {
			times, err := markerTimes(v)
			if err != nil {
				return nil, fmt.Errorf("%s: animation %q, marker %q: %w", path, name, mn, err)
			}
			for _, tm := range times {
				a.Markers = append(a.Markers, Marker{Name: mn, TimeMs: tm})
			}
		}
		slices.SortFunc(a.Markers, func(x, y Marker) int {
			if x.TimeMs != y.TimeMs {
				return int(int64(x.TimeMs) - int64(y.TimeMs))
			}
			return strings.Compare(x.Name, y.Name)
		})
		c.Animations[name] = a
	}
	return c, nil
}

// markerTimes reads a marker's value: one time in ms, or a list of them.
func markerTimes(v any) ([]uint32, error) {
	one := func(x any) (uint32, error) {
		n, ok := x.(int64)
		if !ok || n < 0 || n > math.MaxUint32 {
			return 0, fmt.Errorf("want a time in ms, got %v", x)
		}
		return uint32(n), nil
	}
	if list, ok := v.([]any); ok {
		out := make([]uint32, 0, len(list))
		for _, x := range list {
			t, err := one(x)
			if err != nil {
				return nil, err
			}
			out = append(out, t)
		}
		return out, nil
	}
	t, err := one(v)
	if err != nil {
		return nil, err
	}
	return []uint32{t}, nil
}

// LoadTrack reads an .anif, the sheets it names and the .anifs it nests.
func (l *Library) LoadTrack(path string) (*Track, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if t := l.tracks[abs]; t != nil {
		return t, nil
	}
	var tt tomlTrack
	if _, err := toml.DecodeFile(abs, &tt); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	t := &Track{
		Name:           tt.Metadata.Name,
		Path:           abs,
		DirectionCount: tt.Metadata.Directions,
		Directions:     map[int]*Direction{},
	}
	partIdx := map[int]int{}
	for _, tp := range tt.Parts {
		if _, dup := partIdx[tp.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate part id %d", path, tp.ID)
		}
		partIdx[tp.ID] = len(t.Parts)
		t.Parts = append(t.Parts, Part{
			ID: tp.ID, Name: tp.Name, GoverningProp: tp.GoverningProp,
			FixedSheet: tp.FixedSheet, IsNested: tp.Kind == "nested_ani",
			DirMode: parseDirMode(tp.DirectionMode), StaticDirection: tp.StaticDirection,
		})
	}
	for key, td := range tt.Directions {
		k, err := strconv.Atoi(key)
		if err != nil || k < 0 || k >= 16 {
			return nil, fmt.Errorf("%s: direction %q is not 0-15", path, key)
		}
		d := &Direction{Key: k, parts: make([][]Keyframe, len(t.Parts))}
		for _, kf := range td.Keyframes {
			pi, ok := partIdx[kf.PartID]
			if !ok {
				return nil, fmt.Errorf("%s: direction %s has a keyframe for unknown part id %d", path, key, kf.PartID)
			}
			d.parts[pi] = append(d.parts[pi], Keyframe{
				TimeMs: kf.TimeMs, X: kf.X, Y: kf.Y, Z: kf.Z,
				RotationDeg: kf.RotationDeg, Row: kf.Row, Col: kf.Col, Direction: kf.Direction,
			})
			d.DurationMs = max(d.DurationMs, kf.TimeMs)
		}
		for pi, kfs := range d.parts {
			d.parts[pi] = sortKeyframes(kfs)
		}
		t.Directions[k] = d
	}
	if t.DirectionCount == 0 { // older file: judge by the keys, as the editor does
		maxKey := 0
		for k := range t.Directions {
			maxKey = max(maxKey, k)
		}
		switch {
		case maxKey < 4:
			t.DirectionCount = 4
		case maxKey < 8:
			t.DirectionCount = 8
		default:
			t.DirectionCount = 16
		}
	}
	t.index()

	t.Sheets = l.loadSheets(abs, tt)
	// Registered before nested tracks load, so a track that (directly or
	// not) nests itself gets this one back instead of recursing forever;
	// playback bounds the depth.
	l.tracks[abs] = t

	value := func(v string) PropValue {
		if isAnimValue(v) {
			if nt := l.loadNested(abs, v); nt != nil {
				return PropValue{Track: nt}
			}
			return PropValue{}
		}
		return PropValue{Sheet: t.Sheets[v]}
	}
	for _, p := range tt.Props {
		t.Props = append(t.Props, Prop{Name: p.Name, Default: value(p.Default)})
	}
	for pi, tp := range tt.Parts {
		part := &t.Parts[pi]
		part.Sheet = t.Sheets[tp.FixedSheet]
		if !part.IsNested {
			continue
		}
		if tp.NestedAniPath != "" {
			part.Nested = l.loadNested(abs, tp.NestedAniPath)
		}
		for name, b := range tp.NestedBindings {
			if name == "direction" {
				continue // the old way of pinning direction; see parseDirMode
			}
			if part.Bindings == nil {
				part.Bindings = map[string]Binding{}
			}
			bind := Binding{PassthroughFrom: b.PassthroughFrom}
			if b.PassthroughFrom == "" && b.StaticValue != "" {
				// A static sheet name means a sheet of the nested track's,
				// which resolve looks up there; only .anifs load here.
				if isAnimValue(b.StaticValue) {
					bind.Static = value(b.StaticValue)
				} else {
					bind.Static = PropValue{Sheet: l.sheetNamed(part.Nested, t, tt.Sheets, b.StaticValue)}
				}
			}
			part.Bindings[name] = bind
		}
		// Files from before direction_mode pinned it with a binding.
		if b, ok := tp.NestedBindings["direction"]; ok && tp.DirectionMode == "" && b.PassthroughFrom == "" {
			if n, err := strconv.Atoi(strings.TrimSpace(b.StaticValue)); err == nil {
				part.DirMode, part.StaticDirection = NestedDirStatic, n
			}
		}
	}
	return t, nil
}

// sheetNamed finds the sheet a static binding names: the nested track's,
// else one the parent has, else one found from the parent's folder.
func (l *Library) sheetNamed(nested, parent *Track, refs []tomlSheetRef, name string) *Sheet {
	if nested != nil && nested.Sheets[name] != nil {
		return nested.Sheets[name]
	}
	if s := parent.Sheets[name]; s != nil {
		return s
	}
	found := l.findSheets(parent.Path, refs, map[string]bool{name: true})
	return found[name]
}

// loadNested loads an .anif named (relative to from) by a nested part or
// prop. A missing one is a problem, not an error: that part draws nothing.
func (l *Library) loadNested(from, rel string) *Track {
	t, err := l.LoadTrack(resolve(from, rel))
	if err != nil {
		l.problem("%s: nested animation: %v", from, err)
		return nil
	}
	return t
}

// sortKeyframes sorts a part's keyframes by time, keeping the last of any
// that share a time (as the editor does on load).
func sortKeyframes(kfs []Keyframe) []Keyframe {
	sort.SliceStable(kfs, func(i, j int) bool { return kfs[i].TimeMs < kfs[j].TimeMs })
	out := kfs[:0]
	for _, kf := range kfs {
		if n := len(out); n > 0 && out[n-1].TimeMs == kf.TimeMs {
			out[n-1] = kf
			continue
		}
		out = append(out, kf)
	}
	return out
}

// loadSheets finds a .sprsh for every sheet name the track's parts and
// props name. The track's [[sheets]] list is tried first; otherwise its
// folder is searched for a .sprsh declaring the name, the same fallback
// the editor uses (art lives beside the .anif). A sheet still missing is a
// problem: a nested track's part may draw with its parent's sheet
// instead, and otherwise it draws nothing.
func (l *Library) loadSheets(anifPath string, tt tomlTrack) map[string]*Sheet {
	needed := map[string]bool{}
	need := func(name string) {
		if name != "" && !isAnimValue(name) {
			needed[name] = true
		}
	}
	for _, p := range tt.Parts {
		if p.Kind != "nested_ani" {
			need(p.FixedSheet)
		}
	}
	for _, p := range tt.Props {
		need(p.Default)
	}
	return l.findSheets(anifPath, tt.Sheets, needed)
}

// findSheets loads the named sheets for the .anif at anifPath: from its
// [[sheets]] refs, else by searching its folder. Any not found are
// reported as problems.
func (l *Library) findSheets(anifPath string, refs []tomlSheetRef, needed map[string]bool) map[string]*Sheet {
	out := map[string]*Sheet{}
	for _, ref := range refs {
		if !needed[ref.Name] {
			continue
		}
		if s, err := l.LoadSheet(resolve(anifPath, ref.Path)); err == nil && s.Name == ref.Name {
			out[ref.Name] = s
		}
	}
	want := map[string]bool{}
	for n := range needed {
		if out[n] == nil {
			want[n] = true
		}
	}
	if len(want) == 0 {
		return out
	}
	root := filepath.Dir(anifPath)
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// A few levels, like the editor: art lives beside the track,
			// and an .anif at the top of a big tree mustn't walk it all.
			if rel, _ := filepath.Rel(root, p); rel != "." && strings.Count(rel, string(filepath.Separator)) >= maxSheetSearchDepth-1 {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".sprsh" {
			return nil
		}
		if s, err := l.LoadSheet(p); err == nil && want[s.Name] && out[s.Name] == nil {
			out[s.Name] = s
		}
		return nil
	})
	var missing []string
	for n := range want {
		if out[n] == nil {
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	for _, n := range missing {
		l.problem("%s: no .sprsh found for sheet %q", anifPath, n)
	}
	return out
}

// maxSheetSearchDepth is how many folders deep below an .anif its missing
// sheets are looked for.
const maxSheetSearchDepth = 3

// LoadSheet reads a .sprsh. The image itself isn't decoded here - the
// renderer loads ImagePath into a texture.
func (l *Library) LoadSheet(path string) (*Sheet, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if s := l.sheets[abs]; s != nil {
		return s, nil
	}
	var ts tomlSheet
	if _, err := toml.DecodeFile(abs, &ts); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if ts.CellW <= 0 || ts.CellH <= 0 {
		return nil, fmt.Errorf("%s: cell size %dx%d", path, ts.CellW, ts.CellH)
	}
	img := resolve(abs, ts.FilePath)
	if _, err := os.Stat(img); err != nil {
		return nil, fmt.Errorf("%s: image: %w", path, err)
	}
	s := &Sheet{
		Name: ts.Name, ImagePath: img,
		CellW: ts.CellW, CellH: ts.CellH, PivotX: ts.PivotX, PivotY: ts.PivotY,
	}
	l.sheets[abs] = s
	return s, nil
}

// resolve reads rel (forward slashes, as every animaker path is written)
// relative to the folder of the file that names it.
func resolve(from, rel string) string {
	p := filepath.FromSlash(rel)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(filepath.Dir(from), p)
}

// isAnimValue reports whether a prop value names an .anif rather than a
// sheet - decided by the value itself, as in the editor.
func isAnimValue(v string) bool {
	return strings.EqualFold(filepath.Ext(v), ".anif")
}

func parseMode(s string) (PlayMode, error) {
	switch s {
	case "", "loop":
		return PlayLoop, nil
	case "once":
		return PlayOnce, nil
	case "hold":
		return PlayHold, nil
	}
	return PlayLoop, fmt.Errorf("unknown play mode %q (want loop, once or hold)", s)
}

func parseDirMode(s string) NestedDirMode {
	switch s {
	case "static":
		return NestedDirStatic
	case "keyframe":
		return NestedDirPerKeyframe
	}
	return NestedDirInherit
}

// parseShape reads a hitbox shape as written in an .anichar; "" is rect.
func parseShape(s string) (Shape, error) {
	switch s {
	case "", "rect":
		return ShapeRect, nil
	case "oval":
		return ShapeOval, nil
	case "circle":
		return ShapeCircle, nil
	}
	return ShapeRect, fmt.Errorf("unknown shape %q (want rect, oval or circle)", s)
}
