package ui

import (
	"animaker/pkg/editor"
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// CanvasWidget renders every Part of the active direction at the current
// playhead time, Z-sorted. Rotation is stored and saved but not visually
// applied here — Fyne has no simple rotated-image primitive, and the real
// consumer of rotation is the game's own (raylib) renderer, not this
// preview. That's a known, deliberate gap for this pass.
type CanvasWidget struct {
	widget.BaseWidget

	project  *editor.Project
	zoom     float32
	showGrid bool

	objects []fyne.CanvasObject
}

func NewCanvasWidget(project *editor.Project) *CanvasWidget {
	cw := &CanvasWidget{project: project, zoom: 4.0, showGrid: true}
	cw.ExtendBaseWidget(cw)
	return cw
}

func (cw *CanvasWidget) SetProject(project *editor.Project) {
	cw.project = project
	cw.Refresh()
}

func (cw *CanvasWidget) SetZoom(z float32) {
	cw.zoom = z
	cw.Refresh()
}

func (cw *CanvasWidget) ToggleGrid() {
	cw.showGrid = !cw.showGrid
	cw.Refresh()
}

// LocalToAnimXY converts a position local to this widget (e.g. a drag-drop
// point, already offset by the widget's own absolute position) into the
// animation's own X/Y coordinate space - the inverse of how drawPart places
// a resolved transform (see drawPart: screen = center + animXY*zoom).
func (cw *CanvasWidget) LocalToAnimXY(local fyne.Position) (x, y float32) {
	size := cw.Size()
	centerX := size.Width / 2
	centerY := size.Height / 2
	if cw.zoom == 0 {
		return 0, 0
	}
	return (local.X - centerX) / cw.zoom, (local.Y - centerY) / cw.zoom
}

func (cw *CanvasWidget) CreateRenderer() fyne.WidgetRenderer {
	return &canvasRenderer{widget: cw}
}

func (cw *CanvasWidget) MinSize() fyne.Size {
	return fyne.NewSize(300, 300)
}

type canvasRenderer struct {
	widget  *CanvasWidget
	objects []fyne.CanvasObject
}

func (r *canvasRenderer) Layout(size fyne.Size) {
	for _, obj := range r.objects {
		obj.Resize(obj.MinSize())
	}
}

func (r *canvasRenderer) MinSize() fyne.Size { return fyne.NewSize(300, 300) }

func (r *canvasRenderer) Refresh() {
	r.objects = r.buildObjects()
	canvas.Refresh(r.widget)
}

func (r *canvasRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = r.buildObjects()
	}
	return r.objects
}

func (r *canvasRenderer) Destroy() {}

type resolvedDraw struct {
	z      float32
	part   *editor.Part
	tr     editor.ResolvedTransform
	sheet  *editor.SpriteSheetTemplate
}

func (r *canvasRenderer) buildObjects() []fyne.CanvasObject {
	cw := r.widget
	size := cw.Size()
	if size.Width == 0 {
		size = fyne.NewSize(400, 400)
	}
	objs := []fyne.CanvasObject{}

	bg := canvas.NewRectangle(ColorCanvasBackground)
	bg.Resize(size)
	objs = append(objs, bg)

	centerX := size.Width / 2
	centerY := size.Height / 2

	if cw.showGrid {
		objs = append(objs, r.buildGrid(centerX, centerY)...)
	}

	// Origin crosshair
	crossLen := float32(20)
	hLine := canvas.NewLine(ColorOriginCrosshair)
	hLine.Position1 = fyne.NewPos(centerX-crossLen, centerY)
	hLine.Position2 = fyne.NewPos(centerX+crossLen, centerY)
	objs = append(objs, hLine)
	vLine := canvas.NewLine(ColorOriginCrosshair)
	vLine.Position1 = fyne.NewPos(centerX, centerY-crossLen)
	vLine.Position2 = fyne.NewPos(centerX, centerY+crossLen)
	objs = append(objs, vLine)

	dir := cw.project.ActiveDirection()
	if dir == nil {
		return objs
	}

	elapsed := cw.project.Playback.ElapsedMs
	var draws []resolvedDraw
	for _, part := range dir.Parts {
		tr := part.ValueAt(elapsed)
		var sheet *editor.SpriteSheetTemplate
		if part.Kind == editor.PartKindSheet {
			sheet = cw.project.ResolveActiveSheet(part)
		}
		draws = append(draws, resolvedDraw{z: tr.Z, part: part, tr: tr, sheet: sheet})
	}
	sort.SliceStable(draws, func(i, j int) bool { return draws[i].z < draws[j].z })

	for _, d := range draws {
		objs = append(objs, r.drawPart(d, centerX, centerY)...)
	}

	return objs
}

func (r *canvasRenderer) drawPart(d resolvedDraw, centerX, centerY float32) []fyne.CanvasObject {
	cw := r.widget
	zoom := cw.zoom

	if d.part.Kind == editor.PartKindNestedAni {
		// Placeholder box - see the type doc comment on CanvasWidget for why
		// full nested playback isn't rendered here.
		w, h := float32(24)*zoom/4, float32(24)*zoom/4
		x := centerX + d.tr.X*zoom - w/2
		y := centerY + d.tr.Y*zoom - h/2
		box := canvas.NewRectangle(color.RGBA{R: 0, G: 0, B: 0, A: 0})
		box.StrokeColor = ColorOriginCrosshair
		box.StrokeWidth = 1
		box.Resize(fyne.NewSize(w, h))
		box.Move(fyne.NewPos(x, y))
		label := canvas.NewText(fmt.Sprintf("%s -> %s", d.part.Name, d.part.NestedAniPath), ColorOriginCrosshair)
		label.TextSize = 9
		label.Move(fyne.NewPos(x, y+h+2))
		return []fyne.CanvasObject{box, label}
	}

	if d.sheet == nil || d.sheet.Image == nil {
		return nil
	}
	cellImg, err := d.sheet.CellImage(d.tr.Row, d.tr.Col)
	if err != nil {
		return nil
	}

	w := float32(d.sheet.CellW) * zoom
	h := float32(d.sheet.CellH) * zoom
	x := centerX + d.tr.X*zoom - d.sheet.PivotX*zoom
	y := centerY + d.tr.Y*zoom - d.sheet.PivotY*zoom

	img := canvas.NewImageFromImage(cellImg)
	img.ScaleMode = canvas.ImageScalePixels
	img.FillMode = canvas.ImageFillOriginal
	img.Resize(fyne.NewSize(w, h))
	img.Move(fyne.NewPos(x, y))
	return []fyne.CanvasObject{img}
}

func (r *canvasRenderer) buildGrid(centerX, centerY float32) []fyne.CanvasObject {
	zoom := r.widget.zoom
	gridSpacing := float32(16) * zoom
	gridArea := float32(256) * zoom
	startX := centerX - gridArea/2
	startY := centerY - gridArea/2

	var objs []fyne.CanvasObject
	for x := startX; x <= centerX+gridArea/2; x += gridSpacing {
		line := canvas.NewLine(ColorGrid)
		line.Position1 = fyne.NewPos(x, startY)
		line.Position2 = fyne.NewPos(x, centerY+gridArea/2)
		objs = append(objs, line)
	}
	for y := startY; y <= centerY+gridArea/2; y += gridSpacing {
		line := canvas.NewLine(ColorGrid)
		line.Position1 = fyne.NewPos(startX, y)
		line.Position2 = fyne.NewPos(centerX+gridArea/2, y)
		objs = append(objs, line)
	}
	return objs
}
