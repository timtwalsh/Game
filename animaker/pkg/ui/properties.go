package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// PropertiesPanel displays and edits properties of the currently selected keyframe.
type PropertiesPanel struct {
	project *editor.Project

	// Widgets
	frameLabel    *widget.Label
	durationEntry *widget.Entry
	speedSlider   *widget.Slider
	speedLabel    *widget.Label

	// Hitbox fields
	collisionSection fyne.CanvasObject
	attackSection    fyne.CanvasObject

	// Sprite picker
	sheetSelect   *widget.Select
	spriteGrid    *fyne.Container

	// Events list
	eventsSection fyne.CanvasObject

	// Callbacks
	OnDurationChanged func(uint32)
	OnSpeedChanged    func(float32)
	OnSpriteSelected  func(sheetName string, index int)
	OnBoxChanged      func()
	OnAddEvent        func(eventType string)
	OnDeleteEvent     func(int)
}

// NewPropertiesPanel creates a new properties panel.
func NewPropertiesPanel(project *editor.Project) *PropertiesPanel {
	return &PropertiesPanel{
		project: project,
	}
}

// SetProject updates the project reference.
func (pp *PropertiesPanel) SetProject(project *editor.Project) {
	pp.project = project
}

// Build creates the full properties panel container.
func (pp *PropertiesPanel) Build() fyne.CanvasObject {
	// -- Frame Info Section --
	pp.frameLabel = widget.NewLabel("No frame selected")
	pp.frameLabel.TextStyle = fyne.TextStyle{Bold: true}

	pp.durationEntry = widget.NewEntry()
	pp.durationEntry.SetPlaceHolder("100")
	pp.durationEntry.OnChanged = func(s string) {
		if val, err := strconv.ParseUint(s, 10, 32); err == nil {
			kf := pp.project.GetCurrentKeyFrame()
			if kf != nil {
				kf.Duration = uint32(val)
				pp.project.Dirty = true
				if pp.OnDurationChanged != nil {
					pp.OnDurationChanged(uint32(val))
				}
			}
		}
	}

	pp.speedSlider = widget.NewSlider(0.1, 3.0)
	pp.speedSlider.Step = 0.1
	pp.speedSlider.Value = 1.0
	pp.speedLabel = widget.NewLabel("1.0x")
	pp.speedSlider.OnChanged = func(v float64) {
		pp.speedLabel.SetText(fmt.Sprintf("%.1fx", v))
		kf := pp.project.GetCurrentKeyFrame()
		if kf != nil {
			kf.Speed = float32(v)
			pp.project.Dirty = true
			if pp.OnSpeedChanged != nil {
				pp.OnSpeedChanged(float32(v))
			}
		}
	}

	frameSection := container.NewVBox(
		pp.frameLabel,
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			widget.NewLabel("Duration (ms):"), pp.durationEntry,
		),
		container.NewGridWithColumns(3,
			widget.NewLabel("Speed:"), pp.speedSlider, pp.speedLabel,
		),
	)

	// -- Sprite Picker Section --
	sheetNames := pp.getSheetNames()
	pp.sheetSelect = widget.NewSelect(sheetNames, func(name string) {
		pp.refreshSpritePicker(name)
	})
	if len(sheetNames) > 0 {
		pp.sheetSelect.SetSelected(sheetNames[0])
	}

	pp.spriteGrid = container.NewGridWrap(fyne.NewSize(36, 36))

	spriteSection := container.NewVBox(
		newSectionHeader("SPRITE PICKER"),
		pp.sheetSelect,
		container.NewVScroll(pp.spriteGrid),
	)

	// -- Collision Box Section --
	collisionContent := pp.buildHitboxSection("COLLISION BOX", editor.Box{}, func() *editor.Box {
		kf := pp.project.GetCurrentKeyFrame()
		if kf != nil {
			return kf.HitBox
		}
		return nil
	}, func(box *editor.Box) {
		kf := pp.project.GetCurrentKeyFrame()
		if kf != nil {
			kf.HitBox = box
			pp.project.RecordUndo()
		}
	})

	// -- Attack Box Section --
	attackContent := pp.buildHitboxSection("ATTACK BOX", editor.Box{}, func() *editor.Box {
		kf := pp.project.GetCurrentKeyFrame()
		if kf != nil {
			return kf.AttackHitBox
		}
		return nil
	}, func(box *editor.Box) {
		kf := pp.project.GetCurrentKeyFrame()
		if kf != nil {
			kf.AttackHitBox = box
			pp.project.RecordUndo()
		}
	})

	// -- Events Section --
	addSoundBtn := widget.NewButton("+ Sound", func() {
		if pp.OnAddEvent != nil {
			pp.OnAddEvent("sound")
		}
	})
	addParticleBtn := widget.NewButton("+ Particles", func() {
		if pp.OnAddEvent != nil {
			pp.OnAddEvent("particle")
		}
	})
	addShakeBtn := widget.NewButton("+ Shake", func() {
		if pp.OnAddEvent != nil {
			pp.OnAddEvent("shake")
		}
	})
	addFlashBtn := widget.NewButton("+ Flash", func() {
		if pp.OnAddEvent != nil {
			pp.OnAddEvent("flash")
		}
	})

	eventsSection := container.NewVBox(
		newSectionHeader("EVENTS"),
		pp.buildEventsList(),
		container.NewGridWithColumns(2,
			addSoundBtn, addParticleBtn,
			addShakeBtn, addFlashBtn,
		),
	)

	// Assemble all sections
	allSections := container.NewVBox(
		frameSection,
		widget.NewSeparator(),
		spriteSection,
		widget.NewSeparator(),
		collisionContent,
		widget.NewSeparator(),
		attackContent,
		widget.NewSeparator(),
		eventsSection,
	)

	scroll := container.NewVScroll(allSections)
	scroll.SetMinSize(fyne.NewSize(250, 400))

	return scroll
}

// Refresh updates the panel to reflect the current keyframe state.
func (pp *PropertiesPanel) Refresh() {
	kf := pp.project.GetCurrentKeyFrame()
	if kf == nil {
		pp.frameLabel.SetText("No frame selected")
		pp.durationEntry.SetText("")
		return
	}

	pp.frameLabel.SetText(fmt.Sprintf("Frame %d of %d", kf.ID, len(pp.project.CurrentAnimation.KeyFrames)))
	pp.durationEntry.SetText(strconv.FormatUint(uint64(kf.Duration), 10))
	pp.speedSlider.SetValue(float64(kf.Speed))
	pp.speedLabel.SetText(fmt.Sprintf("%.1fx", kf.Speed))

	// Update sheet selector
	sheetNames := pp.getSheetNames()
	pp.sheetSelect.Options = sheetNames
	pp.sheetSelect.Refresh()
}

// RefreshSheetList updates the sprite picker sheet selector options.
func (pp *PropertiesPanel) RefreshSheetList() {
	sheetNames := pp.getSheetNames()
	pp.sheetSelect.Options = sheetNames
	pp.sheetSelect.Refresh()
	if len(sheetNames) > 0 && pp.sheetSelect.Selected == "" {
		pp.sheetSelect.SetSelected(sheetNames[0])
	}
}

// -- Private helpers --

func (pp *PropertiesPanel) getSheetNames() []string {
	names := []string{}
	for name := range pp.project.LoadedSheets {
		names = append(names, name)
	}
	return names
}

func (pp *PropertiesPanel) refreshSpritePicker(sheetName string) {
	pp.spriteGrid.RemoveAll()

	sheet, ok := pp.project.LoadedSheets[sheetName]
	if !ok {
		return
	}

	for i := 0; i < sheet.SpriteCount(); i++ {
		idx := i // capture
		label := fmt.Sprintf("%d", idx)
		btn := widget.NewButton(label, func() {
			if pp.OnSpriteSelected != nil {
				pp.OnSpriteSelected(sheetName, idx)
			}
		})
		btn.Importance = widget.LowImportance
		pp.spriteGrid.Add(btn)
	}

	pp.spriteGrid.Refresh()
}

func (pp *PropertiesPanel) buildHitboxSection(title string, _ editor.Box, getBox func() *editor.Box, setBox func(*editor.Box)) fyne.CanvasObject {
	header := newSectionHeader(title)

	xEntry := widget.NewEntry()
	xEntry.SetPlaceHolder("0")
	yEntry := widget.NewEntry()
	yEntry.SetPlaceHolder("0")
	wEntry := widget.NewEntry()
	wEntry.SetPlaceHolder("0")
	hEntry := widget.NewEntry()
	hEntry.SetPlaceHolder("0")

	updateFields := func() {
		box := getBox()
		if box != nil {
			xEntry.SetText(strconv.Itoa(box.X))
			yEntry.SetText(strconv.Itoa(box.Y))
			wEntry.SetText(strconv.Itoa(box.W))
			hEntry.SetText(strconv.Itoa(box.H))
		} else {
			xEntry.SetText("")
			yEntry.SetText("")
			wEntry.SetText("")
			hEntry.SetText("")
		}
	}

	applyFields := func() {
		box := getBox()
		if box == nil {
			return
		}
		if v, err := strconv.Atoi(xEntry.Text); err == nil {
			box.X = v
		}
		if v, err := strconv.Atoi(yEntry.Text); err == nil {
			box.Y = v
		}
		if v, err := strconv.Atoi(wEntry.Text); err == nil {
			box.W = v
		}
		if v, err := strconv.Atoi(hEntry.Text); err == nil {
			box.H = v
		}
		pp.project.Dirty = true
		if pp.OnBoxChanged != nil {
			pp.OnBoxChanged()
		}
	}

	xEntry.OnChanged = func(_ string) { applyFields() }
	yEntry.OnChanged = func(_ string) { applyFields() }
	wEntry.OnChanged = func(_ string) { applyFields() }
	hEntry.OnChanged = func(_ string) { applyFields() }

	addBtn := widget.NewButton("Add "+title, func() {
		newBox := &editor.Box{X: 4, Y: 4, W: 24, H: 24}
		setBox(newBox)
		updateFields()
		if pp.OnBoxChanged != nil {
			pp.OnBoxChanged()
		}
	})

	deleteBtn := widget.NewButton("Delete", func() {
		setBox(nil)
		updateFields()
		if pp.OnBoxChanged != nil {
			pp.OnBoxChanged()
		}
	})
	deleteBtn.Importance = widget.DangerImportance

	fields := container.NewGridWithColumns(4,
		container.NewVBox(widget.NewLabel("X"), xEntry),
		container.NewVBox(widget.NewLabel("Y"), yEntry),
		container.NewVBox(widget.NewLabel("W"), wEntry),
		container.NewVBox(widget.NewLabel("H"), hEntry),
	)

	// Show fields or "Add" button based on whether box exists
	updateFields()

	return container.NewVBox(
		header,
		fields,
		container.NewHBox(addBtn, layout.NewSpacer(), deleteBtn),
	)
}

func (pp *PropertiesPanel) buildEventsList() fyne.CanvasObject {
	kf := pp.project.GetCurrentKeyFrame()
	if kf == nil || len(kf.Events) == 0 {
		return widget.NewLabel("No events")
	}

	items := []fyne.CanvasObject{}
	for i, e := range kf.Events {
		idx := i
		label := fmt.Sprintf("%s", e.EventType())
		switch v := e.(type) {
		case *editor.SoundEvent:
			label = fmt.Sprintf("♪ Sound: %s (pitch: %.1f)", v.FilePath, v.Pitch)
		case *editor.ParticleEvent:
			label = fmt.Sprintf("✦ Particle: %s at (%d,%d)", v.Type, v.X, v.Y)
		case *editor.ShakeEvent:
			label = fmt.Sprintf("⚡ Shake: %dms (%.0f%%)", v.DurationMs, v.Intensity*100)
		case *editor.FlashEvent:
			label = fmt.Sprintf("◆ Flash: %s %dms", v.Color, v.DurationMs)
		}

		deleteBtn := widget.NewButton("✕", func() {
			if pp.OnDeleteEvent != nil {
				pp.OnDeleteEvent(idx)
			}
		})
		deleteBtn.Importance = widget.DangerImportance

		row := container.NewBorder(nil, nil, nil, deleteBtn, widget.NewLabel(label))
		items = append(items, row)
	}

	return container.NewVBox(items...)
}

// newSectionHeader creates a styled section header.
func newSectionHeader(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}
