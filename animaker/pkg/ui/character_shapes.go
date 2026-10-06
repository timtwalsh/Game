package ui

import (
	"animaker/pkg/editor"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// The character panel's footprint and hitbox section. The shapes belong to
// the character, so they're shown whichever of its animations is open.
// Fields apply when they lose focus or Enter is pressed, like markers;
// selecting a shape (its name button) lets it be dragged on the canvas.

func fmtNum(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) }

func parseNum(s string) (float32, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 32)
	return float32(v), err == nil
}

// buildShapes fills the shape section.
func (cp *CharacterPanel) buildShapes() {
	c := cp.Character
	cp.shapesBox.Add(newSectionHeader("FOOTPRINT & HITBOXES"))
	hint := widget.NewLabel("Shared by every animation, fixed to the origin, in animation pixels. " +
		"Footprint: what blocks movement. Hitboxes: where it can be hit. Select one to drag it on the canvas.")
	hint.Wrapping = fyne.TextWrapWord
	cp.shapesBox.Add(hint)

	scale := newEntry()
	scale.SetPlaceHolder("not set")
	if c.Scale > 0 {
		scale.SetText(fmtNum(c.Scale))
	}
	applyScale := func() {
		s := strings.TrimSpace(scale.Text)
		v := float32(0)
		if s != "" {
			n, ok := parseNum(s)
			if !ok || n < 0 {
				scale.SetText(fmtNum(c.Scale))
				return
			}
			v = n
		}
		if v == c.Scale {
			return
		}
		c.Scale = v
		cp.changed()
		cp.Refresh()
	}
	scale.OnSubmitted = func(string) { applyScale() }
	scale.onFocusLost = applyScale
	cp.shapesBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Scale"), widget.NewLabel("game px per px"), scale))

	if c.Footprint == nil {
		cp.shapesBox.Add(widget.NewButton("+ Add Footprint", func() {
			c.SetFootprint(editor.DefaultFootprint)
			cp.selectShape(FootprintRef())
			cp.changed()
			cp.Refresh()
		}))
	} else {
		sel := widget.NewButton("select", func() { cp.toggleShape(FootprintRef()) })
		if cp.Selected.Footprint {
			sel.Importance = widget.HighImportance
		}
		remove := widget.NewButton("×", func() {
			c.ClearFootprint()
			if cp.Selected.Footprint {
				cp.selectShape(NoShape)
			}
			cp.changed()
			cp.Refresh()
		})
		remove.Importance = widget.DangerImportance
		cp.shapesBox.Add(container.NewBorder(nil, nil, sel, remove, widget.NewLabel("footprint")))
		cp.shapesBox.Add(cp.boxRow(*c.Footprint, func(b editor.Box) error { return c.SetFootprint(b) }))
		cp.shapesBox.Add(cp.gameSizeLabel(*c.Footprint))
	}

	for i, h := range c.Hitboxes {
		for _, row := range cp.hitboxRows(i, h) {
			cp.shapesBox.Add(row)
		}
	}
	cp.shapesBox.Add(widget.NewButton("+ Add Hitbox", func() {
		i := c.AddHitbox()
		cp.selectShape(HitboxRef(i))
		cp.changed()
		cp.Refresh()
	}))
}

// gameSizeLabel shows a box's size in game pixels, when there's a scale.
func (cp *CharacterPanel) gameSizeLabel(b editor.Box) fyne.CanvasObject {
	text := "Set a scale to see this in game pixels."
	if w, ok := cp.Character.GameSize(b.W); ok {
		h, _ := cp.Character.GameSize(b.H)
		text = "In game: " + fmtNum(round(w*10)/10) + " x " + fmtNum(round(h*10)/10) + " px"
	}
	l := widget.NewLabel(text)
	l.Importance = widget.LowImportance
	return l
}

// hitboxRows edits one hitbox: select/rename, shape, box. Each edit finds
// the hitbox again by the name it was drawn with: the row may have been
// rebuilt away (a field losing focus after another hitbox was removed)
// since.
func (cp *CharacterPanel) hitboxRows(i int, h editor.Hitbox) []fyne.CanvasObject {
	c := cp.Character
	ref := HitboxRef(i)
	find := func() int { return c.FindHitbox(h.Name) }
	sel := widget.NewButton("select", func() { cp.toggleShape(ref) })
	if cp.Selected == ref {
		sel.Importance = widget.HighImportance
	}
	name := newEntry()
	name.SetText(h.Name)
	applyName := func() {
		n := strings.TrimSpace(name.Text)
		j := find()
		if n == h.Name || j < 0 {
			return
		}
		next := c.Hitboxes[j]
		next.Name = n
		if err := c.SetHitbox(j, next); err != nil {
			name.SetText(h.Name)
			return
		}
		cp.changed()
		cp.Refresh()
	}
	name.OnSubmitted = func(string) { applyName() }
	name.onFocusLost = applyName

	kinds := make([]string, len(editor.ShapeKinds))
	for j, k := range editor.ShapeKinds {
		kinds[j] = k.String()
	}
	// Seeded before OnChanged is set, so seeding doesn't count as an edit.
	kind := widget.NewSelect(kinds, nil)
	kind.SetSelected(h.Kind.String())
	kind.OnChanged = func(s string) {
		k, err := editor.ParseShapeKind(s)
		j := find()
		if err != nil || j < 0 || k == c.Hitboxes[j].Kind {
			return
		}
		next := c.Hitboxes[j]
		next.Kind = k
		if c.SetHitbox(j, next) == nil {
			cp.changed()
			cp.Refresh()
		}
	}
	remove := widget.NewButton("×", func() {
		j := find()
		if j < 0 {
			return
		}
		c.RemoveHitbox(j)
		cp.selectShape(NoShape)
		cp.changed()
		cp.Refresh()
	})
	remove.Importance = widget.DangerImportance
	top := container.NewBorder(nil, nil, sel, container.NewHBox(kind, remove), name)
	box := cp.boxRow(h.Box, func(b editor.Box) error {
		j := find()
		if j < 0 {
			return nil
		}
		next := c.Hitboxes[j]
		next.Box = b
		return c.SetHitbox(j, next)
	})
	return []fyne.CanvasObject{top, box}
}

// boxRow edits a box's x, y, w, h; set applies a new box, refusing an
// invalid one (the fields then revert).
func (cp *CharacterPanel) boxRow(b editor.Box, set func(editor.Box) error) fyne.CanvasObject {
	entries := make([]*selectAllEntry, 4)
	vals := []float32{b.X, b.Y, b.W, b.H}
	labels := []string{"x", "y", "w", "h"}
	cells := make([]fyne.CanvasObject, 4)
	apply := func() {
		next := make([]float32, 4)
		for i, e := range entries {
			v, ok := parseNum(e.Text)
			if !ok {
				e.SetText(fmtNum(vals[i]))
				return
			}
			next[i] = v
		}
		nb := editor.Box{X: next[0], Y: next[1], W: next[2], H: next[3]}
		if nb == b {
			return
		}
		if err := set(nb); err != nil {
			for i, e := range entries {
				e.SetText(fmtNum(vals[i]))
			}
			return
		}
		cp.changed()
		cp.Refresh()
	}
	for i := range entries {
		e := newEntry()
		e.SetText(fmtNum(vals[i]))
		e.OnSubmitted = func(string) { apply() }
		e.onFocusLost = apply
		entries[i] = e
		cells[i] = container.NewBorder(nil, nil, widget.NewLabel(labels[i]), nil, e)
	}
	return container.NewGridWithColumns(4, cells...)
}

// toggleShape selects a shape, or deselects it if it's already selected.
func (cp *CharacterPanel) toggleShape(r ShapeRef) {
	if cp.Selected == r {
		r = NoShape
	}
	cp.selectShape(r)
	cp.Refresh()
}

func (cp *CharacterPanel) selectShape(r ShapeRef) {
	cp.Selected = r
	if cp.OnSelectShape != nil {
		cp.OnSelectShape(r)
	}
}
