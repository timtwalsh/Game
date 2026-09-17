package ui

import (
	"animaker/pkg/editor"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// ShowNewAnimationDialog displays a dialog for creating a new animation.
func ShowNewAnimationDialog(win fyne.Window, onCreate func(name string, charSize string, loop bool, anchor string)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText("untitled")
	nameEntry.SetPlaceHolder("animation_name")

	sizeSelect := widget.NewSelect(
		[]string{"16x16", "24x24", "32x32", "48x32", "64x64", "custom"},
		nil,
	)
	sizeSelect.SetSelected("32x32")

	loopCheck := widget.NewCheck("Loop animation", nil)
	loopCheck.Checked = true

	anchorSelect := widget.NewSelect([]string{"feet", "center"}, nil)
	anchorSelect.SetSelected("feet")

	form := dialog.NewForm(
		"New Animation",
		"Create", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
			{Text: "Character Size", Widget: sizeSelect},
			{Text: "Loop", Widget: loopCheck},
			{Text: "Root Anchor", Widget: anchorSelect},
		},
		func(confirmed bool) {
			if confirmed && onCreate != nil {
				onCreate(nameEntry.Text, sizeSelect.Selected, loopCheck.Checked, anchorSelect.Selected)
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 300))
	form.Show()
}

// ShowImportSheetDialog displays a dialog for importing a sprite sheet.
func ShowImportSheetDialog(win fyne.Window, onImport func(filePath string, config editor.GridConfig)) {
	// Step 1: File picker
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()

		// Step 2: Grid config dialog
		showGridConfigDialog(win, filePath, onImport)
	}, win)

	fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

// showGridConfigDialog shows the grid configuration after a file is selected.
func showGridConfigDialog(win fyne.Window, filePath string, onImport func(string, editor.GridConfig)) {
	colsEntry := widget.NewEntry()
	colsEntry.SetText("4")
	colsEntry.SetPlaceHolder("Columns")

	rowsEntry := widget.NewEntry()
	rowsEntry.SetText("4")
	rowsEntry.SetPlaceHolder("Rows")

	tileWEntry := widget.NewEntry()
	tileWEntry.SetText("32")
	tileWEntry.SetPlaceHolder("Tile Width")

	tileHEntry := widget.NewEntry()
	tileHEntry.SetText("32")
	tileHEntry.SetPlaceHolder("Tile Height")

	form := dialog.NewForm(
		"Import Sprite Sheet",
		"Import", "Cancel",
		[]*widget.FormItem{
			{Text: "File", Widget: widget.NewLabel(filePath)},
			{Text: "Columns", Widget: colsEntry},
			{Text: "Rows", Widget: rowsEntry},
			{Text: "Tile Width", Widget: tileWEntry},
			{Text: "Tile Height", Widget: tileHEntry},
		},
		func(confirmed bool) {
			if !confirmed || onImport == nil {
				return
			}

			cols, _ := strconv.Atoi(colsEntry.Text)
			rows, _ := strconv.Atoi(rowsEntry.Text)
			tileW, _ := strconv.Atoi(tileWEntry.Text)
			tileH, _ := strconv.Atoi(tileHEntry.Text)

			if cols <= 0 {
				cols = 1
			}
			if rows <= 0 {
				rows = 1
			}
			if tileW <= 0 {
				tileW = 32
			}
			if tileH <= 0 {
				tileH = 32
			}

			onImport(filePath, editor.GridConfig{
				Cols:  cols,
				Rows:  rows,
				TileW: tileW,
				TileH: tileH,
			})
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 350))
	form.Show()
}

// ShowSoundEventDialog displays a dialog for adding/editing a sound event.
func ShowSoundEventDialog(win fyne.Window, existing *editor.SoundEvent, onSave func(*editor.SoundEvent)) {
	fileEntry := widget.NewEntry()
	fileEntry.SetPlaceHolder("path/to/sound.wav")
	pitchEntry := widget.NewEntry()
	pitchEntry.SetText("1.0")

	if existing != nil {
		fileEntry.SetText(existing.FilePath)
		pitchEntry.SetText(strconv.FormatFloat(float64(existing.Pitch), 'f', 1, 32))
	}

	form := dialog.NewForm(
		"Sound Event",
		"Save", "Cancel",
		[]*widget.FormItem{
			{Text: "File Path", Widget: fileEntry},
			{Text: "Pitch", Widget: pitchEntry},
		},
		func(confirmed bool) {
			if confirmed && onSave != nil {
				pitch, _ := strconv.ParseFloat(pitchEntry.Text, 32)
				if pitch <= 0 {
					pitch = 1.0
				}
				onSave(&editor.SoundEvent{FilePath: fileEntry.Text, Pitch: float32(pitch)})
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 200))
	form.Show()
}

// ShowParticleEventDialog displays a dialog for adding/editing a particle event.
func ShowParticleEventDialog(win fyne.Window, existing *editor.ParticleEvent, onSave func(*editor.ParticleEvent)) {
	typeEntry := widget.NewEntry()
	typeEntry.SetPlaceHolder("slash_spark")
	xEntry := widget.NewEntry()
	xEntry.SetText("0")
	yEntry := widget.NewEntry()
	yEntry.SetText("0")

	if existing != nil {
		typeEntry.SetText(existing.Type)
		xEntry.SetText(strconv.Itoa(existing.X))
		yEntry.SetText(strconv.Itoa(existing.Y))
	}

	form := dialog.NewForm(
		"Particle Event",
		"Save", "Cancel",
		[]*widget.FormItem{
			{Text: "Type", Widget: typeEntry},
			{Text: "X Offset", Widget: xEntry},
			{Text: "Y Offset", Widget: yEntry},
		},
		func(confirmed bool) {
			if confirmed && onSave != nil {
				x, _ := strconv.Atoi(xEntry.Text)
				y, _ := strconv.Atoi(yEntry.Text)
				onSave(&editor.ParticleEvent{Type: typeEntry.Text, X: x, Y: y})
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 250))
	form.Show()
}

// ShowShakeEventDialog displays a dialog for adding/editing a shake event.
func ShowShakeEventDialog(win fyne.Window, existing *editor.ShakeEvent, onSave func(*editor.ShakeEvent)) {
	durEntry := widget.NewEntry()
	durEntry.SetText("50")
	intensitySlider := widget.NewSlider(0, 1)
	intensitySlider.Step = 0.05
	intensitySlider.Value = 0.5

	if existing != nil {
		durEntry.SetText(strconv.FormatUint(uint64(existing.DurationMs), 10))
		intensitySlider.Value = float64(existing.Intensity)
	}

	items := []*widget.FormItem{
		{Text: "Duration (ms)", Widget: durEntry},
		{Text: "Intensity", Widget: container.NewHBox(intensitySlider)},
	}

	form := dialog.NewForm(
		"Shake Event",
		"Save", "Cancel",
		items,
		func(confirmed bool) {
			if confirmed && onSave != nil {
				dur, _ := strconv.ParseUint(durEntry.Text, 10, 32)
				onSave(&editor.ShakeEvent{
					DurationMs: uint32(dur),
					Intensity:  float32(intensitySlider.Value),
				})
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 200))
	form.Show()
}

// ShowFlashEventDialog displays a dialog for adding/editing a flash event.
func ShowFlashEventDialog(win fyne.Window, existing *editor.FlashEvent, onSave func(*editor.FlashEvent)) {
	colorEntry := widget.NewEntry()
	colorEntry.SetText("#FFFFFF")
	durEntry := widget.NewEntry()
	durEntry.SetText("30")
	opacitySlider := widget.NewSlider(0, 1)
	opacitySlider.Step = 0.05
	opacitySlider.Value = 0.3

	if existing != nil {
		colorEntry.SetText(existing.Color)
		durEntry.SetText(strconv.FormatUint(uint64(existing.DurationMs), 10))
		opacitySlider.Value = float64(existing.Opacity)
	}

	form := dialog.NewForm(
		"Flash Event",
		"Save", "Cancel",
		[]*widget.FormItem{
			{Text: "Color (hex)", Widget: colorEntry},
			{Text: "Duration (ms)", Widget: durEntry},
			{Text: "Opacity", Widget: container.NewHBox(opacitySlider)},
		},
		func(confirmed bool) {
			if confirmed && onSave != nil {
				dur, _ := strconv.ParseUint(durEntry.Text, 10, 32)
				onSave(&editor.FlashEvent{
					Color:      colorEntry.Text,
					DurationMs: uint32(dur),
					Opacity:    float32(opacitySlider.Value),
				})
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 250))
	form.Show()
}
