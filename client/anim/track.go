package anim

import (
	"math"
	"slices"
)

// Track is one loaded .anif: shared, read-only asset data. Never copy it
// per instance.
type Track struct {
	Name string
	Path string // absolute
	// DirectionCount is 1, 2, 4, 8 or 16. Keys run clockwise from north
	// (k faces k*360/N degrees), except 2, which is E, W.
	DirectionCount int
	Parts          []Part
	Props          []Prop
	Directions     map[int]*Direction
	// Sheets are the sheets this track names, by name. A nested track's
	// part may also draw with a sheet only its parent has (see resolve).
	Sheets map[string]*Sheet

	// byCompass is the key that plays for each of the 16 compass points,
	// and byKey the direction each key plays (a key with no keyframes
	// plays the first posed direction). Both are worked out once at load,
	// so choosing a direction during playback is two array reads.
	byCompass [16]int
	byKey     [16]*Direction
}

// Prop is a customization slot the track declares. Its default is a sheet
// name, or the path of an .anif for a prop governing nested parts.
type Prop struct {
	Name    string
	Default PropValue
}

// PropValue is what a prop is set to: a sheet for props governing sheet
// parts, or an animation for props governing nested parts.
type PropValue struct {
	Sheet *Sheet
	Track *Track
}

// IsZero reports whether v is unset.
func (v PropValue) IsZero() bool { return v.Sheet == nil && v.Track == nil }

type Part struct {
	ID            int
	Name          string
	GoverningProp string
	// Sheet is a sheet part's fixed sheet, used when it has no governing
	// prop; nil if the track's own sheets lack it (resolve then tries the
	// parent's). FixedSheet is its name.
	Sheet      *Sheet
	FixedSheet string

	// Nested parts play another track (Nested is non-nil, or a governing
	// prop supplies one).
	Nested          *Track
	IsNested        bool
	DirMode         NestedDirMode
	StaticDirection int
	Bindings        map[string]Binding
}

// NestedDirMode is how a nested part picks its animation's direction.
type NestedDirMode int

const (
	NestedDirInherit     NestedDirMode = iota // turns with the parent
	NestedDirStatic                           // always Part.StaticDirection
	NestedDirPerKeyframe                      // each keyframe's Direction, stepping like a cell
)

// Binding sets one of a nested track's props from its parent: mirror a
// parent prop (PassthroughFrom), or pin a value (Static).
type Binding struct {
	PassthroughFrom string
	Static          PropValue
}

type Direction struct {
	Key        int
	DurationMs uint32       // the latest keyframe: what playback spans
	parts      [][]Keyframe // by Track.Parts index, sorted by time
}

type Keyframe struct {
	TimeMs      uint32
	X, Y, Z     float32
	RotationDeg float32
	Row, Col    int // sheet parts; step, not interpolated
	Direction   int // nested parts in per-keyframe mode; steps like Row/Col
}

// Sheet is one .sprsh: a grid over an image, with every cell drawn about
// the same pivot.
type Sheet struct {
	Name           string
	ImagePath      string
	CellW, CellH   int
	PivotX, PivotY float32
}

// valueAt is part pi's pose at timeMs, matching the editor's preview:
// X/Y/Z/rotation interpolate linearly between the surrounding keyframes;
// Row/Col/Direction step (they hold the earlier keyframe's value until the
// next is reached). ok is false if the part has no keyframes here.
func (d *Direction) valueAt(pi int, timeMs uint32) (kf Keyframe, ok bool) {
	kfs := d.parts[pi]
	if len(kfs) == 0 {
		return Keyframe{}, false
	}
	// The first keyframe at or after timeMs.
	i, found := slices.BinarySearchFunc(kfs, timeMs, func(k Keyframe, t uint32) int {
		return int(int64(k.TimeMs) - int64(t))
	})
	switch {
	case found:
		return kfs[i], true
	case i == 0:
		return kfs[0], true
	case i == len(kfs):
		return kfs[i-1], true
	}
	a, b := kfs[i-1], kfs[i]
	t := float32(timeMs-a.TimeMs) / float32(b.TimeMs-a.TimeMs)
	a.X = lerp(a.X, b.X, t)
	a.Y = lerp(a.Y, b.Y, t)
	a.Z = lerp(a.Z, b.Z, t)
	a.RotationDeg = lerp(a.RotationDeg, b.RotationDeg, t)
	return a, true
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// DirectionFor is the direction to play for the game's 8-way facing (0=N,
// 1=NE ... 7=NW, client/prediction.go), or nil if nothing is posed.
func (t *Track) DirectionFor(facing uint8) *Direction {
	return t.byKey[t.byCompass[int(facing%8)*2]]
}

// directionForKey is the direction that plays when key k is asked for -
// by a nested part's static or per-keyframe direction.
func (t *Track) directionForKey(k int) *Direction {
	if k < 0 || k >= len(t.byKey) {
		k = 0 // not a key of any track; byKey[0] falls back like any unposed key
	}
	return t.byKey[k]
}

// inheritedDirection is the direction a nested part set to inherit plays
// when its parent plays direction parentKey of a parentN-direction track:
// the one facing the same way.
func (t *Track) inheritedDirection(parentKey, parentN int) *Direction {
	p, ok := compassPoint(parentKey, parentN)
	if !ok {
		p = 0
	}
	return t.byKey[t.byCompass[p]]
}

// index fills byCompass and byKey. A compass point between two of the
// track's directions - NE on a 4-direction track - goes to the sideways
// one (E), as the editor's MapDirection does. A key with no keyframes
// plays the lowest-keyed posed direction, as the editor's pickDirection
// does. Call it once the track's directions are final.
func (t *Track) index() {
	var first *Direction
	for k, d := range t.Directions {
		if d.posed() && (first == nil || k < first.Key) {
			first = d
		}
	}
	for k := range t.byKey {
		if d := t.Directions[k]; d != nil && d.posed() {
			t.byKey[k] = d
		} else {
			t.byKey[k] = first
		}
	}
	for p := range t.byCompass {
		t.byCompass[p] = t.nearestKey(float64(p) * 360 / 16)
	}
}

func (d *Direction) posed() bool {
	for _, kfs := range d.parts {
		if len(kfs) > 0 {
			return true
		}
	}
	return false
}

func (t *Track) nearestKey(want float64) int {
	n := t.DirectionCount
	if n <= 1 {
		return 0
	}
	best, bestDist, bestSide := 0, math.Inf(1), math.Inf(1)
	for k := 0; k < n; k++ {
		p, _ := compassPoint(k, n)
		a := float64(p) * 360 / 16
		dist := math.Abs(math.Mod(a-want+540, 360) - 180)
		// How far from straight sideways (90 or 270): the tie-break.
		side := 90 - math.Abs(math.Mod(a+90, 180)-90)
		if dist < bestDist-1e-9 || (math.Abs(dist-bestDist) < 1e-9 && side < bestSide) {
			best, bestDist, bestSide = k, dist, side
		}
	}
	return best
}

// compassPoint is which of the 16 compass points (0=N, clockwise) key k
// of an n-direction track faces; false if n isn't a count that divides
// the compass or k is outside 0..n-1.
func compassPoint(k, n int) (int, bool) {
	switch {
	case n <= 0 || n > 16 || 16%n != 0 || k < 0 || k >= n:
		return 0, false
	case n == 2: // side-scrolling: 0=E, 1=W
		return 4 + 8*k, true
	}
	return k * 16 / n, true
}
