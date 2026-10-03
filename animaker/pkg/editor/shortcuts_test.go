package editor

import "testing"

// twoParts is a direction with body keyed at 0/200/400 and head at 100,
// both selected-free, playhead at 0.
func twoParts(t *testing.T) (*Project, *Part, *Part) {
	t.Helper()
	p := NewProject("test")
	dir := p.ActiveDirection()
	body := AddPart(p.CurrentTrack, NewSheetPart("body", "", "sheet"))
	head := AddPart(p.CurrentTrack, NewSheetPart("head", "", "sheet"))
	for _, ms := range []uint32{0, 200, 400} {
		AddKeyframe(dir, body.ID, ms).Z = 1
	}
	AddKeyframe(dir, head.ID, 100).Z = 5
	return p, body, head
}

func TestTogglePlayPausesWithoutRewinding(t *testing.T) {
	p, _, _ := twoParts(t)
	p.TogglePlay()
	if !p.Playback.IsPlaying {
		t.Fatal("first toggle didn't play")
	}
	p.AdvancePlayback(150)
	p.TogglePlay()
	if p.Playback.IsPlaying || p.Playback.ElapsedMs != 150 {
		t.Errorf("paused: playing=%v at %dms, want stopped at 150ms", p.Playback.IsPlaying, p.Playback.ElapsedMs)
	}
}

func TestStepToKeyframeVisitsEveryPartsKeys(t *testing.T) {
	p, _, _ := twoParts(t)
	var got []uint32
	for p.StepToKeyframe(true) {
		got = append(got, p.Playback.ElapsedMs)
	}
	want := []uint32{100, 200, 400}
	if len(got) != len(want) {
		t.Fatalf("forward steps %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("forward steps %v, want %v", got, want)
		}
	}
	if !p.StepToKeyframe(false) || p.Playback.ElapsedMs != 200 {
		t.Errorf("step back from 400 landed at %dms, want 200", p.Playback.ElapsedMs)
	}
	p.Seek(0)
	if p.StepToKeyframe(false) {
		t.Error("step back from the first keyframe moved, want no-op")
	}
}

func TestEditTargetKeysThePlayheadOnlyWhenNeeded(t *testing.T) {
	p, body, _ := twoParts(t)
	if p.EditTarget() != nil {
		t.Error("no part selected: want nil")
	}
	p.Selection.PartIndex = 0
	p.Scrub(300)
	kf := p.EditTarget()
	if kf == nil || kf.TimeMs != 300 || len(p.ActiveDirection().KeyframesFor(body.ID)) != 4 {
		t.Fatalf("want a new keyframe at the playhead (300ms), got %+v", kf)
	}
	if p.EditTarget() != kf {
		t.Error("second call made another keyframe, want the same one")
	}
}

func TestKeyframeToDeleteOnlyWhatsVisible(t *testing.T) {
	p, body, _ := twoParts(t)
	p.Selection.PartIndex = 0
	p.Scrub(300) // between keys: nothing at the playhead
	if _, _, ok := p.KeyframeToDelete(); ok {
		t.Error("nothing keyed at the playhead or selected: want no target")
	}
	p.Scrub(200)
	if id, idx, ok := p.KeyframeToDelete(); !ok || id != body.ID || idx != 1 {
		t.Errorf("at 200ms: (%d, %d, %v), want body's keyframe 1", id, idx, ok)
	}
}

func TestZOrderTargetFrontAndBack(t *testing.T) {
	p, _, _ := twoParts(t)
	p.Selection.PartIndex = 0 // body, Z 1; head is Z 5
	p.Seek(100)
	if z, ok := p.ZOrderTarget(true); !ok || z != 6 {
		t.Errorf("front: %v, %v, want 6", z, ok)
	}
	if z, ok := p.ZOrderTarget(false); !ok || z != 4 {
		t.Errorf("back: %v, %v, want 4 (below head's 5)", z, ok)
	}
}
