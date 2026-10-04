package ui

import (
	"animaker/pkg/editor"
	"image"
	"image/draw"
	"math"
)

// Fyne has no rotated-image primitive, so the canvas rotates a cell's
// pixels itself: nearest-neighbour, so pixel art stays crisp rather than
// smeared, about the cell's pivot, clockwise on screen (editor.RotatePoint).
// Rotated cells are cached, since playback asks for the same few angles of
// the same few cells over and over.

// rotationStepDeg is the precision rotation is drawn at. Angles are
// rounded to it so that an interpolated rotation, which is a new float
// every frame, mostly hits the cache; half a degree is invisible on
// sprites this size.
const rotationStepDeg = 0.5

// drawnRotation is the rotation actually drawn for deg: rounded to
// rotationStepDeg and normalised to [0, 360), so 0 means "not rotated".
func drawnRotation(deg float32) float32 {
	q := math.Round(float64(deg)/rotationStepDeg) * rotationStepDeg
	q = math.Mod(q, 360)
	if q < 0 {
		q += 360
	}
	return float32(q)
}

// rotateCell returns cell turned deg degrees about (pivotX, pivotY), in the
// cell's own pixels. The result covers editor.RotatedCellBox for the same
// arguments: its top-left is at (minX, minY) relative to the pivot.
func rotateCell(cell image.Image, pivotX, pivotY, deg float32) *image.RGBA {
	b := cell.Bounds()
	w, h := float32(b.Dx()), float32(b.Dy())
	minX, minY, maxX, maxY := editor.RotatedCellBox(w, h, pivotX, pivotY, deg)
	dst := image.NewRGBA(image.Rect(0, 0, int(maxX-minX), int(maxY-minY)))

	src, ok := cell.(*image.RGBA)
	if !ok { // cells are RGBA already (SpriteSheetTemplate); this is a fallback
		src = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(src, src.Bounds(), cell, b.Min, draw.Src)
	}
	sb := src.Bounds()
	// Each destination pixel's centre, turned back by -deg, lands on the
	// source pixel it shows.
	sin, cos := math.Sincos(-float64(deg) * math.Pi / 180)
	s, c := float32(sin), float32(cos)
	for j := 0; j < dst.Rect.Dy(); j++ {
		y := minY + float32(j) + 0.5
		for i := 0; i < dst.Rect.Dx(); i++ {
			x := minX + float32(i) + 0.5
			u := x*c - y*s + pivotX
			v := x*s + y*c + pivotY
			if u < 0 || v < 0 || u >= w || v >= h {
				continue
			}
			si := src.PixOffset(sb.Min.X+int(u), sb.Min.Y+int(v))
			di := dst.PixOffset(i, j)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

// rotationCacheBytes bounds the rotated-cell cache. Past it the cache is
// emptied and refilled with whatever is drawn next.
const rotationCacheBytes = 32 << 20

type rotationKey struct {
	cell           image.Image
	pivotX, pivotY float32
	deg            float32
}

type rotationCache struct {
	cells map[rotationKey]*image.RGBA
	bytes int
}

// get returns cell rotated by deg (already a drawnRotation) about its
// pivot, from the cache when it's been drawn before.
func (rc *rotationCache) get(cell image.Image, pivotX, pivotY, deg float32) *image.RGBA {
	k := rotationKey{cell, pivotX, pivotY, deg}
	if img, ok := rc.cells[k]; ok {
		return img
	}
	img := rotateCell(cell, pivotX, pivotY, deg)
	if rc.cells == nil || rc.bytes+len(img.Pix) > rotationCacheBytes {
		rc.cells, rc.bytes = map[rotationKey]*image.RGBA{}, 0
	}
	rc.cells[k] = img
	rc.bytes += len(img.Pix)
	return img
}
