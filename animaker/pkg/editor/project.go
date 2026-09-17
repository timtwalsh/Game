package editor

import "time"

// Project is the top-level state container for the animation tool.
type Project struct {
	CurrentAnimation *Animation
	LoadedSheets     map[string]*SpriteSheet
	SavePath         string
	Dirty            bool
	UndoStack        *UndoStack
	Playback         *PlaybackState
}

// PlaybackState tracks animation preview playback.
type PlaybackState struct {
	IsPlaying    bool
	CurrentFrame int
	ElapsedMs    uint32
	LoopEnabled  bool
	SpeedFactor  float32 // 1.0 = normal, 2.0 = 2x fast
}

// NewProject creates a new project with default settings.
func NewProject(name string) *Project {
	return &Project{
		CurrentAnimation: NewAnimation(name),
		LoadedSheets:     make(map[string]*SpriteSheet),
		Dirty:            false,
		UndoStack:        NewUndoStack(100),
		Playback: &PlaybackState{
			IsPlaying:   false,
			LoopEnabled: true,
			SpeedFactor: 1.0,
		},
	}
}

// TakeSnapshot creates a snapshot of the current project state for undo.
func (p *Project) TakeSnapshot() *ProjectSnapshot {
	// Deep copy animation
	animCopy := p.deepCopyAnimation()
	return &ProjectSnapshot{
		Animation: animCopy,
		Timestamp: time.Now(),
	}
}

// RestoreSnapshot restores project state from a snapshot.
func (p *Project) RestoreSnapshot(snap *ProjectSnapshot) {
	if snap == nil {
		return
	}
	p.CurrentAnimation = snap.Animation
	p.Dirty = true
}

// RecordUndo saves current state and marks project dirty.
func (p *Project) RecordUndo() {
	p.UndoStack.Push(p.TakeSnapshot())
	p.Dirty = true
}

// Undo reverts to the previous state.
func (p *Project) Undo() bool {
	snap := p.UndoStack.Undo()
	if snap == nil {
		return false
	}
	p.RestoreSnapshot(snap)
	return true
}

// Redo reapplies the next undone state.
func (p *Project) Redo() bool {
	snap := p.UndoStack.Redo()
	if snap == nil {
		return false
	}
	p.RestoreSnapshot(snap)
	return true
}

// AdvancePlayback updates playback by the given delta time.
func (p *Project) AdvancePlayback(deltaMs uint32) {
	if !p.Playback.IsPlaying || len(p.CurrentAnimation.KeyFrames) == 0 {
		return
	}

	totalMs := p.CurrentAnimation.TotalDurationMs()
	if totalMs == 0 {
		return
	}

	p.Playback.ElapsedMs += uint32(float32(deltaMs) * p.Playback.SpeedFactor)

	if p.Playback.ElapsedMs >= totalMs {
		if p.Playback.LoopEnabled {
			p.Playback.ElapsedMs = p.Playback.ElapsedMs % totalMs
		} else {
			p.Playback.IsPlaying = false
			p.Playback.ElapsedMs = totalMs - 1
		}
	}

	p.Playback.CurrentFrame = p.CurrentAnimation.KeyFrameAtTime(p.Playback.ElapsedMs)
}

// TogglePlayback starts or stops animation playback.
func (p *Project) TogglePlayback() {
	if len(p.CurrentAnimation.KeyFrames) == 0 {
		return
	}
	p.Playback.IsPlaying = !p.Playback.IsPlaying
	if p.Playback.IsPlaying {
		p.Playback.ElapsedMs = 0
	}
}

// StepForward moves to the next frame.
func (p *Project) StepForward() {
	if len(p.CurrentAnimation.KeyFrames) == 0 {
		return
	}
	p.Playback.IsPlaying = false
	p.Playback.CurrentFrame++
	if p.Playback.CurrentFrame >= len(p.CurrentAnimation.KeyFrames) {
		if p.Playback.LoopEnabled {
			p.Playback.CurrentFrame = 0
		} else {
			p.Playback.CurrentFrame = len(p.CurrentAnimation.KeyFrames) - 1
		}
	}
}

// StepBackward moves to the previous frame.
func (p *Project) StepBackward() {
	if len(p.CurrentAnimation.KeyFrames) == 0 {
		return
	}
	p.Playback.IsPlaying = false
	p.Playback.CurrentFrame--
	if p.Playback.CurrentFrame < 0 {
		if p.Playback.LoopEnabled {
			p.Playback.CurrentFrame = len(p.CurrentAnimation.KeyFrames) - 1
		} else {
			p.Playback.CurrentFrame = 0
		}
	}
}

// GetCurrentKeyFrame returns the currently selected/playing keyframe.
func (p *Project) GetCurrentKeyFrame() *KeyFrame {
	if len(p.CurrentAnimation.KeyFrames) == 0 {
		return nil
	}
	idx := p.Playback.CurrentFrame
	if idx < 0 || idx >= len(p.CurrentAnimation.KeyFrames) {
		return nil
	}
	return p.CurrentAnimation.KeyFrames[idx]
}

// deepCopyAnimation creates a full deep copy of the current animation.
func (p *Project) deepCopyAnimation() *Animation {
	src := p.CurrentAnimation
	dst := &Animation{
		Metadata: src.Metadata,
		Config:   src.Config,
	}

	dst.KeyFrames = make([]*KeyFrame, len(src.KeyFrames))
	for i, kf := range src.KeyFrames {
		dst.KeyFrames[i] = kf.Clone(kf.ID)
	}

	dst.Nested = make([]*NestedAnimation, len(src.Nested))
	for i, n := range src.Nested {
		nc := *n
		dst.Nested[i] = &nc
	}

	return dst
}
