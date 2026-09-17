package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// TimelineWidget displays animation frames horizontally with playback controls.
type TimelineWidget struct {
	project       *editor.Project
	selectedFrame int
	infoLabel     *widget.Label

	// Callbacks
	OnFrameSelected   func(int)
	OnFrameDuplicated func(int)
	OnFrameDeleted    func(int)
	OnPlayToggle      func()
	OnStepForward     func()
	OnStepBackward    func()
	OnAddFrame        func()
}

// NewTimelineWidget creates a new timeline widget.
func NewTimelineWidget(project *editor.Project) *TimelineWidget {
	return &TimelineWidget{
		project:       project,
		selectedFrame: 0,
	}
}

// SetProject updates the project reference.
func (tw *TimelineWidget) SetProject(project *editor.Project) {
	tw.project = project
	tw.selectedFrame = 0
}

// SetSelectedFrame updates which frame is highlighted.
func (tw *TimelineWidget) SetSelectedFrame(idx int) {
	tw.selectedFrame = idx
}

// SelectedFrame returns the currently selected frame index.
func (tw *TimelineWidget) SelectedFrame() int {
	return tw.selectedFrame
}

// Build creates the full timeline container with controls and frame boxes.
func (tw *TimelineWidget) Build() fyne.CanvasObject {
	// Playback controls
	playBtn := widget.NewButton("▶ Play", func() {
		if tw.OnPlayToggle != nil {
			tw.OnPlayToggle()
		}
	})
	stepBackBtn := widget.NewButton("◀", func() {
		if tw.OnStepBackward != nil {
			tw.OnStepBackward()
		}
	})
	stepFwdBtn := widget.NewButton("▶▶", func() {
		if tw.OnStepForward != nil {
			tw.OnStepForward()
		}
	})
	addFrameBtn := widget.NewButton("+ Frame", func() {
		if tw.OnAddFrame != nil {
			tw.OnAddFrame()
		}
	})

	loopCheck := widget.NewCheck("Loop", func(checked bool) {
		tw.project.Playback.LoopEnabled = checked
	})
	loopCheck.Checked = tw.project.Playback.LoopEnabled

	speedSelect := widget.NewSelect([]string{"50%", "100%", "200%"}, func(s string) {
		switch s {
		case "50%":
			tw.project.Playback.SpeedFactor = 0.5
		case "100%":
			tw.project.Playback.SpeedFactor = 1.0
		case "200%":
			tw.project.Playback.SpeedFactor = 2.0
		}
	})
	speedSelect.SetSelected("100%")

	controls := container.NewHBox(
		stepBackBtn, playBtn, stepFwdBtn,
		widget.NewSeparator(),
		widget.NewLabel("Speed:"), speedSelect,
		widget.NewSeparator(),
		loopCheck,
		widget.NewSeparator(),
		addFrameBtn,
	)

	// Info bar
	tw.infoLabel = widget.NewLabel(tw.buildInfoText())

	// Timeline background
	timelineBg := canvas.NewRectangle(ColorTimelineBackground)
	timelineBg.SetMinSize(fyne.NewSize(0, 140))

	frameBoxes := tw.BuildFrameBoxes()
	frameScroll := container.NewHScroll(frameBoxes)
	frameScroll.SetMinSize(fyne.NewSize(0, 80))

	content := container.NewVBox(
		controls,
		frameScroll,
		tw.infoLabel,
	)

	return container.NewStack(timelineBg, content)
}

// BuildFrameBoxes creates the horizontal list of frame boxes.
func (tw *TimelineWidget) BuildFrameBoxes() *fyne.Container {
	boxes := []fyne.CanvasObject{}

	anim := tw.project.CurrentAnimation
	for i, kf := range anim.KeyFrames {
		idx := i // capture for closure
		box := tw.createFrameBox(kf, idx)
		boxes = append(boxes, box)
	}

	if len(boxes) == 0 {
		emptyLabel := widget.NewLabel("No frames — click '+ Frame' to add one")
		emptyLabel.Alignment = fyne.TextAlignCenter
		return container.NewHBox(emptyLabel)
	}

	return container.NewHBox(boxes...)
}

// RefreshInfo updates the info label text.
func (tw *TimelineWidget) RefreshInfo() {
	if tw.infoLabel != nil {
		tw.infoLabel.SetText(tw.buildInfoText())
	}
}

// createFrameBox creates a single frame box for the timeline.
func (tw *TimelineWidget) createFrameBox(kf *editor.KeyFrame, idx int) fyne.CanvasObject {
	// Determine background color
	bgColor := ColorFrameBox
	if idx == tw.selectedFrame {
		bgColor = ColorFrameBoxSelected
	}

	bg := canvas.NewRectangle(bgColor)
	bg.SetMinSize(fyne.NewSize(90, 60))

	// Frame label
	frameLabel := canvas.NewText(fmt.Sprintf("Frame %d", kf.ID), color.RGBA{R: 200, G: 200, B: 210, A: 255})
	frameLabel.TextSize = 10

	// Duration
	durLabel := canvas.NewText(fmt.Sprintf("%d ms", kf.Duration), color.RGBA{R: 240, G: 240, B: 245, A: 255})
	durLabel.TextSize = 14
	durLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Event icons
	eventIcons := tw.buildEventIcons(kf)

	content := container.NewVBox(
		frameLabel,
		durLabel,
		eventIcons,
	)

	// Wrap in a tappable button
	btn := widget.NewButton("", func() {
		tw.selectedFrame = idx
		if tw.OnFrameSelected != nil {
			tw.OnFrameSelected(idx)
		}
	})
	btn.Importance = widget.LowImportance

	return container.NewStack(bg, content, btn)
}

// buildEventIcons creates a line of event indicator icons.
func (tw *TimelineWidget) buildEventIcons(kf *editor.KeyFrame) fyne.CanvasObject {
	icons := ""

	hasSound := false
	hasParticle := false
	hasShake := false
	hasFlash := false

	for _, e := range kf.Events {
		switch e.EventType() {
		case "sound":
			hasSound = true
		case "particle":
			hasParticle = true
		case "shake":
			hasShake = true
		case "flash":
			hasFlash = true
		}
	}

	if hasSound {
		icons += "♪ "
	}
	if hasParticle {
		icons += "✦ "
	}
	if hasShake {
		icons += "⚡ "
	}
	if hasFlash {
		icons += "◆ "
	}
	if kf.HitBox != nil {
		icons += "█ "
	}
	if kf.AttackHitBox != nil {
		icons += "⚔ "
	}

	if icons == "" {
		icons = "—"
	}

	iconColor := color.RGBA{R: 150, G: 170, B: 200, A: 200}
	text := canvas.NewText(icons, iconColor)
	text.TextSize = 10
	return text
}

// buildInfoText creates the info bar text.
func (tw *TimelineWidget) buildInfoText() string {
	anim := tw.project.CurrentAnimation
	totalMs := anim.TotalDurationMs()
	loopStr := "No"
	if tw.project.Playback.LoopEnabled {
		loopStr = "Yes"
	}
	return fmt.Sprintf("Frame: %d / %d   |   Total: %dms   |   Loop: %s",
		tw.selectedFrame, len(anim.KeyFrames), totalMs, loopStr)
}
