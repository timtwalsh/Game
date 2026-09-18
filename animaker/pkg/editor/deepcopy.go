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
		Directions:   make(map[int]*Direction, len(t.Directions)),
	}
	for key, dir := range t.Directions {
		dst.Directions[key] = dir.deepCopy()
	}
	return dst
}

func (d *Direction) deepCopy() *Direction {
	dst := &Direction{Parts: make([]*Part, len(d.Parts))}
	for i, p := range d.Parts {
		dst.Parts[i] = p.deepCopy()
	}
	return dst
}

func (p *Part) deepCopy() *Part {
	dst := &Part{
		Name:          p.Name,
		Kind:          p.Kind,
		GoverningProp: p.GoverningProp,
		FixedSheet:    p.FixedSheet,
		NestedAniPath: p.NestedAniPath,
		Keyframes:     make([]*Keyframe, len(p.Keyframes)),
	}
	if p.NestedBindings != nil {
		dst.NestedBindings = make(map[string]PropBinding, len(p.NestedBindings))
		for k, v := range p.NestedBindings {
			dst.NestedBindings[k] = v
		}
	}
	for i, kf := range p.Keyframes {
		dst.Keyframes[i] = kf.Clone()
	}
	return dst
}
