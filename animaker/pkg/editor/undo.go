package editor

import "time"

// UndoStack manages undo/redo history using full project snapshots.
type UndoStack struct {
	past    []*ProjectSnapshot
	future  []*ProjectSnapshot
	maxSize int
}

// ProjectSnapshot captures the animation state at a point in time.
type ProjectSnapshot struct {
	Animation *Animation
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
	us.future = nil // Clear redo stack on new action

	// Limit size
	if len(us.past) > us.maxSize {
		us.past = us.past[1:]
	}
}

// Undo restores the previous state. Returns nil if nothing to undo.
func (us *UndoStack) Undo() *ProjectSnapshot {
	if len(us.past) == 0 {
		return nil
	}

	state := us.past[len(us.past)-1]
	us.past = us.past[:len(us.past)-1]
	us.future = append(us.future, state)

	// Return the state before the one we just undid
	if len(us.past) > 0 {
		return us.past[len(us.past)-1]
	}
	return state
}

// Redo restores the next state. Returns nil if nothing to redo.
func (us *UndoStack) Redo() *ProjectSnapshot {
	if len(us.future) == 0 {
		return nil
	}

	state := us.future[len(us.future)-1]
	us.future = us.future[:len(us.future)-1]
	us.past = append(us.past, state)

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
