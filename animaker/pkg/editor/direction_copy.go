package editor

// TimingSources lists the directions, other than target, that have any
// keyframes to copy timing from, in ascending key order — so direction 0
// comes first whenever it has keyframes.
func (t *Track) TimingSources(target int) []int {
	var keys []int
	for _, k := range t.SortedDirectionKeys() {
		if k != target && t.Directions[k].TotalKeyframes() > 0 {
			keys = append(keys, k)
		}
	}
	return keys
}

// TotalKeyframes counts every keyframe of every part in this direction.
func (d *Direction) TotalKeyframes() int {
	n := 0
	for _, kfs := range d.Keyframes {
		n += len(kfs)
	}
	return n
}

// CopyKeyframeTimes gives each part in dst a keyframe at every time it has
// one in src, and nothing else: each new keyframe has a zero pose (origin,
// no rotation, cell 0,0). It carries over only the animation's *timing*,
// as a scaffold for posing a new facing — positions and cells are the
// artist's to author per direction (art commonly differs by facing), so
// none are copied. Times dst already has a keyframe at are left alone.
// Only parts still in the rig are copied. Returns how many keyframes were
// added.
func CopyKeyframeTimes(t *Track, srcKey, dstKey int) int {
	src, dst := t.Directions[srcKey], t.Directions[dstKey]
	if src == nil || dst == nil || srcKey == dstKey {
		return 0
	}
	added := 0
	for _, part := range t.Parts {
		for _, kf := range src.KeyframesFor(part.ID) {
			before := len(dst.KeyframesFor(part.ID))
			AddKeyframe(dst, part.ID, kf.TimeMs)
			if len(dst.KeyframesFor(part.ID)) > before {
				added++
			}
		}
	}
	return added
}

// CopyKeyframes copies every keyframe of every part from src into dst
// whole — time, position, rotation, cell and nested direction — for when
// the artist opts to start a facing as a duplicate of another rather than
// from timing alone. As with CopyKeyframeTimes, times dst already has a
// keyframe at are left alone and only parts still in the rig are copied.
// Returns how many keyframes were added.
func CopyKeyframes(t *Track, srcKey, dstKey int) int {
	src, dst := t.Directions[srcKey], t.Directions[dstKey]
	if src == nil || dst == nil || srcKey == dstKey {
		return 0
	}
	added := 0
	for _, part := range t.Parts {
		for _, kf := range src.KeyframesFor(part.ID) {
			before := len(dst.KeyframesFor(part.ID))
			nk := AddKeyframe(dst, part.ID, kf.TimeMs)
			if len(dst.KeyframesFor(part.ID)) > before {
				idx := nk.Index
				*nk = *kf
				nk.Index = idx
				added++
			}
		}
	}
	return added
}
