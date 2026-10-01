package editor

import "testing"

// torchAnim is a nested animation: one flame part flickering between two
// cells over 600ms, in directions 0 and 2 (direction 2 uses row 1).
func torchAnim(path string) *NestedAnim {
	t := NewTrack("torch")
	flame := AddPart(t, NewSheetPart("flame", "", "fire"))
	for _, d := range []int{0, 2} {
		a := AddKeyframe(t.Directions[d], flame.ID, 0)
		a.X, a.Y, a.Col, a.Row = 1, -4, 0, d/2
		b := AddKeyframe(t.Directions[d], flame.ID, 300)
		b.X, b.Y, b.Col, b.Row = 1, -4, 1, d/2
		c := AddKeyframe(t.Directions[d], flame.ID, 600)
		c.X, c.Y, c.Col, c.Row = 1, -4, 0, d/2
	}
	delete(t.Directions, 1)
	delete(t.Directions, 3)
	return &NestedAnim{
		Path:   AnimKey(path),
		Track:  t,
		Sheets: map[string]*SpriteSheetTemplate{"fire": newTestSheet("fire", 32, 32, 16, 16)},
	}
}

func walkWithTorch(t *testing.T) (*Project, *Part) {
	t.Helper()
	p := NewProject("walk_torch")
	torch := torchAnim("torch.anif")
	p.LoadedAnims[torch.Path] = torch
	part := AddPart(p.CurrentTrack, NewNestedAniPart("torch_1", torch.Path))
	kf := AddKeyframe(p.ActiveDirection(), part.ID, 0)
	kf.X, kf.Y = 10, 20
	return p, part
}

func TestFlattenNestedPlacesChildSpritesAtThePart(t *testing.T) {
	p, part := walkWithTorch(t)

	sprites := p.FlattenNested(part)
	if len(sprites) != 1 {
		t.Fatalf("%d sprites, want 1", len(sprites))
	}
	s := sprites[0]
	if s.X != 1 || s.Y != -4 || s.Col != 0 {
		t.Errorf("sprite = %+v, want the flame's own pose (1,-4) col 0, relative to the part", s)
	}
}

// The nested animation loops at its own 600ms, on its own clock, not the
// parent's playhead.
func TestNestedAnimationPlaysOnItsOwnClock(t *testing.T) {
	p, part := walkWithTorch(t)

	p.Playback.NestedClockMs = 300
	if got := p.FlattenNested(part)[0].Col; got != 1 {
		t.Errorf("at 300ms col = %d, want 1", got)
	}
	p.Playback.NestedClockMs = 600 + 300 // second loop
	if got := p.FlattenNested(part)[0].Col; got != 1 {
		t.Errorf("at 900ms (looped) col = %d, want 1", got)
	}

	// The parent here has a single keyframe, so its own duration is 0 - the
	// torch must still play.
	p.Playback.IsPlaying, p.Playback.NestedClockMs = true, 0
	p.AdvancePlayback(300)
	if p.Playback.NestedClockMs != 300 {
		t.Errorf("nested clock = %d after 300ms of play, want 300", p.Playback.NestedClockMs)
	}
}

func TestNestedDirectionFollowsParentOrBinding(t *testing.T) {
	p, part := walkWithTorch(t)

	p.SetActiveDirection(2)
	AddKeyframe(p.ActiveDirection(), part.ID, 0)
	if got := p.FlattenNested(part)[0].Row; got != 1 {
		t.Errorf("parent facing 2: torch row = %d, want 1 (turns with the parent)", got)
	}

	part.NestedBindings = map[string]PropBinding{"direction": {StaticValue: "0"}}
	if got := p.FlattenNested(part)[0].Row; got != 0 {
		t.Errorf("direction pinned to 0: row = %d, want 0", got)
	}

	// A facing the torch doesn't have falls back to its first.
	part.NestedBindings = nil
	p.CurrentTrack.Directions[1] = NewDirection()
	p.SetActiveDirection(1)
	AddKeyframe(p.ActiveDirection(), part.ID, 0)
	if got := p.FlattenNested(part); len(got) != 1 || got[0].Row != 0 {
		t.Errorf("parent facing 1 (torch has no 1): %+v, want direction 0's sprite", got)
	}
}

// Requested: a nested .anif as a prop - the part plays whichever animation
// the prop is set to.
func TestAnimPropSwapsTheNestedAnimation(t *testing.T) {
	p, part := walkWithTorch(t)
	lantern := torchAnim("lantern.anif")
	lantern.Track.Directions[0].Keyframes[1][0].X = 99
	p.LoadedAnims[lantern.Path] = lantern

	EnsureProp(p.CurrentTrack, "light", AnimKey("torch.anif"))
	part.GoverningProp = "light"
	if got := p.FlattenNested(part)[0].X; got != 1 {
		t.Errorf("light=torch: x = %v, want 1", got)
	}

	p.PreviewProps["light"] = AnimKey("lantern.anif")
	if got := p.FlattenNested(part)[0].X; got != 99 {
		t.Errorf("light previewed as lantern: x = %v, want 99", got)
	}

	if names := p.CurrentTrack.ReferencedSheetNames(); len(names) != 0 {
		t.Errorf("an .anif prop default was treated as a sheet: %v", names)
	}
}

// An animation that nests itself must not hang or overflow the stack.
func TestSelfNestingIsBounded(t *testing.T) {
	p := NewProject("loop")
	self := torchAnim("loop.anif")
	inner := AddPart(self.Track, NewNestedAniPart("again", self.Path))
	AddKeyframe(self.Track.Directions[0], inner.ID, 0)
	p.LoadedAnims[self.Path] = self

	part := AddPart(p.CurrentTrack, NewNestedAniPart("loop_1", self.Path))
	AddKeyframe(p.ActiveDirection(), part.ID, 0)

	if got := len(p.FlattenNested(part)); got != maxNestDepth {
		t.Errorf("%d sprites, want one flame per level up to depth %d", got, maxNestDepth)
	}
}

func TestNestedExtentCoversEveryPose(t *testing.T) {
	p, part := walkWithTorch(t)
	minX, minY, maxX, maxY, ok := p.NestedExtent(part)
	if !ok {
		t.Fatal("no extent")
	}
	// The flame's pivot (newTestSheet's 3,4) sits at (1,-4); a 16x16 cell.
	if minX != -2 || minY != -8 || maxX != 14 || maxY != 8 {
		t.Errorf("extent = (%v,%v)-(%v,%v), want (-2,-8)-(14,8)", minX, minY, maxX, maxY)
	}
}

func TestStillParentWithNestedAnimationCanPlay(t *testing.T) {
	p, _ := walkWithTorch(t)
	p.Play()
	if !p.Playback.IsPlaying {
		t.Error("Play refused: the parent holds still but its torch should flicker")
	}
}
