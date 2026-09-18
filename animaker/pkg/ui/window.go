package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// BuildMainLayout assembles the main window layout with all panels.
// directionBar sits above everything (direction tabs + track name).
func BuildMainLayout(
	directionBar fyne.CanvasObject,
	canvasWidget fyne.CanvasObject,
	propertiesPanel fyne.CanvasObject,
	timelinePanel fyne.CanvasObject,
) fyne.CanvasObject {
	// Horizontal split: Canvas (50%) | Properties (50%)
	topSplit := container.NewHSplit(canvasWidget, propertiesPanel)
	topSplit.SetOffset(0.50)

	// Vertical split: Top panels (75%) | Timeline (25%)
	mainSplit := container.NewVSplit(topSplit, timelinePanel)
	mainSplit.SetOffset(0.75)

	return container.NewBorder(directionBar, nil, nil, nil, mainSplit)
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
