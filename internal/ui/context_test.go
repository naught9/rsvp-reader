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

func TestContextSegments(t *testing.T) {
	words := []string{"one", "two", "three", "four", "five", "six"}
	starts := []int{0, 3}
	segs, active := contextSegments(words, starts, 4, 60)
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

func TestContextSegmentsTruncates(t *testing.T) {
	words := []string{"a", "b", "c", "d", "e"}
	segs, active := contextSegments(words, []int{0}, 4, 3)
	if got := activeText(segs, active); got != "e" {
		t.Fatalf("active = %q, want e", got)
	}
	first := segs[0].(*widget.TextSegment)
	if first.Text != "… " {
		t.Fatalf("window should open with ellipsis, got %q", first.Text)
	}
	// Window holds c d e only.
	n := 0
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && ts.Style.Inline && ts.Text != " " && ts.Text != "… " {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("window holds %d words, want 3", n)
	}
}

func TestContextSegmentsEmpty(t *testing.T) {
	if segs, active := contextSegments(nil, nil, 0, 60); segs != nil || active != -1 {
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
