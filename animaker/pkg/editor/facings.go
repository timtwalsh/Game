package editor

import (
	"fmt"
	"math"
	"sort"
)

// How a track's direction keys map to compass facings.
//
// A track declares how many directions it has - 1, 4, 8 or 16 - and its
// keys are 0..N-1 clockwise from north, so key k faces k*360/N degrees.
// 8 is exactly the game's own 8-way facing (client/prediction.go: 0=N,
// 1=NE, 2=E ... 7=NW), and 4 is exactly what tracks always used (0=up,
// 1=right, 2=down, 3=left), so neither needs translating. Every key is
// named from the 16-point compass.

// DirectionCounts are the direction counts a track can have.
var DirectionCounts = []int{1, 4, 8, 16}

// CompassNames are the 16 compass points, clockwise from north.
var CompassNames = [16]string{
	"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE",
	"S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW",
}

// ValidDirectionCount reports whether n is one of DirectionCounts.
func ValidDirectionCount(n int) bool {
	for _, c := range DirectionCounts {
		if c == n {
			return true
		}
	}
	return false
}

// Facings is how many directions the track has. A track saved before the
// count was recorded is judged by its keys: 4 if they're all 0-3 (what
// tracks always used), 8 if all 0-7, else 16.
func (t *Track) Facings() int {
	if ValidDirectionCount(t.DirectionCount) {
		return t.DirectionCount
	}
	max := 0
	for k := range t.Directions {
		if k > max {
			max = k
		}
	}
	switch {
	case max < 4:
		return 4
	case max < 8:
		return 8
	}
	return 16
}

// FacingName names key k of an n-direction track by compass point: N, E,
// S, W for 4; N, NE, E ... for 8; all 16 points for 16. A 1-direction
// track's one direction is "All". A key outside 0..n-1 (only possible in
// a hand-edited file) is "Dir k".
func FacingName(k, n int) string {
	switch {
	case n == 1 && k == 0:
		return "All"
	case n <= 0 || n > 16 || 16%n != 0 || k < 0 || k >= n:
		return fmt.Sprintf("Dir %d", k)
	}
	return CompassNames[k*16/n]
}

// facingAngle is the compass bearing of key k of an n-direction track.
func facingAngle(k, n int) float64 {
	if n <= 0 {
		return 0
	}
	return float64(k) * 360 / float64(n)
}

// MapDirection is the key of a toN-direction track that faces closest to
// key k of a fromN-direction track - how a 4-direction torch held by an
// 8-direction character picks its facing. A bearing exactly between two
// keys (NE on a 4-direction track) goes to the sideways one (E or W), the
// way top-down games usually show diagonals; the engine should do the
// same.
func MapDirection(k, fromN, toN int) int {
	if toN <= 1 {
		return 0
	}
	if fromN == toN {
		return k
	}
	want := facingAngle(k, fromN)
	best, bestDist, bestSide := 0, math.Inf(1), math.Inf(1)
	for j := 0; j < toN; j++ {
		a := facingAngle(j, toN)
		d := math.Abs(math.Mod(a-want+540, 360) - 180)
		// How far this key is from straight sideways (90 or 270): the
		// tie-break, smaller is more sideways.
		side := math.Abs(math.Mod(a+90, 180) - 90)
		side = 90 - side
		if d < bestDist-1e-9 || (math.Abs(d-bestDist) < 1e-9 && side < bestSide) {
			best, bestDist, bestSide = j, d, side
		}
	}
	return best
}

// SetDirectionCount changes how many directions the track has. Growing
// moves every direction to the key facing the same way (4 to 8: E goes
// from key 1 to key 2) and adds the new ones empty. Shrinking keeps the
// directions the new count can face and removes the rest; if any of those
// have keyframes it changes nothing and returns their keys unless
// dropPosed, so the caller can ask first. The active direction follows
// its facing. Record an undo step first.
func (p *Project) SetDirectionCount(n int, dropPosed bool) (lost []int, err error) {
	if !ValidDirectionCount(n) {
		return nil, fmt.Errorf("a track can have 1, 4, 8 or 16 directions, not %d", n)
	}
	t := p.CurrentTrack
	from := t.Facings()
	if n == from && t.DirectionCount == n {
		return nil, nil
	}
	moved := map[int]*Direction{}
	for _, k := range t.SortedDirectionKeys() {
		d := t.Directions[k]
		if k >= from || (k*n)%from != 0 { // no key faces this way in n
			if d.TotalKeyframes() > 0 {
				lost = append(lost, k)
			}
			continue
		}
		moved[k*n/from] = d
	}
	if len(lost) > 0 && !dropPosed {
		return lost, nil
	}
	for k := 0; k < n; k++ {
		if moved[k] == nil {
			moved[k] = NewDirection()
		}
	}
	active := p.Playback.ActiveDirection
	t.Directions = moved
	t.DirectionCount = n
	p.Playback.ActiveDirection = MapDirection(active, from, n)
	p.clampSelection()
	p.Dirty = true
	sort.Ints(lost)
	return lost, nil
}

// MissingFacings lists the keys 0..N-1 the track doesn't have (after
// deleting or Remove Empty Directions), for the menu to offer back.
func (t *Track) MissingFacings() []int {
	var out []int
	for k := 0; k < t.Facings(); k++ {
		if _, ok := t.Directions[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}
