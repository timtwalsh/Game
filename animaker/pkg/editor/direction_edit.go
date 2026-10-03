package editor

import (
	"errors"
	"fmt"
	"sort"
)

// Deleting and re-keying directions. New tracks have one direction, but
// files saved with the old four-direction default carry empty facings,
// and a facing posed under the wrong key had to be re-posed by hand.
//
// None of these touch nested parts: a nested part's static or
// per-keyframe direction names a direction of the *nested* animation, not
// of this track, and an inherited one follows whatever facing is active.

// ErrLastDirection refuses removing a track's only direction.
var ErrLastDirection = errors.New("a track needs at least one direction")

// DeleteDirection removes a direction and its keyframes. The active
// direction moves to the nearest remaining one if it was the one deleted.
// Record an undo step first.
func (p *Project) DeleteDirection(key int) error {
	t := p.CurrentTrack
	if _, ok := t.Directions[key]; !ok {
		return fmt.Errorf("there's no direction %d", key)
	}
	if len(t.Directions) == 1 {
		return ErrLastDirection
	}
	RemoveDirection(t, key)
	if p.Playback.ActiveDirection == key {
		p.SetActiveDirection(nearestDirection(t, key))
	}
	p.Dirty = true
	return nil
}

// ChangeDirectionKey moves direction from to key to, keyframes and all,
// unchanged - for a facing posed under the wrong key. A target that
// already exists is refused if it has keyframes; an empty one is simply
// replaced, since it holds nothing to lose. The active direction follows
// the move. Record an undo step first.
func (p *Project) ChangeDirectionKey(from, to int) error {
	t := p.CurrentTrack
	d, ok := t.Directions[from]
	switch {
	case !ok:
		return fmt.Errorf("there's no direction %d", from)
	case from == to:
		return nil
	case to < 0:
		return errors.New("a direction is a whole number, 0 or above")
	}
	if existing, ok := t.Directions[to]; ok && existing.TotalKeyframes() > 0 {
		return fmt.Errorf("direction %d already has %d keyframes - delete it first, or change it to another direction",
			to, existing.TotalKeyframes())
	}
	delete(t.Directions, from)
	t.Directions[to] = d
	if p.Playback.ActiveDirection == from {
		p.Playback.ActiveDirection = to
	}
	p.Dirty = true
	return nil
}

// RemoveEmptyDirections deletes every direction with no keyframes, and
// returns their keys. It always leaves one: if every direction is empty,
// the active one stays. Record an undo step first if anything will go.
func (p *Project) RemoveEmptyDirections() []int {
	t := p.CurrentTrack
	var empty []int
	for _, k := range t.SortedDirectionKeys() {
		if t.Directions[k].TotalKeyframes() == 0 {
			empty = append(empty, k)
		}
	}
	if len(empty) == len(t.Directions) { // keep one: the facing on screen
		keep := p.Playback.ActiveDirection
		if _, ok := t.Directions[keep]; !ok {
			keep = empty[0]
		}
		empty = removeInt(empty, keep)
	}
	if len(empty) == 0 {
		return nil
	}
	active := p.Playback.ActiveDirection
	for _, k := range empty {
		RemoveDirection(t, k)
	}
	if _, ok := t.Directions[active]; !ok {
		p.SetActiveDirection(nearestDirection(t, active))
	}
	p.Dirty = true
	return empty
}

// EmptyDirections lists the track's directions with no keyframes, for
// the menu to say whether Remove Empty Directions would do anything.
func (t *Track) EmptyDirections() []int {
	var out []int
	for _, k := range t.SortedDirectionKeys() {
		if t.Directions[k].TotalKeyframes() == 0 {
			out = append(out, k)
		}
	}
	return out
}

// nearestDirection is the remaining direction key closest to key, the
// lower one on a tie.
func nearestDirection(t *Track, key int) int {
	keys := t.SortedDirectionKeys()
	sort.SliceStable(keys, func(i, j int) bool { return absInt(keys[i]-key) < absInt(keys[j]-key) })
	if len(keys) == 0 {
		return 0
	}
	return keys[0]
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func removeInt(s []int, v int) []int {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
