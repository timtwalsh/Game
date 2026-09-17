package editor

import "time"

// Animation represents a complete frame-by-frame animation.
type Animation struct {
	Metadata  AnimationMetadata
	Config    AnimationConfig
	KeyFrames []*KeyFrame
	Nested    []*NestedAnimation
}

// AnimationMetadata stores descriptive info about the animation.
type AnimationMetadata struct {
	Name        string
	Version     string
	Description string
	Author      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AnimationConfig stores animation playback settings.
type AnimationConfig struct {
	Loop          bool
	DefaultSpeed  float32
	CharacterSize string // "16x16", "24x24", "32x32", "48x32", "custom"
	RootAnchor    string // "feet" or "center"
}

// NestedAnimation references another .anif file to compose animations.
type NestedAnimation struct {
	KeyFrameID    int
	AnimationPath string
	Offset        Point
	Scale         float32
	Opacity       float32
}

// NewAnimation creates an animation with sensible defaults.
func NewAnimation(name string) *Animation {
	return &Animation{
		Metadata: AnimationMetadata{
			Name:      name,
			Version:   "1.0",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Config: AnimationConfig{
			Loop:          true,
			DefaultSpeed:  1.0,
			CharacterSize: "32x32",
			RootAnchor:    "feet",
		},
		KeyFrames: []*KeyFrame{},
		Nested:    []*NestedAnimation{},
	}
}

// TotalDurationMs returns the total duration of all keyframes in milliseconds.
func (a *Animation) TotalDurationMs() uint32 {
	var total uint32
	for _, kf := range a.KeyFrames {
		total += kf.Duration
	}
	return total
}

// KeyFrameAtTime returns the keyframe index at the given elapsed time.
func (a *Animation) KeyFrameAtTime(elapsedMs uint32) int {
	var accum uint32
	for i, kf := range a.KeyFrames {
		accum += kf.Duration
		if elapsedMs < accum {
			return i
		}
	}
	if len(a.KeyFrames) == 0 {
		return 0
	}
	return len(a.KeyFrames) - 1
}
