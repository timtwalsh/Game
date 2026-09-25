package editor

import (
	"sort"
	"time"
)

// Project is the top-level state container for the editor: the track being
// edited, the sheet templates it currently has loaded, and UI/playback
// state that doesn't belong in the saved .anif itself.
type Project struct {
	CurrentTrack *Track
	LoadedSheets map[string]*SpriteSheetTemplate // sheet name -> template
	SavePath     string
	Dirty        bool
	UndoStack    *UndoStack
	Playback     *PlaybackState

	// PreviewProps overrides a prop's resolved sheet for editing/preview
	// only — it's never saved into the .anif, which only stores each
	// prop's Default.
	PreviewProps map[string]string

	// PaletteSheet is the sheet the left palette is currently showing.
	// The palette is no longer tied to the selected part: dragging a tile
	// creates a *new* part, so the palette has to stand on its own rather
	// than following a selection that may not exist yet. Set on import,
	// and followed along when a part is selected so clicking a part still
	// brings up the sheet it draws from.
	PaletteSheet string

	Selection *Selection
}

// Selection tracks which Part/Keyframe the properties panel and canvas
// currently reflect, within the active direction.
type Selection struct {
	PartIndex     int // -1 = none
	KeyframeIndex int // -1 = none
}

type PlaybackState struct {
	ActiveDirection int
	IsPlaying       bool
	ElapsedMs       uint32
	LoopEnabled     bool
	SpeedFactor     float32
}

// NewProject creates a new project with a fresh, empty track.
func NewProject(name string) *Project {
	track := NewTrack(name)
	return &Project{
		CurrentTrack: track,
		LoadedSheets: make(map[string]*SpriteSheetTemplate),
		PreviewProps: make(map[string]string),
		UndoStack:    NewUndoStack(100),
		Selection:    &Selection{PartIndex: -1, KeyframeIndex: -1},
		Playback: &PlaybackState{
			ActiveDirection: firstDirectionKey(track),
			LoopEnabled:     true,
			SpeedFactor:     1.0,
		},
	}
}

func firstDirectionKey(t *Track) int {
	keys := t.SortedDirectionKeys()
	if len(keys) == 0 {
		return 0
	}
	return keys[0]
}

// ActiveDirection returns the direction currently selected for editing/
// playback, or nil if it doesn't exist (shouldn't normally happen).
func (p *Project) ActiveDirection() *Direction {
	return p.CurrentTrack.Directions[p.Playback.ActiveDirection]
}

// SelectedPart returns the part the Selection currently points at, or nil.
// Selection.PartIndex indexes the Track's shared part list, so it stays
// meaningful across a direction change.
func (p *Project) SelectedPart() *Part {
	if p.CurrentTrack == nil || p.Selection.PartIndex < 0 || p.Selection.PartIndex >= len(p.CurrentTrack.Parts) {
		return nil
	}
	return p.CurrentTrack.Parts[p.Selection.PartIndex]
}

// SelectedKeyframe returns the keyframe the Selection currently points at,
// on the selected part in the active direction, or nil.
func (p *Project) SelectedKeyframe() *Keyframe {
	part := p.SelectedPart()
	dir := p.ActiveDirection()
	if part == nil || dir == nil {
		return nil
	}
	kfs := dir.KeyframesFor(part.ID)
	if p.Selection.KeyframeIndex < 0 || p.Selection.KeyframeIndex >= len(kfs) {
		return nil
	}
	return kfs[p.Selection.KeyframeIndex]
}

// ResolveActiveSheetName returns the sheet name a Sheet-kind part should
// currently draw from: PreviewProps override, else the governing prop's
// declared default, else the part's fixed sheet.
func (p *Project) ResolveActiveSheetName(part *Part) string {
	if part.GoverningProp == "" {
		return part.FixedSheet
	}
	if v, ok := p.PreviewProps[part.GoverningProp]; ok && v != "" {
		return v
	}
	if def := p.CurrentTrack.FindProp(part.GoverningProp); def != nil {
		return def.Default
	}
	return ""
}

// PaletteSheetTemplate is the loaded template for PaletteSheet, or nil.
func (p *Project) PaletteSheetTemplate() *SpriteSheetTemplate {
	if p.PaletteSheet == "" {
		return nil
	}
	return p.LoadedSheets[p.PaletteSheet]
}

// LoadedSheetNames returns every currently loaded sheet's name in stable
// sorted order. The UI uses it to offer sheets as a pick-list rather than
// making the artist retype a name they have to remember exactly.
func (p *Project) LoadedSheetNames() []string {
	names := make([]string, 0, len(p.LoadedSheets))
	for n := range p.LoadedSheets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ResolveActiveSheet is ResolveActiveSheetName plus the LoadedSheets lookup.
func (p *Project) ResolveActiveSheet(part *Part) *SpriteSheetTemplate {
	name := p.ResolveActiveSheetName(part)
	if name == "" {
		return nil
	}
	return p.LoadedSheets[name]
}

// -- Undo/redo --

func (p *Project) TakeSnapshot() *ProjectSnapshot {
	return &ProjectSnapshot{Track: p.CurrentTrack.DeepCopy(), Timestamp: time.Now()}
}

func (p *Project) RestoreSnapshot(snap *ProjectSnapshot) {
	if snap == nil {
		return
	}
	p.CurrentTrack = snap.Track
	p.Dirty = true
	p.clampSelection()
}

func (p *Project) RecordUndo() {
	p.UndoStack.Push(p.TakeSnapshot())
	p.Dirty = true
}

func (p *Project) Undo() bool {
	snap := p.UndoStack.Undo()
	if snap == nil {
		return false
	}
	p.RestoreSnapshot(snap)
	return true
}

func (p *Project) Redo() bool {
	snap := p.UndoStack.Redo()
	if snap == nil {
		return false
	}
	p.RestoreSnapshot(snap)
	return true
}

func (p *Project) clampSelection() {
	if p.ActiveDirection() == nil {
		p.Playback.ActiveDirection = firstDirectionKey(p.CurrentTrack)
	}
	if p.Selection.PartIndex >= len(p.CurrentTrack.Parts) {
		p.Selection.PartIndex = -1
	}
	if p.SelectedKeyframe() == nil {
		p.Selection.KeyframeIndex = -1
	}
}

// -- Playback --

// AdvancePlayback moves ElapsedMs forward by deltaMs (scaled by
// SpeedFactor) within the active direction's timeline, looping or
// stopping at the end per LoopEnabled.
func (p *Project) AdvancePlayback(deltaMs uint32) {
	dir := p.ActiveDirection()
	if !p.Playback.IsPlaying || dir == nil {
		return
	}
	total := dir.TotalDurationMs()
	if total == 0 {
		return
	}
	p.Playback.ElapsedMs += uint32(float32(deltaMs) * p.Playback.SpeedFactor)
	if p.Playback.ElapsedMs >= total {
		if p.Playback.LoopEnabled {
			p.Playback.ElapsedMs %= total
		} else {
			p.Playback.IsPlaying = false
			p.Playback.ElapsedMs = total
		}
	}
}

// Play resumes/starts playback from wherever the playhead currently is
// (does not reset to 0 - that's Stop's job).
func (p *Project) Play() {
	// Nothing to play until this facing has keyframes spanning some time —
	// AdvancePlayback no-ops on a zero duration, so this just avoids
	// leaving IsPlaying stuck on with a frozen playhead.
	if dir := p.ActiveDirection(); dir == nil || dir.TotalDurationMs() == 0 {
		return
	}
	p.Playback.IsPlaying = true
}

// Stop pauses playback and resets the playhead to 0.
func (p *Project) Stop() {
	p.Playback.IsPlaying = false
	p.Playback.ElapsedMs = 0
}

const scrubStepMs = 50

func (p *Project) StepForward() {
	p.Playback.IsPlaying = false
	dir := p.ActiveDirection()
	limit := uint32(0)
	if dir != nil {
		limit = dir.EditableDurationMs()
	}
	p.Playback.ElapsedMs += scrubStepMs
	if limit > 0 && p.Playback.ElapsedMs > limit {
		p.Playback.ElapsedMs = 0
	}
}

func (p *Project) StepBackward() {
	p.Playback.IsPlaying = false
	if p.Playback.ElapsedMs < scrubStepMs {
		dir := p.ActiveDirection()
		if dir != nil {
			p.Playback.ElapsedMs = dir.EditableDurationMs()
			return
		}
		p.Playback.ElapsedMs = 0
		return
	}
	p.Playback.ElapsedMs -= scrubStepMs
}

// SetActiveDirection switches which direction is being edited/previewed,
// resetting playback position and selection.
func (p *Project) SetActiveDirection(key int) {
	if _, ok := p.CurrentTrack.Directions[key]; !ok {
		return
	}
	p.Playback.ActiveDirection = key
	p.Playback.ElapsedMs = 0
	p.Playback.IsPlaying = false
	// The selected part carries across: the part list belongs to the Track,
	// so index N is the same part in every facing and switching direction
	// just changes which keyframes you're editing. Only the keyframe
	// selection is dropped, since keyframes are per-direction.
	p.Selection.KeyframeIndex = -1
}

// Seek moves the playhead directly to timeMs without changing IsPlaying.
// Used by the timeline's scrub bar. It clamps to EditableDurationMs, not
// TotalDurationMs — clamping to the latter made the playhead unmovable on a
// track whose only keyframe is at 0ms, which in turn made it impossible to
// ever add a second keyframe. See Direction.EditableDurationMs.
func (p *Project) Seek(timeMs uint32) {
	dir := p.ActiveDirection()
	if dir == nil {
		p.Playback.ElapsedMs = 0
		return
	}
	if limit := dir.EditableDurationMs(); timeMs > limit {
		timeMs = limit
	}
	p.Playback.ElapsedMs = timeMs
}
