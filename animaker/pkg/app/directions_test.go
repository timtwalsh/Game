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
	menuItem(t, a.directionMenu(3), "Delete W").Action()
	if _, ok := a.Project.CurrentTrack.Directions[3]; ok {
		t.Fatal("W still there after Delete")
	}
	a.onUndo()
	if _, ok := a.Project.CurrentTrack.Directions[3]; !ok {
		t.Fatal("undo didn't bring W back")
	}

	// Change N (posed) to E: its keyframes move.
	change := menuItem(t, a.directionMenu(0), "Change N to")
	menuItem(t, change.ChildMenu, "E").Action()
	if n := len(a.Project.CurrentTrack.Directions[1].KeyframesFor(body.ID)); n != 2 {
		t.Errorf("E has %d body keyframes after the change, want N's 2", n)
	}
	// Now E is posed, so changing S to E is offered but disabled.
	if it := menuItem(t, menuItem(t, a.directionMenu(2), "Change S to").ChildMenu, "E"); !it.Disabled {
		t.Error("changing onto a posed direction isn't disabled")
	}

	// Remove Empty Directions leaves only the posed one.
	menuItem(t, a.directionMenu(1), "Remove Empty").Action()
	if got := a.Project.CurrentTrack.SortedDirectionKeys(); len(got) != 1 || got[0] != 1 {
		t.Errorf("directions %v after Remove Empty, want [1]", got)
	}
	if !menuItem(t, a.directionMenu(1), "Delete E").Disabled {
		t.Error("deleting the last direction isn't disabled")
	}
	if !menuItem(t, a.directionMenu(1), "Remove Empty").Disabled {
		t.Error("Remove Empty enabled with nothing to remove")
	}
}

// Directions > 8 regrows a 4-direction track by facing; going back to 4
// with a diagonal posed asks first.
func TestDirectionCountMenu(t *testing.T) {
	a, body := builtApp(t) // posed in direction 0 (N)
	editor.AddStandardDirections(a.Project.CurrentTrack)
	a.refreshAll()

	menuItem(t, menuItem(t, a.directionMenu(0), "Directions").ChildMenu, "8").Action()
	tr := a.Project.CurrentTrack
	if tr.Facings() != 8 || len(tr.Directions) != 8 || len(tr.Directions[0].KeyframesFor(body.ID)) != 2 {
		t.Fatalf("after Directions > 8: %d facings, %d directions; want 8 with N's keyframes kept", tr.Facings(), len(tr.Directions))
	}
	if it := menuItem(t, menuItem(t, a.directionMenu(0), "Directions").ChildMenu, "8"); !it.Checked {
		t.Error("the current count isn't ticked")
	}

	editor.AddKeyframe(tr.Directions[1], body.ID, 0) // pose NE
	menuItem(t, menuItem(t, a.directionMenu(0), "Directions").ChildMenu, "4").Action()
	if a.Project.CurrentTrack.Facings() != 8 {
		t.Error("went to 4 directions without asking about the posed NE")
	}
	if a.Window.Canvas().Overlays().Top() == nil {
		t.Error("no confirmation shown")
	}
}
