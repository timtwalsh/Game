package app

import (
	"animaker/pkg/editor"
	"animaker/pkg/file"
	"path/filepath"
	"testing"
)

// New Track from Rig opens a keyframe-free copy of the chosen track as
// new, unsaved work - the source file stays as it was.
func TestNewTrackFromRigOpensAnUnsavedCopy(t *testing.T) {
	src := editor.NewTrack("human_walk")
	editor.EnsureProp(src, "hair", "hair_short")
	part := editor.AddPart(src, editor.NewSheetPart("hair", "hair", "hair_short"))
	editor.AddKeyframe(src.Directions[0], part.ID, 0)
	path := filepath.Join(t.TempDir(), "human_walk.anif")
	if err := file.SaveTrack(src, path, nil); err != nil {
		t.Fatal(err)
	}

	a := testApp(t)
	a.build()
	a.loadAndOpen(path, func(tr *editor.Track) (*editor.Track, string) { return editor.RigFrom(tr, "human_idle"), "" })

	tr := a.Project.CurrentTrack
	switch {
	case tr.Metadata.Name != "human_idle", len(tr.Parts) != 1, len(tr.Props) != 1:
		t.Errorf("opened %q with %d parts, %d props; want human_idle, 1, 1", tr.Metadata.Name, len(tr.Parts), len(tr.Props))
	case tr.Directions[0].TotalKeyframes() != 0:
		t.Error("rig kept keyframes")
	case a.Project.SavePath != "" || !a.Project.Dirty:
		t.Errorf("save path %q, dirty %v; want an unsaved new track", a.Project.SavePath, a.Project.Dirty)
	}
	if reloaded, _, err := file.LoadTrack(path); err != nil || reloaded.Directions[0].TotalKeyframes() != 1 {
		t.Errorf("source changed on disk (err %v)", err)
	}
}
