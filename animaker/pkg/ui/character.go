package ui

import (
	"fmt"
	"strconv"
	"strings"

	"animaker/pkg/editor"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// CharacterPanel lists an .anichar's animations, like Spine's animation
// panel: clicking one opens its track. For the open animation it edits
// the play mode and the markers. The app owns the character and saves it
// whenever OnChanged fires.
type CharacterPanel struct {
	Character *editor.Character
	// Open is the animation whose track is open in the editor ("" when
	// the open track isn't one of the character's).
	Open string

	OnSelect  func(name string) // open this animation's track
	OnAdd     func()
	OnRename  func(name string)
	OnRemove  func(name string)
	OnChanged func() // mode, markers or controller edited
	// PlayheadMs is where "Add Marker" places a new marker.
	PlayheadMs func() uint32

	title      *widget.Label
	controller *selectAllEntry
	list       *fyne.Container
	openBox    *fyne.Container
	root       fyne.CanvasObject
}

// NewCharacterPanel returns an empty panel; Build makes its widgets.
func NewCharacterPanel() *CharacterPanel { return &CharacterPanel{} }

// Build assembles the panel.
func (cp *CharacterPanel) Build() fyne.CanvasObject {
	cp.title = widget.NewLabel("")
	cp.title.TextStyle = fyne.TextStyle{Bold: true}
	cp.controller = newEntry()
	cp.controller.SetPlaceHolder("(none)")
	applyController := func() {
		s := strings.TrimSpace(cp.controller.Text)
		if cp.Character == nil || cp.Character.Controller == s {
			return
		}
		cp.Character.Controller = s
		cp.changed()
	}
	cp.controller.OnSubmitted = func(string) { applyController() }
	cp.controller.onFocusLost = applyController
	add := widget.NewButton("+ Add Animation", func() {
		if cp.OnAdd != nil {
			cp.OnAdd()
		}
	})
	cp.list = container.NewVBox()
	cp.openBox = container.NewVBox()
	cp.root = container.NewVScroll(container.NewVBox(
		cp.title,
		container.NewBorder(nil, nil, widget.NewLabel("Controller"), nil, cp.controller),
		add,
		cp.list,
		widget.NewSeparator(),
		cp.openBox,
	))
	cp.Refresh()
	return cp.root
}

// Refresh rebuilds the panel from Character and Open.
func (cp *CharacterPanel) Refresh() {
	if cp.list == nil {
		return
	}
	cp.list.RemoveAll()
	cp.openBox.RemoveAll()
	c := cp.Character
	if c == nil {
		cp.title.SetText("CHARACTER")
		return
	}
	cp.title.SetText("CHARACTER: " + c.Name)
	if cp.controller.Text != c.Controller {
		cp.controller.SetText(c.Controller)
	}
	if len(c.Animations) == 0 {
		hint := widget.NewLabel("No animations yet. Add one: a new track from this character's rig, or an existing .anif.")
		hint.Wrapping = fyne.TextWrapWord
		cp.list.Add(hint)
	}
	for _, a := range c.Animations {
		name := a.Name
		btn := widget.NewButton(fmt.Sprintf("%s  (%s)", name, a.Mode), func() {
			if cp.OnSelect != nil && name != cp.Open {
				cp.OnSelect(name)
			}
		})
		btn.Alignment = widget.ButtonAlignLeading
		if name == cp.Open {
			btn.Importance = widget.HighImportance
		}
		rename := widget.NewButton("Rename", func() {
			if cp.OnRename != nil {
				cp.OnRename(name)
			}
		})
		remove := widget.NewButton("×", func() {
			if cp.OnRemove != nil {
				cp.OnRemove(name)
			}
		})
		remove.Importance = widget.DangerImportance
		cp.list.Add(container.NewBorder(nil, nil, nil, container.NewHBox(rename, remove), btn))
	}
	if a := c.Find(cp.Open); a != nil {
		cp.buildOpen(a)
	}
}

// buildOpen shows the open animation's play mode and markers.
func (cp *CharacterPanel) buildOpen(a *editor.CharAnim) {
	cp.openBox.Add(newSectionHeader("ANIMATION: " + a.Name))
	modes := make([]string, len(editor.PlayModes))
	for i, m := range editor.PlayModes {
		modes[i] = m.String()
	}
	// Seeded before OnChanged is set, so seeding doesn't count as an edit.
	mode := widget.NewSelect(modes, nil)
	mode.SetSelected(a.Mode.String())
	mode.OnChanged = func(s string) {
		m, err := editor.ParsePlayMode(s)
		if err != nil || m == a.Mode {
			return
		}
		a.Mode = m
		cp.changed()
		cp.Refresh()
	}
	cp.openBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Plays"), nil, mode))
	modeHint := widget.NewLabel("loop: repeats. once: plays through and reports it finished. hold: stays on its last frame.")
	modeHint.Wrapping = fyne.TextWrapWord
	cp.openBox.Add(modeHint)

	cp.openBox.Add(newSectionHeader("MARKERS"))
	markerHint := widget.NewLabel("Named instants for presentation (footstep sounds, hit sparks), drawn on the timeline. " +
		"Not gameplay timing: when a hit lands is gameplay data, which `animaker lint` checks these against.")
	markerHint.Wrapping = fyne.TextWrapWord
	cp.openBox.Add(markerHint)
	for _, m := range a.Markers {
		cp.openBox.Add(cp.markerRow(a, m))
	}
	cp.openBox.Add(widget.NewButton("+ Add Marker at Playhead", func() { cp.addMarker(a) }))
}

// markerRow edits one marker's name and time. Edits apply when the field
// loses focus or Enter is pressed, so typing doesn't re-sort the rows
// mid-word.
func (cp *CharacterPanel) markerRow(a *editor.CharAnim, m editor.Marker) fyne.CanvasObject {
	name := newEntry()
	name.SetText(m.Name)
	at := newEntry()
	at.SetText(strconv.FormatUint(uint64(m.TimeMs), 10))
	apply := func() {
		t, err := strconv.ParseUint(strings.TrimSpace(at.Text), 10, 32)
		if err != nil {
			at.SetText(strconv.FormatUint(uint64(m.TimeMs), 10))
			return
		}
		if strings.TrimSpace(name.Text) == m.Name && uint32(t) == m.TimeMs {
			return
		}
		// Found again by value: this row may have been rebuilt away, and
		// the markers re-sorted, since it was drawn.
		i := a.MarkerIndex(m)
		if i < 0 {
			return
		}
		if err := a.SetMarker(name.Text, i, uint32(t)); err != nil {
			name.SetText(m.Name)
			return
		}
		cp.changed()
		cp.Refresh()
	}
	name.OnSubmitted = func(string) { apply() }
	at.OnSubmitted = func(string) { apply() }
	name.onFocusLost = apply
	at.onFocusLost = apply
	remove := widget.NewButton("×", func() {
		i := a.MarkerIndex(m)
		if i < 0 {
			return
		}
		a.RemoveMarker(i)
		cp.changed()
		cp.Refresh()
	})
	remove.Importance = widget.DangerImportance
	ms := container.NewBorder(nil, nil, nil, widget.NewLabel("ms"), at)
	return container.NewBorder(nil, nil, nil, remove, container.NewGridWithColumns(2, name, ms))
}

func (cp *CharacterPanel) addMarker(a *editor.CharAnim) {
	var t uint32
	if cp.PlayheadMs != nil {
		t = cp.PlayheadMs()
	}
	if err := a.SetMarker("marker", -1, t); err != nil {
		return
	}
	cp.changed()
	cp.Refresh()
}

func (cp *CharacterPanel) changed() {
	if cp.OnChanged != nil {
		cp.OnChanged()
	}
}

// AddAnimationSource is where a new animation's track comes from.
type AddAnimationSource int

const (
	// FromRig makes a new track from the character's rig (editor.RigFrom).
	FromRig AddAnimationSource = iota
	// FromEmpty makes a new, empty track.
	FromEmpty
	// FromExisting uses an .anif that already exists.
	FromExisting
)

// ShowAddAnimationDialog asks for a new animation's name and where its
// track comes from. rigFrom names the track a rig would be copied from
// ("" when there's none, which leaves that choice out). onAdd returns an
// error to refuse; the dialog then reopens with what was typed.
func ShowAddAnimationDialog(win fyne.Window, rigFrom string, onAdd func(name string, src AddAnimationSource) error) {
	showAddAnimationDialog(win, rigFrom, "", -1, onAdd)
}

func showAddAnimationDialog(win fyne.Window, rigFrom, typed string, picked AddAnimationSource, onAdd func(string, AddAnimationSource) error) {
	name := newEntry()
	name.SetPlaceHolder("walk")
	name.SetText(typed)
	var labels []string
	var sources []AddAnimationSource
	if rigFrom != "" {
		labels = append(labels, "New track from "+rigFrom+"'s rig")
		sources = append(sources, FromRig)
	}
	labels = append(labels, "New empty track", "Existing .anif...")
	sources = append(sources, FromEmpty, FromExisting)
	src := widget.NewRadioGroup(labels, nil)
	src.Required = true
	src.SetSelected(labels[0])
	for i, s := range sources {
		if s == picked {
			src.SetSelected(labels[i])
		}
	}
	form := dialog.NewForm("Add Animation", "Add", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: name, HintText: "What the game plays it by, e.g. walk, sword_attack"},
			{Text: "Track", Widget: src},
		},
		func(ok bool) {
			if !ok || onAdd == nil {
				return
			}
			chosen := FromEmpty
			for i, l := range labels {
				if l == src.Selected {
					chosen = sources[i]
				}
			}
			if err := onAdd(strings.TrimSpace(name.Text), chosen); err != nil {
				typed := name.Text
				d := dialog.NewError(err, win)
				d.SetOnClosed(func() { showAddAnimationDialog(win, rigFrom, typed, chosen, onAdd) })
				d.Show()
			}
		}, win)
	form.Resize(fyne.NewSize(460, 280))
	form.Show()
	win.Canvas().Focus(name)
}

// ShowNameDialog asks for a name, prefilled with current. onName returns
// an error to refuse it; the dialog then reopens with what was typed.
func ShowNameDialog(win fyne.Window, title, action, current string, onName func(string) error) {
	entry := newEntry()
	entry.SetText(current)
	form := dialog.NewForm(title, action, "Cancel",
		[]*widget.FormItem{{Text: "Name", Widget: entry}},
		func(ok bool) {
			if !ok || onName == nil {
				return
			}
			if err := onName(strings.TrimSpace(entry.Text)); err != nil {
				typed := entry.Text
				d := dialog.NewError(err, win)
				d.SetOnClosed(func() { ShowNameDialog(win, title, action, typed, onName) })
				d.Show()
			}
		}, win)
	form.Resize(fyne.NewSize(400, 160))
	form.Show()
	win.Canvas().Focus(entry)
}
