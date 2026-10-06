package main

import "unicode"

// textField is a single-line text input's editing state, kept apart from
// drawing so it can be tested. Following the animaker convention, a field
// selects its whole contents when it gains focus, so typing replaces the
// value instead of appending to it.
type textField struct {
	Label    string
	Text     string
	MaxLen   int
	Allow    func(rune) bool // nil allows any printable rune
	selected bool
}

// Focus is called when the field gains focus (tab or click).
func (t *textField) Focus() { t.selected = true }

// Selected reports whether the whole contents are selected.
func (t *textField) Selected() bool { return t.selected }

// Type inserts a typed rune, replacing the contents if they're selected.
func (t *textField) Type(r rune) {
	if !unicode.IsPrint(r) || (t.Allow != nil && !t.Allow(r)) {
		return
	}
	if t.selected {
		t.Text = ""
		t.selected = false
	}
	if t.MaxLen > 0 && len([]rune(t.Text)) >= t.MaxLen {
		return
	}
	t.Text += string(r)
}

// Backspace deletes the last rune, or everything if it's selected.
func (t *textField) Backspace() {
	if t.selected {
		t.Text = ""
		t.selected = false
		return
	}
	if r := []rune(t.Text); len(r) > 0 {
		t.Text = string(r[:len(r)-1])
	}
}

// Field filters.
func levelNameRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-'
}

func intRune(r rune) bool { return (r >= '0' && r <= '9') || r == '-' }
