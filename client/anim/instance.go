package anim

import (
	"cmp"
	"math"
	"slices"
)

// PlayMode is how an animation plays, from its .anichar entry.
type PlayMode int

const (
	PlayLoop PlayMode = iota // repeats until something else is played
	PlayOnce                 // plays through once; Finished reports the end
	PlayHold                 // plays through once and stays on its last frame
)

// Character is one loaded .anichar: named animations, nothing more. Which
// one plays when is the engine's call (see client/character.go), never
// the asset's.
type Character struct {
	Name       string
	Controller string
	Animations map[string]*Animation

	// Scale is world pixels per animation pixel; 0 when the file doesn't
	// set one. Footprint (nil if unset) is the rectangle that blocks
	// movement and Hitboxes are where the character can be hit: shapes
	// fixed to the animation's origin, in animation pixels, shared by every
	// animation (docs/ANI_MAKER_SPEC.md, "Footprint and hitboxes"). The
	// game doesn't use the footprint or hitboxes yet.
	Scale     float32
	Footprint *Box
	Hitboxes  []Hitbox
}

// Box is a rectangle relative to the animation's origin, in animation
// pixels: X, Y its top-left. An oval or circle is the one inscribed in it.
type Box struct{ X, Y, W, H float32 }

// Shape is a hitbox's shape.
type Shape int

const (
	ShapeRect Shape = iota
	ShapeOval
	ShapeCircle
)

// Hitbox is one named shape where a character can be hit.
type Hitbox struct {
	Name  string
	Shape Shape
	Box   Box
}

type Animation struct {
	Name    string
	Track   *Track
	Mode    PlayMode
	Markers []Marker // sorted by time, then name
}

// Marker is a named instant in an animation, for presentation timing -
// footstep sounds, hit sparks. Gameplay timing never comes from markers;
// the server owns it (docs/ANI_MAKER_SPEC.md, ".anichar").
type Marker struct {
	Name   string
	TimeMs uint32
}

// maxNestDepth bounds nesting, as in the editor, so an animation that
// (directly or not) contains itself stops instead of recursing forever.
const maxNestDepth = 4

// Instance is one on-screen character's playback state - small, and
// separate from the shared asset data it points at.
//
// Its fields are private so everything that changes what's drawn goes
// through a method that keeps the caches right: which sheet or nested
// animation each part plays is resolved into a tree when the animation or
// a prop changes, never per frame (docs/ANI_MAKER_SPEC.md, "Runtime design
// guidance"), and the facing's direction when the facing changes.
type Instance struct {
	char    *Character
	anim    *Animation
	facing  uint8
	props   map[string]PropValue // per-instance values; unset props use the track's default
	elapsed uint32
	// clock is what nested animations play at: it keeps running across
	// animation changes and loops at each nested track's own length, as in
	// the editor.
	clock uint32

	dir  *Direction // anim's direction for facing
	root *node

	// The time the last Advance covered, for AppendCrossedMarkers: after
	// markFrom, up to and including markTo, through the loop point if
	// wrapped. fresh marks a restart not yet advanced, whose first Advance
	// covers 0ms too.
	markFrom, markTo int64
	wrapped, fresh   bool
}

// node is one track's parts resolved for an instance: what each draws
// with. Built by resolve; per frame it only holds scratch.
type node struct {
	track *Track
	parts []nodePart // by track.Parts index
	poses []posed    // scratch, reused every frame
}

type nodePart struct {
	sheet *Sheet // a sheet part's art
	child *node  // a nested part's animation
}

type posed struct {
	pi int
	kf Keyframe
}

func NewInstance(c *Character) *Instance {
	return &Instance{char: c, facing: 4}
}

// Has reports whether the character has an animation called name.
func (i *Instance) Has(name string) bool { return i.char.Animations[name] != nil }

// Playing is the current animation's name, or "".
func (i *Instance) Playing() string {
	if i.anim == nil {
		return ""
	}
	return i.anim.Name
}

// Play switches to the named animation from its start. Playing what's
// already playing changes nothing - use Restart to replay it. False if the
// character has no such animation.
func (i *Instance) Play(name string) bool {
	if i.anim != nil && i.anim.Name == name {
		return true
	}
	return i.Restart(name)
}

// Restart plays the named animation from its start, even if it's playing.
func (i *Instance) Restart(name string) bool {
	a := i.char.Animations[name]
	if a == nil {
		return false
	}
	if a != i.anim {
		i.anim = a
		i.resolve()
	}
	i.elapsed = 0
	i.markFrom, i.markTo, i.wrapped, i.fresh = 0, 0, false, true
	i.dir = a.Track.DirectionFor(i.facing)
	return true
}

// SetFacing turns the character to the game's 8-way facing (0=N ... 7=NW).
// Playback carries on from the same time in the new direction.
func (i *Instance) SetFacing(f uint8) {
	if f == i.facing {
		return
	}
	i.facing = f
	if i.anim != nil {
		i.dir = i.anim.Track.DirectionFor(f)
	}
}

// SetProp sets one of this instance's props: a sheet (e.g. "hair" to a
// long_blonde sheet from Library.LoadSheet) for a prop governing sheet
// parts, or a track for one governing nested parts. The zero value goes
// back to the track's default. A sheet swap reuses the default sheet's
// grid - (Row, Col) means the same pose on every sheet of a template. It
// carries over when the animation changes, as long as the tracks agree on
// the prop (which `animaker lint` checks), and into nested animations
// whose bindings pass it through.
func (i *Instance) SetProp(name string, v PropValue) {
	if i.props[name] == v {
		return
	}
	if v.IsZero() {
		delete(i.props, name)
	} else {
		if i.props == nil {
			i.props = map[string]PropValue{}
		}
		i.props[name] = v
	}
	if i.anim != nil {
		i.resolve()
	}
}

// Seek sets the playback position, e.g. 0 to hold a walk's first frame as
// a standing pose. It crosses no markers.
func (i *Instance) Seek(ms uint32) {
	i.elapsed = ms
	i.markFrom, i.markTo, i.wrapped, i.fresh = int64(ms), int64(ms), false, false
}

// Advance moves playback on by deltaMs. A looping animation wraps; once
// and hold stop at their end. Nested animations keep their own clock.
func (i *Instance) Advance(deltaMs uint32) {
	i.clock += deltaMs
	i.markFrom, i.wrapped = int64(i.elapsed), false
	if i.fresh {
		i.markFrom, i.fresh = -1, false
	}
	if i.dir != nil {
		i.elapsed += deltaMs
		if end := i.dir.DurationMs; i.elapsed > end {
			if i.anim.Mode == PlayLoop && end > 0 {
				i.elapsed %= end
				i.wrapped = true
			} else {
				i.elapsed = end
			}
		}
	}
	i.markTo = int64(i.elapsed)
}

// Finished reports whether a once or hold animation has reached its end.
// A looping one never finishes.
func (i *Instance) Finished() bool {
	if i.dir == nil {
		return true
	}
	return i.anim.Mode != PlayLoop && i.elapsed >= i.dir.DurationMs
}

// AppendCrossedMarkers appends the markers the last Advance passed - after
// where it started, up to and including where it stopped, and from 0ms on
// the first Advance after a Restart - and returns dst. Pass a reused slice
// cut to [:0] to avoid allocating.
func (i *Instance) AppendCrossedMarkers(dst []Marker) []Marker {
	if i.anim == nil {
		return dst
	}
	for _, m := range i.anim.Markers {
		t := int64(m.TimeMs)
		if i.wrapped && (t > i.markFrom || t <= i.markTo) || !i.wrapped && t > i.markFrom && t <= i.markTo {
			dst = append(dst, m)
		}
	}
	return dst
}

// Sprite is one sheet cell to draw this frame. X, Y are where the sheet's
// pivot goes, relative to the character's origin; RotationDeg turns the
// cell about its pivot (positive = clockwise on screen). Z is the Z of the
// top-level part the sprite belongs to.
type Sprite struct {
	Sheet       *Sheet
	Row, Col    int
	X, Y, Z     float32
	RotationDeg float32
}

// AppendSprites appends the current frame's sprites to dst, back to front,
// and returns it. Pass the previous frame's slice cut to [:0] and a
// steady-state frame allocates nothing.
//
// Draw order is by Z within each track. A nested animation's sprites stay
// together in their part's slot of the parent's order, whatever Z values
// the nested track uses, as in the editor.
func (i *Instance) AppendSprites(dst []Sprite) []Sprite {
	if i.dir == nil || i.root == nil {
		return dst
	}
	return i.root.emit(dst, i.dir, i.elapsed, i.clock, xform{}, true)
}

// xform is where a nested animation's origin sits in the character.
type xform struct {
	x, y, rot, z float32
}

// place puts a pose given relative to x into the character's space.
func (x xform) place(kf Keyframe) xform {
	px, py := kf.X, kf.Y
	if x.rot != 0 {
		s, c := math.Sincos(float64(x.rot) * math.Pi / 180)
		px, py = px*float32(c)-py*float32(s), px*float32(s)+py*float32(c)
	}
	return xform{x: x.x + px, y: x.y + py, rot: x.rot + kf.RotationDeg, z: x.z}
}

func (n *node) emit(dst []Sprite, dir *Direction, at, clock uint32, x xform, top bool) []Sprite {
	n.poses = n.poses[:0]
	for pi, np := range n.parts {
		if np.sheet == nil && np.child == nil {
			continue
		}
		if kf, ok := dir.valueAt(pi, at); ok {
			n.poses = append(n.poses, posed{pi, kf})
		}
	}
	// Z can be keyframed, so the order can change mid-animation.
	slices.SortStableFunc(n.poses, func(a, b posed) int { return cmp.Compare(a.kf.Z, b.kf.Z) })

	for _, p := range n.poses {
		at := x.place(p.kf)
		if top {
			at.z = p.kf.Z
		}
		np := n.parts[p.pi]
		if np.sheet != nil {
			dst = append(dst, Sprite{
				Sheet: np.sheet, Row: p.kf.Row, Col: p.kf.Col,
				X: at.x, Y: at.y, Z: at.z, RotationDeg: at.rot,
			})
			continue
		}
		c := np.child
		part := &n.track.Parts[p.pi]
		var cdir *Direction
		switch part.DirMode {
		case NestedDirStatic:
			cdir = c.track.directionForKey(part.StaticDirection)
		case NestedDirPerKeyframe:
			cdir = c.track.directionForKey(p.kf.Direction)
		default:
			cdir = c.track.inheritedDirection(dir.Key, n.track.DirectionCount)
		}
		if cdir == nil {
			continue
		}
		var cat uint32
		if cdir.DurationMs > 0 {
			cat = clock % cdir.DurationMs // its own clock, looping at its own length
		}
		dst = c.emit(dst, cdir, cat, clock, at, false)
	}
	return dst
}

// resolve rebuilds the part tree for the current animation and props.
func (i *Instance) resolve() {
	t := i.anim.Track
	props := make(map[string]PropValue, len(t.Props))
	for _, p := range t.Props {
		v := p.Default
		if iv, ok := i.props[p.Name]; ok {
			v = iv
		}
		props[p.Name] = v
	}
	i.root = resolveNode(t, nil, props, 1)
}

// resolveNode works out what each of t's parts draws with, given t's prop
// values. parent is the track nesting t, whose sheets a part of t may use
// when t lacks them (as in the editor).
func resolveNode(t, parent *Track, props map[string]PropValue, depth int) *node {
	n := &node{track: t, parts: make([]nodePart, len(t.Parts))}
	for pi := range t.Parts {
		p := &t.Parts[pi]
		v := props[p.GoverningProp] // zero if no prop
		if !p.IsNested {
			switch {
			case p.GoverningProp != "" && v.Sheet != nil:
				n.parts[pi].sheet = v.Sheet
			case p.GoverningProp == "" && p.Sheet != nil:
				n.parts[pi].sheet = p.Sheet
			case p.GoverningProp == "" && parent != nil:
				n.parts[pi].sheet = parent.Sheets[p.FixedSheet]
			}
			continue
		}
		child := p.Nested
		if v.Track != nil {
			child = v.Track
		}
		if child == nil || depth >= maxNestDepth {
			continue
		}
		n.parts[pi].child = resolveNode(child, t, childProps(child, props, p.Bindings), depth+1)
	}
	return n
}

// childProps resolves a nested track's prop values: each binding either
// mirrors a parent prop or pins a value; unbound props use their default.
func childProps(t *Track, parentProps map[string]PropValue, bindings map[string]Binding) map[string]PropValue {
	props := make(map[string]PropValue, len(t.Props))
	for _, p := range t.Props {
		v := p.Default
		if b, ok := bindings[p.Name]; ok {
			switch {
			case b.PassthroughFrom != "":
				if pv := parentProps[b.PassthroughFrom]; !pv.IsZero() {
					v = pv
				}
			case !b.Static.IsZero():
				v = b.Static
			}
		}
		props[p.Name] = v
	}
	return props
}
