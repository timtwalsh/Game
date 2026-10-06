package main

import (
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// A few hand-made immediate-mode widgets. raygui was considered (spec
// 5.2) but its text box can't select its contents on focus, which is the
// animaker convention the spec asks for, and these few widgets avoid
// another cgo dependency.

const (
	fontSize = 16
	rowH     = 26
	pad      = 10
)

var (
	colPanel     = rl.NewColor(36, 38, 44, 255)
	colButton    = rl.NewColor(58, 62, 72, 255)
	colButtonHot = rl.NewColor(78, 84, 98, 255)
	colActive    = rl.NewColor(70, 110, 170, 255)
	colText      = rl.NewColor(225, 228, 235, 255)
	colDim       = rl.NewColor(150, 155, 165, 255)
	colError     = rl.NewColor(240, 110, 100, 255)
	colAccent    = rl.NewColor(250, 210, 80, 255)
)

func mouseIn(r rl.Rectangle) bool { return rl.CheckCollisionPointRec(rl.GetMousePosition(), r) }

// button draws a button and reports whether it was clicked this frame.
// enabled=false draws it dimmed and ignores clicks.
func button(r rl.Rectangle, label string, active, enabled bool) bool {
	bg := colButton
	switch {
	case active:
		bg = colActive
	case enabled && mouseIn(r):
		bg = colButtonHot
	}
	rl.DrawRectangleRec(r, bg)
	fg := colText
	if !enabled {
		fg = colDim
	}
	tw := rl.MeasureText(label, fontSize)
	rl.DrawText(label, int32(r.X+(r.Width-float32(tw))/2), int32(r.Y+(r.Height-fontSize)/2), fontSize, fg)
	return enabled && rl.IsMouseButtonPressed(rl.MouseButtonLeft) && mouseIn(r)
}

// label draws text and returns its height.
func label(text string, x, y float32, col rl.Color) float32 {
	rl.DrawText(text, int32(x), int32(y), fontSize, col)
	return fontSize + 4
}

// wrap breaks text into lines no wider than width pixels.
func wrap(text string, width int32) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			try := word
			if line != "" {
				try = line + " " + word
			}
			if rl.MeasureText(try, fontSize) > width && line != "" {
				lines = append(lines, line)
				line = word
				continue
			}
			line = try
		}
		lines = append(lines, line)
	}
	return lines
}

// dialog is a modal form: text fields, an optional checkbox, and a row of
// buttons. Enter presses the first button, Esc the last; Tab and
// Shift+Tab move between fields, selecting each one's contents.
type dialog struct {
	title     string
	message   string
	fields    []*textField
	focus     int
	checkbox  string // label; "" for none
	checked   bool
	buttons   []string
	err       string
	onButton  func(d *dialog, i int) (close bool)
	firstDraw bool
}

func (d *dialog) focusField(i int) {
	if len(d.fields) == 0 {
		return
	}
	d.focus = (i + len(d.fields)) % len(d.fields)
	d.fields[d.focus].Focus()
}

// update handles keyboard input; it returns a button index pressed by key,
// or -1.
func (d *dialog) update() int {
	if !d.firstDraw {
		d.firstDraw = true
		d.focusField(0)
	}
	if len(d.fields) > 0 {
		f := d.fields[d.focus]
		for r := rl.GetCharPressed(); r != 0; r = rl.GetCharPressed() {
			f.Type(rune(r))
		}
		if rl.IsKeyPressed(rl.KeyBackspace) || rl.IsKeyPressedRepeat(rl.KeyBackspace) {
			f.Backspace()
		}
		if rl.IsKeyPressed(rl.KeyTab) {
			if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
				d.focusField(d.focus - 1)
			} else {
				d.focusField(d.focus + 1)
			}
		}
	}
	switch {
	case rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter):
		return 0
	case rl.IsKeyPressed(rl.KeyEscape):
		return len(d.buttons) - 1
	}
	return -1
}

// draw draws the dialog centred on the screen and returns the index of a
// clicked button, or -1.
func (d *dialog) draw() int {
	sw, sh := float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
	rl.DrawRectangle(0, 0, int32(sw), int32(sh), rl.Fade(rl.Black, 0.5))
	w := float32(420)
	msgLines := wrap(d.message, int32(w-2*pad))
	errLines := wrap(d.err, int32(w-2*pad))
	h := float32(pad*2+fontSize+8) + float32(len(msgLines))*(fontSize+4) + float32(len(d.fields))*(rowH+6) + rowH + pad
	if d.message == "" {
		h -= fontSize + 4
	}
	if d.checkbox != "" {
		h += rowH + 6
	}
	if d.err != "" {
		h += float32(len(errLines)) * (fontSize + 4)
	}
	x, y := (sw-w)/2, (sh-h)/2
	rl.DrawRectangleRec(rl.NewRectangle(x, y, w, h), colPanel)
	rl.DrawRectangleLinesEx(rl.NewRectangle(x, y, w, h), 1, colDim)
	cy := y + pad
	cy += label(d.title, x+pad, cy, colAccent) + 4
	if d.message != "" {
		for _, l := range msgLines {
			cy += label(l, x+pad, cy, colText)
		}
	}
	for i, f := range d.fields {
		label(f.Label, x+pad, cy+5, colDim)
		box := rl.NewRectangle(x+130, cy, w-130-pad, rowH)
		bg := colButton
		if i == d.focus {
			bg = colButtonHot
		}
		rl.DrawRectangleRec(box, bg)
		if f.Selected() && i == d.focus && f.Text != "" {
			tw := rl.MeasureText(f.Text, fontSize)
			rl.DrawRectangle(int32(box.X+5), int32(box.Y+4), tw+2, rowH-8, colActive)
		}
		rl.DrawText(f.Text, int32(box.X+6), int32(box.Y+5), fontSize, colText)
		if i == d.focus && !f.Selected() && (rl.GetTime()*2)-float64(int(rl.GetTime()*2)) < 0.5 {
			cx := box.X + 7 + float32(rl.MeasureText(f.Text, fontSize))
			rl.DrawRectangle(int32(cx), int32(box.Y+5), 2, fontSize, colText)
		}
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && mouseIn(box) {
			d.focusField(i)
		}
		cy += rowH + 6
	}
	if d.checkbox != "" {
		box := rl.NewRectangle(x+130, cy+3, rowH-6, rowH-6)
		rl.DrawRectangleRec(box, colButton)
		if d.checked {
			rl.DrawRectangleRec(rl.NewRectangle(box.X+4, box.Y+4, box.Width-8, box.Height-8), colAccent)
		}
		label(d.checkbox, box.X+box.Width+8, cy+5, colText)
		hit := rl.NewRectangle(box.X, box.Y, w-130-pad, box.Height)
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && mouseIn(hit) {
			d.checked = !d.checked
		}
		cy += rowH + 6
	}
	for _, l := range errLines {
		if d.err != "" {
			cy += label(l, x+pad, cy, colError)
		}
	}
	clicked := -1
	bw := (w - pad*float32(len(d.buttons)+1)) / float32(len(d.buttons))
	for i, b := range d.buttons {
		r := rl.NewRectangle(x+pad+float32(i)*(bw+pad), cy+4, bw, rowH)
		if button(r, b, false, true) {
			clicked = i
		}
	}
	return clicked
}
