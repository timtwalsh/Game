package editor

import "testing"

func TestFacingNamesFollowTheCompass(t *testing.T) {
	cases := []struct {
		k, n int
		want string
	}{
		{0, 1, "All"},
		{0, 4, "N"}, {1, 4, "E"}, {2, 4, "S"}, {3, 4, "W"},
		{1, 8, "NE"}, {2, 8, "E"}, {7, 8, "NW"},
		{1, 16, "NNE"}, {3, 16, "ENE"}, {5, 16, "ESE"}, {11, 16, "WSW"}, {13, 16, "WNW"}, {15, 16, "NNW"},
		{4, 4, "Dir 4"}, // outside the count
	}
	for _, c := range cases {
		if got := FacingName(c.k, c.n); got != c.want {
			t.Errorf("FacingName(%d, %d) = %q, want %q", c.k, c.n, got, c.want)
		}
	}
}

// The game's facing is 8-way (client/prediction.go: 0=N, 1=NE, 2=E ...),
// so an 8-direction track's keys are the game's own and need no mapping.
func TestEightDirectionKeysAreTheGames(t *testing.T) {
	for k, want := range []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"} {
		if got := FacingName(k, 8); got != want {
			t.Errorf("8-direction key %d = %s, want %s", k, got, want)
		}
	}
}

func TestMapDirectionByFacing(t *testing.T) {
	cases := []struct {
		k, from, to, want int
		why               string
	}{
		{2, 8, 4, 1, "E -> E"},
		{1, 4, 8, 2, "E -> E"},
		{1, 8, 4, 1, "NE: a tie between N and E goes sideways, to E"},
		{7, 8, 4, 3, "NW -> W"},
		{3, 8, 4, 1, "SE -> E"},
		{5, 8, 4, 3, "SW -> W"},
		{1, 16, 8, 1, "NNE: a tie between N and NE goes to the more sideways NE"},
		{3, 8, 1, 0, "anything -> a 1-direction track's only key"},
		{6, 16, 16, 6, "same count: unchanged"},
	}
	for _, c := range cases {
		if got := MapDirection(c.k, c.from, c.to); got != c.want {
			t.Errorf("MapDirection(%d, %d->%d) = %d, want %d (%s)", c.k, c.from, c.to, got, c.want, c.why)
		}
	}
}

func TestFacingsInfersOlderFiles(t *testing.T) {
	tr := &Track{Directions: map[int]*Direction{0: NewDirection(), 2: NewDirection()}}
	if got := tr.Facings(); got != 4 {
		t.Errorf("keys 0,2 with no count: %d, want 4 (the old convention)", got)
	}
	tr.Directions[6] = NewDirection()
	if got := tr.Facings(); got != 8 {
		t.Errorf("key 6: %d, want 8", got)
	}
	if got := NewTrack("t").Facings(); got != 1 {
		t.Errorf("new track: %d directions, want 1", got)
	}
}

func TestSetDirectionCountKeepsFacings(t *testing.T) {
	p := NewProject("t")
	AddStandardDirections(p.CurrentTrack) // N E S W
	part := AddPart(p.CurrentTrack, NewSheetPart("body", "", "s"))
	AddKeyframe(p.CurrentTrack.Directions[1], part.ID, 0).X = 11 // E
	p.SetActiveDirection(1)

	// 4 -> 8: E moves from key 1 to key 2; NE etc. are added empty.
	if lost, err := p.SetDirectionCount(8, false); err != nil || lost != nil {
		t.Fatalf("4->8: lost %v, err %v", lost, err)
	}
	tr := p.CurrentTrack
	if len(tr.Directions) != 8 || tr.Directions[2].KeyframesFor(part.ID)[0].X != 11 {
		t.Fatalf("after 4->8: %d directions, E keyframe moved? want 8 and x=11 at key 2", len(tr.Directions))
	}
	if p.Playback.ActiveDirection != 2 {
		t.Errorf("active %d, want 2 (still facing E)", p.Playback.ActiveDirection)
	}

	// Pose NE, then 8 -> 4: NE has no place, so it's reported, not dropped.
	AddKeyframe(tr.Directions[1], part.ID, 0)
	lost, err := p.SetDirectionCount(4, false)
	if err != nil || len(lost) != 1 || lost[0] != 1 || len(tr.Directions) != 8 {
		t.Fatalf("8->4 with NE posed: lost %v, err %v, %d directions; want [1] and nothing changed", lost, err, len(tr.Directions))
	}
	if _, err := p.SetDirectionCount(4, true); err != nil {
		t.Fatal(err)
	}
	if len(p.CurrentTrack.Directions) != 4 || p.CurrentTrack.Directions[1].KeyframesFor(part.ID)[0].X != 11 {
		t.Error("8->4: want 4 directions with E back at key 1")
	}
	if _, err := p.SetDirectionCount(12, false); err == nil {
		t.Error("12 directions accepted")
	}
}

// An 8-direction character holding a 4-direction torch: the torch turns by
// facing, not by key number.
func TestNestedInheritMapsByFacing(t *testing.T) {
	p, part := walkWithTorch(t) // torch: directions 0 and 2 posed, 4-direction
	torch := p.LoadedAnims[AnimKey("torch.anif")].Track
	torch.DirectionCount = 4
	p.CurrentTrack.DirectionCount = 8
	p.CurrentTrack.Directions[4] = NewDirection() // the character facing S
	p.SetActiveDirection(4)
	AddKeyframe(p.ActiveDirection(), part.ID, 0)
	if got := p.FlattenNested(part); len(got) != 1 || got[0].Row != 1 {
		t.Errorf("character facing S (8-way key 4): torch %+v, want its S direction (row 1)", got)
	}
}
