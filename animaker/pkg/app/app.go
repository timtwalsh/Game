package app

import (
	"animaker/pkg/editor"
	"animaker/pkg/file"
	"animaker/pkg/ui"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
)

// Application is the main coordinator tying together project state, UI, and file operations.
type Application struct {
	FyneApp  fyne.App
	Window   fyne.Window
	Project  *editor.Project

	// UI components
	canvasWidget *ui.CanvasWidget
	timeline     *ui.TimelineWidget
	properties   *ui.PropertiesPanel

	// Playback ticker
	playbackTicker *time.Ticker
	playbackDone   chan bool
}

// New creates a new Application instance.
func New(fyneApp fyne.App) *Application {
	return &Application{
		FyneApp:      fyneApp,
		Project:      editor.NewProject("untitled"),
		playbackDone: make(chan bool),
	}
}

// Run initializes the UI and starts the application.
func (a *Application) Run() {
	a.Window = a.FyneApp.NewWindow("ANIFile Animation Maker")
	a.Window.Resize(fyne.NewSize(1200, 800))

	// Create UI components
	a.canvasWidget = ui.NewCanvasWidget(a.Project)
	a.timeline = ui.NewTimelineWidget(a.Project)
	a.properties = ui.NewPropertiesPanel(a.Project)

	// Wire up callbacks
	a.wireCallbacks()

	// Build layout
	propertiesPanel := a.properties.Build()
	timelinePanel := a.timeline.Build()
	mainLayout := ui.BuildMainLayout(a.canvasWidget, propertiesPanel, timelinePanel)

	// Build menu
	menu := ui.BuildMenuBar(
		a.onNewAnimation,
		a.onOpenAnimation,
		a.onSaveAnimation,
		a.onSaveAsAnimation,
		a.onImportSpriteSheet,
		a.onUndo,
		a.onRedo,
		a.canvasWidget.ToggleGrid,
		a.canvasWidget.ToggleHitboxes,
		a.onZoom,
	)
	a.Window.SetMainMenu(menu)

	// Register keyboard shortcuts
	a.registerShortcuts()

	a.Window.SetContent(mainLayout)
	a.Window.SetOnClosed(a.onClose)

	// Start playback loop
	go a.playbackLoop()

	a.Window.ShowAndRun()
}

// wireCallbacks connects UI callbacks to application logic.
func (a *Application) wireCallbacks() {
	// Timeline callbacks
	a.timeline.OnFrameSelected = func(idx int) {
		a.Project.Playback.CurrentFrame = idx
		a.refreshAll()
	}
	a.timeline.OnPlayToggle = func() {
		a.Project.TogglePlayback()
	}
	a.timeline.OnStepForward = func() {
		a.Project.StepForward()
		a.timeline.SetSelectedFrame(a.Project.Playback.CurrentFrame)
		a.refreshAll()
	}
	a.timeline.OnStepBackward = func() {
		a.Project.StepBackward()
		a.timeline.SetSelectedFrame(a.Project.Playback.CurrentFrame)
		a.refreshAll()
	}
	a.timeline.OnAddFrame = func() {
		a.addFrame()
	}

	// Properties callbacks
	a.properties.OnDurationChanged = func(d uint32) {
		a.timeline.RefreshInfo()
	}
	a.properties.OnSpriteSelected = func(sheetName string, index int) {
		a.selectSprite(sheetName, index)
	}
	a.properties.OnBoxChanged = func() {
		a.canvasWidget.Refresh()
	}
	a.properties.OnAddEvent = func(eventType string) {
		a.addEvent(eventType)
	}
	a.properties.OnDeleteEvent = func(idx int) {
		a.deleteEvent(idx)
	}

	// Canvas callbacks
	a.canvasWidget.OnBoxChanged = func() {
		a.properties.Refresh()
	}
}

// registerShortcuts adds keyboard shortcuts.
func (a *Application) registerShortcuts() {
	// Only works with desktop driver
	if deskCanvas, ok := a.Window.Canvas().(desktop.Canvas); ok {
		// Ctrl+N - New
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName:  fyne.KeyN,
			Modifier: fyne.KeyModifierControl,
		}, func(_ fyne.Shortcut) {
			a.onNewAnimation()
		})

		// Ctrl+O - Open
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName:  fyne.KeyO,
			Modifier: fyne.KeyModifierControl,
		}, func(_ fyne.Shortcut) {
			a.onOpenAnimation()
		})

		// Ctrl+S - Save
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName:  fyne.KeyS,
			Modifier: fyne.KeyModifierControl,
		}, func(_ fyne.Shortcut) {
			a.onSaveAnimation()
		})

		// Ctrl+Z - Undo
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName:  fyne.KeyZ,
			Modifier: fyne.KeyModifierControl,
		}, func(_ fyne.Shortcut) {
			a.onUndo()
		})

		// Ctrl+Shift+Z - Redo
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName:  fyne.KeyZ,
			Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
		}, func(_ fyne.Shortcut) {
			a.onRedo()
		})

		// D - Duplicate frame
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName: fyne.KeyD,
		}, func(_ fyne.Shortcut) {
			a.duplicateFrame()
		})

		// X/Delete - Delete frame
		deskCanvas.AddShortcut(&desktop.CustomShortcut{
			KeyName: fyne.KeyX,
		}, func(_ fyne.Shortcut) {
			a.deleteFrame()
		})
	}
}

// -- Menu handlers --

func (a *Application) onNewAnimation() {
	ui.ShowNewAnimationDialog(a.Window, func(name, charSize string, loop bool, anchor string) {
		a.Project = editor.NewProject(name)
		a.Project.CurrentAnimation.Config.CharacterSize = charSize
		a.Project.CurrentAnimation.Config.Loop = loop
		a.Project.CurrentAnimation.Config.RootAnchor = anchor
		a.Project.Playback.LoopEnabled = loop

		a.canvasWidget.SetProject(a.Project)
		a.timeline.SetProject(a.Project)
		a.properties.SetProject(a.Project)
		a.refreshAll()
		a.Window.SetTitle("ANIFile Animation Maker — " + name)
	})
}

func (a *Application) onOpenAnimation() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		filePath := reader.URI().Path()
		reader.Close()

		anim, err := file.LoadAnimation(filePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load animation: %w", err), a.Window)
			return
		}

		a.Project.CurrentAnimation = anim
		a.Project.SavePath = filePath
		a.Project.Dirty = false
		a.Project.UndoStack.Clear()

		a.canvasWidget.SetProject(a.Project)
		a.timeline.SetProject(a.Project)
		a.properties.SetProject(a.Project)
		a.refreshAll()
		a.Window.SetTitle("ANIFile Animation Maker — " + anim.Metadata.Name)
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func (a *Application) onSaveAnimation() {
	if a.Project.SavePath == "" {
		a.onSaveAsAnimation()
		return
	}
	a.saveToPath(a.Project.SavePath)
}

func (a *Application) onSaveAsAnimation() {
	fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		filePath := writer.URI().Path()
		writer.Close()

		a.saveToPath(filePath)
	}, a.Window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".anif"}))
	fd.SetFileName(a.Project.CurrentAnimation.Metadata.Name + ".anif")
	fd.Resize(fyne.NewSize(600, 400))
	fd.Show()
}

func (a *Application) saveToPath(path string) {
	a.Project.CurrentAnimation.Metadata.UpdatedAt = time.Now()
	if err := file.SaveAnimation(a.Project.CurrentAnimation, path); err != nil {
		dialog.ShowError(fmt.Errorf("failed to save: %w", err), a.Window)
		return
	}
	a.Project.SavePath = path
	a.Project.Dirty = false
	a.Window.SetTitle("ANIFile Animation Maker — " + a.Project.CurrentAnimation.Metadata.Name)
}

func (a *Application) onImportSpriteSheet() {
	ui.ShowImportSheetDialog(a.Window, func(filePath string, config editor.GridConfig) {
		img, err := file.LoadImage(filePath)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load image: %w", err), a.Window)
			return
		}

		// Derive sheet name from filename
		baseName := filepath.Base(filePath)
		sheetName := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		sheet := editor.NewSpriteSheet(sheetName, filePath, img, config)
		a.Project.LoadedSheets[sheetName] = sheet

		// Save .sprsh metadata alongside the image
		metaPath := filePath + ".sprsh"
		if err := file.SaveSpriteSheetMeta(sheet, metaPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save sheet metadata: %w", err), a.Window)
		}

		a.properties.RefreshSheetList()
		dialog.ShowInformation("Import Complete",
			fmt.Sprintf("Imported '%s' with %d sprites (%dx%d grid)",
				sheetName, sheet.SpriteCount(), config.Cols, config.Rows),
			a.Window)
	})
}

func (a *Application) onUndo() {
	if a.Project.Undo() {
		a.refreshAll()
	}
}

func (a *Application) onRedo() {
	if a.Project.Redo() {
		a.refreshAll()
	}
}

func (a *Application) onZoom(factor float32) {
	a.canvasWidget.SetZoom(factor)
}

// -- Frame operations --

func (a *Application) addFrame() {
	a.Project.RecordUndo()
	selectedIdx := a.timeline.SelectedFrame()
	editor.AddKeyFrame(a.Project.CurrentAnimation, selectedIdx)
	a.Project.Dirty = true
	a.refreshAll()
}

func (a *Application) duplicateFrame() {
	idx := a.timeline.SelectedFrame()
	if idx < 0 || idx >= len(a.Project.CurrentAnimation.KeyFrames) {
		return
	}
	a.Project.RecordUndo()
	_, err := editor.DuplicateKeyFrame(a.Project.CurrentAnimation, idx)
	if err != nil {
		dialog.ShowError(err, a.Window)
		return
	}
	a.Project.Dirty = true
	a.timeline.SetSelectedFrame(idx + 1)
	a.Project.Playback.CurrentFrame = idx + 1
	a.refreshAll()
}

func (a *Application) deleteFrame() {
	idx := a.timeline.SelectedFrame()
	if idx < 0 || idx >= len(a.Project.CurrentAnimation.KeyFrames) {
		return
	}
	a.Project.RecordUndo()
	if err := editor.DeleteKeyFrame(a.Project.CurrentAnimation, idx); err != nil {
		dialog.ShowError(err, a.Window)
		return
	}
	a.Project.Dirty = true

	// Adjust selection
	if idx >= len(a.Project.CurrentAnimation.KeyFrames) && idx > 0 {
		idx--
	}
	a.timeline.SetSelectedFrame(idx)
	a.Project.Playback.CurrentFrame = idx
	a.refreshAll()
}

// -- Sprite selection --

func (a *Application) selectSprite(sheetName string, index int) {
	kf := a.Project.GetCurrentKeyFrame()
	if kf == nil {
		return
	}

	a.Project.RecordUndo()
	kf.Sprite = editor.SpriteReference{SheetName: sheetName, Index: index}
	a.Project.Dirty = true

	// Update canvas with new sprite image
	a.updateCanvasSprite()
	a.refreshAll()
}

func (a *Application) updateCanvasSprite() {
	kf := a.Project.GetCurrentKeyFrame()
	if kf == nil {
		a.canvasWidget.SetSpriteImage(nil)
		return
	}

	if kf.Sprite.SheetName != "" {
		sheet, ok := a.Project.LoadedSheets[kf.Sprite.SheetName]
		if ok {
			img, err := sheet.GetSpriteImage(kf.Sprite.Index)
			if err == nil {
				a.canvasWidget.SetSpriteImage(img)
				return
			}
		}
	}
	a.canvasWidget.SetSpriteImage(nil)
}

// -- Events --

func (a *Application) addEvent(eventType string) {
	kf := a.Project.GetCurrentKeyFrame()
	if kf == nil {
		return
	}

	switch eventType {
	case "sound":
		ui.ShowSoundEventDialog(a.Window, nil, func(e *editor.SoundEvent) {
			a.Project.RecordUndo()
			kf.Events = append(kf.Events, e)
			a.Project.Dirty = true
			a.refreshAll()
		})
	case "particle":
		ui.ShowParticleEventDialog(a.Window, nil, func(e *editor.ParticleEvent) {
			a.Project.RecordUndo()
			kf.Events = append(kf.Events, e)
			a.Project.Dirty = true
			a.refreshAll()
		})
	case "shake":
		ui.ShowShakeEventDialog(a.Window, nil, func(e *editor.ShakeEvent) {
			a.Project.RecordUndo()
			kf.Events = append(kf.Events, e)
			a.Project.Dirty = true
			a.refreshAll()
		})
	case "flash":
		ui.ShowFlashEventDialog(a.Window, nil, func(e *editor.FlashEvent) {
			a.Project.RecordUndo()
			kf.Events = append(kf.Events, e)
			a.Project.Dirty = true
			a.refreshAll()
		})
	}
}

func (a *Application) deleteEvent(idx int) {
	kf := a.Project.GetCurrentKeyFrame()
	if kf == nil || idx < 0 || idx >= len(kf.Events) {
		return
	}

	a.Project.RecordUndo()
	kf.Events = append(kf.Events[:idx], kf.Events[idx+1:]...)
	a.Project.Dirty = true
	a.refreshAll()
}

// -- Playback loop --

func (a *Application) playbackLoop() {
	ticker := time.NewTicker(16 * time.Millisecond) // ~60fps
	defer ticker.Stop()

	lastTick := time.Now()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			deltaMs := uint32(now.Sub(lastTick).Milliseconds())
			lastTick = now

			if a.Project.Playback.IsPlaying {
				a.Project.AdvancePlayback(deltaMs)
				// Update UI on the main thread
				a.timeline.SetSelectedFrame(a.Project.Playback.CurrentFrame)
				a.updateCanvasSprite()
				a.canvasWidget.Refresh()
				a.timeline.RefreshInfo()
			}
		case <-a.playbackDone:
			return
		}
	}
}

// -- Refresh --

func (a *Application) refreshAll() {
	a.updateCanvasSprite()
	a.canvasWidget.Refresh()
	a.properties.Refresh()
	a.timeline.RefreshInfo()
}

func (a *Application) onClose() {
	close(a.playbackDone)
}
