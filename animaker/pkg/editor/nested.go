package editor

import (
	"path/filepath"
	"sort"
	"strings"
)

// NestedAnim is another .anif loaded so it can play inside this one, as a
// NestedAni part. It's read-only here: the editor never saves it.
type NestedAnim struct {
	Path  string // absolute, cleaned; the key in Project.LoadedAnims
	Track *Track
	// Sheets are the nested track's own sheets, kept apart from the
	// parent's: two tracks can each have a sheet called "body".
	Sheets map[string]*SpriteSheetTemplate
}

// DisplayName is how a nested animation is labelled in pickers: its file
// name, e.g. "torch.anif".
func (n *NestedAnim) DisplayName() string { return filepath.Base(n.Path) }

// IsAnimValue reports whether a prop value names a nested animation (an
// .anif path) rather than a sprite sheet. A prop's value is a sheet name for
// props governing sheet parts, and an .anif path for props governing nested
// parts; which one is decided by the value itself, so the file format needs
// no separate prop type.
func IsAnimValue(v string) bool {
	return strings.EqualFold(filepath.Ext(v), ".anif")
}

// AnimKey normalizes an .anif path to the form LoadedAnims is keyed by.
func AnimKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

// IsAnimProp reports whether a prop holds nested animations.
func (pd PropDef) IsAnimProp() bool { return IsAnimValue(pd.Default) }

// propValue is a prop's current value: the preview override if set, else
// its default.
func (p *Project) propValue(pd PropDef) string {
	if v, ok := p.PreviewProps[pd.Name]; ok && v != "" {
		return v
	}
	return pd.Default
}

// ResolveNestedAnimPath is the .anif a nested part currently plays: its
// governing prop's value when that prop holds animations, else its own
// NestedAniPath.
func (p *Project) ResolveNestedAnimPath(part *Part) string {
	if part.GoverningProp != "" {
		if pd := p.CurrentTrack.FindProp(part.GoverningProp); pd != nil && pd.IsAnimProp() {
			return p.propValue(*pd)
		}
	}
	return part.NestedAniPath
}

// ResolveNestedAnim is ResolveNestedAnimPath plus the lookup, or nil if
// that animation isn't loaded.
func (p *Project) ResolveNestedAnim(part *Part) *NestedAnim {
	path := p.ResolveNestedAnimPath(part)
	if path == "" {
		return nil
	}
	return p.LoadedAnims[AnimKey(path)]
}

// LoadedAnimPaths lists loaded nested animations' paths, sorted by file
// name, for pickers.
func (p *Project) LoadedAnimPaths() []string {
	paths := make([]string, 0, len(p.LoadedAnims))
	for k := range p.LoadedAnims {
		paths = append(paths, k)
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.ToLower(filepath.Base(paths[i])) < strings.ToLower(filepath.Base(paths[j]))
	})
	return paths
}

// FlatSprite is one sheet cell to draw, resolved through any amount of
// nesting, positioned relative to the nested part's own origin.
type FlatSprite struct {
	Sheet    *SpriteSheetTemplate
	Row, Col int
	X, Y     float32 // where the cell's pivot goes
	Z        float32
}

// maxNestDepth bounds recursion, so an animation that (directly or not)
// contains itself can't hang the editor.
const maxNestDepth = 4

// NestedClockMs is the time nested animations play at. They run on their
// own clock, looping at their own length rather than being cut off when
// the parent loops (docs/ANI_MAKER_SPEC.md, "Nested-animation part").
func (p *Project) NestedClockMs() uint32 { return p.Playback.NestedClockMs }

// FlattenNested resolves a nested part into the sprites to draw at the
// current nested clock, back-to-front, relative to the part's position.
// Returns nil if the animation isn't loaded or has nothing posed.
func (p *Project) FlattenNested(part *Part) []FlatSprite {
	anim := p.ResolveNestedAnim(part)
	if anim == nil {
		return nil
	}
	parentProps := map[string]string{}
	for _, pd := range p.CurrentTrack.Props {
		parentProps[pd.Name] = p.propValue(pd)
	}
	var tr ResolvedTransform
	if dir := p.ActiveDirection(); dir != nil {
		tr = dir.ValueAt(part.ID, p.Playback.ElapsedMs)
	}
	want := part.NestedDirectionAt(p.Playback.ActiveDirection, tr)
	sprites := p.flatten(anim, want, parentProps, part.NestedBindings, p.NestedClockMs(), 1)
	sort.SliceStable(sprites, func(i, j int) bool { return sprites[i].Z < sprites[j].Z })
	return sprites
}

// flatten draws one nested animation in the direction its part asked for
// (want; see Part.NestedDirectionAt). parentProps are what its bindings can
// pass through from.
func (p *Project) flatten(anim *NestedAnim, want int, parentProps map[string]string,
	bindings map[string]PropBinding, clockMs uint32, depth int) []FlatSprite {
	if depth > maxNestDepth {
		return nil
	}
	t := anim.Track
	dirKey, ok := pickDirection(t, want)
	if !ok {
		return nil
	}
	dir := t.Directions[dirKey]

	// Its own clock: loop at its own length.
	at := clockMs
	if total := dir.TotalDurationMs(); total > 0 {
		at %= total
	} else {
		at = 0
	}

	props := childProps(t, parentProps, bindings)
	var out []FlatSprite
	for _, part := range t.Parts {
		if len(dir.KeyframesFor(part.ID)) == 0 {
			continue
		}
		tr := dir.ValueAt(part.ID, at)
		switch part.Kind {
		case PartKindSheet:
			name := part.FixedSheet
			if part.GoverningProp != "" {
				name = props[part.GoverningProp]
			}
			sheet := anim.Sheets[name]
			if sheet == nil {
				sheet = p.LoadedSheets[name] // the parent may have it loaded
			}
			if sheet == nil {
				continue
			}
			out = append(out, FlatSprite{Sheet: sheet, Row: tr.Row, Col: tr.Col, X: tr.X, Y: tr.Y, Z: tr.Z})
		case PartKindNestedAni:
			path := part.NestedAniPath
			if part.GoverningProp != "" && IsAnimValue(props[part.GoverningProp]) {
				path = props[part.GoverningProp]
			}
			child := p.LoadedAnims[AnimKey(path)]
			if child == nil {
				continue
			}
			childWant := part.NestedDirectionAt(dirKey, tr)
			for _, s := range p.flatten(child, childWant, props, part.NestedBindings, clockMs, depth+1) {
				s.X += tr.X
				s.Y += tr.Y
				// Kept within this part's slot in the draw order: a nested
				// sprite's own Z only orders it among its siblings.
				s.Z = tr.Z + s.Z/1000
				out = append(out, s)
			}
		}
	}
	return out
}

// pickDirection is the direction of t that plays when want is asked for:
// want itself, or t's first direction if t doesn't have it.
func pickDirection(t *Track, want int) (int, bool) {
	if _, ok := t.Directions[want]; ok {
		return want, true
	}
	keys := t.SortedDirectionKeys()
	if len(keys) == 0 {
		return 0, false
	}
	return keys[0], true
}

// childProps resolves a nested track's prop values: each binding either
// mirrors a parent prop or pins a value; unbound props use their default.
func childProps(t *Track, parentProps map[string]string, bindings map[string]PropBinding) map[string]string {
	props := make(map[string]string, len(t.Props))
	for _, pd := range t.Props {
		v := pd.Default
		if b, ok := bindings[pd.Name]; ok {
			switch {
			case b.PassthroughFrom != "":
				if pv, ok := parentProps[b.PassthroughFrom]; ok && pv != "" {
					v = pv
				}
			case b.StaticValue != "":
				v = b.StaticValue
			}
		}
		props[pd.Name] = v
	}
	return props
}

// NestedExtent is the box, relative to the nested part's origin, covering
// every keyframe of the animation it currently plays in the direction it
// would show - so the canvas can size itself without resizing as the
// nested animation plays. ok is false when there's nothing to measure.
func (p *Project) NestedExtent(part *Part) (minX, minY, maxX, maxY float32, ok bool) {
	anim := p.ResolveNestedAnim(part)
	if anim == nil {
		return 0, 0, 0, 0, false
	}
	parentProps := map[string]string{}
	for _, pd := range p.CurrentTrack.Props {
		parentProps[pd.Name] = p.propValue(pd)
	}
	// Every direction the part can show here: one, unless it changes per
	// keyframe, in which case the box covers each of them.
	wants := map[int]bool{}
	if parentDir := p.ActiveDirection(); part.DirectionMode == NestedDirPerKeyframe && parentDir != nil {
		for _, kf := range parentDir.KeyframesFor(part.ID) {
			wants[kf.Direction] = true
		}
	}
	if len(wants) == 0 {
		wants[part.NestedDirectionAt(p.Playback.ActiveDirection, ResolvedTransform{})] = true
	}

	first := true
	for want := range wants {
		dirKey, found := pickDirection(anim.Track, want)
		if !found {
			continue
		}
		dir := anim.Track.Directions[dirKey]
		// Sampling every keyframe time of every part covers each keyframed
		// pose; in-between positions are lerps of those, so they're inside.
		times := map[uint32]bool{0: true}
		for _, kfs := range dir.Keyframes {
			for _, kf := range kfs {
				times[kf.TimeMs] = true
			}
		}
		total := dir.TotalDurationMs()
		for tm := range times {
			// flatten loops at the total, so the last keyframe itself is
			// sampled just before it wraps.
			if total > 0 && tm >= total {
				tm = total - 1
			}
			for _, s := range p.flatten(anim, want, parentProps, part.NestedBindings, tm, 1) {
				x0, y0 := s.X-s.Sheet.PivotX, s.Y-s.Sheet.PivotY
				x1, y1 := x0+float32(s.Sheet.CellW), y0+float32(s.Sheet.CellH)
				if first {
					minX, minY, maxX, maxY, first = x0, y0, x1, y1, false
					continue
				}
				minX, minY = minF32(minX, x0), minF32(minY, y0)
				maxX, maxY = maxF32(maxX, x1), maxF32(maxY, y1)
			}
		}
	}
	return minX, minY, maxX, maxY, !first
}

func minF32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// SetNestedDirectionMode changes how a nested part picks its animation's
// direction, carrying over what it currently shows so nothing visibly
// jumps: switching to per-keyframe gives every keyframe (in every parent
// direction) the direction it was already playing, and switching to static
// starts from the direction showing at the playhead.
func (p *Project) SetNestedDirectionMode(part *Part, mode NestedDirMode) {
	if part.DirectionMode == mode {
		return
	}
	switch mode {
	case NestedDirPerKeyframe:
		for parentDir, d := range p.CurrentTrack.Directions {
			for _, kf := range d.KeyframesFor(part.ID) {
				kf.Direction = part.NestedDirectionAt(parentDir, kfToResolved(kf))
			}
		}
	case NestedDirStatic:
		var tr ResolvedTransform
		if d := p.ActiveDirection(); d != nil {
			tr = d.ValueAt(part.ID, p.Playback.ElapsedMs)
		}
		part.StaticDirection = part.NestedDirectionAt(p.Playback.ActiveDirection, tr)
	}
	part.DirectionMode = mode
	p.Dirty = true
}

// SheetFromNested reports whether a loaded nested animation has a sheet
// called name. A nested part's fixed binding can name one of its own
// animation's sheets, so a track that names such a sheet isn't missing it
// even though nothing near the track declares it.
func (p *Project) SheetFromNested(name string) bool {
	for _, a := range p.LoadedAnims {
		if a.Sheets[name] != nil {
			return true
		}
	}
	return false
}
