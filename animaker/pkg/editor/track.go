package editor

import (
	"sort"
	"time"
)

// Track is one named motion (e.g. "human_walk") and corresponds to exactly
// one .anif file. There is no wrapping "character" asset — see
// docs/ANI_MAKER_SPEC.md's Props section for why.
type Track struct {
	Metadata TrackMetadata
	Props    []PropDef

	// RefBoxWidth/Height size the "character-sized" reference box the
	// editor draws from the origin down-right, as a placement guide only.
	// It is NOT a working area or a clip region: parts may sit anywhere,
	// including at negative coordinates above/left of the origin (a raised
	// sword, a trailing cape). The editor's canvas grows to fit whatever
	// is actually placed rather than bounding it — see pkg/ui/canvas.go.
	RefBoxWidth, RefBoxHeight int

	// Directions are keyed by a plain int (0, 1, 2, ...) matching the
	// game's own direction convention (0=up, 1=right, 2=down, 3=left),
	// not free-form names. 0 is the default/first direction.
	Directions map[int]*Direction
}

type TrackMetadata struct {
	Name      string
	Version   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PropDef declares a customization slot. A prop's value is always the name
// of a sprite sheet to use for whichever Parts it governs.
type PropDef struct {
	Name    string
	Default string // a sheet name
}

// Direction is a fully independent set of Parts/Keyframes. Directions are
// not tweened between each other — each is authored separately, since art
// commonly differs by facing.
type Direction struct {
	Parts []*Part
}

// TotalDurationMs is the span of this direction's timeline: the latest
// keyframe time across all its parts. This is the animation's real length —
// what playback loops over.
func (d *Direction) TotalDurationMs() uint32 {
	var max uint32
	for _, p := range d.Parts {
		for _, kf := range p.Keyframes {
			if kf.TimeMs > max {
				max = kf.TimeMs
			}
		}
	}
	return max
}

// MinTimelineMs / TimelineHeadroomMs keep the scrubbable range strictly
// ahead of the last keyframe.
const (
	MinTimelineMs      = 1000
	TimelineHeadroomMs = 500
)

// EditableDurationMs is how far the playhead may be scrubbed, which is
// deliberately longer than TotalDurationMs. Clamping the playhead to the
// real duration is a deadlock: a brand-new part has one keyframe at 0ms, so
// the duration is 0, so the playhead can't leave 0ms, so "New Keyframe"
// (which adds at the playhead, and dedupes an exact time collision) can
// never add a second one. There must always be empty time ahead to scrub
// into and drop the next keyframe onto.
func (d *Direction) EditableDurationMs() uint32 {
	if total := d.TotalDurationMs(); total+TimelineHeadroomMs > MinTimelineMs {
		return total + TimelineHeadroomMs
	}
	return MinTimelineMs
}

type PartKind int

const (
	PartKindSheet PartKind = iota
	PartKindNestedAni
)

func (k PartKind) String() string {
	if k == PartKindNestedAni {
		return "nested_ani"
	}
	return "sheet"
}

// Part is one named slot in a rig (e.g. "Body", "Hair", "Arm_Left").
type Part struct {
	Name string
	Kind PartKind

	// Sheet kind:
	GoverningProp string // which PropDef selects the active sheet; "" = fixed
	FixedSheet    string // used when GoverningProp == ""

	// NestedAni kind:
	NestedAniPath  string
	NestedBindings map[string]PropBinding // child prop name -> binding ("direction" included)

	// Keyframes are sorted by TimeMs. They need not line up 1:1 with other
	// Parts' keyframe times — the "shared timeline" is the common ElapsedMs
	// axis playback evaluates every part against, not a requirement that
	// every part define a keyframe at every same tick.
	Keyframes []*Keyframe
}

// PropBinding configures one prop on a nested Part: either mirror one of
// the parent Track's own props, or pin a static value.
type PropBinding struct {
	PassthroughFrom string // name of a prop on the parent Track; "" if using StaticValue
	StaticValue     string
}

// Keyframe holds a Part's placement at a point in time. Row/Col (Sheet
// parts only) is explicitly chosen by the artist per keyframe — never
// derived from the Track's name, direction, or frame index.
type Keyframe struct {
	ID          int
	TimeMs      uint32
	X, Y, Z     float32 // Z is tweened like X/Y, feeds draw-order sort
	RotationDeg float32
	Row, Col    int // Sheet kind only
}

// DefaultRefBoxWidth/Height size the reference box to roughly one
// character, matching the game's own sprite footprint. Freely editable per
// track; this is just a sane starting guide.
const (
	DefaultRefBoxWidth  = 48
	DefaultRefBoxHeight = 64
)

// DefaultDirectionKeys are the four facings every track starts with, in the
// game's own convention. A track almost always needs all four, and adding
// them up front is cheaper than making the artist create each by hand;
// unused ones simply stay empty and cost nothing on disk beyond a header.
var DefaultDirectionKeys = []int{0, 1, 2, 3} // 0=up, 1=right, 2=down, 3=left

// NewTrack creates a track with the four default directions and no parts.
func NewTrack(name string) *Track {
	now := time.Now()
	dirs := make(map[int]*Direction, len(DefaultDirectionKeys))
	for _, k := range DefaultDirectionKeys {
		dirs[k] = &Direction{Parts: []*Part{}}
	}
	return &Track{
		Metadata:     TrackMetadata{Name: name, Version: "1.0", CreatedAt: now, UpdatedAt: now},
		Props:        []PropDef{},
		RefBoxWidth:  DefaultRefBoxWidth,
		RefBoxHeight: DefaultRefBoxHeight,
		Directions:   dirs,
	}
}

// SortedDirectionKeys returns direction keys in ascending numeric order.
func (t *Track) SortedDirectionKeys() []int {
	keys := make([]int, 0, len(t.Directions))
	for k := range t.Directions {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
