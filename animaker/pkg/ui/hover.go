package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Hover feedback for the editor's custom-drawn widgets (palette tiles,
// canvas parts, timeline markers and names, the import preview). Fyne's
// own buttons get theirs from the theme's hover colour.
var (
	ColorHoverFill   = color.RGBA{R: 255, G: 255, B: 255, A: 36}
	ColorHoverStroke = color.RGBA{R: 255, G: 255, B: 255, A: 170}
	ColorDeleteHover = color.RGBA{R: 220, G: 60, B: 60, A: 70}
)

// hoverBox is a highlight rectangle a widget moves over whatever is under
// the mouse. Widgets keep one and only move or hide it as the mouse moves,
// rather than rebuilding everything they draw on every mouse event.
type hoverBox struct {
	rect *canvas.Rectangle
	on   bool
}

func newHoverBox(fill bool) *hoverBox {
	r := canvas.NewRectangle(color.Transparent)
	if fill {
		r.FillColor = ColorHoverFill
	}
	r.StrokeColor = ColorHoverStroke
	r.StrokeWidth = 1
	r.Hide()
	return &hoverBox{rect: r}
}

// show places the box over pos/size, redrawing it only if that changed.
func (h *hoverBox) show(pos fyne.Position, size fyne.Size) {
	if h.on && h.rect.Position() == pos && h.rect.Size() == size {
		return
	}
	h.on = true
	h.rect.Move(pos)
	h.rect.Resize(size)
	h.rect.Show()
	canvas.Refresh(h.rect)
}

func (h *hoverBox) hide() {
	if !h.on {
		return
	}
	h.on = false
	h.rect.Hide()
	canvas.Refresh(h.rect)
}
