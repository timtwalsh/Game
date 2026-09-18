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

// PropertiesPanel owns two separate panels, built by two separate calls:
// Build() is the right column — direction bar, part list (with delete),
// prop/fixed-sheet linking for the selected part, the selected keyframe's
// transform fields plus nudge buttons (or a bindings editor for a
// NestedAni part), and the props schema / preview overrides. BuildPalette()
// is the left column — the selected part's active sheet as a draggable
// tile grid (SheetGridWidget), GraalShop-style: always visible, not
// nested inside another section. Both share the same underlying state
// (pp.sheetGrid etc.) so a part/prop change refreshes both at once.
type PropertiesPanel struct {
	project *editor.Project

	partListBox  *fyne.Container
	partLinkBox  *fyne.Container
	sheetGrid    *SheetGridWidget
	paletteLabel *widget.Label
	keyframeBox  *fyne.Container
	schemaBox    *fyne.Container
	previewBox   *fyne.Container

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

// Build assembles the right column. directionBar is embedded as a fixed
// header above everything else — built and owned by app.go (it needs the
// window's shortcut/undo plumbing), just placed here so it lives next to
// the rest of the rig controls instead of its own separate top strip.
func (pp *PropertiesPanel) Build(directionBar fyne.CanvasObject) fyne.CanvasObject {
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
	pp.ensureSheetGrid()
	pp.keyframeBox = container.NewVBox()
	pp.schemaBox = container.NewVBox()
	pp.previewBox = container.NewVBox()

	pp.Refresh()

	// Every major section is its own resizable pane (nested VSplits, since
	// Fyne's Split only takes two children) instead of one long scrolling
	// VBox.
	partArea := container.NewVScroll(container.NewVBox(
		newSectionHeader("PARTS"),
		container.NewHBox(importBtn, addPartBtn),
		pp.partListBox,
		pp.partLinkBox,
	))

	keyframeArea := container.NewVScroll(container.NewVBox(
		newSectionHeader("SELECTED KEYFRAME"), pp.keyframeBox,
	))

	propsArea := container.NewVScroll(container.NewVBox(
		container.NewHBox(newSectionHeader("PROPS (schema)"), addPropBtn), pp.schemaBox,
	))
	previewArea := container.NewVScroll(container.NewVBox(
		newSectionHeader("PREVIEW OVERRIDES"), pp.previewBox,
	))
	propsAndPreview := container.NewVSplit(propsArea, previewArea)
	propsAndPreview.SetOffset(0.5)

	keyframeAndBelow := container.NewVSplit(keyframeArea, propsAndPreview)
	keyframeAndBelow.SetOffset(0.4)

	full := container.NewVSplit(partArea, keyframeAndBelow)
	full.SetOffset(0.3)

	return container.NewBorder(directionBar, nil, nil, nil, full)
}

// BuildPalette assembles the left column: a persistent, always-visible
// tile grid for the selected part's active sheet — GraalShop's "Sprite
// Book", scoped to whichever part is currently selected rather than
// showing every loaded sheet at once (which part a dropped tile applies
// to is otherwise ambiguous, since OnTileDropped only carries a row/col,
// not which sheet they came from).
func (pp *PropertiesPanel) BuildPalette() fyne.CanvasObject {
	pp.ensureSheetGrid()
	pp.paletteLabel = widget.NewLabel("No part selected")
	pp.refreshPaletteLabel()
	return container.NewBorder(pp.paletteLabel, nil, nil, nil, container.NewScroll(pp.sheetGrid))
}

func (pp *PropertiesPanel) ensureSheetGrid() {
	if pp.sheetGrid != nil {
		return
	}
	pp.sheetGrid = NewSheetGridWidget()
	pp.sheetGrid.OnTileDropped = func(row, col int, absPos fyne.Position) {
		if pp.OnTileDropped != nil {
			pp.OnTileDropped(pp.project.Selection.PartIndex, row, col, absPos)
		}
	}
}

func (pp *PropertiesPanel) refreshPaletteLabel() {
	if pp.paletteLabel == nil {
		return
	}
	part := pp.project.SelectedPart()
	switch {
	case part == nil && len(pp.project.LoadedSheets) == 0:
		pp.paletteLabel.SetText("Import a sprite sheet to begin")
	case part == nil:
		pp.paletteLabel.SetText("Select a part to show its tiles")
	case part.Kind == editor.PartKindNestedAni:
		pp.paletteLabel.SetText(part.Name + " (nested - no sheet)")
	default:
		name := pp.project.ResolveActiveSheetName(part)
		if name == "" {
			pp.paletteLabel.SetText(part.Name + ": no sheet bound")
		} else if _, ok := pp.project.LoadedSheets[name]; !ok {
			pp.paletteLabel.SetText(part.Name + ": " + name + " (not loaded)")
		} else {
			pp.paletteLabel.SetText(part.Name + ": " + name)
		}
	}
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

	// A pick-list of what's actually loaded, not a free-text box: the sheet
	// name has to match a LoadedSheets key exactly or the part silently
	// draws nothing, and there was no way to see the valid names.
	sheetOptions := sheetPickerOptions(pp.project.LoadedSheetNames(), part.FixedSheet)
	fixedSelect := widget.NewSelect(sheetOptions, nil)
	if part.FixedSheet != "" {
		fixedSelect.SetSelected(part.FixedSheet)
	}
	fixedSelect.PlaceHolder = "(no sheet)"
	if part.GoverningProp != "" {
		fixedSelect.Disable()
	}

	propSelect.OnChanged = func(v string) {
		pp.project.RecordUndo()
		if v == propOptions[0] {
			part.GoverningProp = ""
			fixedSelect.Enable()
		} else {
			part.GoverningProp = v
			fixedSelect.Disable()
		}
		pp.project.Dirty = true
		pp.refreshSheetGrid()
		if pp.OnPartChanged != nil {
			pp.OnPartChanged()
		}
	}
	// Assigned after SetSelected above so seeding the current value doesn't
	// fire the handler — Fyne's Select re-fires OnChanged on SetSelected.
	fixedSelect.OnChanged = func(v string) {
		part.FixedSheet = v
		pp.project.Dirty = true
		pp.refreshSheetGrid()
		if pp.OnPartChanged != nil {
			pp.OnPartChanged()
		}
	}

	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Prop:"), nil, propSelect))
	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Fixed sheet:"), nil, fixedSelect))
	if len(pp.project.LoadedSheets) == 0 {
		pp.partLinkBox.Add(widget.NewLabel("No sheets imported yet — use Import Sprite Sheet."))
	}
	pp.partLinkBox.Refresh()
}

// sheetPickerOptions lists the loaded sheets, keeping current if it names a
// sheet that isn't loaded (e.g. a track opened without its art) so the
// Select can still display it instead of silently blanking the binding.
func sheetPickerOptions(loaded []string, current string) []string {
	if current == "" {
		return loaded
	}
	for _, n := range loaded {
		if n == current {
			return loaded
		}
	}
	return append([]string{current}, loaded...)
}

func (pp *PropertiesPanel) refreshSheetGrid() {
	if pp.sheetGrid == nil {
		return
	}
	part := pp.project.SelectedPart()
	if part == nil || part.Kind != editor.PartKindSheet {
		pp.sheetGrid.SetSheet(nil)
	} else {
		pp.sheetGrid.SetSheet(pp.project.ResolveActiveSheet(part))
	}
	pp.refreshPaletteLabel()
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
	pp.keyframeBox.Add(pp.buildNudgeControls(kf))

	if part.Kind == editor.PartKindNestedAni {
		pp.keyframeBox.Add(pp.buildNestedBindingsEditor(part))
	}

	pp.keyframeBox.Refresh()
}

// nudgeStep is how far one click of an arrow/+/- button moves a value —
// GraalShop-style pixel-by-pixel nudging as a companion to typing exact
// numbers into the entries above, not a replacement for them.
const (
	nudgeStepXY  = 1
	nudgeStepZ   = 1
	nudgeStepRot = 5
)

// buildNudgeControls returns a small D-pad for X/Y plus separate
// forward/back buttons for Z (draw-order) and rotation. Unlike the typed
// entries (which mutate in place without rebuilding, to avoid disrupting
// an in-progress keystroke), a nudge click rebuilds the keyframe section
// afterward so the entries visibly reflect the new value immediately.
func (pp *PropertiesPanel) buildNudgeControls(kf *editor.Keyframe) fyne.CanvasObject {
	nudge := func(apply func()) func() {
		return func() {
			apply()
			pp.notifyKeyframeChanged()
			pp.refreshKeyframe()
		}
	}

	xyPad := container.NewGridWithColumns(3,
		layout.NewSpacer(),
		widget.NewButton("Y-", nudge(func() { kf.Y -= nudgeStepXY })),
		layout.NewSpacer(),
		widget.NewButton("X-", nudge(func() { kf.X -= nudgeStepXY })),
		widget.NewLabel("pos"),
		widget.NewButton("X+", nudge(func() { kf.X += nudgeStepXY })),
		layout.NewSpacer(),
		widget.NewButton("Y+", nudge(func() { kf.Y += nudgeStepXY })),
		layout.NewSpacer(),
	)

	zRow := container.NewHBox(
		widget.NewLabel("Z:"),
		widget.NewButton("Back -", nudge(func() { kf.Z -= nudgeStepZ })),
		widget.NewButton("Fwd +", nudge(func() { kf.Z += nudgeStepZ })),
	)
	rotRow := container.NewHBox(
		widget.NewLabel("Rot:"),
		widget.NewButton("-", nudge(func() { kf.RotationDeg -= nudgeStepRot })),
		widget.NewButton("+", nudge(func() { kf.RotationDeg += nudgeStepRot })),
	)

	return container.NewVBox(xyPad, zRow, rotRow)
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
