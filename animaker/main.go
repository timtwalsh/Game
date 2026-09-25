package main

import (
	"animaker/pkg/app"
	"animaker/pkg/ui"

	fyneApp "fyne.io/fyne/v2/app"
)

func main() {
	// NewWithID, not New: without a unique ID Fyne has nowhere to store
	// preferences, which it reports as an error on every launch and which
	// also breaks the file dialog's favourite locations.
	a := fyneApp.NewWithID("com.game.animaker")
	a.Settings().SetTheme(&ui.DarkEditorTheme{})

	// Create and run the animation maker
	application := app.New(a)
	application.Run()
}
