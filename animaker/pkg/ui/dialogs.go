package ui

import (
	"animaker/pkg/editor"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// ShowNewTrackDialog displays a dialog for creating a new track (.anif).
func ShowNewTrackDialog(win fyne.Window, onCreate func(name string)) {
	nameEntry := newEntry()
	nameEntry.SetText("untitled")
	nameEntry.SetPlaceHolder("human_walk")
	nameEntry.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("the track needs a name")
		}
		return nil
	}

	form := dialog.NewForm(
		"New Track",
		"Create", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
		},
		func(confirmed bool) {
			if confirmed && onCreate != nil {
				onCreate(strings.TrimSpace(nameEntry.Text))
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 200))
	form.Show()
}

// SheetImport is what the import dialog collects.
type SheetImport struct {
	FilePath       string
	Name           string // the sheet's name: what parts, props and the palette refer to it by
	CellW, CellH   int
	PivotX, PivotY float32
	// PropName is the prop this sheet is art for, or "" for a sheet that
	// isn't swappable. See editor.EnsureProp.
	PropName string
}

// ShowImportSheetDialog displays a dialog for importing a sprite sheet
// template: pick a file, then name the sheet, define its fixed cell size
// and one pivot for the whole sheet, and say whether it's swappable art
// for a prop. prefill returns the settings to start from for an image
// that already has a template (ok false for a new one), so re-importing a
// sheet doesn't mean retyping its grid.
func ShowImportSheetDialog(win fyne.Window, prefill func(imagePath string) (SheetImport, bool), onImport func(SheetImport)) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()
		start := SheetImport{CellW: 32, CellH: 32, PivotX: 16, PivotY: 16}
		if prefill != nil {
			if p, ok := prefill(filePath); ok {
				start = p
			}
		}
		showSheetGridDialog(win, filePath, start, onImport)
	}, win)

	fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func showSheetGridDialog(win fyne.Window, filePath string, start SheetImport, onImport func(SheetImport)) {
	baseName := filepath.Base(filePath)
	defaultName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	if start.Name != "" {
		defaultName = start.Name
	}

	// Required: the form's Import button stays disabled while it's blank.
	nameEntry := newEntry()
	nameEntry.SetText(defaultName)
	nameEntry.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("the sheet needs a name")
		}
		return nil
	}

	// Validated, so Import stays disabled on a typo rather than quietly
	// slicing with a stand-in value.
	cellWEntry := validatedEntry(strconv.Itoa(start.CellW), positiveInt)
	cellHEntry := validatedEntry(strconv.Itoa(start.CellH), positiveInt)
	pivotXEntry := validatedEntry(formatFloat(start.PivotX), number)
	pivotYEntry := validatedEntry(formatFloat(start.PivotY), number)

	// The prop name defaults to the sheet name, so the common case (this
	// sheet is the first art for a new slot) is one tick. Naming an
	// existing prop adds this sheet as another option for it instead.
	propEntry := newEntry()
	propEntry.SetPlaceHolder("hair, arms, body, ...")
	propEntry.Disable()
	propCheck := widget.NewCheck("Swappable art (a prop)", func(on bool) {
		if on {
			if propEntry.Text == "" {
				propEntry.SetText(strings.TrimSpace(nameEntry.Text))
			}
			propEntry.Enable()
		} else {
			propEntry.Disable()
		}
	})
	propHint := widget.NewLabel("A prop is a slot whose art can be swapped per character, " +
		"e.g. \"hair\" switching between hair_short and hair_long. " +
		"Parts made from this sheet's tiles will follow it.")
	propHint.Wrapping = fyne.TextWrapWord

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
			{Text: "", Widget: propCheck},
			{Text: "Prop Name", Widget: propEntry},
			{Text: "", Widget: propHint},
		},
		func(confirmed bool) {
			if !confirmed || onImport == nil {
				return
			}
			imp := SheetImport{FilePath: filePath, Name: strings.TrimSpace(nameEntry.Text)}
			imp.CellW, _ = strconv.Atoi(strings.TrimSpace(cellWEntry.Text))
			imp.CellH, _ = strconv.Atoi(strings.TrimSpace(cellHEntry.Text))
			pivotX, _ := strconv.ParseFloat(strings.TrimSpace(pivotXEntry.Text), 32)
			pivotY, _ := strconv.ParseFloat(strings.TrimSpace(pivotYEntry.Text), 32)
			imp.PivotX, imp.PivotY = float32(pivotX), float32(pivotY)
			if propCheck.Checked {
				imp.PropName = strings.TrimSpace(propEntry.Text)
				if imp.PropName == "" {
					imp.PropName = imp.Name
				}
			}
			onImport(imp)
		},
		win,
	)
	form.Resize(fyne.NewSize(460, 520))
	form.Show()
}

func validatedEntry(text string, validate func(string) error) *selectAllEntry {
	e := newEntry()
	e.SetText(text)
	e.Validator = validate
	return e
}

func positiveInt(s string) error {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err != nil || n <= 0 {
		return errors.New("a whole number above 0")
	}
	return nil
}

func number(s string) error {
	if _, err := strconv.ParseFloat(strings.TrimSpace(s), 32); err != nil {
		return errors.New("a number")
	}
	return nil
}

func formatFloat(f float32) string { return strconv.FormatFloat(float64(f), 'f', -1, 32) }

// ShowAddDirectionDialog displays a dialog for adding a new direction.
// Directions are keyed by a whole number 0 or above (0=up, 1=right, 2=down,
// 3=left by the game's own convention; more for diagonals or extra
// facings). exists reports a key the track already has; Add stays
// disabled for that, or for anything that isn't such a number.
func ShowAddDirectionDialog(win fyne.Window, exists func(key int) bool, onCreate func(key int)) {
	keyEntry := newEntry()
	keyEntry.SetPlaceHolder("0=up, 1=right, 2=down, 3=left, ...")
	keyEntry.Validator = func(s string) error {
		k, err := strconv.Atoi(strings.TrimSpace(s))
		switch {
		case err != nil || k < 0:
			return errors.New("a whole number, 0 or above")
		case exists != nil && exists(k):
			return fmt.Errorf("direction %d already exists", k)
		}
		return nil
	}

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
			key, err := strconv.Atoi(strings.TrimSpace(keyEntry.Text))
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

// ShowAddPropDialog displays a dialog for declaring a new prop on the
// track. The default is a pick-list of imported sheets, not free text: a
// default naming no loaded sheet makes every part linked to the prop draw
// nothing, which is exactly how typing "body" here made a sprite vanish.
//
// labels/values are the default's options, in parallel: loaded sheets
// (label = value = sheet name) and loaded nested animations (label =
// "torch.anif", value = its path). The value type decides the prop's kind:
// a sheet prop governs sheet parts, an .anif prop governs nested parts.
func ShowAddPropDialog(win fyne.Window, labels, values []string, onCreate func(name, defaultValue string)) {
	nameEntry := newEntry()
	nameEntry.SetPlaceHolder("hair, arms, legs, ...")
	nameEntry.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("the prop needs a name")
		}
		return nil
	}
	defaultSelect := widget.NewSelect(labels, nil)
	defaultSelect.PlaceHolder = "(import a sheet or animation first)"
	if len(labels) > 0 {
		defaultSelect.SetSelected(labels[0])
	}
	hint := widget.NewLabel("A prop is a swappable art slot. Its value is a sheet (for sheet " +
		"parts) or an .anif (for nested animation parts); linked parts use whichever it's set to.")
	hint.Wrapping = fyne.TextWrapWord

	form := dialog.NewForm(
		"Add Prop",
		"Add", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
			{Text: "Default", Widget: defaultSelect},
			{Text: "", Widget: hint},
		},
		func(confirmed bool) {
			if confirmed && onCreate != nil {
				onCreate(strings.TrimSpace(nameEntry.Text), valueFor(labels, values, defaultSelect.Selected))
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(420, 280))
	form.Show()
}

// AddPartChoices is what the Add Part dialog offers: the track's props by
// kind (a sheet part can only be governed by a sheet prop, a nested part
// only by an animation prop), the loaded sheets, and the loaded animations
// (labels and paths in parallel).
type AddPartChoices struct {
	SheetProps, AnimProps []string
	Sheets                []string
	AnimLabels, AnimPaths []string
}

// ShowAddPartDialog displays a dialog for adding a part to the rig.
// Everything but the name is a pick-list of what exists, so a typo can't
// produce a part that silently draws nothing. validateName vets the name
// as it's typed (Add stays disabled until it passes); onCreate gets the
// new part and may refuse it, which reopens the dialog with the error.
func ShowAddPartDialog(win fyne.Window, c AddPartChoices, validateName func(string) error, onCreate func(*editor.Part) error) {
	nameEntry := newEntry()
	nameEntry.SetPlaceHolder("Body, Hair, Arm_Left, ...")
	nameEntry.Validator = validateName

	const noProp = "(none)"
	propSelect := widget.NewSelect(nil, nil)
	fixedSheetSelect := widget.NewSelect(c.Sheets, nil)
	fixedSheetSelect.PlaceHolder = "(pick a loaded sheet)"
	if len(c.Sheets) == 1 {
		fixedSheetSelect.SetSelected(c.Sheets[0])
	}
	// A pick-list of imported animations rather than a typed path, which
	// was easy to get wrong and gave no hint which files were available.
	nestedSelect := widget.NewSelect(c.AnimLabels, nil)
	nestedSelect.PlaceHolder = "(use Import Animation first)"
	if len(c.AnimLabels) == 1 {
		nestedSelect.SetSelected(c.AnimLabels[0])
	}

	// The prop list and which art picker applies follow the kind.
	kindSelect := widget.NewSelect([]string{"sheet", "nested_ani"}, func(kind string) {
		props := c.SheetProps
		if kind == "nested_ani" {
			props = c.AnimProps
			fixedSheetSelect.Disable()
			nestedSelect.Enable()
		} else {
			fixedSheetSelect.Enable()
			nestedSelect.Disable()
		}
		propSelect.Options = append([]string{noProp}, props...)
		propSelect.SetSelected(noProp)
	})
	kindSelect.SetSelected("sheet")

	items := []*widget.FormItem{
		{Text: "Name", Widget: nameEntry},
		{Text: "Kind", Widget: kindSelect},
		{Text: "Governing Prop", Widget: propSelect},
		{Text: "Fixed Sheet", Widget: fixedSheetSelect},
		{Text: "Nested Animation", Widget: nestedSelect},
	}
	if len(c.Sheets) == 0 {
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
			if !confirmed || onCreate == nil {
				return
			}
			var part *editor.Part
			if kindSelect.Selected == "nested_ani" {
				part = editor.NewNestedAniPart(nameEntry.Text, valueFor(c.AnimLabels, c.AnimPaths, nestedSelect.Selected))
			} else {
				part = editor.NewSheetPart(nameEntry.Text, "", fixedSheetSelect.Selected)
			}
			if propSelect.Selected != noProp {
				part.GoverningProp = propSelect.Selected
			}
			if err := onCreate(part); err != nil {
				errDlg := dialog.NewError(err, win)
				errDlg.SetOnClosed(func() { ShowAddPartDialog(win, c, validateName, onCreate) })
				errDlg.Show()
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(450, 440))
	form.Show()
}

// ShowRenamePartDialog asks for a new name for a part, prefilled with the
// current one. onRename returns an error to refuse the name (empty, or
// taken); the dialog then shows it and reopens with what was typed, so the
// artist can correct it rather than start over.
func ShowRenamePartDialog(win fyne.Window, current string, onRename func(name string) error) {
	nameEntry := newEntry()
	nameEntry.SetText(current)

	form := dialog.NewForm(
		"Rename Part",
		"Rename", "Cancel",
		[]*widget.FormItem{
			{Text: "Name", Widget: nameEntry},
		},
		func(confirmed bool) {
			if !confirmed || onRename == nil {
				return
			}
			if err := onRename(nameEntry.Text); err != nil {
				typed := nameEntry.Text
				errDlg := dialog.NewError(err, win)
				errDlg.SetOnClosed(func() { ShowRenamePartDialog(win, typed, onRename) })
				errDlg.Show()
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(400, 160))
	form.Show()
	// Focused, which selects the text (newEntry), so typing replaces
	// "sprite_1" outright and Enter confirms.
	win.Canvas().Focus(nameEntry)
}

// ShowCopyTimingDialog offers to scaffold an empty direction with another
// direction's keyframe times. sources must be non-empty; its first entry
// is preselected (direction 0 whenever it has keyframes). onCopy runs only
// if the artist chooses to copy.
func ShowCopyTimingDialog(win fyne.Window, target int, sources []int, onCopy func(src int)) {
	options := make([]string, len(sources))
	for i, k := range sources {
		options[i] = directionName(k)
	}
	srcSelect := widget.NewSelect(options, nil)
	srcSelect.SetSelected(options[0])

	msg := widget.NewLabel(fmt.Sprintf("The %s direction has no keyframes yet. Copy the keyframe times "+
		"from another direction as a starting point? Only the timing is copied: every "+
		"copied keyframe starts at the origin on cell (0,0), ready for you to pose.", directionName(target)))
	msg.Wrapping = fyne.TextWrapWord

	form := dialog.NewForm(
		"Empty Direction",
		"Copy Times", "Start Empty",
		[]*widget.FormItem{
			{Text: "", Widget: msg},
			{Text: "Copy from direction", Widget: srcSelect},
		},
		func(confirmed bool) {
			if !confirmed || onCopy == nil {
				return
			}
			if i := slices.Index(options, srcSelect.Selected); i >= 0 {
				onCopy(sources[i])
			}
		},
		win,
	)
	form.Resize(fyne.NewSize(440, 260))
	form.Show()
}
