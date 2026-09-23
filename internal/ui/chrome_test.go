package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// TestScandiChrome asserts the restrained chrome contract: one filled
// primary action, word labels (never bare glyphs) on transport controls,
// and hairline rules dividing chrome from content.
func TestScandiChrome(t *testing.T) {
	a, _ := newTestApp(t)

	if !a.playBtn.Bold {
		t.Fatalf("Play should carry label weight as the primary action, not a competing fill")
	}
	switch a.playBtn.Text {
	case "Play", "Pause", "Resume", "Restart":
	default:
		t.Fatalf("Play button carries no word label: %q", a.playBtn.Text)
	}
	if a.prevBtn.Text != "Previous" || a.nextBtn.Text != "Next" {
		t.Fatalf("transport labels = %q/%q, want words, not bare glyphs", a.prevBtn.Text, a.nextBtn.Text)
	}
	if a.prevBtn.Bold || a.nextBtn.Bold {
		t.Fatalf("secondary transport controls must stay quiet")
	}

	hasSeparator := func(c *fyne.Container) bool {
		for _, o := range c.Objects {
			if _, ok := o.(*widget.Separator); ok {
				return true
			}
		}
		return false
	}
	if !hasSeparator(a.topBar()) {
		t.Fatalf("top bar has no hairline rule dividing chrome from content")
	}
	if !hasSeparator(a.bottomBar()) {
		t.Fatalf("bottom controls have no hairline rule dividing chrome from content")
	}
}
