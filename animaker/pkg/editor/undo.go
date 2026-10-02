package editor

import "time"

// UndoStack manages undo/redo history using full track snapshots.
type UndoStack struct {
	past    []*ProjectSnapshot
	future  []*ProjectSnapshot
	maxSize int
}

// ProjectSnapshot captures the track state at a point in time.
type ProjectSnapshot struct {
	Track     *Track
	Timestamp time.Time
}

// NewUndoStack creates an undo stack with the given max history size.
func NewUndoStack(maxSize int) *UndoStack {
	return &UndoStack{
		past:    make([]*ProjectSnapshot, 0),
		future:  make([]*ProjectSnapshot, 0),
		maxSize: maxSize,
	}
}

// Push saves a snapshot to the undo history, clearing any redo future.
func (us *UndoStack) Push(snapshot *ProjectSnapshot) {
	us.past = append(us.past, snapshot)
	us.future = nil

	if len(us.past) > us.maxSize {
		us.past = us.past[1:]
	}
}

// Undo steps back one change. Snapshots on the stack are each taken just
// *before* a change (Project.RecordUndo), so the one on top is the state to
// return to; current - the live state, which nothing on the stack holds yet
// - goes onto the redo stack so Redo can bring it back. Returns nil if
// there is nothing to undo.
//
// The returned snapshot has left the stack, so the caller may install its
// Track as the live one: nothing in the history shares it.
func (us *UndoStack) Undo(current *ProjectSnapshot) *ProjectSnapshot {
	if len(us.past) == 0 {
		return nil
	}
	state := us.past[len(us.past)-1]
	us.past = us.past[:len(us.past)-1]
	us.future = append(us.future, current)
	return state
}

// Redo is Undo in reverse: current goes back onto the undo stack and the
// most recently undone state is returned. Returns nil if there is nothing
// to redo.
func (us *UndoStack) Redo(current *ProjectSnapshot) *ProjectSnapshot {
	if len(us.future) == 0 {
		return nil
	}
	state := us.future[len(us.future)-1]
	us.future = us.future[:len(us.future)-1]
	us.past = append(us.past, current)
	return state
}

// CanUndo returns true if there is something to undo.
func (us *UndoStack) CanUndo() bool {
	return len(us.past) > 0
}

// CanRedo returns true if there is something to redo.
func (us *UndoStack) CanRedo() bool {
	return len(us.future) > 0
}

// Clear removes all undo/redo history.
func (us *UndoStack) Clear() {
	us.past = us.past[:0]
	us.future = us.future[:0]
}
