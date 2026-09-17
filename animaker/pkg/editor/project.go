package editor

import "time"

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

	Selection *Selection
}

// Selection tracks which Part/Keyframe the properties panel and canvas
// currently reflect, within the active direction.
type Selection struct {
	PartIndex     int // -1 = none
	KeyframeIndex int // -1 = none
}

type PlaybackState struct {
	ActiveDirection string
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
			ActiveDirection: firstDirectionName(track),
			LoopEnabled:     true,
			SpeedFactor:     1.0,
		},
	}
}

func firstDirectionName(t *Track) string {
	names := t.SortedDirectionNames()
	if len(names) == 0 {
		return "default"
	}
	return names[0]
}

// ActiveDirection returns the direction currently selected for editing/
// playback, or nil if it doesn't exist (shouldn't normally happen).
func (p *Project) ActiveDirection() *Direction {
	return p.CurrentTrack.Directions[p.Playback.ActiveDirection]
}

// SelectedPart returns the part the Selection currently points at, or nil.
func (p *Project) SelectedPart() *Part {
	dir := p.ActiveDirection()
	if dir == nil || p.Selection.PartIndex < 0 || p.Selection.PartIndex >= len(dir.Parts) {
		return nil
	}
	return dir.Parts[p.Selection.PartIndex]
}

// SelectedKeyframe returns the keyframe the Selection currently points at
// on the selected part, or nil.
func (p *Project) SelectedKeyframe() *Keyframe {
	part := p.SelectedPart()
	if part == nil || p.Selection.KeyframeIndex < 0 || p.Selection.KeyframeIndex >= len(part.Keyframes) {
		return nil
	}
	return part.Keyframes[p.Selection.KeyframeIndex]
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
	dir := p.ActiveDirection()
	if dir == nil {
		p.Playback.ActiveDirection = firstDirectionName(p.CurrentTrack)
		dir = p.ActiveDirection()
	}
	if dir == nil || p.Selection.PartIndex >= len(dir.Parts) {
		p.Selection.PartIndex = -1
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

func (p *Project) TogglePlayback() {
	dir := p.ActiveDirection()
	if dir == nil || len(dir.Parts) == 0 {
		return
	}
	p.Playback.IsPlaying = !p.Playback.IsPlaying
	if p.Playback.IsPlaying {
		p.Playback.ElapsedMs = 0
	}
}

const scrubStepMs = 50

func (p *Project) StepForward() {
	p.Playback.IsPlaying = false
	dir := p.ActiveDirection()
	total := uint32(0)
	if dir != nil {
		total = dir.TotalDurationMs()
	}
	p.Playback.ElapsedMs += scrubStepMs
	if total > 0 && p.Playback.ElapsedMs > total {
		p.Playback.ElapsedMs = 0
	}
}

func (p *Project) StepBackward() {
	p.Playback.IsPlaying = false
	if p.Playback.ElapsedMs < scrubStepMs {
		dir := p.ActiveDirection()
		if dir != nil {
			p.Playback.ElapsedMs = dir.TotalDurationMs()
			return
		}
		p.Playback.ElapsedMs = 0
		return
	}
	p.Playback.ElapsedMs -= scrubStepMs
}

// SetActiveDirection switches which direction is being edited/previewed,
// resetting playback position and selection.
func (p *Project) SetActiveDirection(name string) {
	if _, ok := p.CurrentTrack.Directions[name]; !ok {
		return
	}
	p.Playback.ActiveDirection = name
	p.Playback.ElapsedMs = 0
	p.Playback.IsPlaying = false
	p.Selection.PartIndex = -1
	p.Selection.KeyframeIndex = -1
}
