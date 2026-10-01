package editor

import (
	"errors"
	"fmt"
	"strings"
)

// NewSheetPart creates a Part that shows a cell from an active sheet. The
// ID is assigned by AddPart, not here.
func NewSheetPart(name, governingProp, fixedSheet string) *Part {
	return &Part{
		Name:          name,
		Kind:          PartKindSheet,
		GoverningProp: governingProp,
		FixedSheet:    fixedSheet,
	}
}

// NewNestedAniPart creates a Part that plays another whole .anif file.
func NewNestedAniPart(name, nestedPath string) *Part {
	return &Part{
		Name:           name,
		Kind:           PartKindNestedAni,
		NestedAniPath:  nestedPath,
		NestedBindings: map[string]PropBinding{},
	}
}

// UniquePartName returns base with a numeric suffix that no existing part
// is using. Part identity is the ID, not the name, so duplicates would work
// — but a rig with three parts all called "sprite" is unreadable in the
// part list and the timeline, which is where the artist actually works.
func UniquePartName(t *Track, base string) string {
	taken := make(map[string]bool, len(t.Parts))
	for _, p := range t.Parts {
		taken[p.Name] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s_%d", base, i)
		if !taken[name] {
			return name
		}
	}
}

// ErrPartNameEmpty / ErrPartNameTaken are RenamePart's refusals.
var (
	ErrPartNameEmpty = errors.New("part name can't be empty")
	ErrPartNameTaken = errors.New("another part already has that name")
)

// RenamePart renames the part at idx, trimming surrounding whitespace. It
// refuses an empty name or one another part already uses: identity is the
// ID, so the data would survive either, but the part list and timeline are
// labelled by name and two rows called "arm" can't be told apart (see
// UniquePartName). Renaming a part to its own current name is allowed.
func RenamePart(t *Track, idx int, name string) error {
	if idx < 0 || idx >= len(t.Parts) {
		return fmt.Errorf("invalid part index: %d", idx)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrPartNameEmpty
	}
	for i, p := range t.Parts {
		if i != idx && p.Name == name {
			return ErrPartNameTaken
		}
	}
	t.Parts[idx].Name = name
	return nil
}

// AddPart adds a part to the track's rig, assigning it a fresh ID. The part
// exists in every direction from this moment on — it just has no keyframes
// in any of them yet.
func AddPart(t *Track, part *Part) *Part {
	part.ID = t.nextPartID()
	t.Parts = append(t.Parts, part)
	return part
}

// RemovePart removes the part at idx from the rig, along with its keyframes
// in every direction — otherwise those keyframes would be orphaned, and a
// later part could be given the same ID and inherit them.
func RemovePart(t *Track, idx int) error {
	if idx < 0 || idx >= len(t.Parts) {
		return fmt.Errorf("invalid part index: %d", idx)
	}
	id := t.Parts[idx].ID
	t.Parts = append(t.Parts[:idx], t.Parts[idx+1:]...)
	for _, dir := range t.Directions {
		delete(dir.Keyframes, id)
	}
	return nil
}

// AddDirection creates a new, empty direction on the track if it doesn't
// already exist. It starts with no keyframes: the track's parts all exist
// in it immediately, but posing them in this facing is the artist's work.
func AddDirection(t *Track, key int) *Direction {
	if d, ok := t.Directions[key]; ok {
		return d
	}
	d := NewDirection()
	t.Directions[key] = d
	return d
}

// RemoveDirection deletes a direction by key.
func RemoveDirection(t *Track, key int) {
	delete(t.Directions, key)
}

// AddProp appends a prop definition to the track.
func AddProp(t *Track, name, def string) {
	t.Props = append(t.Props, PropDef{Name: name, Default: def})
}

// EnsureProp makes sheetName an option of the prop called name, declaring
// the prop with sheetName as its default if the track doesn't have it yet.
// An existing prop keeps its default: importing "hair_long" into an
// existing "hair" prop adds another sheet to swap to, it doesn't change
// what the track shows out of the box. Reports whether a prop was created.
func EnsureProp(t *Track, name, sheetName string) bool {
	if t.FindProp(name) != nil {
		return false
	}
	AddProp(t, name, sheetName)
	return true
}

// RemoveProp removes the prop at idx.
func RemoveProp(t *Track, idx int) error {
	if idx < 0 || idx >= len(t.Props) {
		return fmt.Errorf("invalid prop index: %d", idx)
	}
	t.Props = append(t.Props[:idx], t.Props[idx+1:]...)
	return nil
}

// FindProp returns the PropDef with the given name, or nil.
func (t *Track) FindProp(name string) *PropDef {
	for i := range t.Props {
		if t.Props[i].Name == name {
			return &t.Props[i]
		}
	}
	return nil
}
