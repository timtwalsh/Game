package ui

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// AnimLabels labels nested-animation paths for pickers by file name
// ("torch.anif"), falling back to the full path for any two that share a
// file name, so every label is unique and maps back to one path.
func AnimLabels(paths []string) []string {
	count := map[string]int{}
	for _, p := range paths {
		count[strings.ToLower(filepath.Base(p))]++
	}
	labels := make([]string, len(paths))
	for i, p := range paths {
		if count[strings.ToLower(filepath.Base(p))] > 1 {
			labels[i] = p
		} else {
			labels[i] = filepath.Base(p)
		}
	}
	return labels
}

// labelFor / valueFor map between a picker's labels and the values they
// stand for (parallel slices).
func labelFor(labels, values []string, value string) string {
	for i, v := range values {
		if v == value {
			return labels[i]
		}
	}
	return value
}

func valueFor(labels, values []string, label string) string {
	for i, l := range labels {
		if l == label {
			return values[i]
		}
	}
	return label
}

// ShowImportAnimDialog picks an .anif to nest in this track - the
// counterpart of importing a sprite sheet - and asks whether it's
// swappable art for a prop, exactly as the sheet import does.
func ShowImportAnimDialog(win fyne.Window, onImport func(path, propName string)) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()
		showImportAnimOptions(win, path, onImport)
	}, win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func showImportAnimOptions(win fyne.Window, path string, onImport func(path, propName string)) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	propEntry := newEntry()
	propEntry.SetPlaceHolder("held_item, light, ...")
	propEntry.Disable()
	propCheck := widget.NewCheck("Swappable (a prop)", func(on bool) {
		if on {
			if propEntry.Text == "" {
				propEntry.SetText(base)
			}
			propEntry.Enable()
		} else {
			propEntry.Disable()
		}
	})
	hint := widget.NewLabel("It's added as a part playing this animation, at the origin and " +
		"the playhead - drag it into place. As a prop, it can be swapped for another .anif " +
		"(e.g. torch for lantern); naming an existing prop adds this as another option instead.")
	hint.Wrapping = fyne.TextWrapWord

	form := dialog.NewForm(
		"Import Animation",
		"Import", "Cancel",
		[]*widget.FormItem{
			{Text: "File", Widget: widget.NewLabel(path)},
			{Text: "", Widget: propCheck},
			{Text: "Prop Name", Widget: propEntry},
			{Text: "", Widget: hint},
		},
		func(confirmed bool) {
			if !confirmed || onImport == nil {
				return
			}
			prop := ""
			if propCheck.Checked {
				if prop = strings.TrimSpace(propEntry.Text); prop == "" {
					prop = base
				}
			}
			onImport(path, prop)
		},
		win,
	)
	form.Resize(fyne.NewSize(480, 340))
	form.Show()
}
