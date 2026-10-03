package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// BuildMainLayout assembles the main window layout, GraalShop-style: a
// persistent sprite palette on the far left, canvas in the middle,
// direction/part/keyframe/props controls on the right, timeline across
// the bottom. propertiesPanel is expected to already have the direction
// bar embedded as its own header (see PropertiesPanel.Build) rather than
// this function owning a separate top strip.
func BuildMainLayout(
	palettePanel fyne.CanvasObject,
	canvasWidget fyne.CanvasObject,
	propertiesPanel fyne.CanvasObject,
	timelinePanel fyne.CanvasObject,
) fyne.CanvasObject {
	// Canvas (65%) | Properties (35%)
	canvasAndProps := container.NewHSplit(canvasWidget, propertiesPanel)
	canvasAndProps.SetOffset(0.65)

	// Palette (20%) | everything else (80%)
	topRow := container.NewHSplit(palettePanel, canvasAndProps)
	topRow.SetOffset(0.20)

	// Top row (75%) | Timeline (25%)
	mainSplit := container.NewVSplit(topRow, timelinePanel)
	mainSplit.SetOffset(0.75)

	return mainSplit
}

// BuildMenuBar creates the application menu bar. Rig-building actions (add
// direction/prop/part) deliberately live in the panels themselves, not
// here — see the direction bar and PropertiesPanel's PARTS/PROPS sections.
func BuildMenuBar(
	onNew func(),
	onNewFromRig func(),
	onOpen func(),
	onSave func(),
	onSaveAs func(),
	onImportSheet func(),
	onImportAnim func(),
	onUndo func(),
	onRedo func(),
	onToggleGrid func(),
	onToggleOnion func(),
	onZoom func(float32),
	onAbout func(),
	onShortcuts func(),
) *fyne.MainMenu {
	// The shortcuts themselves are registered on the window canvas (see
	// app.registerShortcuts); setting them here shows them in the menu.
	item := func(label string, action func(), key fyne.KeyName, mod fyne.KeyModifier) *fyne.MenuItem {
		mi := fyne.NewMenuItem(label, action)
		mi.Shortcut = &desktop.CustomShortcut{KeyName: key, Modifier: mod}
		return mi
	}
	ctrl, ctrlShift := fyne.KeyModifierControl, fyne.KeyModifierControl|fyne.KeyModifierShift

	fileMenu := fyne.NewMenu("File",
		item("New Track", onNew, fyne.KeyN, ctrl),
		fyne.NewMenuItem("New Track from Rig...", onNewFromRig),
		fyne.NewMenuItemSeparator(),
		item("Open...", onOpen, fyne.KeyO, ctrl),
		item("Save", onSave, fyne.KeyS, ctrl),
		item("Save As...", onSaveAs, fyne.KeyS, ctrlShift),
		fyne.NewMenuItemSeparator(),
		item("Import Sprite Sheet...", onImportSheet, fyne.KeyI, ctrl),
		fyne.NewMenuItem("Import Animation...", onImportAnim),
	)

	editMenu := fyne.NewMenu("Edit",
		item("Undo", onUndo, fyne.KeyZ, ctrl),
		item("Redo", onRedo, fyne.KeyY, ctrl),
	)

	viewMenu := fyne.NewMenu("View",
		fyne.NewMenuItem("Toggle Grid", onToggleGrid),
		fyne.NewMenuItem("Toggle Onion Skin (O)", onToggleOnion),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Zoom 100%", func() { onZoom(1.0) }),
		fyne.NewMenuItem("Zoom 200%", func() { onZoom(2.0) }),
		fyne.NewMenuItem("Zoom 400%", func() { onZoom(4.0) }),
		fyne.NewMenuItem("Zoom 800%", func() { onZoom(8.0) }),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("Keyboard Shortcuts", onShortcuts),
		fyne.NewMenuItem("About", onAbout),
	)

	return fyne.NewMainMenu(fileMenu, editMenu, viewMenu, helpMenu)
}

// ShortcutHelp lists the editor's keyboard shortcuts, as shown by Help >
// Keyboard Shortcuts. Keep it in step with app.registerShortcuts and
// app.onTypedKey.
var ShortcutHelp = [][2]string{
	{"Space", "Play / pause (pause keeps the playhead; Stop rewinds)"},
	{", / .", "Previous / next keyframe"},
	{"Shift+, / Shift+.", "Step back / forward 50ms"},
	{"Arrows", "Nudge the selected part 1px (Shift: 10px), keying it at the playhead"},
	{"Delete", "Delete the selected keyframe, or the selected part's keyframe at the playhead"},
	{"1 / 2 / 3 / 4", "Up / Right / Down / Left direction"},
	{"Right-click a direction tab", "Change its key, delete it, add facings, remove empty directions"},
	{"Esc", "Deselect (the next palette drop adds a new part)"},
	{"F2", "Rename the selected part (also: double-click its name in the timeline)"},
	{"O", "Onion skin: show the selected part's neighbouring keyframe poses faintly"},
	{"Ctrl+] / Ctrl+[", "Selected part forward / back in draw order"},
	{"Ctrl+Shift+] / [", "Selected part to the front / back"},
	{"Alt (while dragging)", "Place between whole pixels"},
	{"Ctrl+wheel (timeline)", "Zoom the timeline"},
	{"Ctrl+N / O / S", "New / Open / Save"},
	{"Ctrl+Shift+S", "Save As"},
	{"Ctrl+I", "Import Sprite Sheet"},
	{"Ctrl+Z / Ctrl+Y", "Undo / Redo (also Ctrl+Shift+Z)"},
}

// ShowShortcutsDialog shows ShortcutHelp. Plain keys work while no text
// field has focus - click the canvas first if one does.
func ShowShortcutsDialog(win fyne.Window) {
	grid := container.NewGridWithColumns(2)
	for _, s := range ShortcutHelp {
		key := widget.NewLabel(s[0])
		key.TextStyle = fyne.TextStyle{Bold: true}
		desc := widget.NewLabel(s[1])
		desc.Wrapping = fyne.TextWrapWord
		grid.Add(key)
		grid.Add(desc)
	}
	note := widget.NewLabel("Single keys work while no text field has focus - click the canvas first if one does.")
	note.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustom("Keyboard Shortcuts", "Close",
		container.NewBorder(nil, note, nil, nil, container.NewVScroll(grid)), win)
	d.Resize(fyne.NewSize(620, 560))
	d.Show()
}
