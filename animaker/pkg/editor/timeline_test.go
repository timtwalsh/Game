package editor

import "testing"

// Regression: reported as "the timeline doesn't let me scrub forward
// because there are no frames past the first 1, clicking add keyframe only
// works the first time (frame on 0ms)". Seek used to clamp to
// TotalDurationMs, which is 0 for a part with a single keyframe at 0ms, so
// the playhead could never leave 0 and a second keyframe could never be
// placed.
func TestSeekCanMovePastASingleKeyframeAtZero(t *testing.T) {
	p := NewProject("test")
	dir := p.ActiveDirection()
	part := AddPart(p.CurrentTrack, NewSheetPart("Body", "", "sheet"))
	AddKeyframe(dir, part.ID, 0)

	if got := dir.TotalDurationMs(); got != 0 {
		t.Fatalf("precondition: TotalDurationMs = %d, want 0", got)
	}

	p.Seek(200)
	if p.Playback.ElapsedMs != 200 {
		t.Fatalf("Seek(200) left the playhead at %dms, want 200ms", p.Playback.ElapsedMs)
	}

	// ...and a keyframe can now actually be added there.
	kf := AddKeyframe(dir, part.ID, p.Playback.ElapsedMs)
	if kf.TimeMs != 200 {
		t.Errorf("new keyframe is at %dms, want 200ms", kf.TimeMs)
	}
	if got := len(dir.KeyframesFor(part.ID)); got != 2 {
		t.Errorf("part has %d keyframes, want 2", got)
	}
}

func TestEditableDurationAlwaysLeadsTheLastKeyframe(t *testing.T) {
	tests := []struct {
		name    string
		lastKf  uint32
		wantMin uint32
	}{
		{"empty direction", 0, MinTimelineMs},
		{"short animation", 100, MinTimelineMs},
		{"animation past the minimum", 2000, 2000 + TimelineHeadroomMs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := NewDirection()
			if tt.lastKf > 0 {
				AddKeyframe(dir, 1, tt.lastKf)
			}
			got := dir.EditableDurationMs()
			if got != tt.wantMin {
				t.Errorf("EditableDurationMs = %d, want %d", got, tt.wantMin)
			}
			if got <= tt.lastKf {
				t.Errorf("EditableDurationMs = %d, must exceed last keyframe at %d", got, tt.lastKf)
			}
		})
	}
}

func TestSeekClampsToEditableDuration(t *testing.T) {
	p := NewProject("test")
	p.Seek(999999)
	if want := p.ActiveDirection().EditableDurationMs(); p.Playback.ElapsedMs != want {
		t.Errorf("Seek past the end left the playhead at %dms, want %dms clamp", p.Playback.ElapsedMs, want)
	}
}

// Playback loops over the real animation length, not the padded editable
// range — the headroom is scrubbing space, not part of the animation.
func TestPlaybackLoopsOverRealDurationNotHeadroom(t *testing.T) {
	p := NewProject("test")
	dir := p.ActiveDirection()
	part := AddPart(p.CurrentTrack, NewSheetPart("Body", "", "sheet"))
	AddKeyframe(dir, part.ID, 0)
	AddKeyframe(dir, part.ID, 100)

	p.Play()
	p.AdvancePlayback(150)
	if p.Playback.ElapsedMs >= 100 {
		t.Errorf("playhead at %dms did not wrap at the 100ms animation length", p.Playback.ElapsedMs)
	}
}

func TestNewTrackHasFourDefaultDirections(t *testing.T) {
	track := NewTrack("human_walk")
	got := track.SortedDirectionKeys()
	if len(got) != 4 {
		t.Fatalf("new track has directions %v, want 4", got)
	}
	for i, key := range got {
		if key != i {
			t.Errorf("direction %d has key %d, want %d", i, key, i)
		}
		if track.Directions[key] == nil {
			t.Errorf("direction %d is nil", key)
		}
	}
}

// Scrubbing drops the keyframe selection but keeps the part; a plain Seek
// (used to follow a keyframe) keeps both.
func TestScrubClearsKeyframeSelection(t *testing.T) {
	p := NewProject("t")
	part := AddPart(p.CurrentTrack, NewSheetPart("body", "", "s"))
	AddKeyframe(p.ActiveDirection(), part.ID, 0)
	AddKeyframe(p.ActiveDirection(), part.ID, 200)
	p.Selection.PartIndex, p.Selection.KeyframeIndex = 0, 1

	p.Seek(200)
	if p.Selection.KeyframeIndex != 1 {
		t.Fatal("Seek dropped the keyframe selection")
	}
	p.Scrub(100)
	if p.Selection.KeyframeIndex != -1 || p.Selection.PartIndex != 0 {
		t.Errorf("after Scrub selection = %+v, want part 0 with no keyframe", *p.Selection)
	}
	if p.Playback.ElapsedMs != 100 {
		t.Errorf("playhead %d, want 100", p.Playback.ElapsedMs)
	}
}

// With looping off, playback stops at the end; Play then starts over
// rather than stopping again straight away.
func TestPlayAtTheEndStartsOver(t *testing.T) {
	p := NewProject("t")
	part := AddPart(p.CurrentTrack, NewSheetPart("body", "", "s"))
	AddKeyframe(p.ActiveDirection(), part.ID, 0)
	AddKeyframe(p.ActiveDirection(), part.ID, 300)
	p.Playback.LoopEnabled = false
	p.Play()
	p.AdvancePlayback(400)
	if p.Playback.IsPlaying || p.Playback.ElapsedMs != 300 {
		t.Fatalf("precondition: playing=%v at %d, want stopped at 300", p.Playback.IsPlaying, p.Playback.ElapsedMs)
	}
	p.Play()
	if !p.Playback.IsPlaying || p.Playback.ElapsedMs != 0 {
		t.Errorf("Play at the end: playing=%v at %d, want playing from 0", p.Playback.IsPlaying, p.Playback.ElapsedMs)
	}
}
