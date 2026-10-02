package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// selectAllEntry is a text entry that selects its whole contents when it
// gains focus, so tabbing into a field and typing replaces the value
// instead of appending to it - the usual desktop behaviour for form and
// property fields, and what makes tabbing through X / Y / Z / Time quick.
// Fyne's own Entry just puts the caret at the end.
//
// A click still places the caret where you clicked: Fyne positions the
// caret from the press after focus is gained, which replaces the
// selection made here. So Tab selects, click edits.
type selectAllEntry struct {
	widget.Entry

	// inBurst is set by the first change since the entry gained focus and
	// cleared when focus moves on: one stretch of typing in one field is
	// one edit, which is what an undo step should take back.
	inBurst bool
}

// newEntry is the editor's text entry; use it instead of widget.NewEntry
// so every field behaves the same under Tab.
func newEntry() *selectAllEntry {
	e := &selectAllEntry{}
	e.ExtendBaseWidget(e)
	return e
}

func (e *selectAllEntry) FocusGained() {
	e.Entry.FocusGained()
	e.inBurst = false
	e.Entry.TypedShortcut(&fyne.ShortcutSelectAll{})
}

func (e *selectAllEntry) FocusLost() {
	e.Entry.FocusLost()
	e.inBurst = false
}

// startEdit reports whether a change is the first of a new editing burst
// (see inBurst), so its caller records one undo step per burst rather than
// one per keystroke - typing "-12.5" is one undo, not five.
func (e *selectAllEntry) startEdit() bool {
	if e.inBurst {
		return false
	}
	e.inBurst = true
	return true
}
