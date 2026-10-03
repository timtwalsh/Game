package editor

import "sort"

// The operations behind the editor's keyboard shortcuts, kept here rather
// than in the app so they can be tested without a window.

// TogglePlay is Space: pause if playing - the playhead stays where it is,
// unlike Stop, which rewinds to 0 - else play from the playhead.
func (p *Project) TogglePlay() {
	if p.Playback.IsPlaying {
		p.Playback.IsPlaying = false
		return
	}
	p.Play()
}

// StepToKeyframe moves the playhead to the next (forward) or previous
// keyframe time of any part in the active direction, stopping playback.
// It reports false, changing nothing, when there's no keyframe that way.
// Like any scrub it drops the keyframe selection, so the Selected Keyframe
// fields then edit what's keyed at the new time.
func (p *Project) StepToKeyframe(forward bool) bool {
	dir := p.ActiveDirection()
	if dir == nil {
		return false
	}
	var times []uint32
	for _, kfs := range dir.Keyframes {
		for _, kf := range kfs {
			times = append(times, kf.TimeMs)
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	now := p.Playback.ElapsedMs
	target, found := uint32(0), false
	if forward {
		for _, t := range times {
			if t > now {
				target, found = t, true
				break
			}
		}
	} else {
		for i := len(times) - 1; i >= 0; i-- {
			if times[i] < now {
				target, found = times[i], true
				break
			}
		}
	}
	if !found {
		return false
	}
	p.Playback.IsPlaying = false
	p.Scrub(target)
	return true
}

// EditTarget is the keyframe an edit of the selected part applies to: the
// selected keyframe, else the part's keyframe at the playhead, created
// from the interpolated pose if there isn't one - the same rule the
// Selected Keyframe fields and a canvas drag follow. nil with no part
// selected. Record an undo step before calling it, since it may add a
// keyframe.
func (p *Project) EditTarget() *Keyframe {
	if kf := p.SelectedKeyframe(); kf != nil {
		return kf
	}
	part, dir := p.SelectedPart(), p.ActiveDirection()
	if part == nil || dir == nil {
		return nil
	}
	kf, _ := EnsureKeyframe(dir, part.ID, p.Playback.ElapsedMs)
	p.Selection.KeyframeIndex = kf.Index
	return kf
}

// KeyframeToDelete is what Delete removes: the selected keyframe, else the
// selected part's keyframe at the playhead. ok is false when there's
// neither - Delete never removes a keyframe the artist can't see.
func (p *Project) KeyframeToDelete() (partID, index int, ok bool) {
	part, dir := p.SelectedPart(), p.ActiveDirection()
	if part == nil || dir == nil {
		return 0, 0, false
	}
	if kf := p.SelectedKeyframe(); kf != nil {
		return part.ID, kf.Index, true
	}
	for i, kf := range dir.KeyframesFor(part.ID) {
		if kf.TimeMs == p.Playback.ElapsedMs {
			return part.ID, i, true
		}
	}
	return 0, 0, false
}

// ZOrderTarget is where "bring to front" (front) or "send to back" puts
// the selected part: one past the highest (or below the lowest) Z of every
// other part posed at the playhead, as interpolated there. ok is false
// with no part selected or no other posed part to be in front of.
func (p *Project) ZOrderTarget(front bool) (z float32, ok bool) {
	part, dir := p.SelectedPart(), p.ActiveDirection()
	if part == nil || dir == nil {
		return 0, false
	}
	at := p.Playback.ElapsedMs
	for _, other := range p.CurrentTrack.Parts {
		if other.ID == part.ID || len(dir.KeyframesFor(other.ID)) == 0 {
			continue
		}
		oz := dir.ValueAt(other.ID, at).Z
		switch {
		case !ok:
			z, ok = oz, true
		case front && oz > z, !front && oz < z:
			z = oz
		}
	}
	if !ok {
		return 0, false
	}
	if front {
		return z + 1, true
	}
	return z - 1, true
}
