package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// PropertiesPanel owns two separate panels, built by two separate calls:
// Build() is the right column — direction bar, then one scrolling pane of
// part list (with delete), prop/fixed-sheet linking for the selected part,
// the selected keyframe's transform fields plus nudge buttons (or a
// bindings editor for a NestedAni part) and preview overrides, and below
// it the props schema. BuildPalette()
// is the left column — the selected part's active sheet as a draggable
// tile grid (SheetGridWidget), GraalShop-style: always visible, not
// nested inside another section. Both share the same underlying state
// (pp.sheetGrid etc.) so a part/prop change refreshes both at once.
type PropertiesPanel struct {
	project *editor.Project

	partListBox   *fyne.Container
	partLinkBox   *fyne.Container
	sheetGrid     *SheetGridWidget
	paletteLabel  *widget.Label
	paletteSelect *widget.Select
	keyframeBox   *fyne.Container
	schemaBox     *fyne.Container
	previewBox    *fyne.Container

	// rigBox/rigScroll are the one scrolling column holding the part list,
	// part link, selected keyframe and preview overrides. See relayout.
	rigBox    *fyne.Container
	rigScroll *container.Scroll

	// OnTileDropped is forwarded from the active part's SheetGridWidget —
	// app.go is the one that knows about the canvas, so it handles the
	// actual drop-to-keyframe logic.
	OnTileDragStart func()
	OnTileDragMove  func(sheetName string, row, col int, absPos fyne.Position)
	OnTileDropped   func(sheetName string, row, col int, absPos fyne.Position)
	OnTileTapped    func(row, col int)
	OnImport        func()
	OnImportAnim    func()
	OnAddPart       func() // app.go owns the dialog (needs the current prop list)
	OnAddProp       func()
	OnPartChanged   func()
	// OnPartDelete asks app.go to delete a part; it confirms first.
	OnPartDelete      func(idx int)
	OnKeyframeChanged func()
	OnKeyframeRetimed func() // the selected keyframe's time was typed in
	OnPropsChanged    func()
	// OnError shows an error to the artist (app.go owns the window).
	OnError func(err error)
	// OnLoadPreviewSheet asks app.go (which owns the window, for the file
	// dialog) to load an image as a preview-only option for a prop.
	OnLoadPreviewSheet func(propName string)
	// OnRemoveSheet asks app.go to remove an imported sheet; it confirms.
	OnRemoveSheet func(sheetName string)
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
	importAnimBtn := widget.NewButton("Import Animation...", func() {
		if pp.OnImportAnim != nil {
			pp.OnImportAnim()
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

	// Two resizable panes. Everything about the rig as you're posing it -
	// the part list, the selected part's sheet/prop link, its selected
	// keyframe, and which sheet each prop previews as - is one scrolling
	// pane, so editing a part doesn't mean hunting across four small
	// splits. The props schema, edited far less often, sits below it.
	pp.rigBox = container.NewVBox(
		newSectionHeader("PARTS"),
		container.NewGridWithColumns(2, importBtn, importAnimBtn),
		addPartBtn,
		pp.partListBox,
		pp.partLinkBox,
		widget.NewSeparator(),
		newSectionHeader("SELECTED KEYFRAME"),
		pp.keyframeBox,
		widget.NewSeparator(),
		newSectionHeader("PREVIEW OVERRIDES"),
		pp.previewBox,
	)
	pp.rigScroll = container.NewVScroll(pp.rigBox)

	propsHint := widget.NewLabel("A prop is a swappable art slot (e.g. hair). Its value is a sheet; " +
		"parts linked to it draw from that sheet. Tick \"Swappable art\" when importing to make one; pick its default below.")
	propsHint.Wrapping = fyne.TextWrapWord
	propsArea := container.NewVScroll(container.NewVBox(
		container.NewHBox(newSectionHeader("PROPS (schema)"), addPropBtn), propsHint, pp.schemaBox,
	))

	full := container.NewVSplit(pp.rigScroll, propsArea)
	full.SetOffset(0.75)

	return container.NewBorder(directionBar, nil, nil, nil, full)
}

// BuildPalette assembles the left column: GraalShop's "Sprite Book" — a
// picker for which loaded sheet to show, and that sheet's cells as a
// draggable grid. It is deliberately independent of the part selection:
// dragging a tile creates a *new* part, so the palette has to work before
// any part exists, and it names its own sheet so a drop knows which sheet
// the cell came from.
func (pp *PropertiesPanel) BuildPalette() fyne.CanvasObject {
	pp.ensureSheetGrid()
	pp.paletteLabel = widget.NewLabel("")
	// Assigned after the options/selection are seeded below, per the Fyne
	// re-fire hazard noted on refreshPartList.
	pp.paletteSelect = widget.NewSelect(nil, nil)
	pp.paletteSelect.PlaceHolder = "(no sheet imported)"
	pp.refreshPalette()
	pp.paletteSelect.OnChanged = func(name string) {
		pp.project.PaletteSheet = name
		pp.refreshSheetGrid()
	}

	hint := widget.NewLabel("Drag a tile onto the canvas: with a part selected it keys that part " +
		"at the playhead; with nothing selected (Esc) it adds a new part. " +
		"Click a tile to make it the selected part's frame at the playhead. " +
		"Parts snap to whole pixels; hold Alt while dropping or dragging to place between them.")
	hint.Wrapping = fyne.TextWrapWord

	// Removes the sheet on show from the track - e.g. one imported by
	// mistake, which otherwise stays recorded in the .anif for good.
	removeBtn := widget.NewButton("Remove", func() {
		if pp.OnRemoveSheet != nil && pp.project.PaletteSheet != "" {
			pp.OnRemoveSheet(pp.project.PaletteSheet)
		}
	})
	removeBtn.Importance = widget.DangerImportance

	header := container.NewVBox(
		container.NewBorder(nil, nil, nil, removeBtn, pp.paletteSelect),
		pp.paletteLabel,
	)
	return container.NewBorder(header, hint, nil, nil, container.NewScroll(pp.sheetGrid))
}

// refreshPalette re-seeds the sheet picker from what's loaded, keeping the
// current choice when it still exists and falling back to the first sheet
// so the palette is never blank while a sheet is available.
func (pp *PropertiesPanel) refreshPalette() {
	if pp.paletteSelect == nil {
		return
	}
	names := pp.project.LoadedSheetNames()
	pp.paletteSelect.Options = names
	if pp.project.PaletteSheet == "" && len(names) > 0 {
		pp.project.PaletteSheet = names[0]
	}
	// Set directly, not with SetSelected, which would re-fire OnChanged.
	// Covers the palette's sheet being removed, too: Selected must not keep
	// naming a sheet that's no longer an option.
	if pp.paletteSelect.Selected != pp.project.PaletteSheet {
		pp.paletteSelect.Selected = pp.project.PaletteSheet
	}
	pp.paletteSelect.Refresh()
	pp.refreshSheetGrid()
}

func (pp *PropertiesPanel) ensureSheetGrid() {
	if pp.sheetGrid != nil {
		return
	}
	pp.sheetGrid = NewSheetGridWidget()
	pp.sheetGrid.OnDragStart = func() {
		if pp.OnTileDragStart != nil {
			pp.OnTileDragStart()
		}
	}
	pp.sheetGrid.OnDragMove = func(row, col int, absPos fyne.Position) {
		if pp.OnTileDragMove != nil {
			pp.OnTileDragMove(pp.project.PaletteSheet, row, col, absPos)
		}
	}
	pp.sheetGrid.OnTileDropped = func(row, col int, absPos fyne.Position) {
		if pp.OnTileDropped != nil {
			pp.OnTileDropped(pp.project.PaletteSheet, row, col, absPos)
		}
	}
	pp.sheetGrid.OnTileTapped = func(row, col int) {
		if pp.OnTileTapped != nil {
			pp.OnTileTapped(row, col)
		}
	}
}

// refreshPaletteLabel describes the sheet on show. Getting Cell
// Width/Height wrong at import is the easiest mistake to make here and the
// resulting slicing is otherwise only visible by eye, so the grid is
// spelled out.
func (pp *PropertiesPanel) refreshPaletteLabel() {
	if pp.paletteLabel == nil {
		return
	}
	sheet := pp.project.PaletteSheetTemplate()
	switch {
	case len(pp.project.LoadedSheets) == 0:
		pp.paletteLabel.SetText("Import a sprite sheet to begin")
	case sheet == nil:
		pp.paletteLabel.SetText("Pick a sheet above")
	default:
		pp.paletteLabel.SetText(fmt.Sprintf("%dx%d cells of %dx%dpx",
			sheet.Cols(), sheet.Rows(), sheet.CellW, sheet.CellH))
	}
}

// Refresh rebuilds every section from current project state.
func (pp *PropertiesPanel) Refresh() {
	pp.refreshPartList()
	pp.refreshPalette()
	pp.refreshDependentSections()
}

// refreshDependentSections rebuilds everything that depends on which part
// is selected, but not the part list itself. It exists as a separate step
// because of a Fyne trap: Select.SetSelected/ClearSelected re-fire the
// Select's own OnChanged, so a handler that rebuilt the widget it was
// called from recursed forever — an actual startup stack-overflow back
// when the part list was a Select. The part list is now plain buttons, but
// the split is kept for any Select-driven section that refreshes from its
// own handler.
func (pp *PropertiesPanel) refreshDependentSections() {
	pp.refreshPartLink()
	pp.refreshSheetGrid()
	pp.refreshKeyframe()
	pp.refreshSchema()
	pp.refreshPreview()
}

// relayout re-lays out the shared rig column after one of its sections
// was rebuilt. A section's own Refresh only re-lays out *its* children at
// its old size; the column around it never hears that the section grew or
// shrank, so the sections below kept their old positions and drew on top
// of each other. Refreshing the column positions the sections again, and
// refreshing the scroll resizes the column if its total height changed.
func (pp *PropertiesPanel) relayout() {
	if pp.rigBox == nil {
		return
	}
	pp.rigBox.Refresh()
	pp.rigScroll.Refresh()
}

// -- Part list + prop linking --

// refreshPartList rebuilds the part list as plain buttons+delete rows
// rather than a Select widget — a Select's SetSelected/ClearSelected
// re-fire its own OnChanged even when nothing actually changed, which is
// exactly what caused a real stack-overflow crash here before (see the
// note in map/objects/animaker.md); a list of buttons has no such
// self-triggering hazard.
func (pp *PropertiesPanel) refreshPartList() {
	defer pp.relayout()
	if pp.partListBox == nil {
		return
	}
	pp.partListBox.RemoveAll()

	track := pp.project.CurrentTrack
	dir := pp.project.ActiveDirection()
	if track == nil {
		pp.partListBox.Refresh()
		return
	}
	sel := pp.project.Selection
	// The part list comes from the Track, so it's identical in every
	// direction; only the "posed here" marker below varies by facing.
	for i, part := range track.Parts {
		idx := i
		kindTag := "sheet"
		if part.Kind == editor.PartKindNestedAni {
			kindTag = "nested"
		}
		label := fmt.Sprintf("%s [%s]", part.Name, kindTag)
		if dir != nil && len(dir.KeyframesFor(part.ID)) == 0 {
			label += "  (no keyframes here)"
		}
		btn := widget.NewButton(label, func() {
			pp.selectPart(idx)
		})
		if sel != nil && sel.PartIndex == idx {
			btn.Importance = widget.HighImportance
		}
		delBtn := widget.NewButton("Delete", func() {
			if pp.OnPartDelete != nil {
				pp.OnPartDelete(idx)
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
	// Follow the palette to this part's sheet. The palette doesn't depend
	// on the selection any more, but bringing up the sheet a part draws
	// from is what you almost always want next after clicking it.
	if part := pp.project.SelectedPart(); part != nil && part.Kind == editor.PartKindSheet {
		if name := pp.project.ResolveActiveSheetName(part); name != "" {
			if _, ok := pp.project.LoadedSheets[name]; ok {
				pp.project.PaletteSheet = name
				pp.refreshPalette()
			}
		}
	}
	pp.refreshPartList()
	pp.refreshDependentSections()
	if pp.OnPartChanged != nil {
		pp.OnPartChanged()
	}
}

func (pp *PropertiesPanel) refreshPartLink() {
	defer pp.relayout()
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
		pp.buildNestedLink(part)
		pp.partLinkBox.Refresh()
		return
	}

	// Only sheet-valued props: an .anif prop can't give a sheet part a cell.
	propOptions := []string{"(none - fixed sheet)"}
	for _, pd := range pp.project.CurrentTrack.Props {
		if !pd.IsAnimProp() {
			propOptions = append(propOptions, pd.Name)
		}
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
		pp.project.RecordUndo()
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

// buildNestedLink is the nested-part counterpart of the sheet link: which
// animation-valued prop (if any) chooses the animation, else which loaded
// animation it plays. Changing either is undoable.
func (pp *PropertiesPanel) buildNestedLink(part *editor.Part) {
	const none = "(none - fixed animation)"
	propOptions := []string{none}
	for _, pd := range pp.project.CurrentTrack.Props {
		if pd.IsAnimProp() {
			propOptions = append(propOptions, pd.Name)
		}
	}
	propSelect := widget.NewSelect(propOptions, nil)
	if part.GoverningProp == "" {
		propSelect.SetSelected(none)
	} else {
		propSelect.SetSelected(part.GoverningProp)
	}

	paths := pp.project.LoadedAnimPaths()
	if part.NestedAniPath != "" && pp.project.LoadedAnims[editor.AnimKey(part.NestedAniPath)] == nil {
		paths = append([]string{part.NestedAniPath}, paths...) // keep showing an unloaded binding
	}
	labels := AnimLabels(paths)
	animSelect := widget.NewSelect(labels, nil)
	animSelect.PlaceHolder = "(no animation)"
	if part.NestedAniPath != "" {
		animSelect.SetSelected(labelFor(labels, paths, part.NestedAniPath))
	}
	if part.GoverningProp != "" {
		animSelect.Disable()
	}

	changed := func() {
		pp.project.Dirty = true
		if pp.OnPartChanged != nil {
			pp.OnPartChanged()
		}
	}
	// Assigned after the SetSelected calls above, which would re-fire them.
	propSelect.OnChanged = func(v string) {
		pp.project.RecordUndo()
		if v == none {
			part.GoverningProp = ""
			animSelect.Enable()
		} else {
			part.GoverningProp = v
			animSelect.Disable()
		}
		changed()
	}
	animSelect.OnChanged = func(label string) {
		pp.project.RecordUndo()
		part.NestedAniPath = valueFor(labels, paths, label)
		changed()
	}

	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Prop:"), nil, propSelect))
	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Animation:"), nil, animSelect))
	pp.buildNestedDirection(part)
	pp.buildNestedBindings(part)
	if len(pp.project.LoadedAnims) == 0 {
		pp.partLinkBox.Add(widget.NewLabel("No animations imported yet - use Import Animation."))
	}
}

// nestedDirModeLabels are the Direction picker's choices, indexed by
// editor.NestedDirMode.
var nestedDirModeLabels = []string{"Inherit from parent", "Static", "Per keyframe"}

// buildNestedDirection is how a nested part picks which direction of its
// animation plays: turn with the parent, one fixed direction, or set on
// each keyframe (edited in the Selected Keyframe section). Switching mode
// keeps showing the same direction (Project.SetNestedDirectionMode).
func (pp *PropertiesPanel) buildNestedDirection(part *editor.Part) {
	modeSelect := widget.NewSelect(nestedDirModeLabels, nil)
	modeSelect.SetSelected(nestedDirModeLabels[part.DirectionMode])

	keys := pp.nestedDirectionKeys(part)
	labels := directionLabels(keys)
	staticSelect := widget.NewSelect(labels, nil)
	staticSelect.SetSelected(directionLabel(part.StaticDirection))

	// Assigned after the SetSelected calls above, which would re-fire them.
	modeSelect.OnChanged = func(label string) {
		for i, l := range nestedDirModeLabels {
			if l == label && editor.NestedDirMode(i) != part.DirectionMode {
				pp.project.RecordUndo()
				pp.project.SetNestedDirectionMode(part, editor.NestedDirMode(i))
				pp.notifyPartChanged()
				// The static picker and the keyframe section depend on it.
				pp.refreshPartLink()
				pp.refreshKeyframe()
			}
		}
	}
	staticSelect.OnChanged = func(label string) {
		if k, ok := directionKeyFor(keys, labels, label); ok && k != part.StaticDirection {
			pp.project.RecordUndo()
			part.StaticDirection = k
			pp.notifyPartChanged()
		}
	}

	pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Direction:"), nil, modeSelect))
	if part.DirectionMode == editor.NestedDirStatic {
		pp.partLinkBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Plays:"), nil, staticSelect))
	}
}

// nestedDirectionKeys lists the directions the part's nested animation
// has, or the four standard facings if it isn't loaded.
func (pp *PropertiesPanel) nestedDirectionKeys(part *editor.Part) []int {
	if anim := pp.project.ResolveNestedAnim(part); anim != nil {
		if keys := anim.Track.SortedDirectionKeys(); len(keys) > 0 {
			return keys
		}
	}
	return editor.StandardDirectionKeys
}

// directionLabel names a direction key the way the game numbers them.
func directionLabel(k int) string {
	switch k {
	case 0:
		return "0 (up)"
	case 1:
		return "1 (right)"
	case 2:
		return "2 (down)"
	case 3:
		return "3 (left)"
	}
	return strconv.Itoa(k)
}

func directionLabels(keys []int) []string {
	labels := make([]string, len(keys))
	for i, k := range keys {
		labels[i] = directionLabel(k)
	}
	return labels
}

func directionKeyFor(keys []int, labels []string, label string) (int, bool) {
	for i, l := range labels {
		if l == label {
			return keys[i], true
		}
	}
	return 0, false
}

func (pp *PropertiesPanel) notifyPartChanged() {
	pp.project.Dirty = true
	if pp.OnPartChanged != nil {
		pp.OnPartChanged()
	}
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

// refreshSheetGrid shows whichever sheet the palette picker names. It is
// no longer derived from the selected part: a drag creates a new part, so
// the palette must work with nothing selected.
func (pp *PropertiesPanel) refreshSheetGrid() {
	if pp.sheetGrid == nil {
		return
	}
	pp.sheetGrid.SetSheet(pp.project.PaletteSheetTemplate())
	pp.refreshPaletteLabel()
}

// -- Selected keyframe --

func (pp *PropertiesPanel) refreshKeyframe() {
	defer pp.relayout()
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
	dir := pp.project.ActiveDirection()
	if dir == nil {
		pp.keyframeBox.Refresh()
		return
	}

	// The position fields are always shown for a selected part - requested,
	// since they used to appear only once a keyframe was selected, and
	// clicking a part never selected one. With no keyframe selected they
	// edit the part's keyframe at the playhead, adding one on the first
	// edit if there isn't one there (as a canvas drag does), seeded from
	// the pose the part is showing.
	kf := pp.project.SelectedKeyframe()
	playhead := pp.project.Playback.ElapsedMs
	if kf == nil {
		for _, existing := range dir.KeyframesFor(part.ID) {
			if existing.TimeMs == playhead {
				kf = existing // edited in place; the selection is left alone
			}
		}
	}
	pose := dir.ValueAt(part.ID, playhead)
	if kf != nil {
		pose = editor.ResolvedTransform{X: kf.X, Y: kf.Y, Z: kf.Z, RotationDeg: kf.RotationDeg,
			Row: kf.Row, Col: kf.Col, Direction: kf.Direction}
	}
	// target is the keyframe an edit applies to, created on the first edit
	// when there isn't one at the playhead.
	target := func() *editor.Keyframe {
		if kf == nil {
			kf, _ = editor.EnsureKeyframe(dir, part.ID, playhead)
			pp.project.Selection.KeyframeIndex = kf.Index
		}
		return kf
	}
	// beginEdit records the undo step for one edit - a button click, a
	// picker change, or one burst of typing in a field - before it changes
	// anything, creating the keyframe it applies to if need be.
	beginEdit := func() *editor.Keyframe {
		pp.project.RecordUndo()
		return target()
	}
	field := func(v float32, set func(kf *editor.Keyframe, v float32)) *selectAllEntry {
		var e *selectAllEntry
		e = numEntry(fmt.Sprintf("%v", v), func(v float32) {
			if e.startEdit() {
				beginEdit()
			}
			set(target(), v)
			pp.notifyKeyframeChanged()
		})
		return e
	}

	xEntry := field(pose.X, func(kf *editor.Keyframe, v float32) { kf.X = v })
	yEntry := field(pose.Y, func(kf *editor.Keyframe, v float32) { kf.Y = v })
	zEntry := field(pose.Z, func(kf *editor.Keyframe, v float32) { kf.Z = v })
	rotEntry := field(pose.RotationDeg, func(kf *editor.Keyframe, v float32) { kf.RotationDeg = v })

	grid := container.NewGridWithColumns(2,
		widget.NewLabel("X"), xEntry,
		widget.NewLabel("Y"), yEntry,
		widget.NewLabel("Z"), zEntry,
		// The canvas can't draw rotation (see canvas.go), so say so rather
		// than leave a field that seems to do nothing.
		widget.NewLabel("Rotation (not previewed)"), rotEntry,
	)
	if kf != nil {
		pp.keyframeBox.Add(widget.NewLabel(fmt.Sprintf("%s @ %dms  (row %d, col %d)", part.Name, kf.TimeMs, kf.Row, kf.Col)))
		pp.keyframeBox.Add(pp.buildTimeEntry(part, kf))
	} else {
		note := widget.NewLabel(fmt.Sprintf("%s @ %dms - not keyed here yet; editing adds a keyframe at the playhead.",
			part.Name, playhead))
		note.Wrapping = fyne.TextWrapWord
		pp.keyframeBox.Add(note)
	}
	pp.keyframeBox.Add(grid)
	if part.Kind == editor.PartKindNestedAni && part.DirectionMode == editor.NestedDirPerKeyframe {
		keys := pp.nestedDirectionKeys(part)
		labels := directionLabels(keys)
		dirSelect := widget.NewSelect(labels, nil)
		dirSelect.SetSelected(directionLabel(pose.Direction))
		// Assigned after SetSelected, which would re-fire it.
		dirSelect.OnChanged = func(label string) {
			if k, ok := directionKeyFor(keys, labels, label); ok {
				beginEdit().Direction = k
				pp.notifyKeyframeChanged()
			}
		}
		pp.keyframeBox.Add(container.NewBorder(nil, nil, widget.NewLabel("Direction"), nil, dirSelect))
	}
	pp.keyframeBox.Add(pp.buildNudgeControls(beginEdit))

	pp.keyframeBox.Refresh()
}

// buildTimeEntry is an exact alternative to dragging a keyframe's marker
// along the timeline, which lands wherever the mouse happens to stop
// (592ms when you meant 600). Unlike the X/Y entries, it applies on Enter
// or "Set" rather than per keystroke: retiming re-sorts the keyframes, and
// typing "600" would otherwise pass through 6ms and 60ms on the way,
// colliding with whatever keyframes sit there. It ignores "Lock timing",
// which guards against accidental drags, not deliberate typing.
func (pp *PropertiesPanel) buildTimeEntry(part *editor.Part, kf *editor.Keyframe) fyne.CanvasObject {
	entry := newEntry()
	entry.SetText(strconv.FormatUint(uint64(kf.TimeMs), 10))
	status := widget.NewLabel("")

	apply := func() {
		dir := pp.project.ActiveDirection()
		if dir == nil {
			return
		}
		v, err := strconv.ParseUint(strings.TrimSpace(entry.Text), 10, 32)
		if err != nil {
			status.SetText("Enter a whole number of ms")
			return
		}
		newMs := uint32(v)
		if newMs == kf.TimeMs {
			status.SetText("")
			return
		}
		for _, other := range dir.KeyframesFor(part.ID) {
			if other != kf && other.TimeMs == newMs {
				status.SetText(fmt.Sprintf("%s already has a keyframe at %dms", part.Name, newMs))
				return
			}
		}
		pp.project.RecordUndo()
		if err := editor.MoveKeyframe(dir, part.ID, kf.Index, newMs); err != nil {
			status.SetText(err.Error())
			return
		}
		// Moving re-sorts, so re-read the index; and keep the playhead on
		// the keyframe so the canvas still shows the pose being edited.
		pp.project.Selection.KeyframeIndex = kf.Index
		pp.project.Seek(kf.TimeMs)
		pp.project.Dirty = true
		if pp.OnKeyframeRetimed != nil {
			pp.OnKeyframeRetimed()
		}
	}
	entry.OnSubmitted = func(string) { apply() }
	setBtn := widget.NewButton("Set", apply)

	return container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Time (ms)"), setBtn, entry),
		status,
	)
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
//
// beginEdit records an undo step and returns the keyframe to nudge,
// creating it at the playhead on first use (see refreshKeyframe). Each
// click is its own undo step.
func (pp *PropertiesPanel) buildNudgeControls(beginEdit func() *editor.Keyframe) fyne.CanvasObject {
	nudge := func(apply func(kf *editor.Keyframe)) func() {
		return func() {
			apply(beginEdit())
			pp.notifyKeyframeChanged()
			pp.refreshKeyframe()
		}
	}
	// To Front / To Back put the part in front of / behind every other
	// part posed at the playhead (Ctrl+Shift+] / [ do the same).
	zOrder := func(label string, front bool) *widget.Button {
		return widget.NewButton(label, func() {
			if z, ok := pp.project.ZOrderTarget(front); ok {
				nudge(func(kf *editor.Keyframe) { kf.Z = z })()
			}
		})
	}

	xyPad := container.NewGridWithColumns(3,
		layout.NewSpacer(),
		widget.NewButton("Y-", nudge(func(kf *editor.Keyframe) { kf.Y -= nudgeStepXY })),
		layout.NewSpacer(),
		widget.NewButton("X-", nudge(func(kf *editor.Keyframe) { kf.X -= nudgeStepXY })),
		widget.NewLabel("pos"),
		widget.NewButton("X+", nudge(func(kf *editor.Keyframe) { kf.X += nudgeStepXY })),
		layout.NewSpacer(),
		widget.NewButton("Y+", nudge(func(kf *editor.Keyframe) { kf.Y += nudgeStepXY })),
		layout.NewSpacer(),
	)

	zRow := container.NewHBox(
		widget.NewLabel("Z:"),
		widget.NewButton("Back -", nudge(func(kf *editor.Keyframe) { kf.Z -= nudgeStepZ })),
		widget.NewButton("Fwd +", nudge(func(kf *editor.Keyframe) { kf.Z += nudgeStepZ })),
	)
	zOrderRow := container.NewHBox(
		widget.NewLabel("Draw order:"),
		zOrder("To Back", false),
		zOrder("To Front", true),
	)
	rotRow := container.NewHBox(
		widget.NewLabel("Rot:"),
		widget.NewButton("-", nudge(func(kf *editor.Keyframe) { kf.RotationDeg -= nudgeStepRot })),
		widget.NewButton("+", nudge(func(kf *editor.Keyframe) { kf.RotationDeg += nudgeStepRot })),
	)

	return container.NewVBox(xyPad, zRow, zOrderRow, rotRow)
}

func (pp *PropertiesPanel) notifyKeyframeChanged() {
	pp.project.Dirty = true
	if pp.OnKeyframeChanged != nil {
		pp.OnKeyframeChanged()
	}
}

// Binding modes, as the bindings editor labels them.
const (
	bindDefault     = "Default"
	bindPassthrough = "From parent prop"
	bindStatic      = "Fixed"
)

// buildNestedBindings sets, for each prop the nested animation declares,
// where its value comes from: the nested track's own default, one of this
// (the parent) track's props passed through, or a fixed value. E.g. a city
// guard pins its torch's torch_base to "torchbase_metal", while the
// player's walk declares its own torch_base prop and passes it through.
//
// Every choice is a pick-list - the nested track's props, the parent's
// props, loaded sheets or animations - because a typed name that matches
// nothing silently falls back to the default.
func (pp *PropertiesPanel) buildNestedBindings(part *editor.Part) {
	anim := pp.project.ResolveNestedAnim(part)
	if anim == nil {
		// Can't list the nested track's props; keep showing what's bound
		// rather than hiding or dropping it.
		if len(part.NestedBindings) > 0 {
			pp.partLinkBox.Add(newSectionHeader("Bindings"))
			for _, name := range sortedBindingNames(part) {
				b := part.NestedBindings[name]
				desc := "fixed: " + b.StaticValue
				if b.PassthroughFrom != "" {
					desc = "from parent prop " + b.PassthroughFrom
				}
				pp.partLinkBox.Add(widget.NewLabel(name + " - " + desc + " (animation not loaded)"))
			}
		}
		return
	}

	declared := map[string]bool{}
	var rows []fyne.CanvasObject
	for _, pd := range anim.Track.Props {
		declared[pd.Name] = true
		rows = append(rows, pp.bindingRow(part, anim, pd))
	}
	// Bindings for props the nested track doesn't declare (renamed or
	// removed there since) do nothing; show them so they can be cleared.
	for _, name := range sortedBindingNames(part) {
		if declared[name] {
			continue
		}
		name := name
		clear := widget.NewButton("Remove", func() {
			pp.project.RecordUndo()
			delete(part.NestedBindings, name)
			pp.notifyPartChanged()
			pp.refreshPartLink()
		})
		rows = append(rows, container.NewBorder(nil, nil, nil, clear,
			widget.NewLabel(fmt.Sprintf("%s - not a prop of %s", name, anim.DisplayName()))))
	}
	if len(rows) == 0 {
		return
	}
	pp.partLinkBox.Add(newSectionHeader("Bindings (" + anim.DisplayName() + " props)"))
	for _, r := range rows {
		pp.partLinkBox.Add(r)
	}
}

func sortedBindingNames(part *editor.Part) []string {
	names := make([]string, 0, len(part.NestedBindings))
	for n := range part.NestedBindings {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// bindingRow is one nested prop's binding: a mode picker and, unless it's
// left at the default, a value picker.
func (pp *PropertiesPanel) bindingRow(part *editor.Part, anim *editor.NestedAnim, pd editor.PropDef) fyne.CanvasObject {
	b, bound := part.NestedBindings[pd.Name]
	mode := bindDefault
	switch {
	case bound && b.PassthroughFrom != "":
		mode = bindPassthrough
	case bound && b.StaticValue != "":
		mode = bindStatic
	}

	defLabel := pd.Default
	if pd.IsAnimProp() {
		defLabel = filepath.Base(defLabel)
	}
	modeSelect := widget.NewSelect([]string{bindDefault, bindPassthrough, bindStatic}, nil)
	modeSelect.SetSelected(mode)

	// The value picker's options depend on the mode: the parent's props of
	// the same kind (sheet or animation), or the values the prop can take.
	var values, labels []string
	current := ""
	switch mode {
	case bindPassthrough:
		for _, parent := range pp.project.CurrentTrack.Props {
			if parent.IsAnimProp() == pd.IsAnimProp() {
				values = append(values, parent.Name)
			}
		}
		current = b.PassthroughFrom
		values = sheetPickerOptions(values, current)
		labels = values
	case bindStatic:
		values, labels = pp.bindingValueOptions(anim, pd)
		current = b.StaticValue
		if current != "" && !contains(values, current) { // keep showing a value that isn't loaded
			values = append([]string{current}, values...)
			labels = append([]string{current}, labels...)
		}
	}
	valueSelect := widget.NewSelect(labels, nil)
	if current != "" {
		valueSelect.SetSelected(labelFor(labels, values, current))
	}

	set := func(nb editor.PropBinding, keep bool) {
		pp.project.RecordUndo()
		if !keep {
			delete(part.NestedBindings, pd.Name)
		} else {
			if part.NestedBindings == nil {
				part.NestedBindings = map[string]editor.PropBinding{}
			}
			part.NestedBindings[pd.Name] = nb
		}
		pp.notifyPartChanged()
	}
	// Assigned after the SetSelected calls above, which would re-fire them.
	modeSelect.OnChanged = func(m string) {
		if m == mode {
			return
		}
		switch m {
		case bindDefault:
			set(editor.PropBinding{}, false)
		case bindPassthrough:
			// Start from a parent prop of the same name if there is one -
			// the usual case, e.g. torch_base passed through as torch_base.
			from := ""
			if parent := pp.project.CurrentTrack.FindProp(pd.Name); parent != nil && parent.IsAnimProp() == pd.IsAnimProp() {
				from = parent.Name
			}
			set(editor.PropBinding{PassthroughFrom: from}, true)
		case bindStatic:
			set(editor.PropBinding{StaticValue: pd.Default}, true)
		}
		pp.refreshPartLink()
	}
	valueSelect.OnChanged = func(label string) {
		v := valueFor(labels, values, label)
		if mode == bindPassthrough {
			set(editor.PropBinding{PassthroughFrom: v}, true)
		} else {
			set(editor.PropBinding{StaticValue: v}, true)
		}
	}

	row := container.NewGridWithColumns(2, modeSelect)
	if mode == bindDefault {
		row.Add(widget.NewLabel("(" + defLabel + ")"))
	} else {
		if len(values) == 0 {
			valueSelect.PlaceHolder = "(no parent props of this kind)"
		}
		row.Add(valueSelect)
	}
	return container.NewBorder(nil, nil, widget.NewLabel(pd.Name+":"), nil, row)
}

// bindingValueOptions lists the fixed values a nested prop can be pinned
// to: for a sheet prop, the nested animation's own sheets and the parent's
// loaded sheets (the order a nested sheet name is looked up in); for an
// animation prop, the loaded animations.
func (pp *PropertiesPanel) bindingValueOptions(anim *editor.NestedAnim, pd editor.PropDef) (values, labels []string) {
	if pd.IsAnimProp() {
		values = pp.project.LoadedAnimPaths()
		return values, AnimLabels(values)
	}
	seen := map[string]bool{}
	for n := range anim.Sheets {
		seen[n] = true
	}
	for _, n := range pp.project.LoadedSheetNames() {
		seen[n] = true
	}
	for n := range seen {
		values = append(values, n)
	}
	sort.Strings(values)
	return values, values
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// -- Props schema --

func (pp *PropertiesPanel) refreshSchema() {
	if pp.schemaBox == nil {
		return
	}
	pp.schemaBox.RemoveAll()
	for i, prop := range pp.project.CurrentTrack.Props {
		idx := i
		name := prop.Name
		// The default is a pick-list of values of the prop's own kind, so it
		// can be changed after the prop is made (Project.SetPropDefault).
		// Preview-only sheets aren't offered: they're never saved.
		values := sheetPickerOptions(pp.project.LoadedSheetNames(), prop.Default)
		labels := values
		if prop.IsAnimProp() {
			values = sheetPickerOptions(pp.project.LoadedAnimPaths(), prop.Default)
			labels = AnimLabels(values)
		}
		defSelect := widget.NewSelect(labels, nil)
		defSelect.SetSelected(labelFor(labels, values, prop.Default))
		// Assigned after SetSelected, which would re-fire it.
		defSelect.OnChanged = func(label string) {
			changed, err := pp.project.SetPropDefault(name, valueFor(labels, values, label))
			if err != nil && pp.OnError != nil {
				pp.OnError(err)
			}
			if (changed || err != nil) && pp.OnPropsChanged != nil {
				pp.OnPropsChanged() // on an error too, to put the picker back
			}
		}
		label := widget.NewLabel(name + ":")
		delBtn := widget.NewButton("x", func() {
			if err := pp.project.RemoveProp(idx); err != nil {
				if pp.OnError != nil {
					pp.OnError(err)
				}
				return
			}
			if pp.OnPropsChanged != nil {
				pp.OnPropsChanged()
			}
		})
		delBtn.Importance = widget.DangerImportance
		pp.schemaBox.Add(container.NewBorder(nil, nil, label, delBtn, defSelect))
	}
	pp.schemaBox.Refresh()
}

// -- Preview overrides --

func (pp *PropertiesPanel) refreshPreview() {
	defer pp.relayout()
	if pp.previewBox == nil {
		return
	}
	pp.previewBox.RemoveAll()
	if len(pp.project.CurrentTrack.Props) == 0 {
		hint := widget.NewLabel("No props declared. Only a part linked to a prop can preview other art.")
		hint.Wrapping = fyne.TextWrapWord
		pp.previewBox.Add(hint)
	}
	for _, prop := range pp.project.CurrentTrack.Props {
		name := prop.Name
		current := prop.Default
		if v, ok := pp.project.PreviewProps[name]; ok && v != "" {
			current = v
		}
		// A pick-list, for the same reason as the part's sheet picker: a
		// value naming nothing loaded makes linked parts draw nothing. An
		// animation prop lists loaded animations instead of sheets.
		values := sheetPickerOptions(pp.project.PreviewOptionNames(), current)
		labels := values
		if prop.IsAnimProp() {
			values = sheetPickerOptions(pp.project.LoadedAnimPaths(), current)
			labels = AnimLabels(values)
		}
		sel := widget.NewSelect(labels, nil)
		if current != "" {
			sel.SetSelected(labelFor(labels, values, current))
		}
		// Assigned after SetSelected so seeding doesn't fire it.
		sel.OnChanged = func(label string) {
			pp.project.PreviewProps[name] = valueFor(labels, values, label)
			pp.refreshSheetGrid()
			if pp.OnPartChanged != nil {
				pp.OnPartChanged()
			}
		}
		// Loads any image, sliced on this prop's grid, as a preview-only
		// option — not saved, not part of the .anif.
		loadBtn := widget.NewButton("Load file...", func() {
			if pp.OnLoadPreviewSheet != nil {
				pp.OnLoadPreviewSheet(name)
			}
		})
		pp.previewBox.Add(container.NewBorder(nil, nil, widget.NewLabel(name+":"), loadBtn, sel))
	}
	pp.previewBox.Refresh()
}

// -- Helpers --

// numEntry is a field that calls onChange with every value typed into it
// that parses as a number.
func numEntry(initial string, onChange func(float32)) *selectAllEntry {
	e := newEntry()
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
