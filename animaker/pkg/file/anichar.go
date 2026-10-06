package file

import (
	"animaker/pkg/editor"
	"bytes"
	"fmt"
	"math"
	"sort"

	"github.com/BurntSushi/toml"
)

// ---- TOML structure for .anichar files (a Character) ----

type tomlCharacter struct {
	Name       string `toml:"name"`
	Controller string `toml:"controller,omitempty"`
	// Scale, Footprint and Hitboxes: see editor/shapes.go. Shapes are in
	// animation pixels relative to the origin.
	Scale      float32                      `toml:"scale,omitzero"`
	Footprint  *tomlBox                     `toml:"footprint,omitempty"`
	Hitboxes   []tomlHitbox                 `toml:"hitboxes,omitempty"`
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
	// (footstep = [0, 250]); decoded by hand for that reason.
	Markers map[string]any `toml:"markers,omitempty"`
}

// SaveCharacter writes a character to an .anichar, with each animation's
// .anif relative to it.
func SaveCharacter(c *editor.Character, path string) error {
	tc := tomlCharacter{Name: c.Name, Controller: c.Controller, Scale: c.Scale, Animations: map[string]tomlCharAnimation{}}
	if b := c.Footprint; b != nil {
		tc.Footprint = &tomlBox{b.X, b.Y, b.W, b.H}
	}
	for _, h := range c.Hitboxes {
		tc.Hitboxes = append(tc.Hitboxes, tomlHitbox{h.Name, h.Kind.String(), h.Box.X, h.Box.Y, h.Box.W, h.Box.H})
	}
	for _, a := range c.Animations {
		ta := tomlCharAnimation{Anif: relAnimPath(path, a.AnifPath), Mode: a.Mode.String()}
		for name, times := range a.MarkerTimes() {
			if ta.Markers == nil {
				ta.Markers = map[string]any{}
			}
			if len(times) == 1 {
				ta.Markers[name] = int64(times[0])
				continue
			}
			ts := make([]int64, len(times))
			for i, t := range times {
				ts[i] = int64(t)
			}
			ta.Markers[name] = ts
		}
		tc.Animations[a.Name] = ta
	}
	buf := &bytes.Buffer{}
	if err := toml.NewEncoder(buf).Encode(tc); err != nil {
		return fmt.Errorf("failed to encode character: %w", err)
	}
	return writeFileAtomic(path, buf.Bytes())
}

// LoadCharacter reads an .anichar. Each animation's .anif path comes back
// absolute; the tracks themselves aren't loaded.
func LoadCharacter(path string) (*editor.Character, error) {
	var tc tomlCharacter
	if _, err := toml.DecodeFile(path, &tc); err != nil {
		return nil, fmt.Errorf("failed to decode character: %w", err)
	}
	c := &editor.Character{Name: tc.Name, Controller: tc.Controller}
	if tc.Scale < 0 {
		return nil, fmt.Errorf("scale %g must be positive", tc.Scale)
	}
	c.Scale = tc.Scale
	if b := tc.Footprint; b != nil {
		if err := c.SetFootprint(editor.Box{X: b.X, Y: b.Y, W: b.W, H: b.H}); err != nil {
			return nil, err
		}
	}
	for _, th := range tc.Hitboxes {
		kind, err := editor.ParseShapeKind(th.Shape)
		if err != nil {
			return nil, fmt.Errorf("hitbox %q: %w", th.Name, err)
		}
		i := len(c.Hitboxes)
		c.Hitboxes = append(c.Hitboxes, editor.Hitbox{})
		h := editor.Hitbox{Name: th.Name, Kind: kind, Box: editor.Box{X: th.X, Y: th.Y, W: th.W, H: th.H}}
		if err := c.SetHitbox(i, h); err != nil {
			return nil, err
		}
	}
	names := make([]string, 0, len(tc.Animations))
	for n := range tc.Animations {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ta := tc.Animations[n]
		if err := editor.ValidAnimName(n); err != nil {
			return nil, fmt.Errorf("animation %w", err)
		}
		if ta.Anif == "" {
			return nil, fmt.Errorf("animation %q names no .anif", n)
		}
		mode, err := editor.ParsePlayMode(ta.Mode)
		if err != nil {
			return nil, fmt.Errorf("animation %q: %w", n, err)
		}
		a := &editor.CharAnim{Name: n, AnifPath: absAnimPath(path, ta.Anif), Mode: mode}
		for mn, v := range ta.Markers {
			times, err := markerTimes(v)
			if err != nil {
				return nil, fmt.Errorf("animation %q, marker %q: %w", n, mn, err)
			}
			for _, t := range times {
				a.Markers = append(a.Markers, editor.Marker{Name: mn, TimeMs: t})
			}
		}
		a.SortMarkers()
		c.Animations = append(c.Animations, a)
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
