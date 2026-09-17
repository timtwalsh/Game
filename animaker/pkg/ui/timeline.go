package ui

import (
	"animaker/pkg/editor"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// TimelineWidget shows one row per Part in the active direction, each row
// listing that part's keyframes in time order. Clicking a keyframe selects
// it; each row has an "Add here" button that inserts a keyframe for that
// part at the current playhead time.
type TimelineWidget struct {
	project   *editor.Project
	infoLabel *widget.Label
	rowsBox   *fyne.Container

	OnPartSelected     func(partIdx int)
	OnKeyframeSelected func(partIdx, kfIdx int)
	OnKeyframeAdded    func(partIdx int)
	OnKeyframeDeleted  func(partIdx, kfIdx int)
	OnPlayToggle       func()
	OnStepForward      func()
	OnStepBackward     func()
}

func NewTimelineWidget(project *editor.Project) *TimelineWidget {
	return &TimelineWidget{project: project}
}

func (tw *TimelineWidget) SetProject(project *editor.Project) {
	tw.project = project
}

func (tw *TimelineWidget) Build() fyne.CanvasObject {
	playBtn := widget.NewButton("Play/Pause", func() {
		if tw.OnPlayToggle != nil {
			tw.OnPlayToggle()
		}
	})
	stepBackBtn := widget.NewButton("<", func() {
		if tw.OnStepBackward != nil {
			tw.OnStepBackward()
		}
	})
	stepFwdBtn := widget.NewButton(">", func() {
		if tw.OnStepForward != nil {
			tw.OnStepForward()
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
	)

	tw.infoLabel = widget.NewLabel(tw.buildInfoText())

	tw.rowsBox = container.NewVBox()
	tw.refreshRows()

	content := container.NewVBox(
		controls,
		container.NewVScroll(tw.rowsBox),
		tw.infoLabel,
	)

	bg := canvas.NewRectangle(ColorTimelineBackground)
	bg.SetMinSize(fyne.NewSize(0, 220))

	return container.NewStack(bg, content)
}

// Refresh rebuilds the part rows and info text to match current project state.
func (tw *TimelineWidget) Refresh() {
	tw.refreshRows()
	tw.RefreshInfo()
}

func (tw *TimelineWidget) RefreshInfo() {
	if tw.infoLabel != nil {
		tw.infoLabel.SetText(tw.buildInfoText())
	}
}

func (tw *TimelineWidget) refreshRows() {
	if tw.rowsBox == nil {
		return
	}
	tw.rowsBox.RemoveAll()

	dir := tw.project.ActiveDirection()
	if dir == nil || len(dir.Parts) == 0 {
		tw.rowsBox.Add(widget.NewLabel("No parts in this direction — add one from the Rig menu"))
		return
	}

	for i, part := range dir.Parts {
		tw.rowsBox.Add(tw.buildPartRow(i, part))
	}
	tw.rowsBox.Refresh()
}

func (tw *TimelineWidget) buildPartRow(partIdx int, part *editor.Part) fyne.CanvasObject {
	kindTag := "[sheet]"
	if part.Kind == editor.PartKindNestedAni {
		kindTag = "[nested]"
	}
	nameLabel := widget.NewLabel(fmt.Sprintf("%s %s", part.Name, kindTag))
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	selectBtn := widget.NewButton("select", func() {
		if tw.OnPartSelected != nil {
			tw.OnPartSelected(partIdx)
		}
	})
	selectBtn.Importance = widget.LowImportance

	addBtn := widget.NewButton("+ here", func() {
		if tw.OnKeyframeAdded != nil {
			tw.OnKeyframeAdded(partIdx)
		}
	})
	addBtn.Importance = widget.LowImportance

	kfRow := container.NewHBox()
	sel := tw.project.Selection
	for kfIdx, kf := range part.Keyframes {
		kIdx := kfIdx
		label := fmt.Sprintf("%dms", kf.TimeMs)
		btn := widget.NewButton(label, func() {
			if tw.OnKeyframeSelected != nil {
				tw.OnKeyframeSelected(partIdx, kIdx)
			}
		})
		if sel != nil && sel.PartIndex == partIdx && sel.KeyframeIndex == kIdx {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		delBtn := widget.NewButton("x", func() {
			if tw.OnKeyframeDeleted != nil {
				tw.OnKeyframeDeleted(partIdx, kIdx)
			}
		})
		delBtn.Importance = widget.DangerImportance
		kfRow.Add(container.NewHBox(btn, delBtn))
	}

	row := container.NewBorder(nil, nil, container.NewHBox(nameLabel, selectBtn, addBtn), nil, container.NewHScroll(kfRow))
	return container.NewVBox(row, widget.NewSeparator())
}

func (tw *TimelineWidget) buildInfoText() string {
	dir := tw.project.ActiveDirection()
	total := uint32(0)
	partCount := 0
	if dir != nil {
		total = dir.TotalDurationMs()
		partCount = len(dir.Parts)
	}
	loopStr := "No"
	if tw.project.Playback.LoopEnabled {
		loopStr = "Yes"
	}
	return fmt.Sprintf("Direction: %s   |   Elapsed: %dms / %dms   |   Parts: %d   |   Loop: %s",
		tw.project.Playback.ActiveDirection, tw.project.Playback.ElapsedMs, total, partCount, loopStr)
}
