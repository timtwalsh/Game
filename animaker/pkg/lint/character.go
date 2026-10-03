package lint

import (
	"fmt"

	"animaker/pkg/editor"
	"animaker/pkg/file"
)

// Character checks an .anichar: its animations are the family, so they
// must agree on their props as Family checks, and each animation's
// markers must fall within its track, or they never fire.
func Character(path string) []Finding {
	c, err := file.LoadCharacter(path)
	if err != nil {
		return []Finding{{Error, path, fmt.Sprintf("can't read: %v", err)}}
	}
	if len(c.Animations) == 0 {
		return []Finding{{Info, path, "has no animations"}}
	}
	paths := make([]string, len(c.Animations))
	for i, a := range c.Animations {
		paths[i] = a.AnifPath
	}
	out := Family(paths)
	for _, a := range c.Animations {
		t, _, err := file.LoadTrack(a.AnifPath)
		if err != nil {
			continue // Family reported it
		}
		end := editor.TrackDurationMs(t)
		for _, m := range a.Markers {
			if m.TimeMs > end {
				out = append(out, Finding{Error, path, fmt.Sprintf(
					"animation %q: marker %q at %dms is past the track's end (%dms), so it never fires",
					a.Name, m.Name, m.TimeMs, end)})
			}
		}
	}
	return out
}
