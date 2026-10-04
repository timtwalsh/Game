package editor

import "testing"

func TestCopyKeyframeTimesCopiesOnlyTiming(t *testing.T) {
	track := NewTrack("t")
	AddStandardDirections(track)
	body := AddPart(track, NewSheetPart("body", "", "sheet"))
	head := AddPart(track, NewSheetPart("head", "", "sheet"))
	up := track.Directions[0]
	for _, ms := range []uint32{0, 200, 600} {
		kf := AddKeyframe(up, body.ID, ms)
		kf.X, kf.Y, kf.Z, kf.RotationDeg, kf.Row, kf.Col = 10, 20, 3, 45, 1, 2
	}
	AddKeyframe(up, head.ID, 100)

	if got := CopyKeyframeTimes(track, 0, 2); got != 4 {
		t.Errorf("added %d keyframes, want 4", got)
	}
	down := track.Directions[2]
	var times []uint32
	for _, kf := range down.KeyframesFor(body.ID) {
		times = append(times, kf.TimeMs)
		if kf.X != 0 || kf.Y != 0 || kf.Z != 0 || kf.RotationDeg != 0 || kf.Row != 0 || kf.Col != 0 {
			t.Errorf("keyframe at %dms carried a pose: %+v", kf.TimeMs, *kf)
		}
	}
	if len(times) != 3 || times[0] != 0 || times[1] != 200 || times[2] != 600 {
		t.Errorf("body times in direction 2 = %v, want [0 200 600]", times)
	}
	if got := len(down.KeyframesFor(head.ID)); got != 1 {
		t.Errorf("head has %d keyframes in direction 2, want 1", got)
	}
	if got := len(up.KeyframesFor(body.ID)); got != 3 {
		t.Errorf("source direction changed: body has %d keyframes", got)
	}
}

func TestTimingSourcesSkipsEmptyAndTarget(t *testing.T) {
	track := NewTrack("t")
	AddStandardDirections(track)
	part := AddPart(track, NewSheetPart("body", "", "sheet"))
	AddKeyframe(track.Directions[0], part.ID, 0)
	AddKeyframe(track.Directions[3], part.ID, 0)

	got := track.TimingSources(2)
	if len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Errorf("TimingSources(2) = %v, want [0 3]", got)
	}
	if got := track.TimingSources(0); len(got) != 1 || got[0] != 3 {
		t.Errorf("TimingSources(0) = %v, want [3]", got)
	}
}

func TestCopyKeyframesCopiesWholePose(t *testing.T) {
	track := NewTrack("t")
	AddStandardDirections(track)
	body := AddPart(track, NewSheetPart("body", "", "sheet"))
	up := track.Directions[0]
	for _, ms := range []uint32{0, 200} {
		kf := AddKeyframe(up, body.ID, ms)
		kf.X, kf.Y, kf.Z, kf.RotationDeg, kf.Row, kf.Col, kf.Direction = 10, 20, 3, 45, 1, 2, 1
	}

	if got := CopyKeyframes(track, 0, 2); got != 2 {
		t.Errorf("added %d keyframes, want 2", got)
	}
	kfs := track.Directions[2].KeyframesFor(body.ID)
	if len(kfs) != 2 {
		t.Fatalf("body has %d keyframes in direction 2, want 2", len(kfs))
	}
	for i, kf := range kfs {
		want := *up.KeyframesFor(body.ID)[i]
		if *kf != want {
			t.Errorf("keyframe %d = %+v, want %+v", i, *kf, want)
		}
		if kf == up.KeyframesFor(body.ID)[i] {
			t.Errorf("keyframe %d shares its pointer with the source", i)
		}
	}
}
