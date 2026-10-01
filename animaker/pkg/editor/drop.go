package editor

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
//     as a new part. That is how a rig of several simultaneously-visible
//     pieces gets built — deselect (click empty canvas, or Esc) and drop.
//
// Returns (-1, nil) if there is no active direction or no sheet.
func (p *Project) DropTile(sheetName string, row, col int, x, y float32) (int, *Keyframe) {
	dir := p.ActiveDirection()
	if dir == nil || sheetName == "" {
		return -1, nil
	}
	timeMs := p.Playback.ElapsedMs

	if part := p.SelectedPart(); part != nil && part.Kind == PartKindSheet &&
		p.ResolveActiveSheetName(part) == sheetName {
		kf, _ := EnsureKeyframe(dir, part.ID, timeMs)
		kf.Row, kf.Col = row, col
		kf.X, kf.Y = x, y
		return p.Selection.PartIndex, kf
	}

	track := p.CurrentTrack
	part := AddPart(track, NewSheetPart(UniquePartName(track, sheetName), "", sheetName))
	kf := AddKeyframe(dir, part.ID, timeMs)
	kf.Row, kf.Col = row, col
	kf.X, kf.Y = x, y
	// Stack new parts in front of what's already there, so a piece dropped
	// later isn't hidden behind one dropped earlier.
	kf.Z = float32(len(track.Parts))
	return len(track.Parts) - 1, kf
}
