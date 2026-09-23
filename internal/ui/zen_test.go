package ui

import (
	"testing"
)

func TestZenChromeHidesWhilePlaying(t *testing.T) {
	a, _ := newTestApp(t)
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("chrome must start visible")
	}
	a.togglePlay() // playing: visible + idle timer armed
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("chrome hidden immediately after play")
	}
	a.hideChrome()
	if !a.topWrap.Hidden || !a.bottomWrap.Hidden {
		t.Fatalf("hideChrome did not melt the chrome away")
	}
	a.cancelHideTimer()
	a.togglePlay() // pause reveals
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("pause did not reveal the chrome")
	}
}

func TestZenChromeNeverHidesWhilePaused(t *testing.T) {
	a, _ := newTestApp(t)
	a.hideChrome() // paused: must refuse
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("chrome hid while paused")
	}
}

func TestZenTapTogglesPlayback(t *testing.T) {
	a, _ := newTestApp(t)
	a.detector.Tapped(nil)
	if !a.player.Playing() {
		t.Fatalf("tap on the reader did not start playback")
	}
	a.cancelHideTimer()
	a.detector.Tapped(nil)
	if a.player.Playing() {
		t.Fatalf("tap on the reader did not pause playback")
	}
}

func TestStatusCollapsesWhenEmpty(t *testing.T) {
	a, _ := newTestApp(t)
	if !a.statusLabel.Hidden {
		t.Fatalf("empty status must collapse, not reserve dead space")
	}
	a.setStatus("hello")
	if a.statusLabel.Hidden || a.statusLabel.Text != "hello" {
		t.Fatalf("status not shown")
	}
	a.setStatus("")
	if !a.statusLabel.Hidden {
		t.Fatalf("cleared status must collapse again")
	}
}

func TestWPMEntryExactValue(t *testing.T) {
	a, _ := newTestApp(t)
	a.setWPM(317) // direct numeric input: exact, not snapped
	if a.player.WPM() != 317 {
		t.Fatalf("WPM = %d, want exact 317", a.player.WPM())
	}
	if a.wpmEntry.Text != "317" {
		t.Fatalf("entry = %q, want 317", a.wpmEntry.Text)
	}
	a.wpmEntry.OnSubmitted("not a number")
	if a.wpmEntry.Text != "317" {
		t.Fatalf("invalid entry corrupted speed, entry = %q", a.wpmEntry.Text)
	}
	a.wpmEntry.OnSubmitted("500 wpm")
	if a.player.WPM() != 500 {
		t.Fatalf("entry submit WPM = %d, want 500", a.player.WPM())
	}
	a.cancelHideTimer()
}
