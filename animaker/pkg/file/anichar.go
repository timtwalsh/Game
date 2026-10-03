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
	Name       string                       `toml:"name"`
	Controller string                       `toml:"controller,omitempty"`
	Animations map[string]tomlCharAnimation `toml:"animations"`
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
	tc := tomlCharacter{Name: c.Name, Controller: c.Controller, Animations: map[string]tomlCharAnimation{}}
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
