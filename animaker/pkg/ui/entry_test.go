package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

// Reported: tabbing into a field should select its text, so typing
// replaces it.
func TestTabIntoEntrySelectsAndTypingReplaces(t *testing.T) {
	test.NewTempApp(t)
	first, second := newEntry(), newEntry()
	first.SetText("592")
	second.SetText("120")
	w := test.NewWindow(container.NewVBox(first, second))
	defer w.Close()

	w.Canvas().Focus(first)
	w.Canvas().FocusNext() // what Tab does
	if w.Canvas().Focused() != second {
		t.Fatal("Tab didn't move focus to the second entry")
	}
	if got := second.SelectedText(); got != "120" {
		t.Fatalf("selected text after Tab = %q, want %q", got, "120")
	}

	test.Type(second, "600")
	if second.Text != "600" {
		t.Errorf("text after typing = %q, want 600 (replaced, not appended)", second.Text)
	}
}

// A click places the caret instead of selecting everything, so clicking
// into the middle of a value to fix one digit still works.
func TestClickIntoEntryPlacesCaret(t *testing.T) {
	test.NewTempApp(t)
	e := newEntry()
	e.SetText("592")
	w := test.NewWindow(container.NewVBox(e))
	defer w.Close()
	w.Resize(fyne.NewSize(200, 100))

	// Fyne's Entry focuses on mouse-down (test.Tap only sends the tap).
	e.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(8, 10)},
		Button:     desktop.MouseButtonPrimary,
	})
	if w.Canvas().Focused() != e {
		t.Fatal("click didn't focus the entry")
	}
	if got := e.SelectedText(); got != "" {
		t.Errorf("click selected %q, want just a caret", got)
	}
}
