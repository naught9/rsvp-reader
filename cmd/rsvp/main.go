package main

import (
	"os"

	"fyne.io/fyne/v2/app"

	"rsvp-reader/internal/store"
	"rsvp-reader/internal/ui"
)

func main() {
	db, err := store.Open("")
	if err != nil {
		// Progress is best-effort; the app still works without it.
		db = nil
	}
	// SetTheme preserves the system variant, so an explicit Dark/Light
	// choice must force it here: FYNE_THEME is read once at startup.
	// Follow System (or unset) leaves the environment alone.
	if env, ok := ui.ThemeEnvOverride(store.GetThemeOr(db, "")); ok {
		os.Setenv("FYNE_THEME", env)
	}
	fyneApp := app.NewWithID("com.rsvp.reader")
	var cliArg string
	if len(os.Args) > 1 {
		cliArg = os.Args[1]
	}
	a := ui.New(fyneApp, db, cliArg)
	_ = err
	a.ShowAndRun()
}
