package editor

// Event is the interface for frame-level events.
type Event interface {
	EventType() string
}

// SoundEvent triggers a sound effect on this frame.
type SoundEvent struct {
	FilePath string
	Pitch    float32 // 1.0 = normal, 0.5 = lower, 2.0 = higher
}

func (e *SoundEvent) EventType() string { return "sound" }

// ParticleEvent spawns a particle effect on this frame.
type ParticleEvent struct {
	Type     string   // "slash_spark", "dust_cloud", etc.
	X        int      // offset from sprite origin
	Y        int
	Rotation *float32 // optional
	Scale    *float32 // optional
}

func (e *ParticleEvent) EventType() string { return "particle" }

// ShakeEvent triggers a screen shake on this frame.
type ShakeEvent struct {
	DurationMs uint32  // milliseconds
	Intensity  float32 // 0.0 - 1.0
}

func (e *ShakeEvent) EventType() string { return "shake" }

// FlashEvent triggers a screen flash on this frame.
type FlashEvent struct {
	Color      string  // hex color: "#FFFFFF"
	DurationMs uint32
	Opacity    float32 // 0.0 - 1.0
}

func (e *FlashEvent) EventType() string { return "flash" }

// CopyEvent creates a deep copy of an event.
func CopyEvent(e Event) Event {
	if e == nil {
		return nil
	}
	switch v := e.(type) {
	case *SoundEvent:
		return &SoundEvent{FilePath: v.FilePath, Pitch: v.Pitch}
	case *ParticleEvent:
		pe := &ParticleEvent{Type: v.Type, X: v.X, Y: v.Y}
		if v.Rotation != nil {
			r := *v.Rotation
			pe.Rotation = &r
		}
		if v.Scale != nil {
			s := *v.Scale
			pe.Scale = &s
		}
		return pe
	case *ShakeEvent:
		return &ShakeEvent{DurationMs: v.DurationMs, Intensity: v.Intensity}
	case *FlashEvent:
		return &FlashEvent{Color: v.Color, DurationMs: v.DurationMs, Opacity: v.Opacity}
	default:
		return nil
	}
}
