package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PropertiesPanel is the level-editor-style right panel: import a sheet,
// pick a part from a list (with delete), link it to a prop (or a fixed
// sheet) right there, and see that part's active sheet as a draggable
// tile grid (SheetGridWidget) — or a bindings editor for a NestedAni
// part. Below that: the selected keyframe's numeric fields (for
// fine-tuning after a drag-drop or a canvas move), and the track-level
// props schema / preview overrides.
type PropertiesPanel struct {
	project *editor.Project

	partListBox *fyne.Container
	partLinkBox *fyne.Container
	sheetGrid   *SheetGridWidget
	keyframeBox *fyne.Container
	schemaBox   *fyne.Container
	previewBox  *fyne.Container

	// OnTileDropped is forwarded from the active part's SheetGridWidget —
	// app.go is the one that knows about the canvas, so it handles the
	// actual drop-to-keyframe logic.
	OnTileDropped     func(partIdx, row, col int, absPos fyne.Position)
	OnImport          func()
	OnAddPart         func() // app.go owns the dialog (needs the current prop list)
	OnAddProp         func()
	OnPartChanged     func()
	OnPartRemoved     func(idx int)
	OnKeyframeChanged func()
	OnPropsChanged    func()
}

func NewPropertiesPanel(project *editor.Project) *PropertiesPanel {
	return &PropertiesPanel{project: project}
}

func (pp *PropertiesPanel) SetProject(project *editor.Project) {
	pp.project = project
}

func (pp *PropertiesPanel) Build() fyne.CanvasObject {
	importBtn := widget.NewButton("Import Sprite Sheet...", func() {
		if pp.OnImport != nil {
			pp.OnImport()
		}
	})
	addPartBtn := widget.NewButton("+ Add Part", func() {
		if pp.OnAddPart != nil {
			pp.OnAddPart()
		}
	})
	addPropBtn := widget.NewButton("+ Add Prop", func() {
		if pp.OnAddProp != nil {
			pp.OnAddProp()
		}
	})

	pp.partListBox = container.NewVBox()
	pp.partLinkBox = container.NewVBox()
	pp.sheetGrid = NewSheetGridWidget()
	pp.sheetGrid.OnTileDropped = func(row, col int, absPos fyne.Position) {
		if pp.OnTileDropped != nil {
			pp.OnTileDropped(pp.project.Selection.PartIndex, row, col, absPos)
		}
	}
	pp.keyframeBox = container.NewVBox()
	pp.schemaBox = container.NewVBox()
	pp.previewBox = container.NewVBox()

	pp.Refresh()

	partArea := container.NewVBox(
		newSectionHeader("PARTS"),
		container.NewHBox(importBtn, addPartBtn),
		pp.partListBox,
		pp.partLinkBox,
		container.NewScroll(pp.sheetGrid),
	)

	all := container.NewVBox(
		partArea,
		widget.NewSeparator(),
		newSectionHeader("SELECTED KEYFRAME"),
		pp.keyframeBox,
		widget.NewSeparator(),
		container.NewHBox(newSectionHeader("PROPS (schema)"), addPropBtn),
		pp.schemaBox,
		widget.NewSeparator(),
		newSectionHeader("PREVIEW OVERRIDES"),
		pp.previewBox,
	)

	scroll := container.NewVScroll(all)
	scroll.SetMinSize(fyne.NewSize(340, 500))
	return scroll
}

// Refresh rebuilds every section from current project state.
func (pp *PropertiesPanel) Refresh() {
	pp.refreshPartList()
	pp.refreshDependentSections()
}

// refreshDependentSections rebuilds everything that depends on which part
// is selected, but not the part-select widget itself. Callers already
// inside a Select.OnChanged handler must use this instead of Refresh() —
// Select.SetSelected/ClearSelected both re-fire OnChanged, so calling
// refreshPartSelect() from within its own callback recurses forever (hit
// as an actual stack-overflow crash during manual testing).
func (pp *PropertiesPanel) refreshDependentSections() {
	pp.refreshPartLink()
	pp.refreshSheetGrid()
	pp.refreshKeyframe()
	pp.refreshSchema()
	pp.refreshPreview()
}

// -- Part list + prop linking --

// refreshPartList rebuilds the part list as plain buttons+delete rows
// rather than a Select widget — a Select's SetSelected/ClearSelected
// re-fire its own OnChanged even when nothing actually changed, which is
// exactly what caused a real stack-overflow crash here before (see the
// note in map/objects/animaker.md); a list of buttons has no such
// self-triggering hazard.
func (pp *PropertiesPanel) refreshPartList() {
	if pp.partListBox == nil {
		return
	}
	pp.partListBox.RemoveAll()

	dir := pp.project.ActiveDirection()
	if dir == nil {
		pp.partListBox.Refresh()
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
			pp.selectPart(idx)
		})
		if sel != nil && sel.PartIndex == idx {
			btn.Importance = widget.HighImportance
		}
		delBtn := widget.NewButton("Delete", func() {
			pp.project.RecordUndo()
			editor.RemovePart(dir, idx)
			if sel != nil && sel.PartIndex == idx {
				sel.PartIndex = -1
				sel.KeyframeIndex = -1
			}
			pp.Refresh()
			if pp.OnPartRemoved != nil {
				pp.OnPartRemoved(idx)
			}
		})
		delBtn.Importance = widget.DangerImportance
		pp.partListBox.Add(container.NewBorder(nil, nil, nil, delBtn, btn))
	}
	pp.partListBox.Refresh()
}

// SelectPart is called from app.go when a part is tapped directly on the
// canvas, so canvas clicks and list clicks stay in sync. idx < 0 clears
// the selection (a canvas click on empty space).
func (pp *PropertiesPanel) SelectPart(idx int) {
	if idx < 0 {
		pp.project.Selection.PartIndex = -1
		pp.project.Selection.KeyframeIndex = -1
		pp.refreshPartList()
		pp.refreshDependentSections()
		return
	}
	pp.selectPart(idx)
}

func (pp *PropertiesPanel) selectPart(idx int) {
	pp.project.Selection.PartIndex = idx
	pp.project.Selection.KeyframeIndex = -1
	pp.refreshPartList()
	pp.refreshDependentSections()
	if pp.OnPartChanged != nil {
		pp.OnPartChanged()
	}
}

func (pp *PropertiesPanel) refreshPartLink() {
	if pp.partLinkBox == nil {
		return
	}
	pp.partLinkBox.RemoveAll()

	part := pp.project.SelectedPart()
	if part == nil {
		pp.partLinkBox.Refresh()
		return
	}

	if part.Kind == editor.PartKindNestedAni {
		pp.partLinkBox.Add(widget.NewLabel("Nested: " + part.NestedAniPath))
		pp.partLinkBox.Refresh()
		return
	}

	propOptions := []string{"(none - fixed sheet)"}
	for _, pd := range pp.project.CurrentTrack.Props {
		propOptions = append(propOptions, pd.Name)
	}
	propSelect := widget.NewSelect(propOptions, nil)
	if part.GoverningProp == "" {
		propSelect.SetSelected(propOptions[0])
	} else {
		propSelect.SetSelected(part.GoverningProp)
	}

	fixedEntry := widget.NewEntry()
	fixedEntry.SetText(part.FixedSheet)
	fixedEntry.Disable()
	if part.GoverningProp == "" {
		fixedEntry.Enable()
	}

	propSelect.OnChanged = func(v string) {
		pp.project.RecordUndo()
		if v == propOptions[0] {
			part.GoverningProp = ""
			fixedEntry.Enable()
		} else {
			part.GoverningProp = v
			fixedEntry.Disable()
		}
		pp.project.Dirty = true
		pp.refreshSheetGrid()
		if pp.OnPartChanged != nil {
			pp.OnPartChanged()
		}
	}
	fixedEntry.OnChanged = func(v string) {
		part.FixedSheet = v
		pp.project.Dirty = true
		pp.refreshSheetGrid()
		if pp.OnPartChanged != nil {
			pp.OnPartChanged()
		}
	}

	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Prop:"), nil, propSelect))
	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Fixed sheet:"), nil, fixedEntry))
	pp.partLinkBox.Refresh()
}

func (pp *PropertiesPanel) refreshSheetGrid() {
	if pp.sheetGrid == nil {
		return
	}
	part := pp.project.SelectedPart()
	if part == nil || part.Kind != editor.PartKindSheet {
		pp.sheetGrid.SetSheet(nil)
		return
	}
	pp.sheetGrid.SetSheet(pp.project.ResolveActiveSheet(part))
}

// -- Selected keyframe --

func (pp *PropertiesPanel) refreshKeyframe() {
	if pp.keyframeBox == nil {
		return
	}
	pp.keyframeBox.RemoveAll()

	part := pp.project.SelectedPart()
	if part == nil {
		pp.keyframeBox.Add(widget.NewLabel("No part selected"))
		pp.keyframeBox.Refresh()
		return
	}
	kf := pp.project.SelectedKeyframe()
	if kf == nil {
		pp.keyframeBox.Add(widget.NewLabel(fmt.Sprintf("%s: drag a tile onto the canvas to place a keyframe", part.Name)))
		pp.keyframeBox.Refresh()
		return
	}

	xEntry := numEntry(fmt.Sprintf("%v", kf.X), func(v float32) { kf.X = v; pp.notifyKeyframeChanged() })
	yEntry := numEntry(fmt.Sprintf("%v", kf.Y), func(v float32) { kf.Y = v; pp.notifyKeyframeChanged() })
	zEntry := numEntry(fmt.Sprintf("%v", kf.Z), func(v float32) { kf.Z = v; pp.notifyKeyframeChanged() })
	rotEntry := numEntry(fmt.Sprintf("%v", kf.RotationDeg), func(v float32) { kf.RotationDeg = v; pp.notifyKeyframeChanged() })

	grid := container.NewGridWithColumns(2,
		widget.NewLabel("X"), xEntry,
		widget.NewLabel("Y"), yEntry,
		widget.NewLabel("Z"), zEntry,
		widget.NewLabel("Rotation"), rotEntry,
	)
	pp.keyframeBox.Add(widget.NewLabel(fmt.Sprintf("%s @ %dms  (row %d, col %d)", part.Name, kf.TimeMs, kf.Row, kf.Col)))
	pp.keyframeBox.Add(grid)

	if part.Kind == editor.PartKindNestedAni {
		pp.keyframeBox.Add(pp.buildNestedBindingsEditor(part))
	}

	pp.keyframeBox.Refresh()
}

func (pp *PropertiesPanel) notifyKeyframeChanged() {
	pp.project.Dirty = true
	if pp.OnKeyframeChanged != nil {
		pp.OnKeyframeChanged()
	}
}

func (pp *PropertiesPanel) buildNestedBindingsEditor(part *editor.Part) fyne.CanvasObject {
	box := container.NewVBox(newSectionHeader("Bindings"))

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

// -- Props schema --

func (pp *PropertiesPanel) refreshSchema() {
	if pp.schemaBox == nil {
		return
	}
	pp.schemaBox.RemoveAll()
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
		pp.schemaBox.Add(container.NewBorder(nil, nil, nil, delBtn, label))
	}
	pp.schemaBox.Refresh()
}

// -- Preview overrides --

func (pp *PropertiesPanel) refreshPreview() {
	if pp.previewBox == nil {
		return
	}
	pp.previewBox.RemoveAll()
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
			pp.refreshSheetGrid()
			if pp.OnPartChanged != nil {
				pp.OnPartChanged()
			}
		}
		row := container.NewBorder(nil, nil, widget.NewLabel(name+":"), nil, entry)
		pp.previewBox.Add(row)
	}
	pp.previewBox.Refresh()
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

func newSectionHeader(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}
