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

func TestNestedDirectionInheritsOrIsStatic(t *testing.T) {
	p, part := walkWithTorch(t)

	p.SetActiveDirection(2)
	AddKeyframe(p.ActiveDirection(), part.ID, 0)
	if got := p.FlattenNested(part)[0].Row; got != 1 {
		t.Errorf("parent facing 2: torch row = %d, want 1 (turns with the parent)", got)
	}

	part.DirectionMode, part.StaticDirection = NestedDirStatic, 0
	if got := p.FlattenNested(part)[0].Row; got != 0 {
		t.Errorf("static direction 0: row = %d, want 0", got)
	}

	// A facing the torch doesn't have falls back to its first.
	part.DirectionMode = NestedDirInherit
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

// Requested: the nested direction can also be set per keyframe, stepping
// like a cell - e.g. a held torch turning partway through the walk.
func TestNestedDirectionPerKeyframeSteps(t *testing.T) {
	p, part := walkWithTorch(t)
	part.DirectionMode = NestedDirPerKeyframe
	dir := p.ActiveDirection()
	dir.KeyframesFor(part.ID)[0].Direction = 0
	second := AddKeyframe(dir, part.ID, 200)
	second.X, second.Y, second.Direction = 10, 20, 2

	p.Seek(100)
	if got := p.FlattenNested(part)[0].Row; got != 0 {
		t.Errorf("at 100ms row = %d, want direction 0's row 0", got)
	}
	p.Seek(200)
	if got := p.FlattenNested(part)[0].Row; got != 1 {
		t.Errorf("at 200ms row = %d, want direction 2's row 1", got)
	}

	// The canvas box covers both directions the part shows.
	if _, _, _, _, ok := p.NestedExtent(part); !ok {
		t.Error("no extent for a per-keyframe nested part")
	}
}

// Switching mode keeps showing the same direction, so the artist sees no
// jump: an inherited part facing 2 becomes per-keyframe with its direction-2
// keyframes set to 2, or static at 2.
func TestSwitchingDirectionModeDoesNotJump(t *testing.T) {
	p, part := walkWithTorch(t)
	p.SetActiveDirection(2)
	AddKeyframe(p.ActiveDirection(), part.ID, 0)

	before := p.FlattenNested(part)[0].Row
	p.SetNestedDirectionMode(part, NestedDirPerKeyframe)
	if kf := p.ActiveDirection().KeyframesFor(part.ID)[0]; kf.Direction != 2 {
		t.Errorf("seeded keyframe direction = %d, want 2", kf.Direction)
	}
	if got := p.FlattenNested(part)[0].Row; got != before {
		t.Errorf("switching to per-keyframe changed the row %d -> %d", before, got)
	}

	p.SetNestedDirectionMode(part, NestedDirStatic)
	if part.StaticDirection != 2 {
		t.Errorf("static direction seeded as %d, want 2", part.StaticDirection)
	}
	if got := p.FlattenNested(part)[0].Row; got != before {
		t.Errorf("switching to static changed the row %d -> %d", before, got)
	}
}

// A nested sprite stays in its part's slot in the draw order however
// large the nested track's own Z values are. The old encoding (parent Z
// plus child Z / 1000) let a child Z of 5000 jump a sibling part.
func TestNestedDrawOrderIgnoresChildZScale(t *testing.T) {
	torch := torchAnim("torch.anif")
	for _, kfs := range torch.Track.Directions[0].Keyframes {
		for _, kf := range kfs {
			kf.Z = 5000
		}
	}
	holder := NewTrack("holder")
	tp := AddPart(holder, NewNestedAniPart("torch", torch.Path))
	AddKeyframe(holder.Directions[0], tp.ID, 0).Z = 1 // behind the hand
	hand := AddPart(holder, NewSheetPart("hand", "", "skin"))
	AddKeyframe(holder.Directions[0], hand.ID, 0).Z = 2
	holderAnim := &NestedAnim{Path: AnimKey("holder.anif"), Track: holder, Sheets: map[string]*SpriteSheetTemplate{
		"skin": newTestSheet("skin", 32, 32, 16, 16), "fire": newTestSheet("fire", 32, 32, 16, 16),
	}}

	p := NewProject("walk")
	p.LoadedAnims[torch.Path] = torch
	p.LoadedAnims[holderAnim.Path] = holderAnim
	part := AddPart(p.CurrentTrack, NewNestedAniPart("held", holderAnim.Path))
	AddKeyframe(p.ActiveDirection(), part.ID, 0)

	sprites := p.FlattenNested(part)
	if len(sprites) != 2 {
		t.Fatalf("%d sprites, want flame and hand", len(sprites))
	}
	if sprites[0].Sheet.Name != "fire" || sprites[1].Sheet.Name != "skin" {
		t.Errorf("draw order %s, %s; want the flame (torch at Z 1) behind the hand (Z 2)",
			sprites[0].Sheet.Name, sprites[1].Sheet.Name)
	}
}
