package main

import (
	"os"

	"fyne.io/fyne/v2/app"

	"rsvp-reader/internal/store"
	"rsvp-reader/internal/ui"
)

func main() {
	fyneApp := app.NewWithID("com.rsvp.reader")
	db, err := store.Open("")
	if err != nil {
		// Progress is best-effort; the app still works without it.
		db = nil
	}
	var cliArg string
	if len(os.Args) > 1 {
		cliArg = os.Args[1]
	}
	a := ui.New(fyneApp, db, cliArg)
	_ = err
	a.ShowAndRun()
}
