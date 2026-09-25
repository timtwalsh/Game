package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
	onOpen func(),
	onSave func(),
	onSaveAs func(),
	onImportSheet func(),
	onUndo func(),
	onRedo func(),
	onToggleGrid func(),
	onZoom func(float32),
) *fyne.MainMenu {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Track", onNew),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Open...", onOpen),
		fyne.NewMenuItem("Save", onSave),
		fyne.NewMenuItem("Save As...", onSaveAs),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Import Sprite Sheet...", onImportSheet),
	)

	editMenu := fyne.NewMenu("Edit",
		fyne.NewMenuItem("Undo", onUndo),
		fyne.NewMenuItem("Redo", onRedo),
	)

	viewMenu := fyne.NewMenu("View",
		fyne.NewMenuItem("Toggle Grid", onToggleGrid),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Zoom 100%", func() { onZoom(1.0) }),
		fyne.NewMenuItem("Zoom 200%", func() { onZoom(2.0) }),
		fyne.NewMenuItem("Zoom 400%", func() { onZoom(4.0) }),
		fyne.NewMenuItem("Zoom 800%", func() { onZoom(8.0) }),
	)

	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			// TODO: show about dialog
		}),
	)

	return fyne.NewMainMenu(fileMenu, editMenu, viewMenu, helpMenu)
}
