package editor

import "time"

// Track is one named motion (e.g. "human_walk") and corresponds to exactly
// one .anif file. There is no wrapping "character" asset — see
// docs/ANI_MAKER_SPEC.md's Props section for why.
type Track struct {
	Metadata   TrackMetadata
	Props      []PropDef
	Directions map[string]*Direction // "up"/"right"/"down"/"left", or "default"
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
// keyframe time across all its parts.
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

// NewTrack creates a track with a single "default" direction and no parts.
func NewTrack(name string) *Track {
	now := time.Now()
	return &Track{
		Metadata: TrackMetadata{Name: name, Version: "1.0", CreatedAt: now, UpdatedAt: now},
		Props:    []PropDef{},
		Directions: map[string]*Direction{
			"default": {Parts: []*Part{}},
		},
	}
}

// SortedDirectionNames returns direction keys in a stable, sensible order:
// the common four first (if present), then anything else alphabetically.
func (t *Track) SortedDirectionNames() []string {
	preferred := []string{"up", "right", "down", "left", "default"}
	seen := map[string]bool{}
	var out []string
	for _, name := range preferred {
		if _, ok := t.Directions[name]; ok {
			out = append(out, name)
			seen[name] = true
		}
	}
	for name := range t.Directions {
		if !seen[name] {
			out = append(out, name)
		}
	}
	return out
}
