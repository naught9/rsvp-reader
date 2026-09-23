package ui

import (
	"testing"

	"fyne.io/fyne/v2/widget"
)

func activeText(segs []widget.RichTextSegment, active int) string {
	if active < 0 || active >= len(segs) {
		return ""
	}
	if ts, ok := segs[active].(*widget.TextSegment); ok {
		return ts.Text
	}
	return ""
}

func TestContextWindowQuantized(t *testing.T) {
	// Within one estimated line the bounds hold; crossing a line moves
	// them, and the active word is always inside.
	lo1, hi1 := contextWindow(200, 41, 8, 30)
	lo2, hi2 := contextWindow(200, 47, 8, 30)
	if lo1 != lo2 || hi1 != hi2 {
		t.Fatalf("same-line window moved: [%d %d] vs [%d %d]", lo1, hi1, lo2, hi2)
	}
	lo3, hi3 := contextWindow(200, 48, 8, 30)
	if lo3 == lo1 {
		t.Fatalf("line-crossing window held: [%d %d]", lo3, hi3)
	}
	for pos := 0; pos < 200; pos++ {
		lo, hi := contextWindow(200, pos, 8, 30)
		if pos < lo || pos > hi {
			t.Fatalf("pos %d outside [%d %d]", pos, lo, hi)
		}
	}
	// Clamped at the ends.
	if lo, _ := contextWindow(10, 0, 8, 30); lo != 0 {
		t.Fatalf("start lo = %d", lo)
	}
	if _, hi := contextWindow(10, 9, 8, 30); hi != 9 {
		t.Fatalf("end hi = %d", hi)
	}
}

func TestContextSegments(t *testing.T) {
	words := []string{"one", "two", "three", "four", "five", "six"}
	starts := []int{0, 3}
	segs, active, _ := contextSegments(words, starts, 0, 5, 4)
	if got := activeText(segs, active); got != "five" {
		t.Fatalf("active = %q, want five", got)
	}
	// A break must precede the paragraph start at index 3.
	found := false
	for i, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && ts.Text == "four" && i > 0 {
			if prev, ok := segs[i-1].(*widget.TextSegment); ok && !prev.Style.Inline {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no paragraph break before %q", "four")
	}
	if ts := segs[active].(*widget.TextSegment); ts.Style != contextActive {
		t.Fatalf("active segment not styled active")
	}
}

func TestContextSegmentsEllipses(t *testing.T) {
	words := []string{"a", "b", "c", "d", "e"}
	segs, active, frac := contextSegments(words, []int{0}, 1, 3, 2)
	if got := activeText(segs, active); got != "c" {
		t.Fatalf("active = %q, want c", got)
	}
	first := segs[0].(*widget.TextSegment)
	last := segs[len(segs)-1].(*widget.TextSegment)
	if first.Text != "… " || last.Text != " …" {
		t.Fatalf("truncated ends = %q / %q", first.Text, last.Text)
	}
	if frac != 0.5 {
		t.Fatalf("frac = %v, want 0.5", frac)
	}
}

func TestContextSegmentsEmpty(t *testing.T) {
	if segs, active, _ := contextSegments(nil, nil, 0, -1, 0); segs != nil || active != -1 {
		t.Fatalf("empty input = %v, %d", segs, active)
	}
}

func TestToggleContext(t *testing.T) {
	a, _ := newTestApp(t)
	if a.contextOn {
		t.Fatalf("context should default off")
	}
	a.toggleContext()
	if !a.contextOn {
		t.Fatalf("toggle did not enable")
	}
	if !a.prefs().BoolWithFallback("contextPane", false) {
		t.Fatalf("choice not persisted")
	}
	if a.contextRich == nil || len(a.contextRich.Segments) == 0 {
		t.Fatalf("pane has no content after enable")
	}
	a.toggleContext()
	if a.contextOn {
		t.Fatalf("toggle did not disable")
	}
}
