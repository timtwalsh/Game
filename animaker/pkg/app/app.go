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

// directionAndPart resolves a part index (into the track's shared part
// list) together with the active direction, returning nil if either is out
// of range. Nearly every callback below needs exactly this pair.
func (a *Application) directionAndPart(partIdx int) (*editor.Direction, *editor.Part) {
	dir := a.Project.ActiveDirection()
	track := a.Project.CurrentTrack
	if dir == nil || track == nil || partIdx < 0 || partIdx >= len(track.Parts) {
		return nil, nil
	}
	return dir, track.Parts[partIdx]
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
		dir, part := a.directionAndPart(idx)
		if dir == nil {
			return
		}
		// Only an existing keyframe at exactly the current playhead time
		// moves - dragging doesn't implicitly create one. Use "New
		// Keyframe" first, per the intended workflow.
		for _, kf := range dir.KeyframesFor(part.ID) {
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
		dir, part := a.directionAndPart(partIdx)
		if dir != nil {
			if kfs := dir.KeyframesFor(part.ID); kfIdx >= 0 && kfIdx < len(kfs) {
				a.Project.Seek(kfs[kfIdx].TimeMs)
			}
		}
		a.refreshAll()
	}
	a.timeline.OnKeyframeDeleted = func(partIdx, kfIdx int) {
		dir, part := a.directionAndPart(partIdx)
		if dir == nil {
			return
		}
		a.Project.RecordUndo()
		_ = editor.DeleteKeyframe(dir, part.ID, kfIdx)
		a.Project.Selection.KeyframeIndex = -1
		a.Project.Dirty = true
		a.refreshAll()
	}
	a.timeline.OnNewKeyframe = func() {
		sel := a.Project.Selection
		dir, part := a.directionAndPart(sel.PartIndex)
		if dir == nil {
			return
		}
		elapsed := a.Project.Playback.ElapsedMs
		// Seed the new keyframe from wherever the part's interpolated pose
		// currently is, so it starts as a continuation rather than
		// snapping to zero - the artist then drags it into place.
		seed := dir.ValueAt(part.ID, elapsed)
		a.Project.RecordUndo()
		kf := editor.AddKeyframe(dir, part.ID, elapsed)
		kf.X, kf.Y, kf.Z, kf.RotationDeg = seed.X, seed.Y, seed.Z, seed.RotationDeg
		kf.Row, kf.Col = seed.Row, seed.Col
		sel.KeyframeIndex = kf.ID
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
	// Freeze the canvas extent for the whole palette drag, so placing the
	// first keyframe past the current bounds doesn't slide the canvas
	// while the artist is still choosing where to drop.
	a.properties.OnTileDragStart = func() { a.canvasWidget.SetViewFrozen(true) }
	a.properties.OnTileDropped = a.onTileDropped
	a.properties.OnTileTapped = a.onTileTapped
}

// onTileDropped is the core "level editor" interaction: dragging a cell
// out of the palette onto the canvas adds it to the rig as a NEW part,
// placed where it landed and showing the dragged cell. Dragging always
// creates rather than modifying, exactly as dragging from a tile palette
// does in a level editor, which is what makes it possible to build a rig
// of several pieces visible at once — the previous behaviour bound every
// drop to the selected part, so a second drag only ever added another
// keyframe to the one part and a part shows one cell at a time.
//
// The two other jobs get their own gestures: drag a part already on the
// canvas to move it, and click a palette tile to re-cell the selected
// keyframe (onTileTapped).
//
// Drops outside the canvas's bounds are ignored.
func (a *Application) onTileDropped(sheetName string, row, col int, absPos fyne.Position) {
	// The drop ends the gesture, so the view unfreezes here however this
	// returns - including the rejection paths below.
	defer a.canvasWidget.SetViewFrozen(false)

	dir := a.Project.ActiveDirection()
	if dir == nil || sheetName == "" {
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
	part := editor.AddPart(a.Project.CurrentTrack,
		editor.NewSheetPart(editor.UniquePartName(a.Project.CurrentTrack, sheetName), "", sheetName))

	// Keyed at the playhead, so dropping while parked at 0ms builds up the
	// rig's first pose and dropping later in the timeline starts that
	// part's animation where the artist is actually working.
	kf := editor.AddKeyframe(dir, part.ID, a.Project.Playback.ElapsedMs)
	kf.Row, kf.Col = row, col
	kf.X, kf.Y = x, y
	// Stack new parts in front of what's already there, so a piece dropped
	// later isn't hidden behind one dropped earlier.
	kf.Z = float32(len(a.Project.CurrentTrack.Parts))

	a.properties.SelectPart(len(a.Project.CurrentTrack.Parts) - 1)
	a.Project.Selection.KeyframeIndex = kf.ID
	a.Project.Dirty = true
	a.refreshAll()
}

// onTileTapped re-points the selected keyframe at a different cell of the
// palette's sheet. This is the counterpart to dragging: a click edits what
// is already selected, a drag creates something new.
func (a *Application) onTileTapped(row, col int) {
	kf := a.Project.SelectedKeyframe()
	if kf == nil {
		return
	}
	a.Project.RecordUndo()
	kf.Row, kf.Col = row, col
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

// onImportSpriteSheet imports a sheet and shows it in the palette. It does
// NOT create a part any more: dragging a tile out of the palette is what
// makes parts now, so a part created here would just be an empty one with
// no keyframes, drawn nowhere. Pointing the palette at the new sheet is
// what makes the import visibly do something — that was the original
// complaint when import only filled LoadedSheets and changed nothing on
// screen.
func (a *Application) onImportSpriteSheet() {
	ui.ShowImportSheetDialog(a.Window, func(filePath, name string, cellW, cellH int, pivotX, pivotY float32) {
		img, err := file.LoadImage(filePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load image: %w", err), a.Window)
			return
		}

		tmpl := editor.NewSpriteSheetTemplate(name, filePath, img, cellW, cellH, pivotX, pivotY)
		a.Project.LoadedSheets[name] = tmpl
		a.Project.PaletteSheet = name

		sprshPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".sprsh"
		if err := file.SaveSheetTemplate(tmpl, sprshPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save sheet template: %w", err), a.Window)
		}

		a.refreshAll()
		dialog.ShowInformation("Import Complete",
			fmt.Sprintf("Imported %q: %dx%d cells, %d cols x %d rows.\n\n"+
				"Its tiles are in the left panel. Drag one onto the canvas to add it as a part.",
				name, cellW, cellH, tmpl.Cols(), tmpl.Rows()),
			a.Window)
	})
}

// addPart adds a part to the track's rig - so it exists in every
// direction at once - and selects it. Used by the "+ Add Part" dialog,
// which is the way to create a nested-animation part or a prop-governed
// one; a plain sheet part is usually quicker to make by dragging a tile
// out of the palette (onTileDropped).
func (a *Application) addPart(part *editor.Part) bool {
	track := a.Project.CurrentTrack
	if track == nil {
		return false
	}
	a.Project.RecordUndo()
	editor.AddPart(track, part)
	a.properties.SelectPart(len(track.Parts) - 1)
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
