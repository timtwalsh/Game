package app

import (
	"animaker/pkg/applog"
	"animaker/pkg/editor"
	"animaker/pkg/file"
	"animaker/pkg/ui"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// Application is the main coordinator tying together project state, UI, and file operations.
type Application struct {
	FyneApp fyne.App
	Window  fyne.Window
	Project *editor.Project

	// PreviousCrashLog is the last session's log if it crashed; shown once
	// at startup (reportPreviousCrash).
	PreviousCrashLog string

	canvasWidget *ui.CanvasWidget
	timeline     *ui.TimelineWidget
	properties   *ui.PropertiesPanel
	directionTabs *ui.DirectionTabs
	// The .anichar open beside the track, if any (character.go), and the
	// left column it shares with the palette.
	character      *editor.Character
	characterPath  string
	characterPanel *ui.CharacterPanel
	characterView  fyne.CanvasObject
	palettePanel   fyne.CanvasObject
	leftColumn     *fyne.Container
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
// showError logs err to the session log and shows it to the artist. Use
// it instead of dialog.ShowError so every error the editor reports is in
// the log too, next to whatever led up to it.
func (a *Application) showError(err error) {
	applog.Errorf("%v", err)
	dialog.ShowError(err, a.Window)
}

// reportPreviousCrash tells the artist, once, that the last session ended
// in a crash and where its log is, so it can be sent along with the bug
// report instead of being lost.
func (a *Application) reportPreviousCrash() {
	if a.PreviousCrashLog == "" {
		return
	}
	msg := widget.NewLabel("The editor closed unexpectedly last time. What it was doing, " +
		"and the crash details, were saved to this log file:")
	msg.Wrapping = fyne.TextWrapWord
	path := widget.NewEntry()
	path.SetText(a.PreviousCrashLog)
	d := dialog.NewCustom("Previous Session Crashed", "OK", container.NewVBox(msg, path), a.Window)
	d.Resize(fyne.NewSize(560, 200))
	d.Show()
}

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
	a.build()

	go a.playbackLoop()
	a.reportPreviousCrash()

	a.Window.ShowAndRun()
}

// build creates the widgets, wires them up and fills a.Window, without
// showing it - separate from Run so tests can drive the real UI.
func (a *Application) build() {
	a.canvasWidget = ui.NewCanvasWidget(a.Project)
	a.timeline = ui.NewTimelineWidget(a.Project)
	a.properties = ui.NewPropertiesPanel(a.Project)

	a.wireCallbacks()

	directionBar := a.buildDirectionBar()
	canvasScroll := container.NewScroll(a.canvasWidget)
	a.palettePanel = a.properties.BuildPalette()
	a.buildCharacterPanel()
	a.leftColumn = container.NewStack(a.palettePanel)
	propertiesPanel := a.properties.Build(directionBar)
	timelinePanel := a.timeline.Build()
	mainLayout := ui.BuildMainLayout(a.leftColumn, canvasScroll, propertiesPanel, timelinePanel)

	menu := ui.BuildMenuBar(
		a.onNewTrack,
		a.onNewTrackFromRig,
		a.onOpenTrack,
		a.onNewCharacter,
		a.onOpenCharacter,
		a.onCloseCharacter,
		a.onSaveTrack,
		a.onSaveAsTrack,
		a.onImportSpriteSheet,
		a.onImportAnim,
		a.onUndo,
		a.onRedo,
		a.canvasWidget.ToggleGrid,
		a.canvasWidget.ToggleOnion,
		a.onZoom,
		a.showAbout,
		func() { ui.ShowShortcutsDialog(a.Window) },
	)
	a.Window.SetMainMenu(menu)

	a.registerShortcuts()

	a.Window.SetContent(mainLayout)
	// Closing with unsaved changes asks first. Window.Close, which the
	// prompt calls once it's answered, doesn't come back through here.
	a.Window.SetCloseIntercept(func() {
		a.confirmDiscard(a.Window.Close)
	})
	a.Window.SetOnClosed(a.onClose)
	a.updateTitle()
}

func (a *Application) buildDirectionBar() fyne.CanvasObject {
	a.titleLabel = widget.NewLabel(a.Project.CurrentTrack.Metadata.Name)
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	a.directionTabs = ui.NewDirectionTabs()
	a.directionTabs.OnSelect = a.switchDirection
	a.directionTabs.MenuFor = a.directionMenu
	a.refreshDirectionSelect()

	// Adding, deleting and re-keying directions all live in one menu: this
	// button opens it for the active direction, right-clicking a tab for
	// that one.
	var menuBtn *widget.Button
	menuBtn = widget.NewButton("Directions ▾", func() {
		a.directionTabs.ShowMenuBelow(a.Project.Playback.ActiveDirection, menuBtn)
	})

	return container.NewVBox(
		container.NewHBox(a.titleLabel, layout.NewSpacer(), menuBtn),
		a.directionTabs.Object(),
	)
}

// directionMenu is what can be done to direction key, and to the track's
// directions as a whole. Shown for the active direction by the Directions
// button, and for any tab on right-click. Directions are named by compass
// point for the track's direction count (editor.FacingName).
func (a *Application) directionMenu(key int) *fyne.Menu {
	track := a.Project.CurrentTrack
	n := track.Facings()
	name := editor.FacingName(key, n)

	// Change to: every other direction this track's count has.
	var changeItems []*fyne.MenuItem
	for to := 0; to < n; to++ {
		if to == key {
			continue
		}
		to := to
		item := fyne.NewMenuItem(editor.FacingName(to, n), func() { a.changeDirectionKey(key, to) })
		if d, ok := track.Directions[to]; ok && d.TotalKeyframes() > 0 {
			item.Label += " (posed)"
			item.Disabled = true
		}
		changeItems = append(changeItems, item)
	}
	change := fyne.NewMenuItem("Change "+name+" to", nil)
	change.ChildMenu = fyne.NewMenu("", changeItems...)
	change.Disabled = len(changeItems) == 0

	del := fyne.NewMenuItem("Delete "+name+"...", func() { a.confirmDeleteDirection(key) })
	del.Disabled = len(track.Directions) <= 1

	// How many directions: 1, 4, 8 or 16.
	var countItems []*fyne.MenuItem
	for _, c := range editor.DirectionCounts {
		c := c
		item := fyne.NewMenuItem(fmt.Sprintf("%d", c), func() { a.setDirectionCount(c) })
		item.Checked = c == n
		countItems = append(countItems, item)
	}
	count := fyne.NewMenuItem("Directions", nil)
	count.ChildMenu = fyne.NewMenu("", countItems...)

	// Directions this count has but the track doesn't (deleted, or removed
	// as empty), to add back.
	var addItems []*fyne.MenuItem
	for _, k := range track.MissingFacings() {
		k := k
		addItems = append(addItems, fyne.NewMenuItem(editor.FacingName(k, n), func() { a.addDirection(k) }))
	}
	add := fyne.NewMenuItem("Add Direction", nil)
	add.ChildMenu = fyne.NewMenu("", addItems...)
	add.Disabled = len(addItems) == 0

	removeEmpty := fyne.NewMenuItem("Remove Empty Directions", a.onRemoveEmptyDirections)
	if empty := track.EmptyDirections(); len(empty) == 0 || len(empty) == len(track.Directions) && len(empty) == 1 {
		removeEmpty.Disabled = true
	}

	return fyne.NewMenu("",
		change, del,
		fyne.NewMenuItemSeparator(),
		count, add, removeEmpty,
	)
}

// setDirectionCount changes how many directions the track has
// (editor.Project.SetDirectionCount): directions keep their facing, and
// any with keyframes that the new count can't face are removed only after
// asking. Undoable.
func (a *Application) setDirectionCount(n int) {
	snap := a.Project.TakeSnapshot()
	apply := func(dropPosed bool) {
		if _, err := a.Project.SetDirectionCount(n, dropPosed); err != nil {
			a.showError(err)
			return
		}
		a.Project.UndoStack.Push(snap)
		a.refreshDirectionSelect()
		a.refreshAll()
	}
	lost, err := a.Project.SetDirectionCount(n, false)
	switch {
	case err != nil:
		a.showError(err)
		return
	case len(lost) == 0:
		if a.Project.Dirty {
			a.Project.UndoStack.Push(snap)
		}
		a.refreshDirectionSelect()
		a.refreshAll()
		return
	}
	from := a.Project.CurrentTrack.Facings()
	names := make([]string, len(lost))
	for i, k := range lost {
		names[i] = editor.FacingName(k, from)
	}
	msg := fmt.Sprintf("A %d-direction track can't face %s, which have keyframes. "+
		"Change to %d directions and delete them?\n\nYou can undo this with Ctrl+Z.", n, strings.Join(names, ", "), n)
	dialog.ShowConfirm("Fewer Directions", msg, func(ok bool) {
		if ok {
			apply(true)
		}
	}, a.Window)
}

// addDirection adds back one of the track's missing directions, empty.
func (a *Application) addDirection(k int) {
	a.Project.RecordUndo()
	editor.AddDirection(a.Project.CurrentTrack, k)
	a.refreshDirectionSelect()
	a.refreshAll()
}

// confirmDeleteDirection deletes a direction - straight away if it's
// empty, after asking if it has keyframes. Undoable either way.
func (a *Application) confirmDeleteDirection(key int) {
	dir := a.Project.CurrentTrack.Directions[key]
	if dir == nil {
		return
	}
	del := func() {
		snap := a.Project.TakeSnapshot()
		if err := a.Project.DeleteDirection(key); err != nil {
			a.showError(err)
			return
		}
		a.Project.UndoStack.Push(snap)
		a.refreshDirectionSelect()
		a.refreshAll()
	}
	n := dir.TotalKeyframes()
	if n == 0 {
		del()
		return
	}
	msg := fmt.Sprintf("Delete the %s direction and its %d keyframes?\n\nThe parts stay - they belong to every "+
		"direction. You can undo this with Ctrl+Z.", editor.FacingName(key, a.Project.CurrentTrack.Facings()), n)
	dialog.ShowConfirm("Delete Direction", msg, func(ok bool) {
		if ok {
			del()
		}
	}, a.Window)
}

// changeDirectionKey moves a direction's keyframes to another key, e.g. a
// facing posed as Up that should have been Right. Undoable.
func (a *Application) changeDirectionKey(from, to int) {
	snap := a.Project.TakeSnapshot()
	if err := a.Project.ChangeDirectionKey(from, to); err != nil {
		a.showError(err)
		return
	}
	a.Project.UndoStack.Push(snap)
	a.refreshDirectionSelect()
	a.refreshAll()
}

// onRemoveEmptyDirections tidies a track saved with the old four-direction
// default: every direction with no keyframes goes, keeping at least one.
func (a *Application) onRemoveEmptyDirections() {
	snap := a.Project.TakeSnapshot()
	if removed := a.Project.RemoveEmptyDirections(); len(removed) == 0 {
		return
	}
	a.Project.UndoStack.Push(snap)
	a.refreshDirectionSelect()
	a.refreshAll()
}

// switchDirection makes key the facing being edited, offering to copy
// timing into it if it's empty.
func (a *Application) switchDirection(key int) {
	if _, ok := a.Project.CurrentTrack.Directions[key]; !ok || key == a.Project.Playback.ActiveDirection {
		return
	}
	a.Project.SetActiveDirection(key)
	a.refreshAll()
	a.offerTimingCopy(key)
}

// refreshDirectionSelect rebuilds the direction tabs - their keyframe
// counts change with every edit, so refreshAll calls it too.
func (a *Application) refreshDirectionSelect() {
	if a.directionTabs == nil {
		return
	}
	a.directionTabs.Refresh(a.Project.CurrentTrack, a.Project.Playback.ActiveDirection)
}

// offerTimingCopy prompts, on switching to a direction with no keyframes,
// to copy another direction's keyframe times into it. Only times: the
// poses are the artist's to author per facing, so the editor never copies
// them between directions on its own.
func (a *Application) offerTimingCopy(key int) {
	track := a.Project.CurrentTrack
	dir := track.Directions[key]
	if dir == nil || dir.TotalKeyframes() > 0 {
		return
	}
	sources := track.TimingSources(key)
	if len(sources) == 0 {
		return
	}
	ui.ShowCopyTimingDialog(a.Window, key, sources, track.Facings(), func(src int) {
		a.Project.RecordUndo()
		editor.CopyKeyframeTimes(track, src, key)
		a.Project.Dirty = true
		a.refreshAll()
	})
}

// confirmRemoveSheet removes an imported sheet from the track after asking.
// A sheet still in use is refused up front with what uses it, rather than
// after a confirm the artist can't act on.
func (a *Application) confirmRemoveSheet(name string) {
	if users := a.Project.CurrentTrack.SheetUsers(name); len(users) > 0 {
		a.showError(&editor.SheetInUseError{Sheet: name, Users: users})
		return
	}
	msg := fmt.Sprintf("Remove the sheet %q from this track?\n\n"+
		"Nothing uses it. Its image and .sprsh files on disk are not deleted - "+
		"import it again to bring it back.", name)
	dialog.ShowConfirm("Remove Sprite Sheet", msg, func(ok bool) {
		if !ok {
			return
		}
		if err := a.Project.RemoveSheet(name); err != nil {
			a.showError(err)
			return
		}
		a.refreshAll()
	}, a.Window)
}

// confirmDeletePart is what both the timeline's x and the part list's
// Delete call. Deleting removes the part's keyframes in every direction,
// not just the one on screen, so it asks first; it's also undoable.
func (a *Application) confirmDeletePart(idx int) {
	parts := a.Project.CurrentTrack.Parts
	if idx < 0 || idx >= len(parts) {
		return
	}
	name := parts[idx].Name
	msg := fmt.Sprintf("Delete %q?\n\nThis removes the part and its keyframes in every direction. "+
		"You can undo it with Ctrl+Z.", name)
	dialog.ShowConfirm("Delete Part", msg, func(ok bool) {
		if !ok {
			return
		}
		a.Project.RecordUndo()
		if err := a.Project.DeletePart(idx); err != nil {
			a.showError(err)
			return
		}
		a.refreshAll()
	}, a.Window)
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
		a.Project.Selection.KeyframeIndex = kf.Index
		a.Project.Dirty = true
		a.refreshAll()
	}
	a.canvasWidget.OnPartDragged = func(idx int, dx, dy float32) {
		if a.dragKf == nil {
			return
		}
		a.dragKf.X, a.dragKf.Y = editor.SnapPosition(a.dragOrigX+dx, a.dragOrigY+dy, subPixelHeld())
		a.canvasWidget.Refresh()
		a.properties.Refresh()
	}
	a.canvasWidget.OnPartDragEnd = func() {
		a.dragKf = nil
		a.refreshAll()
	}

	// -- Timeline --
	a.timeline.OnScrub = func(ms uint32) {
		a.Project.Scrub(ms)
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
	a.timeline.OnPartDelete = a.confirmDeletePart
	a.timeline.OnPartSelected = func(partIdx int) {
		a.properties.SelectPart(partIdx)
		a.refreshAll()
	}
	// Double-clicking a row's name renames the part, so a rig built by
	// dropping tiles (which names parts sprite_1, sprite_2...) can be
	// relabelled "head", "left_arm", "legs" where the artist is looking.
	a.timeline.OnPartRename = a.renamePart
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
		sel.KeyframeIndex = kf.Index
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
		sel.KeyframeIndex = dup.Index
		a.Project.Seek(dup.TimeMs)
		a.Project.Dirty = true
		a.refreshAll()
	}
	// Retiming: one undo step for the whole drag, recorded at its start.
	a.timeline.OnKeyframeRetimeStart = func(partIdx int, kf *editor.Keyframe) {
		a.Project.Playback.IsPlaying = false
		a.Project.RecordUndo()
		a.properties.SelectPart(partIdx)
		a.Project.Selection.KeyframeIndex = kf.Index
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
		if editor.MoveKeyframe(dir, part.ID, kf.Index, newTimeMs) != nil {
			return
		}
		// Moving can reorder the keyframes, so re-read the index, and keep
		// the playhead on the keyframe so the canvas shows its pose.
		a.Project.Selection.KeyframeIndex = kf.Index
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
	a.properties.OnImportAnim = a.onImportAnim
	a.properties.OnAddPart = a.onAddPart
	a.properties.OnAddProp = a.onAddProp
	a.properties.OnLoadPreviewSheet = a.onLoadPreviewSheet
	a.properties.OnRemoveSheet = a.confirmRemoveSheet
	a.properties.OnEditSheet = a.onEditSheet
	a.properties.OnPropsChanged = func() { a.refreshAll() }
	a.properties.OnError = a.showError
	a.properties.OnPartDelete = a.confirmDeletePart
	a.properties.OnKeyframeRetimed = func() { a.refreshAll() }
	a.properties.OnKeyframeChanged = func() {
		a.canvasWidget.Refresh()
		a.timeline.Refresh()
		a.refreshDirectionSelect() // an edit can create a keyframe: the counts change
	}
	a.properties.OnPartChanged = func() {
		a.canvasWidget.Refresh()
		a.timeline.Refresh()
	}
	// Freeze the canvas extent for the whole palette drag, so placing the
	// first keyframe past the current bounds doesn't slide the canvas
	// while the artist is still choosing where to drop.
	a.properties.OnTileDragStart = func() { a.canvasWidget.SetViewFrozen(true) }
	a.properties.OnTileDragMove = a.onTileDragMove
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
	// The drop ends the gesture, so the view unfreezes and the preview
	// goes here however this returns - including the rejection paths below.
	defer a.canvasWidget.SetViewFrozen(false)
	defer a.canvasWidget.ClearDropPreview()

	if a.Project.ActiveDirection() == nil || sheetName == "" {
		return
	}
	x, y, ok := a.canvasPoint(absPos)
	if !ok {
		return // dropped outside the canvas - not a placement
	}

	before := a.Project.TakeSnapshot()
	parts := len(a.Project.CurrentTrack.Parts)
	partIdx, kf := a.Project.DropTile(sheetName, row, col, x, y)
	if kf == nil {
		return // nothing changed, so no undo step (it would also clear redo)
	}
	a.Project.UndoStack.Push(before)
	a.properties.SelectPart(partIdx)
	a.Project.Selection.KeyframeIndex = kf.Index
	a.Project.Dirty = true
	a.refreshAll()
	// A drop that made a new part asks for its name straight away, so a rig
	// is named as it's built rather than left as sheet_1, sheet_2... Esc
	// keeps the generated name.
	if len(a.Project.CurrentTrack.Parts) > parts {
		a.renamePart(partIdx)
	}
}

// onTileDragMove shows where a palette tile being dragged would land, as
// a faint copy of the cell under the cursor - placed exactly as the drop
// will place it.
func (a *Application) onTileDragMove(sheetName string, row, col int, absPos fyne.Position) {
	sheet := a.Project.LoadedSheets[sheetName]
	x, y, ok := a.canvasPoint(absPos)
	if sheet == nil || !ok {
		a.canvasWidget.ClearDropPreview()
		return
	}
	a.canvasWidget.SetDropPreview(sheet, row, col, x, y)
}

// canvasPoint converts a window position into the animation's own X/Y,
// snapped as a drop is (editor.SnapPosition); ok is false outside the
// canvas.
func (a *Application) canvasPoint(absPos fyne.Position) (x, y float32, ok bool) {
	canvasAbsPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(a.canvasWidget)
	canvasSize := a.canvasWidget.Size()
	local := fyne.NewPos(absPos.X-canvasAbsPos.X, absPos.Y-canvasAbsPos.Y)
	if local.X < 0 || local.Y < 0 || local.X > canvasSize.Width || local.Y > canvasSize.Height {
		return 0, 0, false
	}
	x, y = a.canvasWidget.LocalToAnimXY(local)
	x, y = editor.SnapPosition(x, y, subPixelHeld())
	return x, y, true
}

// onTileTapped makes a clicked palette cell the selected part's frame at
// the playhead, keeping its position (editor.Project.TapTile). This is the
// counterpart to dragging: a click edits what is already selected, a drag
// places something. A click that can't apply says why - it used to do
// nothing at all unless a timeline marker happened to be selected.
func (a *Application) onTileTapped(row, col int) {
	before := a.Project.TakeSnapshot()
	kf, err := a.Project.TapTile(a.Project.PaletteSheet, row, col)
	if err != nil {
		a.showError(err)
		return // nothing changed, so no undo step (it would also clear redo)
	}
	a.Project.UndoStack.Push(before)
	a.Project.Selection.KeyframeIndex = kf.Index
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
	// Ctrl+Y is redo by Windows convention, alongside Ctrl+Shift+Z.
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyY, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		a.onRedo()
	})
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift}, func(_ fyne.Shortcut) {
		a.onSaveAsTrack()
	})
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyI, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
		a.onImportSpriteSheet()
	})
	// Draw order: Ctrl+] / Ctrl+[ one step forward / back, with Shift all
	// the way to the front / back.
	for _, z := range []struct {
		key      fyne.KeyName
		mod      fyne.KeyModifier
		front    bool
		absolute bool
	}{
		{fyne.KeyRightBracket, fyne.KeyModifierControl, true, false},
		{fyne.KeyLeftBracket, fyne.KeyModifierControl, false, false},
		{fyne.KeyRightBracket, fyne.KeyModifierControl | fyne.KeyModifierShift, true, true},
		{fyne.KeyLeftBracket, fyne.KeyModifierControl | fyne.KeyModifierShift, false, true},
	} {
		z := z
		canvas.AddShortcut(&desktop.CustomShortcut{KeyName: z.key, Modifier: z.mod}, func(_ fyne.Shortcut) {
			a.onZOrder(z.front, z.absolute)
		})
	}
	// Plain keys only reach here when no text entry has focus, so none of
	// them fight typing in a field.
	canvas.SetOnTypedKey(a.onTypedKey)
}

// onTypedKey handles the editor's single-key shortcuts (see
// ui.ShortcutHelp for the list shown to the artist).
func (a *Application) onTypedKey(e *fyne.KeyEvent) {
	shift := modifierHeld(fyne.KeyModifierShift)
	switch e.Name {
	case fyne.KeyEscape:
		// Deselect, so the next palette drop creates a new part instead
		// of keying the selected one (see onTileDropped).
		if a.Project.Selection.PartIndex >= 0 {
			a.properties.SelectPart(-1)
			a.refreshAll()
		}
	case fyne.KeySpace:
		a.Project.TogglePlay()
		a.refreshAll()
	case fyne.KeyComma, fyne.KeyPeriod:
		forward := e.Name == fyne.KeyPeriod
		switch {
		case shift && forward:
			a.Project.StepForward()
		case shift:
			a.Project.StepBackward()
		default:
			a.Project.StepToKeyframe(forward)
		}
		a.refreshAll()
	case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown:
		step := float32(1)
		if shift {
			step = 10
		}
		d := map[fyne.KeyName][2]float32{
			fyne.KeyLeft: {-step, 0}, fyne.KeyRight: {step, 0}, fyne.KeyUp: {0, -step}, fyne.KeyDown: {0, step},
		}[e.Name]
		a.editSelected(func(kf *editor.Keyframe) { kf.X += d[0]; kf.Y += d[1] })
	case fyne.KeyDelete, fyne.KeyBackspace:
		if partID, idx, ok := a.Project.KeyframeToDelete(); ok {
			a.Project.RecordUndo()
			_ = editor.DeleteKeyframe(a.Project.ActiveDirection(), partID, idx)
			a.Project.Selection.KeyframeIndex = -1
			a.Project.Dirty = true
			a.refreshAll()
		}
	case fyne.KeyF2:
		a.renamePart(a.Project.Selection.PartIndex)
	case fyne.KeyO:
		a.canvasWidget.ToggleOnion()
	case fyne.Key1, fyne.Key2, fyne.Key3, fyne.Key4:
		// N, E, S, W - whichever key faces that way on this track.
		a.switchDirection(editor.MapDirection(int(e.Name[0]-'1'), 4, a.Project.CurrentTrack.Facings()))
	}
}

// editSelected applies one edit to the selected part's keyframe at the
// playhead (editor.Project.EditTarget), as one undo step. Nothing happens
// with no part selected.
func (a *Application) editSelected(apply func(kf *editor.Keyframe)) {
	if a.Project.SelectedPart() == nil {
		return
	}
	a.Project.Playback.IsPlaying = false
	a.Project.RecordUndo()
	apply(a.Project.EditTarget())
	a.Project.Dirty = true
	a.refreshAll()
}

// onZOrder moves the selected part one step forward/back in draw order at
// the playhead, or (absolute) in front of / behind every other part.
func (a *Application) onZOrder(front, absolute bool) {
	if !absolute {
		step := float32(-1)
		if front {
			step = 1
		}
		a.editSelected(func(kf *editor.Keyframe) { kf.Z += step })
		return
	}
	if z, ok := a.Project.ZOrderTarget(front); ok {
		a.editSelected(func(kf *editor.Keyframe) { kf.Z = z })
	}
}

// modifierHeld reports whether mod is held down right now.
func modifierHeld(mod fyne.KeyModifier) bool {
	d, ok := fyne.CurrentApp().Driver().(desktop.Driver)
	return ok && d.CurrentKeyModifiers()&mod != 0
}

// -- Menu handlers --

// confirmDiscard runs then straight away when there are no unsaved changes;
// otherwise it asks whether to save them first, discard them, or cancel
// (in which case then never runs). Closing the window, New Track and Open
// all go through it, so none of them can silently throw away work.
func (a *Application) confirmDiscard(then func()) {
	if !a.Project.Dirty {
		then()
		return
	}
	msg := widget.NewLabel(fmt.Sprintf("%q has unsaved changes. Save them first?",
		a.Project.CurrentTrack.Metadata.Name))
	msg.Wrapping = fyne.TextWrapWord
	var d *dialog.CustomDialog
	save := widget.NewButton("Save", func() {
		d.Hide()
		a.saveThen(then)
	})
	save.Importance = widget.HighImportance
	discard := widget.NewButton("Don't Save", func() {
		d.Hide()
		then()
	})
	discard.Importance = widget.DangerImportance
	cancel := widget.NewButton("Cancel", func() { d.Hide() })
	d = dialog.NewCustomWithoutButtons("Unsaved Changes",
		container.NewVBox(msg, container.NewHBox(layout.NewSpacer(), cancel, discard, save)), a.Window)
	d.Resize(fyne.NewSize(440, 160))
	d.Show()
}

// saveThen saves (asking where, for a track never saved) and runs then only
// if the save went through - a cancelled Save As or a failed write leaves
// the unsaved work where it is.
func (a *Application) saveThen(then func()) {
	if a.Project.SavePath != "" {
		if a.saveToPath(a.Project.SavePath) {
			then()
		}
		return
	}
	a.showSaveAs(then)
}

// updateTitle shows the track's name in the window title, with a leading
// "*" while it has unsaved changes.
func (a *Application) updateTitle() {
	if a.Window == nil {
		return
	}
	mark := ""
	if a.Project.Dirty {
		mark = "*"
	}
	char := ""
	if a.character != nil {
		char = a.character.Name + " › "
	}
	a.Window.SetTitle("ANIFile Animation Maker — " + char + mark + a.Project.CurrentTrack.Metadata.Name)
}

func (a *Application) onNewTrack() {
	a.confirmDiscard(a.newTrack)
}

func (a *Application) newTrack() {
	ui.ShowNewTrackDialog(a.Window, a.openNewTrack)
}

// openNewTrack replaces the open track with a new, empty one.
func (a *Application) openNewTrack(name string) {
	// Loop and speed are the artist's playback preferences, not part
	// of a track, so they carry over to the new one.
	prev := a.Project.Playback
	a.Project = editor.NewProject(name)
	a.Project.Playback.LoopEnabled, a.Project.Playback.SpeedFactor = prev.LoopEnabled, prev.SpeedFactor
	a.canvasWidget.SetProject(a.Project)
	a.timeline.SetProject(a.Project)
	a.properties.SetProject(a.Project)
	a.refreshDirectionSelect()
	a.refreshAll()
	a.refreshCharacter()
}

func (a *Application) onOpenTrack() {
	a.confirmDiscard(a.openTrack)
}

func (a *Application) openTrack() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()
		a.loadAndOpen(filePath, func(t *editor.Track) (*editor.Track, string) { return t, filePath })
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

// loadAndOpen loads the .anif at filePath with its sheets and nested
// animations, then opens the track as(track) returns - the file itself for
// Open, a keyframe-free copy for New Track from Rig - with the save path
// it gives ("" for a track not saved yet). It reports whether it opened.
func (a *Application) loadAndOpen(filePath string, as func(*editor.Track) (*editor.Track, string)) bool {
	track, refs, err := file.LoadTrack(filePath)
	if err != nil {
		a.showError(fmt.Errorf("failed to load track: %w", err))
		return false
	}

	// The .anif names its sheets but doesn't contain them, so load
	// them now - previously nothing did, and a reopened track drew
	// nothing at all. Sheets already loaded this session are kept.
	sheets, missing, problems := file.LoadSheetsForTrack(filePath, refs, track.ReferencedSheetNames())
	// Nested animations are always re-read, never taken from what
	// was loaded before, so an .anif edited since shows as it is now.
	anims := map[string]*editor.NestedAnim{}
	problems = append(problems, file.LoadNestedAnimsFor(track, anims)...)
	for name, s := range sheets {
		a.Project.LoadedSheets[name] = s
	}
	if _, ok := a.Project.LoadedSheets[a.Project.PaletteSheet]; !ok {
		a.Project.PaletteSheet = ""
	}
	for _, name := range track.ReferencedSheetNames() {
		if _, ok := a.Project.LoadedSheets[name]; ok && a.Project.PaletteSheet == "" {
			a.Project.PaletteSheet = name
		}
	}

	opened, savePath := as(track)
	a.Project.OpenTrack(opened, savePath, anims)
	a.Project.Dirty = savePath == "" // a new track is unsaved work
	missing = slices.DeleteFunc(missing, a.Project.SheetFromNested)

	a.canvasWidget.SetProject(a.Project)
	a.timeline.SetProject(a.Project)
	a.properties.SetProject(a.Project)
	a.refreshDirectionSelect()
	a.refreshAll()
	a.refreshCharacter()
	a.reportUnloadedSheets(missing, problems)
	return true
}

// onNewTrackFromRig starts a track from an existing one's rig - its parts,
// props, sheets and directions, without keyframes (editor.RigFrom) - so a
// character's walk, idle and attack tracks agree on their parts and props.
func (a *Application) onNewTrackFromRig() {
	a.confirmDiscard(func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			filePath := reader.URI().Path()
			reader.Close()
			ui.ShowNewTrackDialog(a.Window, func(name string) {
				a.loadAndOpen(filePath, func(t *editor.Track) (*editor.Track, string) {
					return editor.RigFrom(t, name), ""
				})
			})
		}, a.Window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
		fd.Resize(fyne.NewSize(600, 400))
		fd.Show()
	})
}

func (a *Application) onSaveTrack() {
	if a.Project.SavePath == "" {
		a.onSaveAsTrack()
		return
	}
	a.saveToPath(a.Project.SavePath)
}

func (a *Application) onSaveAsTrack() {
	a.showSaveAs(func() {})
}

// showSaveAs asks where to save, saves there, and runs then if it saved.
func (a *Application) showSaveAs(then func()) {
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		filePath := writer.URI().Path()
		writer.Close()
		if a.saveToPath(filePath) {
			then()
		}
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.SetFileName(a.Project.CurrentTrack.Metadata.Name + ".anif")
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

// sheetRefs is where each imported sheet's .sprsh is, for the .anif to
// record so the track reopens with its art. Preview-only sheets are left
// out: they're never part of the track.
func (a *Application) sheetRefs() []file.SheetRef {
	var refs []file.SheetRef
	for _, name := range a.Project.LoadedSheetNames() {
		if s := a.Project.LoadedSheets[name]; s.SprshPath != "" {
			refs = append(refs, file.SheetRef{Name: name, SprshPath: s.SprshPath})
		}
	}
	return refs
}

// saveToPath saves the track and reports whether it worked.
func (a *Application) saveToPath(path string) bool {
	if err := file.SaveTrack(a.Project.CurrentTrack, path, a.sheetRefs()); err != nil {
		a.showError(fmt.Errorf("failed to save: %w", err))
		return false
	}
	a.Project.SavePath = path
	a.Project.Dirty = false
	a.updateTitle()
	a.refreshCharacter() // Save As may have made it, or stopped it being, one of the character's
	return true
}

// reportUnloadedSheets tells the artist which sheets a just-opened track
// couldn't find, rather than leaving parts silently undrawn.
func (a *Application) reportUnloadedSheets(missing, problems []string) {
	if len(missing) == 0 && len(problems) == 0 {
		return
	}
	applog.Errorf("track opened with unloaded sheets: missing %v, problems %v", missing, problems)
	var b strings.Builder
	if len(missing) > 0 {
		fmt.Fprintf(&b, "Couldn't find these sprite sheets: %s.\n\n", strings.Join(missing, ", "))
		b.WriteString("Parts using them won't draw until they're loaded. Use Import Sprite Sheet " +
			"on each image and give it exactly that Sheet Name - the track's parts refer to " +
			"sheets by name, so they'll pick it up. Saving afterwards records where it is.")
	}
	if len(problems) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("These sheet files were found but couldn't be loaded:\n" + strings.Join(problems, "\n"))
	}
	msg := widget.NewLabel(b.String())
	msg.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustom("Missing Art", "OK", msg, a.Window)
	d.Resize(fyne.NewSize(520, 300))
	d.Show()
}

// onImportSpriteSheet imports a sheet and shows it in the palette. It does
// NOT create a part any more: dragging a tile out of the palette is what
// makes parts now, so a part created here would just be an empty one with
// no keyframes, drawn nowhere. Pointing the palette at the new sheet is
// what makes the import visibly do something — that was the original
// complaint when import only filled LoadedSheets and changed nothing on
// screen.
func (a *Application) onImportSpriteSheet() {
	ui.ShowImportSheetDialog(a.Window, existingSheetSettings, file.LoadImage, a.applySheetImport)
}

// onEditSheet reopens a loaded sheet's grid, pivot and name in the import
// dialog, over its image - changing them used to mean finding the file and
// importing it again.
func (a *Application) onEditSheet(name string) {
	s := a.Project.LoadedSheets[name]
	if s == nil {
		return
	}
	current := ui.SheetImport{Name: s.Name, CellW: s.CellW, CellH: s.CellH, PivotX: s.PivotX, PivotY: s.PivotY}
	ui.ShowEditSheetDialog(a.Window, s.FilePath, s.Image, current, a.applySheetImport)
}

// applySheetImport slices the image as the import (or edit) dialog said,
// asks before replacing a saved template or loaded sheet, and loads it.
func (a *Application) applySheetImport(imp ui.SheetImport) {
	img, err := file.LoadImage(imp.FilePath)
	if err != nil {
		a.showError(fmt.Errorf("failed to load image: %w", err))
		return
	}
	tmpl := editor.NewSpriteSheetTemplate(imp.Name, imp.FilePath, img, imp.CellW, imp.CellH, imp.PivotX, imp.PivotY)
	if tmpl.Cols() == 0 || tmpl.Rows() == 0 {
		b := img.Bounds()
		a.showError(fmt.Errorf("a %dx%d cell doesn't fit in this %dx%d image - check Cell Width and Cell Height",
			imp.CellW, imp.CellH, b.Dx(), b.Dy()))
		return
	}
	replaces := a.importReplaces(imp)
	if len(replaces) == 0 {
		a.importSheet(imp, tmpl)
		return
	}
	msg := widget.NewLabel("This replaces:\n\n- " + strings.Join(replaces, "\n- ") +
		"\n\nParts drawing from the old sheet will be re-sliced with the new settings.")
	msg.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm("Replace Existing Sheet?", "Replace", "Cancel", msg, func(ok bool) {
		if ok {
			a.importSheet(imp, tmpl)
		}
	}, a.Window)
	d.Resize(fyne.NewSize(520, 280))
	d.Show()
}

// existingSheetSettings prefills the import dialog from the .sprsh an image
// already has, so re-importing it starts from its saved grid and pivot.
func existingSheetSettings(imagePath string) (ui.SheetImport, bool) {
	st, err := file.ReadSheetSettings(file.SprshPathFor(imagePath))
	if err != nil {
		return ui.SheetImport{}, false
	}
	return ui.SheetImport{Name: st.Name, CellW: st.CellW, CellH: st.CellH, PivotX: st.PivotX, PivotY: st.PivotY}, true
}

// importReplaces lists what importing imp would overwrite: the image's
// existing .sprsh if the settings differ from it, and an already-loaded
// sheet of the same name if it's a different image or grid. Empty means
// the import changes nothing that exists.
func (a *Application) importReplaces(imp ui.SheetImport) []string {
	var out []string
	sprsh := file.SprshPathFor(imp.FilePath)
	if old, err := file.ReadSheetSettings(sprsh); err == nil {
		now := file.SheetSettings{Name: imp.Name, CellW: imp.CellW, CellH: imp.CellH, PivotX: imp.PivotX, PivotY: imp.PivotY}
		if old != now {
			out = append(out, fmt.Sprintf("the saved template %s (%q, %dx%d cells, pivot %v,%v)",
				filepath.Base(sprsh), old.Name, old.CellW, old.CellH, old.PivotX, old.PivotY))
		}
	}
	if old := a.Project.LoadedSheets[imp.Name]; old != nil {
		if filepath.Clean(old.FilePath) != filepath.Clean(imp.FilePath) || old.CellW != imp.CellW ||
			old.CellH != imp.CellH || old.PivotX != imp.PivotX || old.PivotY != imp.PivotY {
			desc := fmt.Sprintf("the loaded sheet %q (%s, %dx%d cells)", imp.Name, filepath.Base(old.FilePath), old.CellW, old.CellH)
			if users := a.Project.CurrentTrack.SheetUsers(imp.Name); len(users) > 0 {
				desc += ", used by " + strings.Join(users, ", ")
			}
			out = append(out, desc)
		}
	}
	return out
}

// importSheet loads an accepted import into the project and writes its
// .sprsh beside the image.
func (a *Application) importSheet(imp ui.SheetImport, tmpl *editor.SpriteSheetTemplate) {
	a.Project.LoadedSheets[imp.Name] = tmpl
	a.Project.Dirty = true // the .anif records its sheets
	a.Project.PaletteSheet = imp.Name

	sprshPath := file.SprshPathFor(imp.FilePath)
	if err := file.SaveSheetTemplate(tmpl, sprshPath); err != nil {
		a.showError(fmt.Errorf("failed to save sheet template: %w", err))
	} else {
		tmpl.SprshPath = sprshPath
	}

	propNote := ""
	if imp.PropName != "" {
		if a.Project.CurrentTrack.FindProp(imp.PropName) == nil {
			a.Project.RecordUndo()
			editor.EnsureProp(a.Project.CurrentTrack, imp.PropName, imp.Name)
			propNote = fmt.Sprintf("\n\nDeclared prop %q with this sheet as its default.", imp.PropName)
		} else {
			propNote = fmt.Sprintf("\n\nAdded as another option for prop %q - "+
				"pick it under Preview Overrides to see it.", imp.PropName)
		}
	}

	a.refreshAll()
	dialog.ShowInformation("Import Complete",
		fmt.Sprintf("Imported %q: %dx%d cells, %d cols x %d rows.\n\n"+
			"Its tiles are in the left panel. Drag one onto the canvas to add it as a part.%s",
			imp.Name, imp.CellW, imp.CellH, tmpl.Cols(), tmpl.Rows(), propNote),
		a.Window)
}

// onLoadPreviewSheet lets the artist try other art on a prop on the fly —
// "show this as sprite_red.png instead" — without importing it into the
// track. Project.LoadPreviewSheet slices it on the prop's existing grid.
func (a *Application) onLoadPreviewSheet(propName string) {
	if pd := a.Project.CurrentTrack.FindProp(propName); pd != nil && pd.IsAnimProp() {
		a.loadPreviewAnim(propName)
		return
	}
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()

		img, err := file.LoadImage(filePath)
		if err != nil {
			a.showError(fmt.Errorf("failed to load image: %w", err))
			return
		}
		if _, err := a.Project.LoadPreviewSheet(propName, filePath, img); err != nil {
			a.showError(err)
			return
		}
		a.refreshAll()
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

// loadPreviewAnim is onLoadPreviewSheet for an animation prop: load any
// .anif and preview the prop as it, without changing the track.
func (a *Application) loadPreviewAnim(propName string) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()
		anim, problems := file.LoadNestedAnim(path, a.Project.LoadedAnims)
		a.reportAnimProblems(problems)
		if anim == nil {
			return
		}
		a.Project.PreviewProps[propName] = anim.Path
		a.refreshAll()
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

// onImportAnim nests another .anif in this track: the animation
// counterpart of importing a sprite sheet. It's added as a part at the
// origin, keyed at the playhead, ready to be dragged into place - unless
// it's being added as another option for an existing prop, which (as with
// sheets) just makes it available to swap to.
func (a *Application) onImportAnim() {
	ui.ShowImportAnimDialog(a.Window, func(path, propName string) {
		track := a.Project.CurrentTrack
		key := editor.AnimKey(path)
		if a.Project.SavePath != "" && key == editor.AnimKey(a.Project.SavePath) {
			a.showError(fmt.Errorf("a track can't contain itself"))
			return
		}
		var existing *editor.PropDef
		if propName != "" {
			if existing = track.FindProp(propName); existing != nil && !existing.IsAnimProp() {
				a.showError(fmt.Errorf("prop %q holds sprite sheets, not animations - choose another name", propName))
				return
			}
		}

		anim, problems := file.LoadNestedAnim(path, a.Project.LoadedAnims)
		if anim == nil {
			a.reportAnimProblems(problems)
			return
		}

		if existing != nil {
			// Nothing in the track changes: the animation is just loaded,
			// so it can be picked as the prop's preview value.
			a.refreshAll()
			a.reportAnimProblems(problems)
			dialog.ShowInformation("Animation Imported", fmt.Sprintf(
				"%s was added as another option for prop %q. Pick it under Preview Overrides to see it.",
				anim.DisplayName(), propName), a.Window)
			return
		}
		a.Project.RecordUndo()
		if propName != "" {
			editor.EnsureProp(track, propName, anim.Path)
		}

		base := strings.TrimSuffix(anim.DisplayName(), filepath.Ext(anim.DisplayName()))
		if propName != "" {
			base = propName
		}
		part := editor.NewNestedAniPart(editor.UniquePartName(track, base), anim.Path)
		part.GoverningProp = propName
		editor.AddPart(track, part)
		if dir := a.Project.ActiveDirection(); dir != nil {
			kf := editor.AddKeyframe(dir, part.ID, a.Project.Playback.ElapsedMs)
			kf.Z = float32(len(track.Parts)) // in front, like a dropped tile
		}
		a.properties.SelectPart(len(track.Parts) - 1)
		a.Project.Dirty = true
		a.refreshAll()
		a.reportAnimProblems(problems)
	})
}

// reportAnimProblems logs and shows anything that went wrong loading a
// nested animation (a missing file, or a sheet it needs).
func (a *Application) reportAnimProblems(problems []string) {
	if len(problems) == 0 {
		return
	}
	a.showError(fmt.Errorf("some of the nested animation couldn't be loaded:\n%s", strings.Join(problems, "\n")))
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

func (a *Application) onAddProp() {
	values := a.Project.LoadedSheetNames()
	labels := append([]string{}, values...)
	animPaths := a.Project.LoadedAnimPaths()
	values = append(values, animPaths...)
	labels = append(labels, ui.AnimLabels(animPaths)...)
	ui.ShowAddPropDialog(a.Window, labels, values, func(name, def string) {
		if a.Project.CurrentTrack.FindProp(name) != nil {
			a.showError(fmt.Errorf("there's already a prop called %q", name))
			return
		}
		a.Project.RecordUndo()
		editor.AddProp(a.Project.CurrentTrack, name, def)
		a.refreshAll()
	})
}

func (a *Application) onAddPart() {
	track := a.Project.CurrentTrack
	c := ui.AddPartChoices{Sheets: a.Project.LoadedSheetNames(), AnimPaths: a.Project.LoadedAnimPaths()}
	c.AnimLabels = ui.AnimLabels(c.AnimPaths)
	for _, p := range track.Props {
		if p.IsAnimProp() {
			c.AnimProps = append(c.AnimProps, p.Name)
		} else {
			c.SheetProps = append(c.SheetProps, p.Name)
		}
	}
	validateName := func(name string) error {
		_, err := editor.ValidatePartName(track, name, -1)
		return err
	}
	ui.ShowAddPartDialog(a.Window, c, validateName, func(part *editor.Part) error {
		if err := editor.CheckNewPart(track, part); err != nil {
			return err
		}
		// Selected straight away, so the left palette switches to its sheet
		// and the artist can drag a tile without a second click.
		a.addPart(part)
		a.refreshAll()
		return nil
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

// showAbout names the editor and where its session logs are - the file
// to attach to a bug report.
func (a *Application) showAbout() {
	msg := widget.NewLabel("ANIFile Animation Maker - authors rigged, multi-part sprite animations " +
		"(.anif) and sprite sheet templates (.sprsh). See docs/ANI_MAKER_SPEC.md.\n\n" +
		"Session logs (attach the latest to a bug report):")
	msg.Wrapping = fyne.TextWrapWord
	logs := widget.NewEntry()
	logs.SetText(applog.DefaultDir())
	d := dialog.NewCustom("About", "OK", container.NewVBox(msg, logs), a.Window)
	d.Resize(fyne.NewSize(520, 220))
	d.Show()
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

			// All of it on Fyne's main goroutine. This loop runs on its own
			// goroutine, and used to advance the project and redraw from
			// here - so the timeline could be drawing (iterating a part's
			// keyframes) while the UI goroutine was editing them. That is a
			// Go fatal error ("concurrent map iteration and map write"):
			// an instant, uncatchable crash. Fyne 2.6 requires UI work on
			// the main goroutine anyway.
			fyne.Do(func() {
				if a.Project.Playback.IsPlaying {
					a.Project.AdvancePlayback(deltaMs)
					a.canvasWidget.Refresh()
					a.timeline.Refresh()
				}
			})
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
	a.refreshDirectionSelect()
	a.updateTitle()
}

func (a *Application) onClose() {
	close(a.playbackDone)
}

// subPixelHeld reports whether Alt is down: placing a part on the canvas
// snaps to whole pixels unless it is (editor.SnapPosition).
func subPixelHeld() bool { return modifierHeld(fyne.KeyModifierAlt) }

// renamePart asks for a new name for the part, prefilled and selected so
// typing replaces it. Undoable; refused names (empty, taken) reopen the
// dialog with the error.
func (a *Application) renamePart(partIdx int) {
	if partIdx < 0 || partIdx >= len(a.Project.CurrentTrack.Parts) {
		return
	}
	current := a.Project.CurrentTrack.Parts[partIdx].Name
	ui.ShowRenamePartDialog(a.Window, current, func(name string) error {
		if strings.TrimSpace(name) == current {
			return nil
		}
		snap := a.Project.TakeSnapshot()
		if err := editor.RenamePart(a.Project.CurrentTrack, partIdx, name); err != nil {
			return err
		}
		a.Project.UndoStack.Push(snap)
		a.Project.Dirty = true
		a.refreshAll()
		return nil
	})
}
