package app

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func testApp(t *testing.T) *Application {
	t.Helper()
	a := New(test.NewApp())
	a.Window = test.NewWindow(nil)
	t.Cleanup(a.Window.Close)
	return a
}

// promptButton finds a button in the window's open dialog by its label.
func promptButton(t *testing.T, a *Application, label string) *widget.Button {
	t.Helper()
	top := a.Window.Canvas().Overlays().Top()
	if top == nil {
		t.Fatal("no dialog open")
	}
	var found *widget.Button
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == label {
			found = b
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, child := range c.Objects {
				walk(child)
			}
		}
		if w, ok := o.(fyne.Widget); ok {
			for _, child := range test.WidgetRenderer(w).Objects() {
				walk(child)
			}
		}
	}
	walk(top)
	if found == nil {
		t.Fatalf("no %q button in the dialog", label)
	}
	return found
}

func TestNoPromptWithoutUnsavedChanges(t *testing.T) {
	a := testApp(t)
	ran := false
	a.confirmDiscard(func() { ran = true })
	if !ran {
		t.Error("clean project: the action didn't run")
	}
	if a.Window.Canvas().Overlays().Top() != nil {
		t.Error("clean project: a prompt was shown")
	}
}

func TestUnsavedChangesAsk(t *testing.T) {
	for _, tc := range []struct {
		button string
		runs   bool
	}{
		{"Don't Save", true},
		{"Cancel", false},
	} {
		t.Run(tc.button, func(t *testing.T) {
			a := testApp(t)
			a.Project.Dirty = true
			ran := false
			a.confirmDiscard(func() { ran = true })
			if ran {
				t.Fatal("ran before the artist answered")
			}
			test.Tap(promptButton(t, a, tc.button))
			if ran != tc.runs {
				t.Errorf("after %q: ran = %v, want %v", tc.button, ran, tc.runs)
			}
		})
	}
}

// Save on a track that has a path saves it, then carries on.
func TestUnsavedChangesSaveThenContinue(t *testing.T) {
	a := testApp(t)
	a.Project.SavePath = t.TempDir() + "/track.anif"
	a.Project.Dirty = true
	ran := false
	a.confirmDiscard(func() { ran = true })
	test.Tap(promptButton(t, a, "Save"))
	if !ran {
		t.Error("the action didn't run after saving")
	}
	if a.Project.Dirty {
		t.Error("still dirty after saving")
	}
}
