package main

import (
	"log"

	"animaker/pkg/app"
	"animaker/pkg/applog"
	"animaker/pkg/ui"

	fyneApp "fyne.io/fyne/v2/app"
)

func main() {
	// First, so a crash anywhere after this point leaves a log behind
	// (bin\logs\ when built by build_local.ps1). Not fatal if it fails:
	// the editor is still usable, just without a log.
	session, err := applog.Start(applog.DefaultDir())
	if err != nil {
		log.Printf("session log unavailable: %v", err)
	}

	// NewWithID, not New: without a unique ID Fyne has nowhere to store
	// preferences, which it reports as an error on every launch and which
	// also breaks the file dialog's favourite locations.
	a := fyneApp.NewWithID("com.game.animaker")
	a.Settings().SetTheme(&ui.DarkEditorTheme{})

	// Create and run the animation maker
	application := app.New(a)
	if session != nil {
		application.PreviousCrashLog = session.PreviousCrashLog
	}
	application.Run()

	// Run returns when the window closes normally; a crash never gets
	// here, which is exactly what leaves the log without its clean-exit
	// marker for the next launch to notice.
	if session != nil {
		session.Close()
	}
}
