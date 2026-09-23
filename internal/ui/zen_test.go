package ui

import (
	"testing"
	"time"
)

func TestZenPlayHidesImmediately(t *testing.T) {
	a, _ := newTestApp(t)
	a.togglePlay() // play melts the chrome at once
	if !a.topWrap.Hidden || !a.bottomWrap.Hidden {
		t.Fatalf("play did not hide the chrome immediately")
	}
	a.cancelTick()
}

func TestZenPauseShowsAndHolds(t *testing.T) {
	a, _ := newTestApp(t)
	a.togglePlay() // playing, hidden
	a.poke()       // mouse reveals while playing...
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("mouse move did not reveal the chrome")
	}
	a.togglePlay() // ...pause shows and holds
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("pause did not show the chrome")
	}
	if a.hideTimer != nil {
		t.Fatalf("paused chrome must carry no idle timer")
	}
	a.hideChrome() // stale hides are refused while paused
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("chrome hid while paused")
	}
}

func TestZenIdleHidesOnlyWhilePlaying(t *testing.T) {
	a, _ := newTestApp(t)
	a.togglePlay()
	a.poke()
	a.hideChrome() // idle timeout while playing: melts
	if !a.topWrap.Hidden || !a.bottomWrap.Hidden {
		t.Fatalf("idle did not hide the chrome while playing")
	}
	a.cancelHideTimer()
	a.cancelTick()
}

func TestZenTapTogglesPlayback(t *testing.T) {
	a, _ := newTestApp(t)
	a.detector.Tapped(nil)
	if !a.player.Playing() {
		t.Fatalf("tap on the reader did not start playback")
	}
	if !a.topWrap.Hidden {
		t.Fatalf("play-by-tap must hide the chrome immediately")
	}
	a.detector.Tapped(nil)
	if a.player.Playing() {
		t.Fatalf("tap on the reader did not pause playback")
	}
	if a.topWrap.Hidden {
		t.Fatalf("pause-by-tap must show the chrome")
	}
	a.cancelTick()
}

func TestZenEndedShows(t *testing.T) {
	a, _ := newTestApp(t)
	a.player.Seek(a.player.Len() - 2)
	a.player.Play(time.Now())
	a.player.Tick(time.Now().Add(time.Hour))
	a.player.Tick(time.Now().Add(2 * time.Hour))
	a.onEnded()
	if a.topWrap.Hidden || a.bottomWrap.Hidden {
		t.Fatalf("end of book must show the chrome like a pause")
	}
	if a.playBtn.Text != "Restart" {
		t.Fatalf("end-of-book button = %q", a.playBtn.Text)
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

func TestChromeGuttersMirrorWraps(t *testing.T) {
	a, _ := newTestApp(t)
	topHold := newMirrorSpacer(a.topWrap)
	bottomHold := newMirrorSpacer(a.bottomWrap)
	if h := topHold.MinSize().Height; h != a.topWrap.MinSize().Height || h <= 0 {
		t.Fatalf("top gutter %v does not mirror chrome %v", h, a.topWrap.MinSize())
	}
	if h := bottomHold.MinSize().Height; h != a.bottomWrap.MinSize().Height || h <= 0 {
		t.Fatalf("bottom gutter %v does not mirror chrome %v", h, a.bottomWrap.MinSize())
	}
	// Hiding chrome must not change the reserved gutters.
	a.hideNow()
	if h := topHold.MinSize().Height; h != a.topWrap.MinSize().Height {
		t.Fatalf("top gutter moved on hide: %v vs %v", h, a.topWrap.MinSize())
	}
	if h := bottomHold.MinSize().Height; h != a.bottomWrap.MinSize().Height {
		t.Fatalf("bottom gutter moved on hide: %v vs %v", h, a.bottomWrap.MinSize())
	}
}
