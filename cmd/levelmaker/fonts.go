package main

import (
	_ "embed"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The level maker's text matches the animaker's: Fyne's default theme font
// (Noto Sans, the copies Fyne v2.6.3 bundles) at the animaker theme's text
// size, regular for body text and bold for section headers and titles,
// where the animaker uses bold. The fonts are embedded so the editor
// doesn't depend on -root to find them. Licence: fonts/LICENSE.txt (SIL Open
// Font License 1.1).

//go:embed fonts/NotoSans-Regular.ttf
var notoRegular []byte

//go:embed fonts/NotoSans-Bold.ttf
var notoBold []byte

const (
	fontSize = 13           // animaker/pkg/ui/theme.go: SizeNameText
	lineH    = fontSize + 5 // one line of text, with leading
)

var fontRegular, fontBold rl.Font

// loadFonts rasterises both faces at fontSize. It needs the window open.
// A face that fails to load falls back to raylib's built-in font, so text
// still shows.
func loadFonts() {
	load := func(data []byte) rl.Font {
		f := rl.LoadFontFromMemory(".ttf", data, fontSize, nil)
		if !rl.IsFontValid(f) {
			return rl.GetFontDefault()
		}
		return f
	}
	fontRegular, fontBold = load(notoRegular), load(notoBold)
}

func unloadFonts() {
	def := rl.GetFontDefault()
	for _, f := range []rl.Font{fontRegular, fontBold} {
		if f.Texture.ID != def.Texture.ID {
			rl.UnloadFont(f)
		}
	}
}

// Text is drawn at whole pixels and at the size it was rasterised at, so it
// stays as crisp as the animaker's.
func drawWith(f rl.Font, text string, x, y float32, col rl.Color) {
	pos := rl.NewVector2(float32(math.Round(float64(x))), float32(math.Round(float64(y))))
	rl.DrawTextEx(f, text, pos, fontSize, 0, col)
}

func drawText(text string, x, y float32, col rl.Color) { drawWith(fontRegular, text, x, y, col) }
func drawBold(text string, x, y float32, col rl.Color) { drawWith(fontBold, text, x, y, col) }

// measure is the width of text in the regular face, in pixels.
func measure(text string) float32 {
	return rl.MeasureTextEx(fontRegular, text, fontSize, 0).X
}
