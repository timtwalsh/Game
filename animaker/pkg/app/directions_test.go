package app

import (
	"animaker/pkg/editor"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func menuItem(t *testing.T, m *fyne.Menu, prefix string) *fyne.MenuItem {
	t.Helper()
	for _, it := range m.Items {
		if strings.HasPrefix(it.Label, prefix) {
			return it
		}
	}
	t.Fatalf("no %q item in %v", prefix, m.Items)
	return nil
}

// Reported: directions couldn't be deleted or changed in the editor.
func TestDirectionMenuDeletesChangesAndTidies(t *testing.T) {
	a, body := builtApp(t) // posed in direction 0
	track := a.Project.CurrentTrack
	editor.AddStandardDirections(track)
	a.refreshAll()

	// Deleting an empty direction needs no confirmation, and undoes.
	menuItem(t, a.directionMenu(3), "Delete Left").Action()
	if _, ok := a.Project.CurrentTrack.Directions[3]; ok {
		t.Fatal("Left still there after Delete")
	}
	a.onUndo()
	if _, ok := a.Project.CurrentTrack.Directions[3]; !ok {
		t.Fatal("undo didn't bring Left back")
	}

	// Change Up (posed) to Right: its keyframes move.
	change := menuItem(t, a.directionMenu(0), "Change Up to")
	menuItem(t, change.ChildMenu, "Right").Action()
	if n := len(a.Project.CurrentTrack.Directions[1].KeyframesFor(body.ID)); n != 2 {
		t.Errorf("Right has %d body keyframes after the change, want Up's 2", n)
	}
	// Now Right is posed, so changing Down to Right is offered but disabled.
	if it := menuItem(t, menuItem(t, a.directionMenu(2), "Change Down to").ChildMenu, "Right"); !it.Disabled {
		t.Error("changing onto a posed direction isn't disabled")
	}

	// Remove Empty Directions leaves only the posed one.
	menuItem(t, a.directionMenu(1), "Remove Empty").Action()
	if got := a.Project.CurrentTrack.SortedDirectionKeys(); len(got) != 1 || got[0] != 1 {
		t.Errorf("directions %v after Remove Empty, want [1]", got)
	}
	if !menuItem(t, a.directionMenu(1), "Delete Right").Disabled {
		t.Error("deleting the last direction isn't disabled")
	}
	if !menuItem(t, a.directionMenu(1), "Remove Empty").Disabled {
		t.Error("Remove Empty enabled with nothing to remove")
	}
}
