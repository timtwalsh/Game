package editor

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Character is an .anichar: a manifest naming a character's animations -
// "idle" plays human_idle.anif, "walk" human_walk.anif - with facts about
// each (play mode, markers). It never carries transitions, chaining or
// blend rules; the engine's state machine owns those. Each .anif stays
// self-contained and usable without any character.
type Character struct {
	Name string
	// Controller names the engine controller that drives this character
	// ("humanoid"), which decides the animation names it needs. Optional.
	Controller string
	// Animations in name order (kept sorted by AddAnimation/RenameAnimation).
	Animations []*CharAnim

	// Scale is game pixels per animation pixel: how big the character is
	// drawn in the world. 0 means not set; the editor then shows sizes in
	// animation pixels only.
	Scale float32
	// Footprint is the rectangle that blocks movement, relative to the
	// origin; nil when not set. Hitboxes are where the character can be
	// hit. Both are fixed to the origin and shared by every animation
	// (shapes.go).
	Footprint *Box
	Hitboxes  []Hitbox
}

// CharAnim is one named animation of a character.
type CharAnim struct {
	Name     string
	AnifPath string // absolute in memory (AnimKey); relative to the .anichar on disk
	Mode     PlayMode
	Markers  []Marker // sorted by time, then name
}

// PlayMode is how the runtime plays an animation.
type PlayMode int

const (
	// PlayLoop repeats until something else is played.
	PlayLoop PlayMode = iota
	// PlayOnce plays through once and reports that it finished.
	PlayOnce
	// PlayHold plays through once and stays on its last frame.
	PlayHold
)

// PlayModes lists every mode, in menu order.
var PlayModes = []PlayMode{PlayLoop, PlayOnce, PlayHold}

func (m PlayMode) String() string {
	switch m {
	case PlayOnce:
		return "once"
	case PlayHold:
		return "hold"
	}
	return "loop"
}

// ParsePlayMode reads a mode as written in an .anichar; "" is loop.
func ParsePlayMode(s string) (PlayMode, error) {
	for _, m := range PlayModes {
		if s == m.String() {
			return m, nil
		}
	}
	if s == "" {
		return PlayLoop, nil
	}
	return PlayLoop, fmt.Errorf("unknown play mode %q (want loop, once or hold)", s)
}

// Marker is a named instant in an animation, for presentation timing such
// as a footstep sound or a hit spark. Never gameplay timing: when a hit
// lands is gameplay data, so a keyframe nudge can't change balance.
type Marker struct {
	Name   string
	TimeMs uint32
}

// NewCharacter returns an empty character.
func NewCharacter(name string) *Character {
	return &Character{Name: name}
}

// Find returns the animation with this name, or nil.
func (c *Character) Find(name string) *CharAnim {
	for _, a := range c.Animations {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// FindByPath returns the animation playing this .anif, or nil.
func (c *Character) FindByPath(anifPath string) *CharAnim {
	key := AnimKey(anifPath)
	for _, a := range c.Animations {
		if a.AnifPath == key {
			return a
		}
	}
	return nil
}

// ValidAnimName reports why name can't name an animation, or nil. Names
// are what the engine plays by, so they're plain identifiers.
func ValidAnimName(name string) error {
	if name == "" {
		return fmt.Errorf("an animation needs a name")
	}
	for _, r := range name {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return fmt.Errorf("%q: use letters, digits and _ only", name)
		}
	}
	return nil
}

// AddAnimation adds a looping animation playing anifPath under name.
func (c *Character) AddAnimation(name, anifPath string) (*CharAnim, error) {
	if err := ValidAnimName(name); err != nil {
		return nil, err
	}
	if c.Find(name) != nil {
		return nil, fmt.Errorf("the character already has an animation called %q", name)
	}
	if other := c.FindByPath(anifPath); other != nil {
		return nil, fmt.Errorf("that track is already the character's %q animation", other.Name)
	}
	a := &CharAnim{Name: name, AnifPath: AnimKey(anifPath)}
	c.Animations = append(c.Animations, a)
	c.sortAnimations()
	return a, nil
}

// RemoveAnimation drops an animation from the character. Its .anif is
// untouched.
func (c *Character) RemoveAnimation(name string) bool {
	n := len(c.Animations)
	c.Animations = slices.DeleteFunc(c.Animations, func(a *CharAnim) bool { return a.Name == name })
	return len(c.Animations) != n
}

// RenameAnimation renames an animation.
func (c *Character) RenameAnimation(from, to string) error {
	a := c.Find(from)
	if a == nil {
		return fmt.Errorf("no animation called %q", from)
	}
	if from == to {
		return nil
	}
	if err := ValidAnimName(to); err != nil {
		return err
	}
	if c.Find(to) != nil {
		return fmt.Errorf("the character already has an animation called %q", to)
	}
	a.Name = to
	c.sortAnimations()
	return nil
}

func (c *Character) sortAnimations() {
	sort.SliceStable(c.Animations, func(i, j int) bool { return c.Animations[i].Name < c.Animations[j].Name })
}

// SetMarker places marker name at timeMs, adding it if it isn't there.
// A marker may occur more than once (footsteps at 0 and 250ms); at is
// the index of the occurrence to move, or -1 to add another.
func (a *CharAnim) SetMarker(name string, at int, timeMs uint32) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a marker needs a name")
	}
	if err := ValidAnimName(name); err != nil {
		return err
	}
	if at >= 0 && at < len(a.Markers) {
		a.Markers[at] = Marker{name, timeMs}
	} else {
		a.Markers = append(a.Markers, Marker{name, timeMs})
	}
	a.SortMarkers()
	return nil
}

// MarkerIndex is where m is in Markers, or -1. Markers re-sort as they're
// edited, so a row that showed m finds it again by value rather than
// trusting the index it was drawn at.
func (a *CharAnim) MarkerIndex(m Marker) int {
	return slices.Index(a.Markers, m)
}

// RemoveMarker deletes the marker at index i.
func (a *CharAnim) RemoveMarker(i int) {
	if i >= 0 && i < len(a.Markers) {
		a.Markers = slices.Delete(a.Markers, i, i+1)
	}
}

// SortMarkers orders markers by time, then name.
func (a *CharAnim) SortMarkers() {
	sort.SliceStable(a.Markers, func(i, j int) bool {
		if a.Markers[i].TimeMs != a.Markers[j].TimeMs {
			return a.Markers[i].TimeMs < a.Markers[j].TimeMs
		}
		return a.Markers[i].Name < a.Markers[j].Name
	})
}

// MarkerTimes groups the markers by name, each name's times in order -
// the shape an .anichar stores them in.
func (a *CharAnim) MarkerTimes() map[string][]uint32 {
	out := map[string][]uint32{}
	for _, m := range a.Markers {
		out[m.Name] = append(out[m.Name], m.TimeMs)
	}
	for _, ts := range out {
		slices.Sort(ts)
	}
	return out
}

// TrackDurationMs is how long a track plays: its longest direction. A
// marker past it never fires.
func TrackDurationMs(t *Track) uint32 {
	var max uint32
	for _, d := range t.Directions {
		if n := d.TotalDurationMs(); n > max {
			max = n
		}
	}
	return max
}
