package main

import (
	"animaker/pkg/app"
	"animaker/pkg/ui"

	fyneApp "fyne.io/fyne/v2/app"
)

func main() {
	// Create Fyne application with dark editor theme
	a := fyneApp.New()
	a.Settings().SetTheme(&ui.DarkEditorTheme{})

	// Create and run the animation maker
	application := app.New(a)
	application.Run()
}
