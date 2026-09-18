package editor

import "fmt"

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
