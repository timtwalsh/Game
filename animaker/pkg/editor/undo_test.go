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
