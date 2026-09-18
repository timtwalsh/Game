package app

import (
	"animaker/pkg/editor"
	"animaker/pkg/file"
	"animaker/pkg/ui"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// Application is the main coordinator tying together project state, UI, and file operations.
type Application struct {
	FyneApp fyne.App
	Window  fyne.Window
	Project *editor.Project

	canvasWidget *ui.CanvasWidget
	timeline     *ui.TimelineWidget
	properties   *ui.PropertiesPanel
	directionSel *widget.Select
	titleLabel   *widget.Label

	playbackTicker *time.Ticker
	playbackDone   chan bool
}

// New creates a new Application instance.
func New(fyneApp fyne.App) *Application {
	return &Application{
		FyneApp:      fyneApp,
		Project:      editor.NewProject("untitled"),
		playbackDone: make(chan bool),
	}
}

// Run initializes the UI and starts the application.
func (a *Application) Run() {
	a.Window = a.FyneApp.NewWindow("ANIFile Animation Maker")
	a.Window.Resize(fyne.NewSize(1300, 850))

	a.canvasWidget = ui.NewCanvasWidget(a.Project)
	a.timeline = ui.NewTimelineWidget(a.Project)
	a.properties = ui.NewPropertiesPanel(a.Project)

	a.wireCallbacks()

	directionBar := a.buildDirectionBar()
	canvasScroll := container.NewScroll(a.canvasWidget)
	palettePanel := a.properties.BuildPalette()
	propertiesPanel := a.properties.Build(directionBar)
	timelinePanel := a.timeline.Build()
	mainLayout := ui.BuildMainLayout(palettePanel, canvasScroll, propertiesPanel, timelinePanel)

	menu := ui.BuildMenuBar(
		a.onNewTrack,
		a.onOpenTrack,
		a.onSaveTrack,
		a.onSaveAsTrack,
		a.onImportSpriteSheet,
		a.onUndo,
		a.onRedo,
		a.canvasWidget.ToggleGrid,
		a.onZoom,
	)
	a.Window.SetMainMenu(menu)

	a.registerShortcuts()

	a.Window.SetContent(mainLayout)
	a.Window.SetOnClosed(a.onClose)

	go a.playbackLoop()

	a.Window.ShowAndRun()
}

func (a *Application) buildDirectionBar() fyne.CanvasObject {
	a.titleLabel = widget.NewLabel(a.Project.CurrentTrack.Metadata.Name)
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	a.directionSel = widget.NewSelect(nil, func(s string) {
		key, err := strconv.Atoi(s)
		if err != nil {
			return
		}
		a.Project.SetActiveDirection(key)
		a.refreshAll()
	})
	a.refreshDirectionSelect()

	addDirBtn := widget.NewButton("+ Add Direction", a.onAddDirection)

	return container.NewHBox(a.titleLabel, widget.NewSeparator(), widget.NewLabel("Direction:"), a.directionSel, addDirBtn)
}

func (a *Application) refreshDirectionSelect() {
	keys := a.Project.CurrentTrack.SortedDirectionKeys()
	options := make([]string, len(keys))
	for i, k := range keys {
		options[i] = strconv.Itoa(k)
	}
	a.directionSel.Options = options
	if len(options) > 0 {
		a.directionSel.SetSelected(strconv.Itoa(a.Project.Playback.ActiveDirection))
	}
	a.directionSel.Refresh()
}

func (a *Application) wireCallbacks() {
	// -- Canvas: click to select, drag to reposition an existing keyframe --
	a.canvasWidget.OnPartTapped = func(idx int) {
		a.properties.SelectPart(idx)
		a.refreshAll()
	}
	a.canvasWidget.OnPartDragStart = func(idx int) {
		a.Project.RecordUndo()
		a.properties.SelectPart(idx)
	}
	a.canvasWidget.OnPartDragged = func(idx int, x, y float32) {
		dir := a.Project.ActiveDirection()
		if dir == nil || idx < 0 || idx >= len(dir.Parts) {
			return
		}
		part := dir.Parts[idx]
		// Only an existing keyframe at exactly the current playhead time
		// moves - dragging doesn't implicitly create one. Use "New
		// Keyframe" first, per the intended workflow.
		for _, kf := range part.Keyframes {
			if kf.TimeMs == a.Project.Playback.ElapsedMs {
				kf.X, kf.Y = x, y
				a.Project.Dirty = true
				a.canvasWidget.Refresh()
				a.properties.Refresh()
				return
			}
		}
	}
	a.canvasWidget.OnPartDragEnd = func() {
		a.timeline.Refresh()
	}

	// -- Timeline --
	a.timeline.OnScrub = func(ms uint32) {
		a.Project.Seek(ms)
		a.refreshAll()
	}
	a.timeline.OnKeyframeSelected = func(partIdx, kfIdx int) {
		a.Project.Selection.PartIndex = partIdx
		a.Project.Selection.KeyframeIndex = kfIdx
		dir := a.Project.ActiveDirection()
		if dir != nil && partIdx >= 0 && partIdx < len(dir.Parts) {
			part := dir.Parts[partIdx]
			if kfIdx >= 0 && kfIdx < len(part.Keyframes) {
				a.Project.Seek(part.Keyframes[kfIdx].TimeMs)
			}
		}
		a.refreshAll()
	}
	a.timeline.OnKeyframeDeleted = func(partIdx, kfIdx int) {
		dir := a.Project.ActiveDirection()
		if dir == nil || partIdx < 0 || partIdx >= len(dir.Parts) {
			return
		}
		a.Project.RecordUndo()
		_ = editor.DeleteKeyframe(dir.Parts[partIdx], kfIdx)
		a.Project.Selection.KeyframeIndex = -1
		a.Project.Dirty = true
		a.refreshAll()
	}
	a.timeline.OnNewKeyframe = func() {
		dir := a.Project.ActiveDirection()
		sel := a.Project.Selection
		if dir == nil || sel == nil || sel.PartIndex < 0 || sel.PartIndex >= len(dir.Parts) {
			return
		}
		part := dir.Parts[sel.PartIndex]
		elapsed := a.Project.Playback.ElapsedMs
		// Seed the new keyframe from wherever the part's interpolated pose
		// currently is, so it starts as a continuation rather than
		// snapping to zero - the artist then drags it into place.
		seed := part.ValueAt(elapsed)
		a.Project.RecordUndo()
		kf := editor.AddKeyframe(part, elapsed)
		kf.X, kf.Y, kf.Z, kf.RotationDeg = seed.X, seed.Y, seed.Z, seed.RotationDeg
		kf.Row, kf.Col = seed.Row, seed.Col
		for i, k := range part.Keyframes {
			if k == kf {
				sel.KeyframeIndex = i
				break
			}
		}
		a.Project.Dirty = true
		a.refreshAll()
	}
	a.timeline.OnPlay = func() { a.Project.Play() }
	a.timeline.OnStop = func() {
		a.Project.Stop()
		a.refreshAll()
	}

	// -- Properties --
	a.properties.OnImport = a.onImportSpriteSheet
	a.properties.OnAddPart = a.onAddPart
	a.properties.OnAddProp = a.onAddProp
	a.properties.OnPropsChanged = func() { a.refreshAll() }
	a.properties.OnPartRemoved = func(idx int) {
		if a.Project.Selection.PartIndex == idx {
			a.Project.Selection.PartIndex = -1
			a.Project.Selection.KeyframeIndex = -1
		}
		a.refreshAll()
	}
	a.properties.OnKeyframeChanged = func() {
		a.canvasWidget.Refresh()
		a.timeline.Refresh()
	}
	a.properties.OnPartChanged = func() {
		a.canvasWidget.Refresh()
		a.timeline.Refresh()
	}
	a.properties.OnTileDropped = a.onTileDropped
}

// onTileDropped is the core "level editor" interaction: dragging a cell
// from the selected part's sheet grid onto the canvas creates or updates
// a keyframe for that part at the current playhead time, with Row/Col from
// the dragged cell and X/Y from wherever it landed on the canvas. Drops
// outside the canvas's bounds are ignored.
func (a *Application) onTileDropped(partIdx, row, col int, absPos fyne.Position) {
	dir := a.Project.ActiveDirection()
	if dir == nil || partIdx < 0 || partIdx >= len(dir.Parts) {
		return
	}

	canvasAbsPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(a.canvasWidget)
	canvasSize := a.canvasWidget.Size()
	local := fyne.NewPos(absPos.X-canvasAbsPos.X, absPos.Y-canvasAbsPos.Y)
	if local.X < 0 || local.Y < 0 || local.X > canvasSize.Width || local.Y > canvasSize.Height {
		return // dropped outside the canvas - not a placement
	}

	x, y := a.canvasWidget.LocalToAnimXY(local)

	a.Project.RecordUndo()
	part := dir.Parts[partIdx]
	kf := editor.AddKeyframe(part, a.Project.Playback.ElapsedMs)
	kf.Row = row
	kf.Col = col
	kf.X = x
	kf.Y = y

	a.Project.Selection.PartIndex = partIdx
	for i, k := range part.Keyframes {
		if k == kf {
			a.Project.Selection.KeyframeIndex = i
			break
		}
	}
	a.Project.Dirty = true
	a.refreshAll()
}

func (a *Application) registerShortcuts() {
	canvas := a.Window.Canvas()

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		a.onNewTrack()
	})
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		a.onOpenTrack()
	})
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		a.onSaveTrack()
	})
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		a.onUndo()
	})
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(_ fyne.Shortcut) {
		a.onRedo()
	})
}

// -- Menu handlers --

func (a *Application) onNewTrack() {
	ui.ShowNewTrackDialog(a.Window, func(name string) {
		a.Project = editor.NewProject(name)
		a.canvasWidget.SetProject(a.Project)
		a.timeline.SetProject(a.Project)
		a.properties.SetProject(a.Project)
		a.refreshDirectionSelect()
		a.refreshAll()
		a.Window.SetTitle("ANIFile Animation Maker — " + name)
	})
}

func (a *Application) onOpenTrack() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()

		track, err := file.LoadTrack(filePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load track: %w", err), a.Window)
			return
		}

		a.Project.CurrentTrack = track
		a.Project.SavePath = filePath
		a.Project.Dirty = false
		a.Project.UndoStack.Clear()
		keys := track.SortedDirectionKeys()
		if len(keys) > 0 {
			a.Project.Playback.ActiveDirection = keys[0]
		}
		a.Project.Playback.ElapsedMs = 0
		a.Project.Selection = &editor.Selection{PartIndex: -1, KeyframeIndex: -1}

		a.canvasWidget.SetProject(a.Project)
		a.timeline.SetProject(a.Project)
		a.properties.SetProject(a.Project)
		a.refreshDirectionSelect()
		a.refreshAll()
		a.Window.SetTitle("ANIFile Animation Maker — " + track.Metadata.Name)
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func (a *Application) onSaveTrack() {
	if a.Project.SavePath == "" {
		a.onSaveAsTrack()
		return
	}
	a.saveToPath(a.Project.SavePath)
}

func (a *Application) onSaveAsTrack() {
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		filePath := writer.URI().Path()
		writer.Close()
		a.saveToPath(filePath)
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.SetFileName(a.Project.CurrentTrack.Metadata.Name + ".anif")
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func (a *Application) saveToPath(path string) {
	a.Project.CurrentTrack.Metadata.UpdatedAt = time.Now()
	if err := file.SaveTrack(a.Project.CurrentTrack, path); err != nil {
		dialog.ShowError(fmt.Errorf("failed to save: %w", err), a.Window)
		return
	}
	a.Project.SavePath = path
	a.Project.Dirty = false
	a.Window.SetTitle("ANIFile Animation Maker — " + a.Project.CurrentTrack.Metadata.Name)
}

// onImportSpriteSheet imports a sheet AND immediately creates a Sheet part
// bound to it, then selects that part. Importing used to only populate
// Project.LoadedSheets, which left the editor looking completely unchanged:
// the palette only draws the *selected part's* sheet, and a fresh track has
// no parts, so the artist was dropped back on an empty screen with no
// discoverable way forward (the only route was "+ Add Part" and typing the
// sheet's name into a free-text box from memory). Creating the part here is
// what makes the tiles actually appear. It's a normal undoable edit, so an
// artist who wanted the sheet only as a prop target can Ctrl+Z or delete it.
func (a *Application) onImportSpriteSheet() {
	ui.ShowImportSheetDialog(a.Window, func(filePath, name string, cellW, cellH int, pivotX, pivotY float32) {
		img, err := file.LoadImage(filePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load image: %w", err), a.Window)
			return
		}

		tmpl := editor.NewSpriteSheetTemplate(name, filePath, img, cellW, cellH, pivotX, pivotY)
		a.Project.LoadedSheets[name] = tmpl

		sprshPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".sprsh"
		if err := file.SaveSheetTemplate(tmpl, sprshPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save sheet template: %w", err), a.Window)
		}

		created := a.addPartForSheet(name)
		a.refreshAll()

		msg := fmt.Sprintf("Imported %q: %dx%d cells, %d cols x %d rows.",
			name, cellW, cellH, tmpl.Cols(), tmpl.Rows())
		if created {
			msg += fmt.Sprintf("\n\nAdded part %q to direction %d and selected it — its tiles are now in the left panel. Drag one onto the canvas to place a keyframe.",
				name, a.Project.Playback.ActiveDirection)
		} else {
			msg += "\n\nPick a part on the right and set its sheet to this one to draw with it."
		}
		dialog.ShowInformation("Import Complete", msg, a.Window)
	})
}

// addPartForSheet creates a Sheet part named after the sheet and selects it,
// so a freshly imported sheet is immediately visible and draggable. Returns
// false if there's no active direction to add it to.
func (a *Application) addPartForSheet(sheetName string) bool {
	return a.addPart(editor.NewSheetPart(sheetName, "", sheetName))
}

// addPart adds the part to the active direction only and selects it.
// Deliberately scoped to one direction: managing which directions an
// animation has parts in is the artist's call, not the editor's, so nothing
// here fans a part out across facings on their behalf.
func (a *Application) addPart(part *editor.Part) bool {
	dir := a.Project.ActiveDirection()
	if dir == nil {
		return false
	}
	a.Project.RecordUndo()
	editor.AddPart(dir, part)
	a.properties.SelectPart(len(dir.Parts) - 1)
	a.Project.Dirty = true
	return true
}

func (a *Application) onAddDirection() {
	ui.ShowAddDirectionDialog(a.Window, func(key int) {
		a.Project.RecordUndo()
		editor.AddDirection(a.Project.CurrentTrack, key)
		a.refreshDirectionSelect()
		a.refreshAll()
	})
}

func (a *Application) onAddProp() {
	ui.ShowAddPropDialog(a.Window, func(name, def string) {
		a.Project.RecordUndo()
		editor.AddProp(a.Project.CurrentTrack, name, def)
		a.refreshAll()
	})
}

func (a *Application) onAddPart() {
	var propNames []string
	for _, p := range a.Project.CurrentTrack.Props {
		propNames = append(propNames, p.Name)
	}
	ui.ShowAddPartDialog(a.Window, propNames, a.Project.LoadedSheetNames(), func(name string, kind editor.PartKind, governingProp, fixedSheet, nestedPath string) {
		// Selected straight away, so the left palette switches to its sheet
		// and the artist can drag a tile without a second click.
		if kind == editor.PartKindNestedAni {
			a.addPart(editor.NewNestedAniPart(name, nestedPath))
		} else {
			a.addPart(editor.NewSheetPart(name, governingProp, fixedSheet))
		}
		a.refreshAll()
	})
}

func (a *Application) onUndo() {
	if a.Project.Undo() {
		a.refreshDirectionSelect()
		a.refreshAll()
	}
}

func (a *Application) onRedo() {
	if a.Project.Redo() {
		a.refreshDirectionSelect()
		a.refreshAll()
	}
}

func (a *Application) onZoom(factor float32) {
	a.canvasWidget.SetZoom(factor)
}

// -- Playback loop --

func (a *Application) playbackLoop() {
	ticker := time.NewTicker(16 * time.Millisecond) // ~60fps
	defer ticker.Stop()

	lastTick := time.Now()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			deltaMs := uint32(now.Sub(lastTick).Milliseconds())
			lastTick = now

			if a.Project.Playback.IsPlaying {
				a.Project.AdvancePlayback(deltaMs)
				a.canvasWidget.Refresh()
				a.timeline.Refresh()
			}
		case <-a.playbackDone:
			return
		}
	}
}

// -- Refresh --

func (a *Application) refreshAll() {
	a.canvasWidget.Refresh()
	a.properties.Refresh()
	a.timeline.Refresh()
	if a.titleLabel != nil {
		a.titleLabel.SetText(a.Project.CurrentTrack.Metadata.Name)
	}
}

func (a *Application) onClose() {
	close(a.playbackDone)
}
