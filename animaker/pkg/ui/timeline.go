package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	timelineRulerHeight = 24
	timelineRowHeight   = 28
	timelineLabelWidth  = 120
	// timelineStartPadPx is empty ruler before time 0, so a keyframe at 0ms
	// isn't cut in half by the name column and a click just left of "0ms"
	// still lands on 0 (timeForX clamps it).
	timelineStartPadPx = 14
	// timelineZeroX is where time 0 sits in the scroll area's content.
	timelineZeroX = timelineLabelWidth + timelineStartPadPx
	timelineMarkerSize  = 10
	timelineTailPx      = 40 // empty space after the last tick, so the end isn't flush

	// Zoom is ms of animation time per screen pixel: smaller is zoomed in.
	timelineDefaultMsPerPx = 2
	timelineMinMsPerPx     = 0.25 // 4px per ms - enough to place keyframes 1ms apart
	timelineMaxMsPerPx     = 32   // ~10s across a typical panel
	timelineZoomStep       = 1.25

	// Ticks and labels pick the finest "nice" interval that still leaves
	// this much room, so they stay legible at every zoom level instead of
	// a fixed 100ms tick that turns into a solid bar zoomed out.
	timelineMinTickPx  = 8
	timelineMinLabelPx = 60
)

// timelineTickSteps are the candidate ruler intervals, in ms, in 1-2-5
// steps because those read naturally.
var timelineTickSteps = []uint32{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000}

// niceTickStep returns the finest step in timelineTickSteps that is a
// multiple of `of` (0 = any) and spans at least minPx at msPerPx. Labels
// pass the tick step as `of`, which is what guarantees a label always sits
// on a tick.
func niceTickStep(msPerPx, minPx float32, of uint32) uint32 {
	for _, step := range timelineTickSteps {
		if of != 0 && step%of != 0 {
			continue
		}
		if float32(step)/msPerPx >= minPx {
			return step
		}
	}
	return timelineTickSteps[len(timelineTickSteps)-1]
}

// clampMsPerPx keeps a zoom level inside the supported range.
func clampMsPerPx(v float32) float32 {
	if v < timelineMinMsPerPx {
		return timelineMinMsPerPx
	}
	if v > timelineMaxMsPerPx {
		return timelineMaxMsPerPx
	}
	return v
}

// zoomAnchorOffset is the horizontal scroll offset that keeps the
// animation time under contentX at the same place on screen when the zoom
// changes from oldMsPerPx to newMsPerPx. Without it, zooming re-anchors on
// the timeline's left edge and whatever you were looking at slides away.
// An anchor left of time 0 (the label column or the start pad) is
// treated as time 0.
func zoomAnchorOffset(contentX, offsetX, oldMsPerPx, newMsPerPx float32) float32 {
	viewportX := contentX - offsetX
	t := (contentX - timelineZeroX) * oldMsPerPx
	if t < 0 {
		t = 0
	}
	return timelineZeroX + t/newMsPerPx - viewportX
}

type timelineDragMode int

const (
	dragNone timelineDragMode = iota
	dragScrub
	dragRetime
)

// scrubArea is the custom-drawn part: a ruler plus one row per part, each
// row showing that part's keyframes as markers along a shared time axis.
// It's wrapped by TimelineWidget, which adds the transport controls.
//
// Dragging does one of two things, decided by where the drag *started*:
// on a keyframe marker it retimes that keyframe (unless timing is locked),
// anywhere else it scrubs the playhead.
type scrubArea struct {
	widget.BaseWidget

	project *editor.Project

	msPerPixel   float32
	timingLocked bool

	dragMode     timelineDragMode
	dragStartX   float32
	retimePart   int
	retimeKf     *editor.Keyframe // held by pointer: retiming re-sorts, so its index moves
	retimeOrigMs uint32
	// snapMarkMs is the other-part keyframe time a retime drag has snapped
	// onto, drawn as a guide line; -1 when it isn't on one.
	snapMarkMs int64
	// hoverPart/hoverKf is the keyframe marker under the mouse (-1 for
	// none), outlined so it's clear what a click or drag would take.
	hoverPart, hoverKf int

	OnScrub            func(timeMs uint32)
	OnKeyframeSelected func(partIdx, kfIdx int)
	OnRetimeStart      func(partIdx int, kf *editor.Keyframe)
	OnRetime           func(partIdx int, kf *editor.Keyframe, newTimeMs uint32)
	OnRetimeEnd        func()
	OnWheel            func(e *fyne.ScrollEvent)
	OnPartSelected     func(partIdx int)
	OnPartRename       func(partIdx int)
	OnPartDelete       func(partIdx int)

	// rowLabels are kept across renderer rebuilds rather than recreated on
	// every refresh (which playback does each tick), so Fyne isn't left
	// caching a renderer for a fresh throwaway widget per row per frame.
	rowLabels  []*partLabel
	rowDeletes []*rowDeleteButton
}

func newScrubArea(project *editor.Project) *scrubArea {
	s := &scrubArea{project: project, msPerPixel: timelineDefaultMsPerPx, snapMarkMs: -1, hoverPart: -1, hoverKf: -1}
	s.ExtendBaseWidget(s)
	return s
}

// totalMs is the scrubbable span drawn on the ruler — the editable range,
// which always extends past the last keyframe so there's empty time to
// scrub into and place the next one. Project.Seek clamps to the same value.
func (s *scrubArea) totalMs() uint32 {
	dir := s.project.ActiveDirection()
	if dir == nil {
		return editor.MinTimelineMs
	}
	return dir.EditableDurationMs()
}

func (s *scrubArea) widthForDuration() float32 {
	return timelineZeroX + float32(s.totalMs())/s.msPerPixel + timelineTailPx
}

// parts is the rig's shared part list — every part gets a row in every
// direction, whether or not it has keyframes in the one being edited.
func (s *scrubArea) parts() []*editor.Part {
	if s.project.CurrentTrack == nil {
		return nil
	}
	return s.project.CurrentTrack.Parts
}

func (s *scrubArea) MinSize() fyne.Size {
	rows := len(s.parts())
	if rows == 0 {
		rows = 1
	}
	h := timelineRulerHeight + float32(rows)*timelineRowHeight
	return fyne.NewSize(s.widthForDuration(), h)
}

func (s *scrubArea) CreateRenderer() fyne.WidgetRenderer {
	return &scrubAreaRenderer{widget: s}
}

func (s *scrubArea) xForTime(timeMs uint32) float32 {
	return timelineZeroX + float32(timeMs)/s.msPerPixel
}

func (s *scrubArea) timeForX(x float32) uint32 {
	rel := x - timelineZeroX
	if rel < 0 {
		rel = 0
	}
	return uint32(rel*s.msPerPixel + 0.5)
}

// -- Gestures --

var _ fyne.Tappable = (*scrubArea)(nil)
var _ fyne.Draggable = (*scrubArea)(nil)
var _ fyne.Scrollable = (*scrubArea)(nil)

// Tapped: click a marker to select it (and seek there); click anywhere
// else to scrub. Fyne only delivers a tap when the pointer didn't drag, so
// a click on a marker never retimes it.
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
	if s.dragMode == dragNone {
		// The first Dragged event already includes the first movement, so
		// the press point is Position minus Dragged. A 10px marker is
		// easy to miss if the hit-test uses the already-moved point.
		start := e.Position.Subtract(fyne.NewPos(e.Dragged.DX, e.Dragged.DY))
		s.dragStartX = start.X
		s.dragMode = dragScrub
		if !s.timingLocked && start.Y > timelineRulerHeight {
			if partIdx, kfIdx, ok := s.hitTestMarker(start); ok {
				kf := s.project.ActiveDirection().KeyframesFor(s.parts()[partIdx].ID)[kfIdx]
				s.dragMode = dragRetime
				s.retimePart, s.retimeKf, s.retimeOrigMs = partIdx, kf, kf.TimeMs
				if s.OnRetimeStart != nil {
					s.OnRetimeStart(partIdx, kf)
				}
			}
		}
	}

	if s.dragMode != dragRetime {
		s.scrubTo(e.Position.X)
		return
	}
	// Moved by the drag's distance from its start, not to the cursor's
	// absolute time, so grabbing a marker off-centre doesn't make it jump.
	t := float32(s.retimeOrigMs) + (e.Position.X-s.dragStartX)*s.msPerPixel
	if t < 0 {
		t = 0
	}
	ms := uint32(t + 0.5)
	s.snapMarkMs = -1
	if !altHeld() {
		// Onto another part's keyframe if one is near, else the ruler's
		// tick grid - so parts' keys line up and times come out round.
		var others []uint32
		if dir := s.project.ActiveDirection(); dir != nil {
			others = dir.KeyTimes(s.parts()[s.retimePart].ID)
		}
		var onKey bool
		ms, onKey = editor.SnapTime(ms, others, s.snapWithinMs(), niceTickStep(s.msPerPixel, timelineMinTickPx, 0))
		if onKey {
			s.snapMarkMs = int64(ms)
		}
	}
	if s.OnRetime != nil {
		s.OnRetime(s.retimePart, s.retimeKf, ms)
	}
}

// timelineSnapPx is how close, on screen, a drag must come to a keyframe
// to snap onto it - a fixed distance in pixels, so it feels the same at
// every zoom.
const timelineSnapPx = 6

func (s *scrubArea) snapWithinMs() uint32 {
	return uint32(timelineSnapPx * s.msPerPixel)
}

func (s *scrubArea) DragEnd() {
	if s.dragMode == dragRetime && s.OnRetimeEnd != nil {
		s.OnRetimeEnd()
	}
	s.dragMode = dragNone
	s.retimeKf = nil
	s.snapMarkMs = -1
	s.Refresh()
}

// altHeld reports whether Alt is down, which turns timeline snapping off.
func altHeld() bool {
	d, ok := fyne.CurrentApp().Driver().(desktop.Driver)
	return ok && d.CurrentKeyModifiers()&fyne.KeyModifierAlt != 0
}

// Scrolled hands every wheel event to TimelineWidget, which owns both the
// zoom and the scroll container this widget sits in. Implementing
// Scrollable here at all is what lets Ctrl+wheel zoom: without it the
// enclosing container would consume the wheel before this widget saw it.
func (s *scrubArea) Scrolled(e *fyne.ScrollEvent) {
	if s.OnWheel != nil {
		s.OnWheel(e)
	}
}

func (s *scrubArea) scrubTo(x float32) {
	if x < timelineLabelWidth {
		x = timelineLabelWidth
	}
	ms := s.timeForX(x)
	// A click or scrub near a keyframe lands exactly on it, so the canvas
	// shows that keyframe's pose rather than one a few ms off it.
	if dir := s.project.ActiveDirection(); dir != nil && !altHeld() {
		ms, _ = editor.SnapTime(ms, dir.KeyTimes(-1), s.snapWithinMs(), 0)
	}
	if s.OnScrub != nil {
		s.OnScrub(ms)
	}
}

func (s *scrubArea) hitTestMarker(pos fyne.Position) (partIdx, kfIdx int, ok bool) {
	dir := s.project.ActiveDirection()
	if dir == nil {
		return 0, 0, false
	}
	parts := s.parts()
	rowIdx := int((pos.Y - timelineRulerHeight) / timelineRowHeight)
	if rowIdx < 0 || rowIdx >= len(parts) {
		return 0, 0, false
	}
	for ki, kf := range dir.KeyframesFor(parts[rowIdx].ID) {
		mx := s.xForTime(kf.TimeMs)
		if pos.X >= mx-timelineMarkerSize/2 && pos.X <= mx+timelineMarkerSize/2 {
			return rowIdx, ki, true
		}
	}
	return 0, 0, false
}

// rowLabel returns the reusable label widget for a timeline row.
func (s *scrubArea) rowLabel(rowIdx int) *partLabel {
	for len(s.rowLabels) <= rowIdx {
		idx := len(s.rowLabels)
		s.rowLabels = append(s.rowLabels, newPartLabel(
			func() {
				if s.OnPartSelected != nil {
					s.OnPartSelected(idx)
				}
			},
			func() {
				if s.OnPartRename != nil {
					s.OnPartRename(idx)
				}
			}))
	}
	return s.rowLabels[rowIdx]
}

// rowDelete returns the reusable delete button for a timeline row.
func (s *scrubArea) rowDelete(rowIdx int) *rowDeleteButton {
	for len(s.rowDeletes) <= rowIdx {
		idx := len(s.rowDeletes)
		s.rowDeletes = append(s.rowDeletes, newRowDeleteButton(func() {
			if s.OnPartDelete != nil {
				s.OnPartDelete(idx)
			}
		}))
	}
	return s.rowDeletes[rowIdx]
}

// timelineDeleteWidth is the strip at the left of the label column that
// holds each row's delete button.
const timelineDeleteWidth = 18

// rowDeleteButton is the small x at the left of a timeline row that
// deletes that row's part (app.go confirms first). A drawn glyph rather
// than a widget.Button, whose minimum height is taller than a row. It sits
// beside the name rather than inside partLabel, so its click never
// competes with the name's click/double-click.
type rowDeleteButton struct {
	widget.BaseWidget
	glyph *canvas.Text
	// bg turns red under the mouse, so it's clear the x is a button.
	bg    *canvas.Rectangle
	onTap func()
}

var _ fyne.Tappable = (*rowDeleteButton)(nil)
var _ desktop.Hoverable = (*rowDeleteButton)(nil)
var _ desktop.Cursorable = (*rowDeleteButton)(nil)

func newRowDeleteButton(onTap func()) *rowDeleteButton {
	b := &rowDeleteButton{onTap: onTap, bg: canvas.NewRectangle(color.Transparent)}
	b.glyph = canvas.NewText("\u00d7", ColorDelete)
	b.glyph.TextSize = 14
	b.glyph.TextStyle = fyne.TextStyle{Bold: true}
	b.ExtendBaseWidget(b)
	return b
}

func (b *rowDeleteButton) Tapped(*fyne.PointEvent)         { b.onTap() }
func (b *rowDeleteButton) MouseIn(*desktop.MouseEvent)     { setFill(b.bg, ColorDeleteHover) }
func (b *rowDeleteButton) MouseMoved(*desktop.MouseEvent)  {}
func (b *rowDeleteButton) MouseOut()                       { setFill(b.bg, color.Transparent) }
func (b *rowDeleteButton) Cursor() desktop.Cursor          { return desktop.PointerCursor }

func (b *rowDeleteButton) CreateRenderer() fyne.WidgetRenderer {
	return &rowDeleteRenderer{b: b}
}

type rowDeleteRenderer struct{ b *rowDeleteButton }

func (r *rowDeleteRenderer) Layout(size fyne.Size) {
	r.b.bg.Resize(size)
	m := r.b.glyph.MinSize()
	r.b.glyph.Move(fyne.NewPos((size.Width-m.Width)/2, (size.Height-m.Height)/2))
}
func (r *rowDeleteRenderer) MinSize() fyne.Size { return r.b.glyph.MinSize() }
func (r *rowDeleteRenderer) Refresh()           { r.b.glyph.Refresh() }
func (r *rowDeleteRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.b.bg, r.b.glyph}
}
func (r *rowDeleteRenderer) Destroy() {}

// setFill changes a rectangle's fill and redraws just it.
func setFill(r *canvas.Rectangle, c color.Color) {
	if r.FillColor == c {
		return
	}
	r.FillColor = c
	canvas.Refresh(r)
}

// partLabel is a part's name in the timeline's label column. Click selects
// the part; double-click renames it. It's its own widget, rather than the
// scrubArea handling double-taps, because Fyne holds back a widget's every
// single tap while it waits to see whether a second one follows — which
// would make clicking to scrub the timeline feel laggy. Here only clicks on
// the name, where nothing else happens, pay that delay.
type partLabel struct {
	widget.BaseWidget
	text *canvas.Text
	// bg lights up under the mouse, so the name reads as clickable.
	bg          *canvas.Rectangle
	onTap       func()
	onDoubleTap func()
}

var _ fyne.Tappable = (*partLabel)(nil)
var _ fyne.DoubleTappable = (*partLabel)(nil)
var _ desktop.Hoverable = (*partLabel)(nil)
var _ desktop.Cursorable = (*partLabel)(nil)

func (l *partLabel) MouseIn(*desktop.MouseEvent)    { setFill(l.bg, ColorHoverFill) }
func (l *partLabel) MouseMoved(*desktop.MouseEvent) {}
func (l *partLabel) MouseOut()                      { setFill(l.bg, color.Transparent) }
func (l *partLabel) Cursor() desktop.Cursor         { return desktop.PointerCursor }

func newPartLabel(onTap, onDoubleTap func()) *partLabel {
	l := &partLabel{onTap: onTap, onDoubleTap: onDoubleTap, bg: canvas.NewRectangle(color.Transparent)}
	l.text = canvas.NewText("", ColorSectionHeader)
	l.text.TextSize = 11
	l.ExtendBaseWidget(l)
	return l
}

func (l *partLabel) setName(name string) {
	if l.text.Text != name {
		l.text.Text = name
		l.text.Refresh()
	}
}

func (l *partLabel) Tapped(*fyne.PointEvent)       { l.onTap() }
func (l *partLabel) DoubleTapped(*fyne.PointEvent) { l.onDoubleTap() }

func (l *partLabel) CreateRenderer() fyne.WidgetRenderer {
	return &partLabelRenderer{label: l}
}

type partLabelRenderer struct{ label *partLabel }

func (r *partLabelRenderer) Layout(size fyne.Size) {
	r.label.bg.Resize(size)
	r.label.text.Move(fyne.NewPos(4, size.Height/2-8))
}
func (r *partLabelRenderer) MinSize() fyne.Size { return r.label.text.MinSize() }
func (r *partLabelRenderer) Refresh()           { r.label.text.Refresh() }
func (r *partLabelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.label.bg, r.label.text}
}
func (r *partLabelRenderer) Destroy()                     {}

// -- Renderer --

type scrubAreaRenderer struct {
	widget  *scrubArea
	objects []fyne.CanvasObject

	// labels keeps each ruler label's text object across redraws (the
	// timeline redraws every playback frame), so Fyne isn't handed - and
	// left caching a texture for - a new one per label per frame.
	labels map[uint32]*canvas.Text
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

	rulerBg := canvas.NewRectangle(ColorFrameBox)
	rulerBg.Resize(fyne.NewSize(size.Width-timelineLabelWidth, timelineRulerHeight))
	rulerBg.Move(fyne.NewPos(timelineLabelWidth, 0))
	objs = append(objs, rulerBg)

	dir := s.project.ActiveDirection()
	if dir == nil {
		return objs
	}

	total := s.totalMs()
	tickStep := niceTickStep(s.msPerPixel, timelineMinTickPx, 0)
	labelStep := niceTickStep(s.msPerPixel, timelineMinLabelPx, tickStep)
	for t := uint32(0); t <= total; t += tickStep {
		x := s.xForTime(t)
		top := float32(timelineRulerHeight / 2)
		if t%labelStep == 0 {
			top = 0
			lbl := r.label(t)
			lbl.Move(fyne.NewPos(x+2, 2))
			objs = append(objs, lbl)
		}
		tick := canvas.NewLine(ColorGrid)
		tick.Position1 = fyne.NewPos(x, top)
		tick.Position2 = fyne.NewPos(x, timelineRulerHeight)
		objs = append(objs, tick)
	}

	sel := s.project.Selection
	for rowIdx, part := range s.parts() {
		rowY := timelineRulerHeight + float32(rowIdx)*timelineRowHeight

		rowBg := canvas.NewRectangle(ColorCanvasBackground)
		rowBg.Resize(fyne.NewSize(size.Width, timelineRowHeight-2))
		rowBg.Move(fyne.NewPos(0, rowY))
		objs = append(objs, rowBg)

		del := s.rowDelete(rowIdx)
		del.Resize(fyne.NewSize(timelineDeleteWidth, timelineRowHeight-2))
		del.Move(fyne.NewPos(0, rowY))
		objs = append(objs, del)

		label := s.rowLabel(rowIdx)
		label.setName(part.Name)
		label.Resize(fyne.NewSize(timelineLabelWidth-timelineDeleteWidth, timelineRowHeight-2))
		label.Move(fyne.NewPos(timelineDeleteWidth, rowY))
		objs = append(objs, label)

		for ki, kf := range dir.KeyframesFor(part.ID) {
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
			if rowIdx == s.hoverPart && ki == s.hoverKf {
				ring := canvas.NewRectangle(color.Transparent)
				ring.StrokeColor = ColorHoverStroke
				ring.StrokeWidth = 1.5
				ring.Resize(fyne.NewSize(timelineMarkerSize+6, timelineMarkerSize+6))
				ring.Move(marker.Position().SubtractXY(3, 3))
				objs = append(objs, ring)
			}
		}
	}

	// While a retime is snapped onto another part's keyframe, a guide line
	// shows which.
	if s.dragMode == dragRetime && s.snapMarkMs >= 0 {
		x := s.xForTime(uint32(s.snapMarkMs))
		guide := canvas.NewLine(color.RGBA{R: 255, G: 220, B: 60, A: 160})
		guide.StrokeWidth = 1
		guide.Position1 = fyne.NewPos(x, timelineRulerHeight)
		guide.Position2 = fyne.NewPos(x, size.Height)
		objs = append(objs, guide)
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

// label returns the ruler label for time t, reused from earlier redraws.
func (r *scrubAreaRenderer) label(t uint32) *canvas.Text {
	if r.labels == nil {
		r.labels = map[uint32]*canvas.Text{}
	}
	if l := r.labels[t]; l != nil {
		return l
	}
	l := canvas.NewText(formatTimelineMs(t), ColorSectionHeader)
	l.TextSize = 9
	r.labels[t] = l
	return l
}

// formatTimelineMs labels a ruler tick: whole seconds as "2s" once that's
// the natural unit, everything else in ms.
func formatTimelineMs(t uint32) string {
	if t >= 1000 && t%1000 == 0 {
		return fmt.Sprintf("%ds", t/1000)
	}
	return fmt.Sprintf("%dms", t)
}

// TimelineWidget wraps scrubArea with transport and keyframe controls, a
// timing lock, and zoom, and hosts it in a scroll container so long
// animations or many parts don't blow out the panel.
type TimelineWidget struct {
	project   *editor.Project
	scrub     *scrubArea
	scroll    *container.Scroll
	info      *widget.Label
	zoomLabel *widget.Label
	loopCheck *widget.Check
	speed     *widget.Select

	OnKeyframeSelected    func(partIdx, kfIdx int)
	OnKeyframeDeleted     func(partIdx, kfIdx int)
	OnNewKeyframe         func() // for the currently-selected part, at the current playhead
	OnDuplicateKeyframe   func() // the selected keyframe, to the playhead
	OnKeyframeRetimeStart func(partIdx int, kf *editor.Keyframe)
	OnKeyframeRetimed     func(partIdx int, kf *editor.Keyframe, newTimeMs uint32)
	OnKeyframeRetimeEnd   func()
	OnScrub               func(timeMs uint32)
	OnPartSelected        func(partIdx int)
	OnPartRename          func(partIdx int)
	OnPartDelete          func(partIdx int)
	OnPlay                func()
	OnStop                func()
}

func NewTimelineWidget(project *editor.Project) *TimelineWidget {
	return &TimelineWidget{project: project}
}

// SetProject switches to another project and shows its loop and speed
// settings, which otherwise kept showing the previous project's.
func (tw *TimelineWidget) SetProject(project *editor.Project) {
	tw.project = project
	tw.scrub.project = project
	if tw.loopCheck != nil {
		tw.loopCheck.SetChecked(project.Playback.LoopEnabled)
	}
	if tw.speed != nil {
		tw.speed.SetSelected(speedLabel(project.Playback.SpeedFactor))
	}
}

var speedFactors = map[string]float32{"50%": 0.5, "100%": 1.0, "200%": 2.0}

func speedLabel(f float32) string {
	for l, v := range speedFactors {
		if v == f {
			return l
		}
	}
	return "100%"
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
	tw.scrub.OnRetimeStart = func(partIdx int, kf *editor.Keyframe) {
		if tw.OnKeyframeRetimeStart != nil {
			tw.OnKeyframeRetimeStart(partIdx, kf)
		}
	}
	tw.scrub.OnRetime = func(partIdx int, kf *editor.Keyframe, ms uint32) {
		if tw.OnKeyframeRetimed != nil {
			tw.OnKeyframeRetimed(partIdx, kf, ms)
		}
	}
	tw.scrub.OnRetimeEnd = func() {
		if tw.OnKeyframeRetimeEnd != nil {
			tw.OnKeyframeRetimeEnd()
		}
	}
	tw.scrub.OnWheel = tw.onWheel
	tw.scrub.OnPartSelected = func(partIdx int) {
		if tw.OnPartSelected != nil {
			tw.OnPartSelected(partIdx)
		}
	}
	tw.scrub.OnPartDelete = func(partIdx int) {
		if tw.OnPartDelete != nil {
			tw.OnPartDelete(partIdx)
		}
	}
	tw.scrub.OnPartRename = func(partIdx int) {
		if tw.OnPartRename != nil {
			tw.OnPartRename(partIdx)
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
	dupKfBtn := widget.NewButton("Duplicate Keyframe", func() {
		if tw.OnDuplicateKeyframe != nil {
			tw.OnDuplicateKeyframe()
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

	// Off by default so retiming is discoverable; on, dragging a marker
	// scrubs instead, for when you're scrubbing around keyframes and don't
	// want to nudge one by accident.
	lockCheck := widget.NewCheck("Lock timing", func(checked bool) {
		tw.scrub.timingLocked = checked
	})

	loopCheck := widget.NewCheck("Loop", func(checked bool) {
		tw.project.Playback.LoopEnabled = checked
	})
	loopCheck.Checked = tw.project.Playback.LoopEnabled

	speedSelect := widget.NewSelect([]string{"50%", "100%", "200%"}, func(v string) {
		if f, ok := speedFactors[v]; ok {
			tw.project.Playback.SpeedFactor = f
		}
	})
	speedSelect.SetSelected(speedLabel(tw.project.Playback.SpeedFactor))
	tw.loopCheck, tw.speed = loopCheck, speedSelect

	tw.zoomLabel = widget.NewLabel("")
	zoomOut := widget.NewButton("-", func() { tw.zoomAtPlayhead(tw.scrub.msPerPixel * timelineZoomStep) })
	zoomIn := widget.NewButton("+", func() { tw.zoomAtPlayhead(tw.scrub.msPerPixel / timelineZoomStep) })
	zoomFit := widget.NewButton("Fit", tw.zoomToFit)
	tw.refreshZoomLabel()

	left := container.NewHBox(
		playBtn, stopBtn,
		widget.NewSeparator(),
		newKfBtn, dupKfBtn, deleteKfBtn, lockCheck,
		widget.NewSeparator(),
		widget.NewLabel("Speed:"), speedSelect,
		loopCheck,
	)
	right := container.NewHBox(widget.NewLabel("Zoom:"), zoomOut, tw.zoomLabel, zoomIn, zoomFit)
	controls := container.NewBorder(nil, nil, left, right)

	tw.info = widget.NewLabel(tw.buildInfoText())
	tw.scroll = container.NewScroll(tw.scrub)

	return container.NewBorder(controls, tw.info, nil, nil, tw.scroll)
}

// onWheel zooms on Ctrl+wheel (Cmd on macOS), anchored on the time under
// the cursor; any other wheel event is passed straight on to the scroll
// container, so plain scrolling behaves exactly as it did.
func (tw *TimelineWidget) onWheel(e *fyne.ScrollEvent) {
	if !zoomModifierHeld() {
		tw.scroll.Scrolled(e)
		return
	}
	switch {
	case e.Scrolled.DY > 0:
		tw.zoomAround(e.Position.X, tw.scrub.msPerPixel/timelineZoomStep)
	case e.Scrolled.DY < 0:
		tw.zoomAround(e.Position.X, tw.scrub.msPerPixel*timelineZoomStep)
	}
}

func zoomModifierHeld() bool {
	d, ok := fyne.CurrentApp().Driver().(desktop.Driver)
	return ok && d.CurrentKeyModifiers()&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0
}

// zoomAtPlayhead zooms keeping the playhead where it is on screen — the
// sensible anchor for the buttons, which have no cursor position.
func (tw *TimelineWidget) zoomAtPlayhead(msPerPx float32) {
	tw.zoomAround(tw.scrub.xForTime(tw.project.Playback.ElapsedMs), msPerPx)
}

// zoomAround applies a new zoom level, keeping the time at contentX fixed
// on screen.
func (tw *TimelineWidget) zoomAround(contentX, msPerPx float32) {
	old := tw.scrub.msPerPixel
	msPerPx = clampMsPerPx(msPerPx)
	if msPerPx == old {
		return
	}
	offset := zoomAnchorOffset(contentX, tw.scroll.Offset.X, old, msPerPx)
	tw.scrub.msPerPixel = msPerPx
	tw.applyScrollOffset(offset)
	tw.refreshZoomLabel()
}

// zoomToFit sizes the zoom so the whole editable range fills the panel.
func (tw *TimelineWidget) zoomToFit() {
	avail := tw.scroll.Size().Width - timelineZeroX - timelineTailPx
	if avail <= 0 {
		return
	}
	tw.scrub.msPerPixel = clampMsPerPx(float32(tw.scrub.totalMs()) / avail)
	tw.applyScrollOffset(0)
	tw.refreshZoomLabel()
}

// applyScrollOffset re-lays the scroll content at its new zoomed width,
// then moves to offsetX, clamped to the scrollable range. The content is
// resized before the offset is set on purpose: Fyne's Scroll checks the
// content's *current* size when applying an offset, and would otherwise
// snap a freshly-widened timeline back to 0.
func (tw *TimelineWidget) applyScrollOffset(offsetX float32) {
	tw.scrub.Refresh()
	tw.scrub.Resize(tw.scrub.MinSize().Max(tw.scroll.Size()))
	if maxOff := tw.scrub.Size().Width - tw.scroll.Size().Width; offsetX > maxOff {
		offsetX = maxOff
	}
	if offsetX < 0 {
		offsetX = 0
	}
	tw.scroll.ScrollToOffset(fyne.NewPos(offsetX, tw.scroll.Offset.Y))
	tw.scroll.Refresh()
}

func (tw *TimelineWidget) refreshZoomLabel() {
	if tw.zoomLabel == nil || tw.scrub == nil {
		return
	}
	tw.zoomLabel.SetText(fmt.Sprintf("%.0f%%", 100*timelineDefaultMsPerPx/tw.scrub.msPerPixel))
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
	total := uint32(0)
	if dir := tw.project.ActiveDirection(); dir != nil {
		total = dir.TotalDurationMs()
	}
	partCount := 0
	if tw.project.CurrentTrack != nil {
		partCount = len(tw.project.CurrentTrack.Parts)
	}
	// Playhead and animation length are shown separately on purpose: the
	// ruler runs past the end of the animation (see scrubArea.totalMs), so
	// "120ms / 0ms" would otherwise look like a bug rather than a playhead
	// parked in the empty space where the next keyframe goes.
	return fmt.Sprintf("Direction: %s   |   Playhead: %dms   |   Animation length: %dms   |   Parts: %d   |   Ctrl+wheel to zoom, drag a marker to retime (snaps; Alt for free)",
		DirectionName(tw.project.Playback.ActiveDirection), tw.project.Playback.ElapsedMs, total, partCount)
}

// -- Hover: the keyframe marker under the mouse is ringed, and the cursor
// shows a marker can be dragged sideways to retime it (a hand while
// timing is locked, when a drag scrubs instead).

var _ desktop.Hoverable = (*scrubArea)(nil)
var _ desktop.Cursorable = (*scrubArea)(nil)

func (s *scrubArea) MouseIn(e *desktop.MouseEvent) { s.MouseMoved(e) }

func (s *scrubArea) MouseMoved(e *desktop.MouseEvent) {
	part, kf := -1, -1
	if s.dragMode == dragNone {
		if p, k, ok := s.hitTestMarker(e.Position); ok {
			part, kf = p, k
		}
	}
	s.setHover(part, kf)
}

func (s *scrubArea) MouseOut() { s.setHover(-1, -1) }

func (s *scrubArea) setHover(part, kf int) {
	if part == s.hoverPart && kf == s.hoverKf {
		return
	}
	s.hoverPart, s.hoverKf = part, kf
	s.Refresh()
}

func (s *scrubArea) Cursor() desktop.Cursor {
	switch {
	case s.hoverPart < 0:
		return desktop.DefaultCursor
	case s.timingLocked:
		return desktop.PointerCursor
	default:
		return desktop.HResizeCursor
	}
}
