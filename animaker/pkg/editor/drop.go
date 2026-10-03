package editor

import (
	"errors"
	"fmt"
)

// DropTile applies a palette cell dropped onto the canvas at (x, y), at the
// playhead in the active direction, and returns the index of the part it
// landed on and the keyframe now holding it.
//
// The selection decides what a drop means:
//
//   - A part is selected and it draws from the dropped sheet: the cell
//     becomes that part's keyframe at the playhead — the next frame of its
//     animation. An existing keyframe at that time is re-celled and moved
//     rather than duplicated. Its rotation and Z are kept (or, for a new
//     keyframe, continued from the interpolated pose), since a drop only
//     says where and which cell.
//   - Nothing is selected, or the selected part can't show this cell
//     because it draws from a different sheet: the cell is added to the rig
//     as a new part, linked to whichever prop is currently set to that
//     sheet (see PropForSheet), if any. That is how a rig of several
//     simultaneously-visible pieces gets built — deselect (click empty
//     canvas, or Esc) and drop. A new part is placed at the origin, not
//     where it was dropped (decided 2026-10-03): pieces of a rig are
//     usually drawn on one shared grid, so 0,0 lines them up, and a drop
//     by hand never would. Drag it from there.
//
// Returns (-1, nil) if there is no active direction or no sheet.
func (p *Project) DropTile(sheetName string, row, col int, x, y float32) (int, *Keyframe) {
	dir := p.ActiveDirection()
	if dir == nil || sheetName == "" {
		return -1, nil
	}
	timeMs := p.Playback.ElapsedMs

	if !p.DropMakesNewPart(sheetName) {
		part := p.SelectedPart()
		kf, _ := EnsureKeyframe(dir, part.ID, timeMs)
		kf.Row, kf.Col = row, col
		kf.X, kf.Y = x, y
		return p.Selection.PartIndex, kf
	}

	// A sheet imported as a prop's art makes a part linked to that prop,
	// named after the slot ("hair_1") rather than the particular sheet.
	// FixedSheet is set either way, so unlinking it later keeps the art.
	track := p.CurrentTrack
	prop := p.PropForSheet(sheetName)
	base := sheetName
	if prop != "" {
		base = prop
	}
	part := AddPart(track, NewSheetPart(UniquePartName(track, base), prop, sheetName))
	kf := AddKeyframe(dir, part.ID, timeMs)
	kf.Row, kf.Col = row, col // at the origin: X, Y = 0, 0
	// Stack new parts in front of what's already there, so a piece dropped
	// later isn't hidden behind one dropped earlier.
	kf.Z = float32(len(track.Parts))
	return len(track.Parts) - 1, kf
}

// DropMakesNewPart reports whether dropping a cell of sheetName would add
// a new part (placed at the origin) rather than key the selected one - for
// the drag preview, which shows where the drop will land.
func (p *Project) DropMakesNewPart(sheetName string) bool {
	part := p.SelectedPart()
	return part == nil || part.Kind != PartKindSheet || !p.partShowsSheet(part, sheetName)
}

// TapTile applies a palette cell clicked (not dragged) while a part is
// selected: it becomes the part's frame, without moving it. The keyframe
// changed is the one the Selected Keyframe fields are editing - the
// selected keyframe, else the part's keyframe at the playhead, created
// (seeded from the interpolated pose) if there isn't one. So "select a
// part, click the next frame" swaps the frame at the playhead, wherever
// it is.
//
// Refused with an error, changing nothing, when no sheet part is selected
// or the part can't show cells of sheetName: writing another sheet's
// row/col onto it could name a cell its own sheet doesn't have, and the
// part would draw nothing.
func (p *Project) TapTile(sheetName string, row, col int) (*Keyframe, error) {
	dir := p.ActiveDirection()
	part := p.SelectedPart()
	switch {
	case dir == nil || part == nil:
		return nil, errors.New("select a part first, then click a tile to make it that part's frame " +
			"(or drag the tile onto the canvas to add a new part)")
	case part.Kind != PartKindSheet:
		return nil, fmt.Errorf("%q plays an animation, so it has no frames to pick from a sheet", part.Name)
	case !p.partShowsSheet(part, sheetName):
		return nil, fmt.Errorf("%q draws from %q, not %q - pick %q in the palette, or drag this tile onto "+
			"the canvas with nothing selected to add it as a new part",
			part.Name, p.ResolveActiveSheetName(part), sheetName, p.ResolveActiveSheetName(part))
	}
	kf := p.SelectedKeyframe()
	if kf == nil {
		kf, _ = EnsureKeyframe(dir, part.ID, p.Playback.ElapsedMs)
	}
	kf.Row, kf.Col = row, col
	return kf, nil
}
