package editor

import "testing"

func trackName(p *Project) string { return p.CurrentTrack.Metadata.Name }

// edit records an undo step and then renames the track, the way every
// editor action records before it changes anything.
func edit(p *Project, name string) {
	p.RecordUndo()
	p.CurrentTrack.Metadata.Name = name
}

// Each undo steps back exactly one change, and redo replays them in order,
// including the most recent one.
func TestUndoRedoStepOneChangeAtATime(t *testing.T) {
	p := NewProject("start")
	edit(p, "A")
	edit(p, "B")
	edit(p, "C")

	for _, want := range []string{"B", "A", "start"} {
		if !p.Undo() {
			t.Fatalf("Undo failed, want %q", want)
		}
		if got := trackName(p); got != want {
			t.Fatalf("after undo: %q, want %q", got, want)
		}
	}
	if p.Undo() {
		t.Error("Undo past the first change succeeded")
	}
	for _, want := range []string{"A", "B", "C"} {
		if !p.Redo() {
			t.Fatalf("Redo failed, want %q", want)
		}
		if got := trackName(p); got != want {
			t.Fatalf("after redo: %q, want %q", got, want)
		}
	}
	if p.Redo() {
		t.Error("Redo past the last change succeeded")
	}
}

// Editing after an undo must not reach into the history: the state a redo
// or a further undo restores is the one recorded, not whatever the live
// track has since become.
func TestEditingAfterUndoDoesNotCorruptHistory(t *testing.T) {
	p := NewProject("start")
	edit(p, "A")
	edit(p, "B")
	p.Undo()                                   // live = A
	p.CurrentTrack.Metadata.Name = "scribbled" // a change with no undo step
	p.Redo()
	if got := trackName(p); got != "B" {
		t.Fatalf("redo restored %q, want B", got)
	}
	p.CurrentTrack.Metadata.Name = "scribbled again"
	p.Undo()
	p.Undo()
	if got := trackName(p); got != "start" {
		t.Errorf("undo to the beginning gave %q, want start", got)
	}
}

// A new change after an undo discards the redo history, as usual.
func TestNewChangeClearsRedo(t *testing.T) {
	p := NewProject("start")
	edit(p, "A")
	p.Undo()
	edit(p, "B")
	if p.Redo() {
		t.Error("redo available after a new change")
	}
	p.Undo()
	if got := trackName(p); got != "start" {
		t.Errorf("undo gave %q, want start", got)
	}
}

// Opening a track leaves nothing of the previous one behind.
func TestOpenTrackResetsTheSession(t *testing.T) {
	p := NewProject("old")
	AddProp(p.CurrentTrack, "hair", "hair_a")
	p.PreviewProps["hair"] = "hair_b"
	p.PreviewSheets["hair_b"] = &SpriteSheetTemplate{Name: "hair_b"}
	p.LoadedSheets["body"] = &SpriteSheetTemplate{Name: "body"}
	edit(p, "old2")
	p.Selection.PartIndex = 3
	p.Playback.IsPlaying, p.Playback.ElapsedMs, p.Playback.NestedClockMs = true, 500, 900

	track := NewTrack("new")
	AddStandardDirections(track)
	RemoveDirection(track, 0) // first direction is now 1
	anims := map[string]*NestedAnim{}
	p.OpenTrack(track, "new.anif", anims)

	switch {
	case p.CurrentTrack != track, p.SavePath != "new.anif", p.Dirty:
		t.Error("track/path/dirty not set")
	case len(p.PreviewProps) != 0, len(p.PreviewSheets) != 0:
		t.Error("previous track's preview overrides carried over")
	case p.UndoStack.CanUndo():
		t.Error("previous track's undo history carried over")
	case p.Selection.PartIndex != -1:
		t.Error("selection carried over")
	case p.Playback.IsPlaying, p.Playback.ElapsedMs != 0, p.Playback.NestedClockMs != 0:
		t.Error("playback not reset")
	case p.Playback.ActiveDirection != 1:
		t.Errorf("active direction %d, want the track's first (1)", p.Playback.ActiveDirection)
	case p.LoadedSheets["body"] == nil:
		t.Error("imported sheets should stay loaded")
	}
}
