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

	// Parts is the rig: the slot list ("Body", "Hair", "Arm_Left") shared
	// by every direction. A part exists on the Track, not inside one
	// facing, so importing a sheet or renaming a part is immediately true
	// of all directions and switching direction only changes which
	// keyframes you're looking at. What actually differs per facing is the
	// keyframes, which live on Direction.
	Parts []*Part

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

// Direction is one facing's animation data: a set of keyframes per part,
// keyed by Part.ID. Directions are not tweened between each other — each is
// authored separately, since art commonly differs by facing — but they
// share the Track's part list rather than each owning a copy of it.
type Direction struct {
	// Keyframes maps Part.ID to that part's keyframes in this facing,
	// sorted by TimeMs. Keying by ID rather than by index into Track.Parts
	// means removing a part can't silently re-point another part's
	// keyframes, and keying by ID rather than name means renaming is free.
	// A part with no entry here simply isn't animated in this direction.
	Keyframes map[int][]*Keyframe
}

// KeyframesFor returns a part's keyframes in this direction, or nil.
func (d *Direction) KeyframesFor(partID int) []*Keyframe {
	if d.Keyframes == nil {
		return nil
	}
	return d.Keyframes[partID]
}

// TotalDurationMs is the span of this direction's timeline: the latest
// keyframe time across all its parts. This is the animation's real length —
// what playback loops over.
func (d *Direction) TotalDurationMs() uint32 {
	var max uint32
	for _, kfs := range d.Keyframes {
		for _, kf := range kfs {
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

// Part is one named slot in a rig (e.g. "Body", "Hair", "Arm_Left"). It
// holds only the part's identity and art binding — its keyframes live per
// Direction, keyed by ID.
type Part struct {
	// ID is stable for the life of the track and is what each Direction
	// keys its keyframes by. Assigned by AddPart; never reused.
	ID   int
	Name string
	Kind PartKind

	// Sheet kind:
	GoverningProp string // which PropDef selects the active sheet; "" = fixed
	FixedSheet    string // used when GoverningProp == ""

	// NestedAni kind:
	NestedAniPath  string
	NestedBindings map[string]PropBinding // child prop name -> binding ("direction" included)
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
		dirs[k] = NewDirection()
	}
	return &Track{
		Metadata:     TrackMetadata{Name: name, Version: "1.0", CreatedAt: now, UpdatedAt: now},
		Props:        []PropDef{},
		RefBoxWidth:  DefaultRefBoxWidth,
		RefBoxHeight: DefaultRefBoxHeight,
		Parts:        []*Part{},
		Directions:   dirs,
	}
}

// NewDirection creates an empty facing.
func NewDirection() *Direction {
	return &Direction{Keyframes: map[int][]*Keyframe{}}
}

// FindPart returns the part with the given ID, or nil.
func (t *Track) FindPart(id int) *Part {
	for _, p := range t.Parts {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// nextPartID derives the next free ID from the existing parts rather than
// storing a counter, so a loaded track can't hand out an ID already in use.
func (t *Track) nextPartID() int {
	next := 1
	for _, p := range t.Parts {
		if p.ID >= next {
			next = p.ID + 1
		}
	}
	return next
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
