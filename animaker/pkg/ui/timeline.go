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

const (
	timelineRulerHeight = 24
	timelineRowHeight   = 28
	timelineLabelWidth  = 120
	timelineMsPerPixel  = 2 // 1 pixel = 2ms of animation time
	timelineMarkerSize  = 10
)

// scrubArea is the custom-drawn part: a ruler plus one row per part, each
// row showing that part's keyframes as markers along a shared time axis.
// It's wrapped by TimelineWidget, which adds the transport controls.
type scrubArea struct {
	widget.BaseWidget

	project *editor.Project

	OnScrub            func(timeMs uint32)
	OnKeyframeSelected func(partIdx, kfIdx int)
}

func newScrubArea(project *editor.Project) *scrubArea {
	s := &scrubArea{project: project}
	s.ExtendBaseWidget(s)
	return s
}

func (s *scrubArea) totalMs() uint32 {
	dir := s.project.ActiveDirection()
	if dir == nil {
		return 0
	}
	total := dir.TotalDurationMs()
	if total < 500 {
		total = 500 // keep a minimum visible span so an empty/short anim isn't a sliver
	}
	return total
}

func (s *scrubArea) widthForDuration() float32 {
	return timelineLabelWidth + float32(s.totalMs())/timelineMsPerPixel + 40
}

func (s *scrubArea) MinSize() fyne.Size {
	dir := s.project.ActiveDirection()
	rows := 1
	if dir != nil && len(dir.Parts) > 0 {
		rows = len(dir.Parts)
	}
	h := timelineRulerHeight + float32(rows)*timelineRowHeight
	return fyne.NewSize(s.widthForDuration(), h)
}

func (s *scrubArea) CreateRenderer() fyne.WidgetRenderer {
	return &scrubAreaRenderer{widget: s}
}

func (s *scrubArea) xForTime(timeMs uint32) float32 {
	return timelineLabelWidth + float32(timeMs)/timelineMsPerPixel
}

func (s *scrubArea) timeForX(x float32) uint32 {
	rel := x - timelineLabelWidth
	if rel < 0 {
		rel = 0
	}
	return uint32(rel * timelineMsPerPixel)
}

// -- Gestures: click/drag the ruler or a row to scrub; click a marker to select --

var _ fyne.Tappable = (*scrubArea)(nil)
var _ fyne.Draggable = (*scrubArea)(nil)

func (s *scrubArea) Tapped(e *fyne.PointEvent) {
	if e.Position.X < timelineLabelWidth {
		return
	}
	if e.Position.Y > timelineRulerHeight {
		if partIdx, kfIdx, ok := s.hitTestMarker(e.Position); ok {
			if s.OnKeyframeSelected != nil {
				s.OnKeyframeSelected(partIdx, kfIdx)
			}
			return
		}
	}
	s.scrubTo(e.Position.X)
}

func (s *scrubArea) Dragged(e *fyne.DragEvent) {
	s.scrubTo(e.Position.X)
}

func (s *scrubArea) DragEnd() {}

func (s *scrubArea) scrubTo(x float32) {
	if x < timelineLabelWidth {
		x = timelineLabelWidth
	}
	if s.OnScrub != nil {
		s.OnScrub(s.timeForX(x))
	}
}

func (s *scrubArea) hitTestMarker(pos fyne.Position) (partIdx, kfIdx int, ok bool) {
	dir := s.project.ActiveDirection()
	if dir == nil {
		return 0, 0, false
	}
	rowIdx := int((pos.Y - timelineRulerHeight) / timelineRowHeight)
	if rowIdx < 0 || rowIdx >= len(dir.Parts) {
		return 0, 0, false
	}
	part := dir.Parts[rowIdx]
	for ki, kf := range part.Keyframes {
		mx := s.xForTime(kf.TimeMs)
		if pos.X >= mx-timelineMarkerSize/2 && pos.X <= mx+timelineMarkerSize/2 {
			return rowIdx, ki, true
		}
	}
	return 0, 0, false
}

// -- Renderer --

type scrubAreaRenderer struct {
	widget  *scrubArea
	objects []fyne.CanvasObject
}

func (r *scrubAreaRenderer) Layout(size fyne.Size) {}
func (r *scrubAreaRenderer) MinSize() fyne.Size    { return r.widget.MinSize() }
func (r *scrubAreaRenderer) Refresh() {
	r.objects = r.buildObjects()
	canvas.Refresh(r.widget)
}
func (r *scrubAreaRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = r.buildObjects()
	}
	return r.objects
}
func (r *scrubAreaRenderer) Destroy() {}

func (r *scrubAreaRenderer) buildObjects() []fyne.CanvasObject {
	s := r.widget
	size := s.MinSize()
	var objs []fyne.CanvasObject

	bg := canvas.NewRectangle(ColorTimelineBackground)
	bg.Resize(size)
	objs = append(objs, bg)

	// Ruler
	rulerBg := canvas.NewRectangle(ColorFrameBox)
	rulerBg.Resize(fyne.NewSize(size.Width-timelineLabelWidth, timelineRulerHeight))
	rulerBg.Move(fyne.NewPos(timelineLabelWidth, 0))
	objs = append(objs, rulerBg)

	dir := s.project.ActiveDirection()
	if dir == nil {
		return objs
	}

	// Tick marks every 100ms
	total := s.totalMs()
	for t := uint32(0); t <= total; t += 100 {
		x := s.xForTime(t)
		tick := canvas.NewLine(ColorGrid)
		tick.Position1 = fyne.NewPos(x, 0)
		tick.Position2 = fyne.NewPos(x, timelineRulerHeight)
		objs = append(objs, tick)
		if t%500 == 0 {
			lbl := canvas.NewText(fmt.Sprintf("%dms", t), ColorSectionHeader)
			lbl.TextSize = 9
			lbl.Move(fyne.NewPos(x+2, 2))
			objs = append(objs, lbl)
		}
	}

	sel := s.project.Selection
	for rowIdx, part := range dir.Parts {
		rowY := timelineRulerHeight + float32(rowIdx)*timelineRowHeight

		rowBg := canvas.NewRectangle(ColorCanvasBackground)
		rowBg.Resize(fyne.NewSize(size.Width, timelineRowHeight-2))
		rowBg.Move(fyne.NewPos(0, rowY))
		objs = append(objs, rowBg)

		label := canvas.NewText(part.Name, ColorSectionHeader)
		label.TextSize = 11
		label.Move(fyne.NewPos(4, rowY+timelineRowHeight/2-8))
		objs = append(objs, label)

		for ki, kf := range part.Keyframes {
			x := s.xForTime(kf.TimeMs)
			isSelected := sel != nil && sel.PartIndex == rowIdx && sel.KeyframeIndex == ki
			markerColor := ColorFrameBoxSelected
			if isSelected {
				markerColor = color.RGBA{R: 255, G: 220, B: 60, A: 255}
			}
			marker := canvas.NewRectangle(markerColor)
			marker.Resize(fyne.NewSize(timelineMarkerSize, timelineMarkerSize))
			marker.Move(fyne.NewPos(x-timelineMarkerSize/2, rowY+timelineRowHeight/2-timelineMarkerSize/2-1))
			objs = append(objs, marker)
		}
	}

	// Playhead, drawn last so it's always on top.
	playX := s.xForTime(s.project.Playback.ElapsedMs)
	playhead := canvas.NewLine(ColorScrubber)
	playhead.StrokeWidth = 2
	playhead.Position1 = fyne.NewPos(playX, 0)
	playhead.Position2 = fyne.NewPos(playX, size.Height)
	objs = append(objs, playhead)

	return objs
}

// TimelineWidget wraps scrubArea with transport controls (play/stop/step)
// and speed/loop settings, and hosts it in a scroll container so long
// animations or many parts don't blow out the panel.
type TimelineWidget struct {
	project *editor.Project
	scrub   *scrubArea
	info    *widget.Label

	OnKeyframeSelected func(partIdx, kfIdx int)
	OnKeyframeDeleted  func(partIdx, kfIdx int)
	OnNewKeyframe      func() // for the currently-selected part, at the current playhead
	OnScrub            func(timeMs uint32)
	OnPlay             func()
	OnStop             func()
}

func NewTimelineWidget(project *editor.Project) *TimelineWidget {
	return &TimelineWidget{project: project}
}

func (tw *TimelineWidget) SetProject(project *editor.Project) {
	tw.project = project
	tw.scrub.project = project
}

func (tw *TimelineWidget) Build() fyne.CanvasObject {
	tw.scrub = newScrubArea(tw.project)
	tw.scrub.OnScrub = func(ms uint32) {
		if tw.OnScrub != nil {
			tw.OnScrub(ms)
		}
	}
	tw.scrub.OnKeyframeSelected = func(partIdx, kfIdx int) {
		if tw.OnKeyframeSelected != nil {
			tw.OnKeyframeSelected(partIdx, kfIdx)
		}
	}

	playBtn := widget.NewButton("Play", func() {
		if tw.OnPlay != nil {
			tw.OnPlay()
		}
	})
	stopBtn := widget.NewButton("Stop", func() {
		if tw.OnStop != nil {
			tw.OnStop()
		}
	})
	newKfBtn := widget.NewButton("New Keyframe", func() {
		if tw.OnNewKeyframe != nil {
			tw.OnNewKeyframe()
		}
	})
	deleteKfBtn := widget.NewButton("Delete Keyframe", func() {
		sel := tw.project.Selection
		if sel == nil || sel.PartIndex < 0 || sel.KeyframeIndex < 0 {
			return
		}
		if tw.OnKeyframeDeleted != nil {
			tw.OnKeyframeDeleted(sel.PartIndex, sel.KeyframeIndex)
		}
	})
	deleteKfBtn.Importance = widget.DangerImportance

	loopCheck := widget.NewCheck("Loop", func(checked bool) {
		tw.project.Playback.LoopEnabled = checked
	})
	loopCheck.Checked = tw.project.Playback.LoopEnabled

	speedSelect := widget.NewSelect([]string{"50%", "100%", "200%"}, func(v string) {
		switch v {
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
		playBtn, stopBtn,
		widget.NewSeparator(),
		newKfBtn, deleteKfBtn,
		widget.NewSeparator(),
		widget.NewLabel("Speed:"), speedSelect,
		loopCheck,
	)

	tw.info = widget.NewLabel(tw.buildInfoText())

	scrollArea := container.NewScroll(tw.scrub)

	return container.NewBorder(controls, tw.info, nil, nil, scrollArea)
}

func (tw *TimelineWidget) Refresh() {
	if tw.scrub != nil {
		tw.scrub.Refresh()
	}
	tw.RefreshInfo()
}

func (tw *TimelineWidget) RefreshInfo() {
	if tw.info != nil {
		tw.info.SetText(tw.buildInfoText())
	}
}

func (tw *TimelineWidget) buildInfoText() string {
	dir := tw.project.ActiveDirection()
	total := uint32(0)
	partCount := 0
	if dir != nil {
		total = dir.TotalDurationMs()
		partCount = len(dir.Parts)
	}
	return fmt.Sprintf("Direction: %d   |   Elapsed: %dms / %dms   |   Parts: %d",
		tw.project.Playback.ActiveDirection, tw.project.Playback.ElapsedMs, total, partCount)
}
