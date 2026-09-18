package ui

import (
	"animaker/pkg/editor"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// ShowNewTrackDialog displays a dialog for creating a new track (.anif).
func ShowNewTrackDialog(win fyne.Window, onCreate func(name string)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText("untitled")
	nameEntry.SetPlaceHolder("human_walk")

	form := dialog.NewForm(
		"New Track",
		"Create", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
		},
		func(confirmed bool) {
			if confirmed && onCreate != nil {
				onCreate(nameEntry.Text)
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 200))
	form.Show()
}

// ShowImportSheetDialog displays a dialog for importing a sprite sheet
// template: pick a file, then define its fixed cell size and one pivot for
// the whole sheet.
func ShowImportSheetDialog(win fyne.Window, onImport func(filePath, name string, cellW, cellH int, pivotX, pivotY float32)) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()
		showSheetGridDialog(win, filePath, onImport)
	}, win)

	fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func showSheetGridDialog(win fyne.Window, filePath string, onImport func(string, string, int, int, float32, float32)) {
	baseName := filepath.Base(filePath)
	defaultName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	nameEntry := widget.NewEntry()
	nameEntry.SetText(defaultName)

	cellWEntry := widget.NewEntry()
	cellWEntry.SetText("32")
	cellHEntry := widget.NewEntry()
	cellHEntry.SetText("32")
	pivotXEntry := widget.NewEntry()
	pivotXEntry.SetText("16")
	pivotYEntry := widget.NewEntry()
	pivotYEntry.SetText("16")

	form := dialog.NewForm(
		"Import Sprite Sheet",
		"Import", "Cancel",
		[]*widget.FormItem{
			{Text: "File", Widget: widget.NewLabel(filePath)},
			{Text: "Sheet Name", Widget: nameEntry},
			{Text: "Cell Width", Widget: cellWEntry},
			{Text: "Cell Height", Widget: cellHEntry},
			{Text: "Pivot X", Widget: pivotXEntry},
			{Text: "Pivot Y", Widget: pivotYEntry},
		},
		func(confirmed bool) {
			if !confirmed || onImport == nil {
				return
			}
			cellW, _ := strconv.Atoi(cellWEntry.Text)
			cellH, _ := strconv.Atoi(cellHEntry.Text)
			pivotX, _ := strconv.ParseFloat(pivotXEntry.Text, 32)
			pivotY, _ := strconv.ParseFloat(pivotYEntry.Text, 32)
			if cellW <= 0 {
				cellW = 32
			}
			if cellH <= 0 {
				cellH = 32
			}
			onImport(filePath, nameEntry.Text, cellW, cellH, float32(pivotX), float32(pivotY))
		},
		win,
	)
	form.Resize(fyne.NewSize(420, 380))
	form.Show()
}

// ShowAddDirectionDialog displays a dialog for adding a new direction.
// Directions are keyed by int (0=up, 1=right, 2=down, 3=left by the game's
// own convention, but any int is accepted).
func ShowAddDirectionDialog(win fyne.Window, onCreate func(key int)) {
	keyEntry := widget.NewEntry()
	keyEntry.SetPlaceHolder("0=up, 1=right, 2=down, 3=left, ...")

	form := dialog.NewForm(
		"Add Direction",
		"Add", "Cancel",
		[]*widget.FormItem{
			{Text: "Direction (int)", Widget: keyEntry},
		},
		func(confirmed bool) {
			if !confirmed || onCreate == nil {
				return
			}
			key, err := strconv.Atoi(keyEntry.Text)
			if err != nil {
				return
			}
			onCreate(key)
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 180))
	form.Show()
}

// ShowAddPropDialog displays a dialog for declaring a new prop on the track.
func ShowAddPropDialog(win fyne.Window, onCreate func(name, defaultSheet string)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("hair, arms, legs, ...")
	defaultEntry := widget.NewEntry()
	defaultEntry.SetPlaceHolder("sheet name to use by default")

	form := dialog.NewForm(
		"Add Prop",
		"Add", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
			{Text: "Default Sheet", Widget: defaultEntry},
		},
		func(confirmed bool) {
			if confirmed && onCreate != nil && nameEntry.Text != "" {
				onCreate(nameEntry.Text, defaultEntry.Text)
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 220))
	form.Show()
}

// ShowAddPartDialog displays a dialog for adding a part to the active
// direction. propNames lists the track's declared props, for the
// governing-prop dropdown; sheetNames lists the currently loaded sheets, so
// the fixed sheet is picked from what exists rather than typed from memory
// (a typo there produces a part that silently draws nothing).
func ShowAddPartDialog(win fyne.Window, propNames, sheetNames []string, onCreate func(name string, kind editor.PartKind, governingProp, fixedSheet, nestedPath string)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Body, Hair, Arm_Left, ...")

	kindSelect := widget.NewSelect([]string{"sheet", "nested_ani"}, nil)
	kindSelect.SetSelected("sheet")

	propOptions := append([]string{"(none - fixed sheet)"}, propNames...)
	propSelect := widget.NewSelect(propOptions, nil)
	propSelect.SetSelected(propOptions[0])

	fixedSheetSelect := widget.NewSelect(sheetNames, nil)
	fixedSheetSelect.PlaceHolder = "(pick a loaded sheet)"
	if len(sheetNames) == 1 {
		fixedSheetSelect.SetSelected(sheetNames[0])
	}

	nestedPathEntry := widget.NewEntry()
	nestedPathEntry.SetPlaceHolder("base_wood_torch.anif")

	items := []*widget.FormItem{
		{Text: "Name", Widget: nameEntry},
		{Text: "Kind", Widget: kindSelect},
		{Text: "Governing Prop", Widget: propSelect},
		{Text: "Fixed Sheet", Widget: fixedSheetSelect},
		{Text: "Nested .anif Path", Widget: nestedPathEntry},
	}
	if len(sheetNames) == 0 {
		items = append(items, &widget.FormItem{
			Text:   "",
			Widget: widget.NewLabel("No sheets imported yet — use Import Sprite Sheet first."),
		})
	}

	form := dialog.NewForm(
		"Add Part",
		"Add", "Cancel",
		items,
		func(confirmed bool) {
			if !confirmed || onCreate == nil || nameEntry.Text == "" {
				return
			}
			kind := editor.PartKindSheet
			if kindSelect.Selected == "nested_ani" {
				kind = editor.PartKindNestedAni
			}
			governingProp := propSelect.Selected
			if governingProp == propOptions[0] {
				governingProp = ""
			}
			onCreate(nameEntry.Text, kind, governingProp, fixedSheetSelect.Selected, nestedPathEntry.Text)
		},
		win,
	)
	form.Resize(fyne.NewSize(450, 440))
	form.Show()
}
