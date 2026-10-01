package editor

import (
	"fmt"
	"strings"
)

// SheetUsers describes everything in the track that draws from the named
// sheet: parts with it as their fixed sheet, and props with it as their
// default. Empty means nothing would lose its art if the sheet went away.
func (t *Track) SheetUsers(name string) []string {
	var users []string
	for _, p := range t.Parts {
		if p.Kind == PartKindSheet && p.FixedSheet == name {
			users = append(users, fmt.Sprintf("part %q", p.Name))
		}
	}
	for _, pd := range t.Props {
		if pd.Default == name {
			users = append(users, fmt.Sprintf("prop %q (its default)", pd.Name))
		}
	}
	return users
}

// SheetInUseError is RemoveSheet's refusal while something still draws
// from the sheet.
type SheetInUseError struct {
	Sheet string
	Users []string
}

func (e *SheetInUseError) Error() string {
	return fmt.Sprintf("%q is still used by %s. Delete those parts, or re-link them to "+
		"another sheet, then remove it.", e.Sheet, strings.Join(e.Users, ", "))
}

// RemoveSheet unloads an imported sheet from the project, so it stops
// appearing in the palette and pickers and isn't recorded in the .anif on
// the next save. Files on disk are left alone; re-importing brings it back.
//
// It refuses while any part or prop uses the sheet (SheetInUseError):
// removing it then would leave those parts silently drawing nothing, the
// same failure as a sheet that can't be found.
func (p *Project) RemoveSheet(name string) error {
	if _, ok := p.LoadedSheets[name]; !ok {
		return fmt.Errorf("no imported sheet called %q", name)
	}
	if users := p.CurrentTrack.SheetUsers(name); len(users) > 0 {
		return &SheetInUseError{Sheet: name, Users: users}
	}
	delete(p.LoadedSheets, name)
	if p.PaletteSheet == name {
		p.PaletteSheet = ""
	}
	// A preview override can name an imported sheet too; it would now
	// resolve to nothing.
	for prop, v := range p.PreviewProps {
		if v == name {
			delete(p.PreviewProps, prop)
		}
	}
	p.Dirty = true
	return nil
}
