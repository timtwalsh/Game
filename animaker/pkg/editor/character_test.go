package editor

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCharacterAnimations(t *testing.T) {
	dir := t.TempDir()
	c := NewCharacter("human")
	if _, err := c.AddAnimation("walk", filepath.Join(dir, "w.anif")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddAnimation("idle", filepath.Join(dir, "i.anif")); err != nil {
		t.Fatal(err)
	}
	if c.Animations[0].Name != "idle" {
		t.Error("animations should stay in name order")
	}
	if _, err := c.AddAnimation("walk", filepath.Join(dir, "x.anif")); err == nil {
		t.Error("a taken name was accepted")
	}
	if _, err := c.AddAnimation("walk2", filepath.Join(dir, "w.anif")); err == nil {
		t.Error("the same track was added twice")
	}
	if _, err := c.AddAnimation("sword attack", filepath.Join(dir, "s.anif")); err == nil {
		t.Error("a name with a space was accepted")
	}
	if err := c.RenameAnimation("walk", "idle"); err == nil {
		t.Error("rename onto a taken name was accepted")
	}
	if err := c.RenameAnimation("walk", "amble"); err != nil || c.Animations[0].Name != "amble" {
		t.Errorf("rename: %v, order %v", err, c.Animations[0].Name)
	}
	if c.FindByPath(filepath.Join(dir, "w.anif")).Name != "amble" {
		t.Error("FindByPath")
	}
	if !c.RemoveAnimation("amble") || c.Find("amble") != nil || len(c.Animations) != 1 {
		t.Error("remove")
	}
}

func TestCharAnimMarkers(t *testing.T) {
	a := &CharAnim{Name: "walk"}
	a.SetMarker("footstep", -1, 250)
	a.SetMarker("footstep", -1, 0)
	a.SetMarker("dust", -1, 250)
	want := []Marker{{"footstep", 0}, {"dust", 250}, {"footstep", 250}}
	if !reflect.DeepEqual(a.Markers, want) {
		t.Fatalf("markers %v, want %v", a.Markers, want)
	}
	a.SetMarker("footstep", 2, 300) // move the second footstep
	if a.Markers[2] != (Marker{"footstep", 300}) {
		t.Errorf("moved marker: %v", a.Markers)
	}
	if got := a.MarkerTimes()["footstep"]; !reflect.DeepEqual(got, []uint32{0, 300}) {
		t.Errorf("MarkerTimes footstep = %v", got)
	}
	if a.MarkerIndex(Marker{"dust", 250}) != 1 || a.MarkerIndex(Marker{"dust", 999}) != -1 {
		t.Error("MarkerIndex")
	}
	a.RemoveMarker(1)
	if len(a.Markers) != 2 || a.Markers[1].Name != "footstep" {
		t.Errorf("after remove: %v", a.Markers)
	}
	if err := a.SetMarker("", -1, 0); err == nil {
		t.Error("an unnamed marker was accepted")
	}
}
