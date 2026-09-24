package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// DarkEditorTheme is a professional dark theme for the animation editor.
type DarkEditorTheme struct{}

var _ fyne.Theme = (*DarkEditorTheme)(nil)

func (t *DarkEditorTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 30, G: 30, B: 35, A: 255}
	case theme.ColorNameForeground:
		return color.RGBA{R: 220, G: 220, B: 225, A: 255}
	case theme.ColorNameButton:
		return color.RGBA{R: 55, G: 55, B: 65, A: 255}
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 40, G: 40, B: 48, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 40, G: 40, B: 48, A: 255}
	case theme.ColorNameInputBorder:
		return color.RGBA{R: 70, G: 70, B: 80, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 100, G: 160, B: 255, A: 255}
	case theme.ColorNameHover:
		return color.RGBA{R: 60, G: 60, B: 70, A: 255}
	case theme.ColorNameSelection:
		return color.RGBA{R: 50, G: 100, B: 180, A: 128}
	case theme.ColorNameSeparator:
		return color.RGBA{R: 55, G: 55, B: 60, A: 255}
	case theme.ColorNameMenuBackground:
		return color.RGBA{R: 38, G: 38, B: 45, A: 255}
	case theme.ColorNameOverlayBackground:
		return color.RGBA{R: 35, G: 35, B: 42, A: 240}
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 80, G: 80, B: 90, A: 200}
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (t *DarkEditorTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *DarkEditorTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *DarkEditorTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 13
	case theme.SizeNameSeparatorThickness:
		return 1
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// --- Editor-specific colors ---

var (
	// Canvas colors
	ColorCanvasBackground = color.RGBA{R: 24, G: 24, B: 28, A: 255}
	ColorGrid             = color.RGBA{R: 128, G: 128, B: 128, A: 30}
	ColorOriginCrosshair  = color.RGBA{R: 80, G: 200, B: 80, A: 180}
	// ColorRefBox outlines the character-sized placement guide drawn from
	// the origin down-right. Deliberately distinct from the crosshair so
	// the box reads as a guide rather than part of the axes.
	ColorRefBox = color.RGBA{R: 90, G: 140, B: 220, A: 150}

	// Hitbox colors
	ColorCollisionBox       = color.RGBA{R: 0, G: 120, B: 255, A: 80}
	ColorCollisionBoxBorder = color.RGBA{R: 0, G: 150, B: 255, A: 200}
	ColorAttackBox          = color.RGBA{R: 255, G: 60, B: 60, A: 80}
	ColorAttackBoxBorder    = color.RGBA{R: 255, G: 100, B: 100, A: 200}
	ColorHandlePoint        = color.RGBA{R: 255, G: 255, B: 255, A: 220}

	// Timeline colors
	ColorTimelineBackground = color.RGBA{R: 28, G: 28, B: 34, A: 255}
	ColorFrameBox           = color.RGBA{R: 50, G: 50, B: 60, A: 255}
	ColorFrameBoxSelected   = color.RGBA{R: 70, G: 110, B: 180, A: 255}
	ColorFrameBoxHover      = color.RGBA{R: 60, G: 60, B: 75, A: 255}
	ColorScrubber           = color.RGBA{R: 255, G: 180, B: 50, A: 255}

	// Properties colors
	ColorSectionHeader  = color.RGBA{R: 180, G: 200, B: 255, A: 255}
	ColorSpritePickerBg = color.RGBA{R: 35, G: 35, B: 42, A: 255}
	ColorSpriteSelected = color.RGBA{R: 100, G: 160, B: 255, A: 180}
)
