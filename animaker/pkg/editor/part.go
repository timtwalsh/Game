package editor

import "fmt"

// NewSheetPart creates a Part that shows a cell from an active sheet.
func NewSheetPart(name, governingProp, fixedSheet string) *Part {
	return &Part{
		Name:          name,
		Kind:          PartKindSheet,
		GoverningProp: governingProp,
		FixedSheet:    fixedSheet,
		Keyframes:     []*Keyframe{},
	}
}

// NewNestedAniPart creates a Part that plays another whole .anif file.
func NewNestedAniPart(name, nestedPath string) *Part {
	return &Part{
		Name:           name,
		Kind:           PartKindNestedAni,
		NestedAniPath:  nestedPath,
		NestedBindings: map[string]PropBinding{},
		Keyframes:      []*Keyframe{},
	}
}

// AddPart appends a part to a direction.
func AddPart(dir *Direction, part *Part) {
	dir.Parts = append(dir.Parts, part)
}

// RemovePart removes the part at idx from a direction.
func RemovePart(dir *Direction, idx int) error {
	if idx < 0 || idx >= len(dir.Parts) {
		return fmt.Errorf("invalid part index: %d", idx)
	}
	dir.Parts = append(dir.Parts[:idx], dir.Parts[idx+1:]...)
	return nil
}

// AddDirection creates a new, empty direction on the track if it doesn't
// already exist. Directions are never auto-populated from another
// direction — each is authored independently, per spec.
func AddDirection(t *Track, name string) *Direction {
	if d, ok := t.Directions[name]; ok {
		return d
	}
	d := &Direction{Parts: []*Part{}}
	t.Directions[name] = d
	return d
}

// RemoveDirection deletes a direction by name.
func RemoveDirection(t *Track, name string) {
	delete(t.Directions, name)
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
