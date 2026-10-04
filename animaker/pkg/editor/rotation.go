package editor

import "math"

// Rotation is in degrees, clockwise on screen (y grows downward), about a
// cell's pivot: 0 leaves the art as drawn, 90 turns its "up" to the right.
// That's how raylib, the game's renderer, rotates.

// RotatePoint turns (x, y) about the origin by deg degrees clockwise on
// screen: (0,-1), "up", becomes (1,0) at 90.
func RotatePoint(x, y, deg float32) (float32, float32) {
	if deg == 0 {
		return x, y
	}
	sin, cos := math.Sincos(float64(deg) * math.Pi / 180)
	s, c := float32(sin), float32(cos)
	return x*c - y*s, x*s + y*c
}

// RotatedCellBox is the axis-aligned box covering a w x h cell rotated deg
// degrees about its pivot (pivotX, pivotY), relative to the pivot. Its
// width and height are rounded up to whole pixels (the size of the rotated
// image); its top-left is exact, so a quarter turn of a whole-pixel cell is
// exact wherever the pivot is. With no rotation it is exactly the cell:
// (-pivotX, -pivotY) to (w-pivotX, h-pivotY).
func RotatedCellBox(w, h, pivotX, pivotY, deg float32) (minX, minY, maxX, maxY float32) {
	minX, minY, maxX, maxY = -pivotX, -pivotY, w-pivotX, h-pivotY
	if deg == 0 {
		return minX, minY, maxX, maxY
	}
	corners := [4][2]float32{{minX, minY}, {maxX, minY}, {minX, maxY}, {maxX, maxY}}
	for i, c := range corners {
		x, y := RotatePoint(c[0], c[1], deg)
		if i == 0 {
			minX, minY, maxX, maxY = x, y, x, y
			continue
		}
		minX, minY = minF32(minX, x), minF32(minY, y)
		maxX, maxY = maxF32(maxX, x), maxF32(maxY, y)
	}
	// A hair of slack so float error at 90/180/270 doesn't add a pixel.
	const eps = 1e-3
	return minX, minY,
		minX + float32(math.Ceil(float64(maxX-minX-eps))),
		minY + float32(math.Ceil(float64(maxY-minY-eps)))
}
