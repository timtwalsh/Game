package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// BuildMainLayout assembles the main window layout with all panels.
func BuildMainLayout(
	canvasWidget fyne.CanvasObject,
	propertiesPanel fyne.CanvasObject,
	timelinePanel fyne.CanvasObject,
) fyne.CanvasObject {
	// Horizontal split: Canvas (70%) | Properties (30%)
	topSplit := container.NewHSplit(canvasWidget, propertiesPanel)
	topSplit.SetOffset(0.70)

	// Vertical split: Top panels | Timeline
	mainSplit := container.NewVSplit(topSplit, timelinePanel)
	mainSplit.SetOffset(0.70)

	return mainSplit
}

// BuildMenuBar creates the application menu bar.
func BuildMenuBar(
	onNew func(),
	onOpen func(),
	onSave func(),
	onSaveAs func(),
	onImportSheet func(),
	onUndo func(),
	onRedo func(),
	onToggleGrid func(),
	onToggleHitboxes func(),
	onZoom func(float32),
) *fyne.MainMenu {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Animation", onNew),
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
		fyne.NewMenuItem("Toggle Hitboxes", onToggleHitboxes),
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
