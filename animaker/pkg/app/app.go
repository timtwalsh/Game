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

	// The keyframe a canvas drag is moving, and where it sat when the drag
	// began. Held by pointer for the whole gesture rather than looked up
	// again on every mouse-move.
	dragKf               *editor.Keyframe
	dragOrigX, dragOrigY float32
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
	// Dragging a placed part auto-keys it: if the part has no keyframe at
	// the playhead, the drag creates one (seeded from its current pose)
	// and moves that. Previously a drag only moved a keyframe sitting
	// *exactly* at the playhead and silently did nothing otherwise — which,
	// once parts are placed at 0ms, is every drag made anywhere else on
	// the timeline. "New Keyframe" still exists for keying without moving.
	a.canvasWidget.OnPartDragStart = func(idx int) {
		dir, part := a.directionAndPart(idx)
		if dir == nil {
			return
		}
		// Keying lands at the playhead, so it must not be moving.
		a.Project.Playback.IsPlaying = false
		a.Project.RecordUndo()
		a.properties.SelectPart(idx)
		kf, _ := editor.EnsureKeyframe(dir, part.ID, a.Project.Playback.ElapsedMs)
		a.dragKf = kf
		a.dragOrigX, a.dragOrigY = kf.X, kf.Y
		a.Project.Selection.KeyframeIndex = kf.ID
		a.Project.Dirty = true
		a.refreshAll()
	}
	a.canvasWidget.OnPartDragged = func(idx int, dx, dy float32) {
		if a.dragKf == nil {
			return
		}
		a.dragKf.X, a.dragKf.Y = a.dragOrigX+dx, a.dragOrigY+dy
		a.canvasWidget.Refresh()
		a.properties.Refresh()
	}
	a.canvasWidget.OnPartDragEnd = func() {
		a.dragKf = nil
		a.refreshAll()
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
		a.Project.RecordUndo()
		kf, _ := editor.EnsureKeyframe(dir, part.ID, a.Project.Playback.ElapsedMs)
		sel.KeyframeIndex = kf.ID
		a.Project.Dirty = true
		a.refreshAll()
	}
	// Duplicate copies the selected keyframe's pose to the playhead — or
	// just after the source, if the playhead is sitting on it. Onto a time
	// that already has a keyframe, it pastes the pose rather than stacking
	// a second keyframe there (see editor.DuplicateKeyframe).
	a.timeline.OnDuplicateKeyframe = func() {
		sel := a.Project.Selection
		dir, part := a.directionAndPart(sel.PartIndex)
		if dir == nil {
			return
		}
		kfs := dir.KeyframesFor(part.ID)
		if sel.KeyframeIndex < 0 || sel.KeyframeIndex >= len(kfs) {
			return
		}
		target := editor.DuplicateTargetMs(kfs[sel.KeyframeIndex].TimeMs, a.Project.Playback.ElapsedMs)
		a.Project.RecordUndo()
		dup, err := editor.DuplicateKeyframe(dir, part.ID, sel.KeyframeIndex, target)
		if err != nil {
			return
		}
		sel.KeyframeIndex = dup.ID
		a.Project.Seek(dup.TimeMs)
		a.Project.Dirty = true
		a.refreshAll()
	}
	// Retiming: one undo step for the whole drag, recorded at its start.
	a.timeline.OnKeyframeRetimeStart = func(partIdx int, kf *editor.Keyframe) {
		a.Project.Playback.IsPlaying = false
		a.Project.RecordUndo()
		a.properties.SelectPart(partIdx)
		a.Project.Selection.KeyframeIndex = kf.ID
		a.refreshAll()
	}
	a.timeline.OnKeyframeRetimed = func(partIdx int, kf *editor.Keyframe, newTimeMs uint32) {
		dir, part := a.directionAndPart(partIdx)
		if dir == nil || kf.TimeMs == newTimeMs {
			return
		}
		// Refused if it would land exactly on another keyframe of this
		// part; the marker just holds for that one mouse-move, and the
		// next one carries it past.
		if editor.MoveKeyframe(dir, part.ID, kf.ID, newTimeMs) != nil {
			return
		}
		// Moving can reorder the keyframes, so re-read the index, and keep
		// the playhead on the keyframe so the canvas shows its pose.
		a.Project.Selection.KeyframeIndex = kf.ID
		a.Project.Seek(kf.TimeMs)
		a.Project.Dirty = true
		a.refreshAll()
	}
	a.timeline.OnKeyframeRetimeEnd = func() { a.refreshAll() }
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

// onTileDropped handles a cell dragged out of the palette onto the canvas.
// With a part selected, the cell becomes that part's next frame: a
// keyframe at the playhead, at the drop point. With nothing selected it is
// added to the rig as a new part, which is how a rig of several pieces
// visible at once gets built. editor.Project.DropTile owns that rule.
//
// Both behaviours were reported as wanted at different times: first
// "I should be able to have all of the parts on a sheet simultaneously"
// (drops used to always key the selected part), then dragging the next
// frame of a sprite at 200ms "should just insert it onto the timeline of
// the previous sprite" (drops had become always-new-part). Letting the
// selection decide serves both; clicking empty canvas or Esc deselects.
//
// Dragging a part already on the canvas moves it, and clicking a palette
// tile re-cells the selected keyframe (onTileTapped).
//
// Drops outside the canvas's bounds are ignored.
func (a *Application) onTileDropped(sheetName string, row, col int, absPos fyne.Position) {
	// The drop ends the gesture, so the view unfreezes here however this
	// returns - including the rejection paths below.
	defer a.canvasWidget.SetViewFrozen(false)

	if a.Project.ActiveDirection() == nil || sheetName == "" {
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
	partIdx, kf := a.Project.DropTile(sheetName, row, col, x, y)
	if kf == nil {
		return
	}
	a.properties.SelectPart(partIdx)
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
	// Esc deselects, so the next palette drop creates a new part instead
	// of keying the selected one (see onTileDropped). Only reaches here
	// when no text entry has focus, so it won't fight typing in a field.
	canvas.SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape && a.Project.Selection.PartIndex >= 0 {
			a.properties.SelectPart(-1)
			a.refreshAll()
		}
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
