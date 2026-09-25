package editor

// DeepCopy returns a fully independent copy of the track, for undo
// snapshots and any other case where mutating a copy must not affect the
// original.
func (t *Track) DeepCopy() *Track {
	dst := &Track{
		Metadata:     t.Metadata,
		Props:        append([]PropDef(nil), t.Props...),
		RefBoxWidth:  t.RefBoxWidth,
		RefBoxHeight: t.RefBoxHeight,
		Parts:        make([]*Part, len(t.Parts)),
		Directions:   make(map[int]*Direction, len(t.Directions)),
	}
	for i, p := range t.Parts {
		dst.Parts[i] = p.deepCopy()
	}
	for key, dir := range t.Directions {
		dst.Directions[key] = dir.deepCopy()
	}
	return dst
}

func (d *Direction) deepCopy() *Direction {
	dst := &Direction{Keyframes: make(map[int][]*Keyframe, len(d.Keyframes))}
	for partID, kfs := range d.Keyframes {
		copied := make([]*Keyframe, len(kfs))
		for i, kf := range kfs {
			copied[i] = kf.Clone()
		}
		dst.Keyframes[partID] = copied
	}
	return dst
}

func (p *Part) deepCopy() *Part {
	dst := &Part{
		ID:            p.ID,
		Name:          p.Name,
		Kind:          p.Kind,
		GoverningProp: p.GoverningProp,
		FixedSheet:    p.FixedSheet,
		NestedAniPath: p.NestedAniPath,
	}
	if p.NestedBindings != nil {
		dst.NestedBindings = make(map[string]PropBinding, len(p.NestedBindings))
		for k, v := range p.NestedBindings {
			dst.NestedBindings[k] = v
		}
	}
	return dst
}
