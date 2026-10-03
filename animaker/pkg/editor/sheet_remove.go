package editor

import (
	"fmt"
	"strings"
)

// SheetUsers describes everything in the track that draws from the named
// sheet: parts with it as their fixed sheet, nested parts pinning one of
// their animation's props to it, and props with it as their default.
// Empty means nothing would lose its art if the sheet went away.
func (t *Track) SheetUsers(name string) []string {
	var users []string
	for _, p := range t.Parts {
		if p.Kind == PartKindSheet && p.FixedSheet == name {
			users = append(users, fmt.Sprintf("part %q", p.Name))
		}
		for _, v := range p.staticBindingValues() {
			if v == name {
				users = append(users, fmt.Sprintf("part %q (a fixed binding)", p.Name))
				break
			}
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

// PropUsers describes everything in the track that depends on the named
// prop: parts it governs, and nested parts passing it through to their
// animation. Empty means removing the prop changes nothing that draws.
func (t *Track) PropUsers(name string) []string {
	var users []string
	for _, p := range t.Parts {
		if p.GoverningProp == name {
			users = append(users, fmt.Sprintf("part %q", p.Name))
			continue
		}
		for child, b := range p.NestedBindings {
			if b.PassthroughFrom == name {
				users = append(users, fmt.Sprintf("part %q (passes it to %q)", p.Name, child))
				break
			}
		}
	}
	return users
}

// PropInUseError is RemoveProp's refusal while something still depends on
// the prop.
type PropInUseError struct {
	Prop  string
	Users []string
}

func (e *PropInUseError) Error() string {
	return fmt.Sprintf("prop %q is still used by %s. Unlink those parts from it, or delete them, "+
		"then remove it.", e.Prop, strings.Join(e.Users, ", "))
}

// RemoveProp removes the prop at idx from the track, recording an undo
// step. Like RemoveSheet it refuses (PropInUseError) while anything uses
// the prop: removing it then would leave those parts linked to a prop that
// no longer exists, silently drawing nothing. Its preview override goes
// with it, so a later prop of the same name doesn't inherit it.
func (p *Project) RemoveProp(idx int) error {
	props := p.CurrentTrack.Props
	if idx < 0 || idx >= len(props) {
		return fmt.Errorf("invalid prop index: %d", idx)
	}
	name := props[idx].Name
	if users := p.CurrentTrack.PropUsers(name); len(users) > 0 {
		return &PropInUseError{Prop: name, Users: users}
	}
	p.RecordUndo()
	if err := RemoveProp(p.CurrentTrack, idx); err != nil {
		return err
	}
	delete(p.PreviewProps, name)
	p.Dirty = true
	return nil
}

// SetPropDefault changes the named prop's default value - previously the
// only way was to delete the prop and add it again, which is refused
// while parts use it. The value must be of the prop's own kind and
// loaded: an imported sheet for a sheet prop, a loaded animation for an
// .anif prop. Changing kind would leave its linked parts unable to draw
// it, and a preview-only sheet is never saved, so neither is accepted.
// Undoable; reports whether anything changed.
func (p *Project) SetPropDefault(name, value string) (bool, error) {
	pd := p.CurrentTrack.FindProp(name)
	switch {
	case pd == nil:
		return false, fmt.Errorf("there's no prop called %q", name)
	case pd.Default == value:
		return false, nil
	case pd.IsAnimProp() != IsAnimValue(value):
		if pd.IsAnimProp() {
			return false, fmt.Errorf("prop %q holds animations, so its default must be an .anif", name)
		}
		return false, fmt.Errorf("prop %q holds sprite sheets, so its default must be a sheet", name)
	case IsAnimValue(value) && p.LoadedAnims[AnimKey(value)] == nil:
		return false, fmt.Errorf("%q isn't loaded - use Import Animation first", value)
	case !IsAnimValue(value) && p.LoadedSheets[value] == nil:
		return false, fmt.Errorf("no imported sheet is called %q", value)
	}
	p.RecordUndo()
	if IsAnimValue(value) {
		value = AnimKey(value)
	}
	pd.Default = value
	p.Dirty = true
	return true, nil
}
