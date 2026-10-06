package app

import (
	"animaker/pkg/editor"
	"animaker/pkg/file"
	"animaker/pkg/ui"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

// An .anichar (editor.Character) names a character's animations. It's
// open beside the track, never instead of it: the character panel lists
// its animations, and picking one opens that .anif as an ordinary track.
// The manifest is small and holds no poses, so every edit to it is saved
// straight away rather than joining the track's unsaved changes.

func (a *Application) buildCharacterPanel() {
	cp := ui.NewCharacterPanel()
	cp.OnSelect = a.selectAnimation
	cp.OnAdd = a.onAddAnimation
	cp.OnRename = a.onRenameAnimation
	cp.OnRemove = a.onRemoveAnimation
	cp.OnChanged = func() {
		a.saveCharacter()
		a.refreshCharacterMarkers()
		a.canvasWidget.Refresh() // a footprint or hitbox may have changed
	}
	cp.PlayheadMs = func() uint32 { return a.Project.Playback.ElapsedMs }
	cp.OnSelectShape = func(r ui.ShapeRef) {
		a.canvasWidget.SelectedShape = r
		a.canvasWidget.Refresh()
	}
	a.characterPanel = cp
	a.characterView = cp.Build()

	// The character's footprint and hitboxes show on the canvas while one
	// of its animations is open, and the selected one drags there. A drag
	// edits the character live and saves when it ends, like the panel.
	a.canvasWidget.Character = func() *editor.Character {
		if a.openAnimation() == nil {
			return nil
		}
		return a.character
	}
	a.canvasWidget.OnShapeEdited = func(r ui.ShapeRef, b editor.Box) {
		if a.character == nil {
			return
		}
		if r.Footprint {
			a.character.SetFootprint(b)
		} else if r.Hitbox >= 0 && r.Hitbox < len(a.character.Hitboxes) {
			h := a.character.Hitboxes[r.Hitbox]
			h.Box = b
			a.character.SetHitbox(r.Hitbox, h)
		}
		a.canvasWidget.Refresh()
	}
	a.canvasWidget.OnShapeEditEnd = func() {
		if a.character == nil {
			return
		}
		a.saveCharacter()
		a.characterPanel.Refresh()
	}
}

// selectShape selects a footprint or hitbox in both the panel and the
// canvas.
func (a *Application) selectShape(r ui.ShapeRef) {
	a.characterPanel.Selected = r
	a.canvasWidget.SelectedShape = r
}

// openAnimation is the character's animation whose track is open, or nil.
func (a *Application) openAnimation() *editor.CharAnim {
	if a.character == nil || a.Project.SavePath == "" {
		return nil
	}
	return a.character.FindByPath(a.Project.SavePath)
}

// refreshCharacter shows the open character (or hides the panel when
// there's none) and which of its animations is open.
func (a *Application) refreshCharacter() {
	if a.leftColumn == nil {
		return
	}
	if a.character == nil {
		a.leftColumn.Objects = []fyne.CanvasObject{a.palettePanel}
	} else if len(a.leftColumn.Objects) != 1 || a.leftColumn.Objects[0] == a.palettePanel {
		split := container.NewVSplit(a.characterView, a.palettePanel)
		split.SetOffset(0.45)
		a.leftColumn.Objects = []fyne.CanvasObject{split}
	}
	a.leftColumn.Refresh()

	a.characterPanel.Character = a.character
	a.characterPanel.Open = ""
	if anim := a.openAnimation(); anim != nil {
		a.characterPanel.Open = anim.Name
	}
	a.characterPanel.Refresh()
	a.refreshCharacterMarkers()
	a.updateTitle()
}

func (a *Application) refreshCharacterMarkers() {
	var ms []editor.Marker
	if anim := a.openAnimation(); anim != nil {
		ms = anim.Markers
	}
	a.timeline.SetMarkers(ms)
}

// saveCharacter writes the character, reporting whether it worked.
func (a *Application) saveCharacter() bool {
	if err := file.SaveCharacter(a.character, a.characterPath); err != nil {
		a.showError(fmt.Errorf("failed to save the character: %w", err))
		return false
	}
	return true
}

func (a *Application) onNewCharacter() {
	ui.ShowNameDialog(a.Window, "New Character", "Next", "human", func(name string) error {
		if name == "" {
			return errors.New("the character needs a name")
		}
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			path := w.URI().Path()
			w.Close()
			a.openCharacter(editor.NewCharacter(name), path)
			a.saveCharacter()
		}, a.Window)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".anichar"}))
		fd.SetFileName(name + ".anichar")
		fd.Resize(fyne.NewSize(600, 400))
		fd.Show()
		return nil
	})
}

func (a *Application) onOpenCharacter() {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := r.URI().Path()
		r.Close()
		c, err := file.LoadCharacter(path)
		if err != nil {
			a.showError(err)
			return
		}
		a.openCharacter(c, path)
		// Show one of its animations, unless one already is.
		if a.openAnimation() == nil && len(c.Animations) > 0 {
			a.selectAnimation(c.Animations[0].Name)
		}
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anichar"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func (a *Application) openCharacter(c *editor.Character, path string) {
	a.character, a.characterPath = c, path
	a.selectShape(ui.NoShape)
	a.refreshCharacter()
	a.canvasWidget.Refresh()
}

// onCloseCharacter hides the character; the open track stays open.
func (a *Application) onCloseCharacter() {
	a.character, a.characterPath = nil, ""
	a.selectShape(ui.NoShape)
	a.refreshCharacter()
	a.canvasWidget.Refresh()
}

// selectAnimation opens an animation's track, asking first about unsaved
// changes to the one open now.
func (a *Application) selectAnimation(name string) {
	anim := a.character.Find(name)
	if anim == nil {
		return
	}
	a.confirmDiscard(func() {
		if _, err := os.Stat(anim.AnifPath); err != nil {
			a.showError(fmt.Errorf("%q plays %s, which can't be opened: %w", name, anim.AnifPath, err))
			return
		}
		a.loadAndOpen(anim.AnifPath, func(t *editor.Track) (*editor.Track, string) { return t, anim.AnifPath })
	})
}

// rigSource is the track a new animation's rig is copied from: the open
// track if it's one of the character's, else the character's first
// animation whose file exists. "" when there's none.
func (a *Application) rigSource() string {
	if anim := a.openAnimation(); anim != nil {
		return anim.AnifPath
	}
	for _, anim := range a.character.Animations {
		if _, err := os.Stat(anim.AnifPath); err == nil {
			return anim.AnifPath
		}
	}
	return ""
}

// newAnimationPath is where a new animation's track is saved: beside the
// .anichar, named <character>_<animation>.anif.
func (a *Application) newAnimationPath(name string) string {
	return filepath.Join(filepath.Dir(a.characterPath), a.character.Name+"_"+name+".anif")
}

func (a *Application) onAddAnimation() {
	src := a.rigSource()
	rigName := ""
	if src != "" {
		rigName = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	}
	ui.ShowAddAnimationDialog(a.Window, rigName, func(name string, from ui.AddAnimationSource) error {
		if err := editor.ValidAnimName(name); err != nil {
			return err
		}
		if a.character.Find(name) != nil {
			return fmt.Errorf("the character already has an animation called %q", name)
		}
		if from == ui.FromExisting {
			a.addExistingAnimation(name)
			return nil
		}
		path := a.newAnimationPath(name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists - add it as an existing .anif instead", filepath.Base(path))
		}
		a.confirmDiscard(func() { a.addNewAnimation(name, path, from, src) })
		return nil
	})
}

// addNewAnimation opens a new track - from the rig at src, or empty -
// saves it at path and adds it to the character as name.
func (a *Application) addNewAnimation(name, path string, from ui.AddAnimationSource, src string) {
	trackName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if from == ui.FromRig && src != "" {
		if !a.loadAndOpen(src, func(t *editor.Track) (*editor.Track, string) { return editor.RigFrom(t, trackName), "" }) {
			return
		}
	} else {
		a.openNewTrack(trackName)
	}
	if !a.saveToPath(path) {
		return
	}
	a.addAnimation(name, path)
}

func (a *Application) addExistingAnimation(name string) {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := r.URI().Path()
		r.Close()
		if !a.addAnimation(name, path) {
			return
		}
		if a.openAnimation() == nil || a.openAnimation().Name != name {
			a.selectAnimation(name)
		}
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	if dir, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(a.characterPath))); err == nil {
		fd.SetLocation(dir)
	}
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

// addAnimation adds the track at path to the character as name and saves.
func (a *Application) addAnimation(name, path string) bool {
	if _, err := a.character.AddAnimation(name, path); err != nil {
		a.showError(err)
		return false
	}
	ok := a.saveCharacter()
	a.refreshCharacter()
	return ok
}

func (a *Application) onRenameAnimation(name string) {
	ui.ShowNameDialog(a.Window, "Rename Animation", "Rename", name, func(to string) error {
		if err := a.character.RenameAnimation(name, to); err != nil {
			return err
		}
		a.saveCharacter()
		a.refreshCharacter()
		return nil
	})
}

// onRemoveAnimation takes an animation out of the character; its .anif
// stays on disk.
func (a *Application) onRemoveAnimation(name string) {
	anim := a.character.Find(name)
	if anim == nil {
		return
	}
	dialog.ShowConfirm("Remove Animation",
		fmt.Sprintf("Remove %q from %s? Its track, %s, stays on disk.", name, a.character.Name, filepath.Base(anim.AnifPath)),
		func(ok bool) {
			if !ok {
				return
			}
			a.character.RemoveAnimation(name)
			a.saveCharacter()
			a.refreshCharacter()
		}, a.Window)
}
