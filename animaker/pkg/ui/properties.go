package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PropertiesPanel edits the schema (props), the active direction's part
// list, preview overrides, and the selected keyframe's transform/cell/
// bindings.
type PropertiesPanel struct {
	project *editor.Project

	propsSection    *fyne.Container
	previewSection  *fyne.Container
	partsSection    *fyne.Container
	keyframeSection *fyne.Container

	OnPropsChanged     func()
	OnPartRemoved      func(idx int)
	OnKeyframeChanged  func()
	OnPreviewChanged   func()
}

func NewPropertiesPanel(project *editor.Project) *PropertiesPanel {
	return &PropertiesPanel{project: project}
}

func (pp *PropertiesPanel) SetProject(project *editor.Project) {
	pp.project = project
}

func (pp *PropertiesPanel) Build() fyne.CanvasObject {
	pp.propsSection = container.NewVBox()
	pp.previewSection = container.NewVBox()
	pp.partsSection = container.NewVBox()
	pp.keyframeSection = container.NewVBox()

	pp.Refresh()

	all := container.NewVBox(
		newSectionHeader("PROPS (schema)"),
		pp.propsSection,
		widget.NewSeparator(),
		newSectionHeader("PREVIEW OVERRIDES"),
		pp.previewSection,
		widget.NewSeparator(),
		newSectionHeader("PARTS (this direction)"),
		pp.partsSection,
		widget.NewSeparator(),
		newSectionHeader("SELECTED KEYFRAME"),
		pp.keyframeSection,
	)

	scroll := container.NewVScroll(all)
	scroll.SetMinSize(fyne.NewSize(300, 400))
	return scroll
}

// Refresh rebuilds every section from current project state.
func (pp *PropertiesPanel) Refresh() {
	pp.refreshProps()
	pp.refreshPreview()
	pp.refreshParts()
	pp.refreshKeyframe()
}

// -- Props schema --

func (pp *PropertiesPanel) refreshProps() {
	if pp.propsSection == nil {
		return
	}
	pp.propsSection.RemoveAll()
	for i, prop := range pp.project.CurrentTrack.Props {
		idx := i
		label := widget.NewLabel(fmt.Sprintf("%s -> %s", prop.Name, prop.Default))
		delBtn := widget.NewButton("x", func() {
			pp.project.RecordUndo()
			editor.RemoveProp(pp.project.CurrentTrack, idx)
			pp.project.Dirty = true
			if pp.OnPropsChanged != nil {
				pp.OnPropsChanged()
			}
		})
		delBtn.Importance = widget.DangerImportance
		pp.propsSection.Add(container.NewBorder(nil, nil, nil, delBtn, label))
	}
	pp.propsSection.Refresh()
}

// -- Preview overrides --

func (pp *PropertiesPanel) refreshPreview() {
	if pp.previewSection == nil {
		return
	}
	pp.previewSection.RemoveAll()
	for _, prop := range pp.project.CurrentTrack.Props {
		name := prop.Name
		entry := widget.NewEntry()
		if v, ok := pp.project.PreviewProps[name]; ok {
			entry.SetText(v)
		} else {
			entry.SetText(prop.Default)
		}
		entry.OnChanged = func(v string) {
			pp.project.PreviewProps[name] = v
			if pp.OnPreviewChanged != nil {
				pp.OnPreviewChanged()
			}
		}
		row := container.NewBorder(nil, nil, widget.NewLabel(name+":"), nil, entry)
		pp.previewSection.Add(row)
	}
	pp.previewSection.Refresh()
}

// -- Parts list --

func (pp *PropertiesPanel) refreshParts() {
	if pp.partsSection == nil {
		return
	}
	pp.partsSection.RemoveAll()
	dir := pp.project.ActiveDirection()
	if dir == nil {
		return
	}
	sel := pp.project.Selection
	for i, part := range dir.Parts {
		idx := i
		kindTag := "sheet"
		if part.Kind == editor.PartKindNestedAni {
			kindTag = "nested"
		}
		btn := widget.NewButton(fmt.Sprintf("%s [%s]", part.Name, kindTag), func() {
			pp.project.Selection.PartIndex = idx
			pp.project.Selection.KeyframeIndex = -1
			pp.Refresh()
		})
		if sel != nil && sel.PartIndex == idx {
			btn.Importance = widget.HighImportance
		}
		delBtn := widget.NewButton("x", func() {
			pp.project.RecordUndo()
			editor.RemovePart(dir, idx)
			if pp.OnPartRemoved != nil {
				pp.OnPartRemoved(idx)
			}
		})
		delBtn.Importance = widget.DangerImportance
		pp.partsSection.Add(container.NewBorder(nil, nil, nil, delBtn, btn))
	}
	pp.partsSection.Refresh()
}

// -- Selected keyframe --

func (pp *PropertiesPanel) refreshKeyframe() {
	if pp.keyframeSection == nil {
		return
	}
	pp.keyframeSection.RemoveAll()

	part := pp.project.SelectedPart()
	if part == nil {
		pp.keyframeSection.Add(widget.NewLabel("No part selected"))
		pp.keyframeSection.Refresh()
		return
	}
	kf := pp.project.SelectedKeyframe()
	if kf == nil {
		pp.keyframeSection.Add(widget.NewLabel(fmt.Sprintf("%s: no keyframe selected", part.Name)))
		pp.keyframeSection.Refresh()
		return
	}

	xEntry := numEntry(fmt.Sprintf("%v", kf.X), func(v float32) { kf.X = v; pp.notifyKeyframeChanged() })
	yEntry := numEntry(fmt.Sprintf("%v", kf.Y), func(v float32) { kf.Y = v; pp.notifyKeyframeChanged() })
	zEntry := numEntry(fmt.Sprintf("%v", kf.Z), func(v float32) { kf.Z = v; pp.notifyKeyframeChanged() })
	rotEntry := numEntry(fmt.Sprintf("%v", kf.RotationDeg), func(v float32) { kf.RotationDeg = v; pp.notifyKeyframeChanged() })

	transformGrid := container.NewGridWithColumns(2,
		widget.NewLabel("X"), xEntry,
		widget.NewLabel("Y"), yEntry,
		widget.NewLabel("Z"), zEntry,
		widget.NewLabel("Rotation"), rotEntry,
	)
	pp.keyframeSection.Add(widget.NewLabel(fmt.Sprintf("%s @ %dms", part.Name, kf.TimeMs)))
	pp.keyframeSection.Add(transformGrid)

	if part.Kind == editor.PartKindSheet {
		pp.keyframeSection.Add(pp.buildCellPicker(part, kf))
	} else {
		pp.keyframeSection.Add(pp.buildNestedBindingsEditor(part))
	}

	pp.keyframeSection.Refresh()
}

func (pp *PropertiesPanel) notifyKeyframeChanged() {
	pp.project.Dirty = true
	if pp.OnKeyframeChanged != nil {
		pp.OnKeyframeChanged()
	}
}

func (pp *PropertiesPanel) buildCellPicker(part *editor.Part, kf *editor.Keyframe) fyne.CanvasObject {
	rowEntry := intEntry(kf.Row, func(v int) { kf.Row = v; pp.notifyKeyframeChanged(); pp.refreshKeyframe() })
	colEntry := intEntry(kf.Col, func(v int) { kf.Col = v; pp.notifyKeyframeChanged(); pp.refreshKeyframe() })

	grid := container.NewGridWithColumns(2,
		widget.NewLabel("Row"), rowEntry,
		widget.NewLabel("Col"), colEntry,
	)

	sheet := pp.project.ResolveActiveSheet(part)
	preview := container.NewVBox(grid)
	if sheet == nil {
		preview.Add(widget.NewLabel(fmt.Sprintf("Active sheet %q not loaded", pp.project.ResolveActiveSheetName(part))))
		return preview
	}
	preview.Add(widget.NewLabel(fmt.Sprintf("Sheet %q: %dx%d cells", sheet.Name, sheet.Cols(), sheet.Rows())))
	if img, err := sheet.CellImage(kf.Row, kf.Col); err == nil {
		ci := canvas.NewImageFromImage(img)
		ci.ScaleMode = canvas.ImageScalePixels
		ci.FillMode = canvas.ImageFillOriginal
		ci.SetMinSize(fyne.NewSize(float32(sheet.CellW)*2, float32(sheet.CellH)*2))
		preview.Add(ci)
	} else {
		preview.Add(widget.NewLabel(err.Error()))
	}
	return preview
}

func (pp *PropertiesPanel) buildNestedBindingsEditor(part *editor.Part) fyne.CanvasObject {
	box := container.NewVBox(widget.NewLabel("Nested: " + part.NestedAniPath))

	for propName, binding := range part.NestedBindings {
		name := propName
		b := binding
		modeSelect := widget.NewSelect([]string{"passthrough", "static"}, nil)
		valueEntry := widget.NewEntry()
		if b.PassthroughFrom != "" {
			modeSelect.SetSelected("passthrough")
			valueEntry.SetText(b.PassthroughFrom)
		} else {
			modeSelect.SetSelected("static")
			valueEntry.SetText(b.StaticValue)
		}
		apply := func() {
			nb := part.NestedBindings[name]
			if modeSelect.Selected == "passthrough" {
				nb.PassthroughFrom = valueEntry.Text
				nb.StaticValue = ""
			} else {
				nb.StaticValue = valueEntry.Text
				nb.PassthroughFrom = ""
			}
			part.NestedBindings[name] = nb
			pp.notifyKeyframeChanged()
		}
		modeSelect.OnChanged = func(string) { apply() }
		valueEntry.OnChanged = func(string) { apply() }

		delBtn := widget.NewButton("x", func() {
			delete(part.NestedBindings, name)
			pp.notifyKeyframeChanged()
			pp.refreshKeyframe()
		})
		delBtn.Importance = widget.DangerImportance

		row := container.NewBorder(nil, nil, widget.NewLabel(name), delBtn,
			container.NewHBox(modeSelect, valueEntry))
		box.Add(row)
	}

	newPropEntry := widget.NewEntry()
	newPropEntry.SetPlaceHolder("prop name, e.g. direction")
	addBtn := widget.NewButton("+ Binding", func() {
		if newPropEntry.Text == "" {
			return
		}
		if part.NestedBindings == nil {
			part.NestedBindings = map[string]editor.PropBinding{}
		}
		part.NestedBindings[newPropEntry.Text] = editor.PropBinding{}
		newPropEntry.SetText("")
		pp.notifyKeyframeChanged()
		pp.refreshKeyframe()
	})
	box.Add(container.NewBorder(nil, nil, nil, addBtn, newPropEntry))

	return box
}

// -- Helpers --

func numEntry(initial string, onChange func(float32)) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(initial)
	e.OnChanged = func(s string) {
		if v, err := strconv.ParseFloat(s, 32); err == nil {
			onChange(float32(v))
		}
	}
	return e
}

func intEntry(initial int, onChange func(int)) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(strconv.Itoa(initial))
	e.OnChanged = func(s string) {
		if v, err := strconv.Atoi(s); err == nil {
			onChange(v)
		}
	}
	return e
}

func newSectionHeader(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}
