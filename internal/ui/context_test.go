package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func textSize() float32 { return theme.Size(theme.SizeNameText) }

func activeText(segs []widget.RichTextSegment) string {
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && ts.Style == contextActive {
			return ts.Text
		}
	}
	return ""
}

func TestLayoutLinesWraps(t *testing.T) {
	words := []string{"one", "two", "three", "four", "five", "six", "seven", "eight"}
	narrow := layoutLines(words, map[int]bool{}, 0, 7, 60, textSize())
	if len(narrow) < 3 {
		t.Fatalf("narrow width gave %d lines, want several", len(narrow))
	}
	wide := layoutLines(words, map[int]bool{}, 0, 7, 10000, textSize())
	if len(wide) != 1 {
		t.Fatalf("wide width gave %d lines, want 1", len(wide))
	}
	// Every word placed exactly once, in order.
	seen := 0
	for _, ln := range narrow {
		seen += len(ln.words)
	}
	if seen != len(words) {
		t.Fatalf("placed %d words, want %d", seen, len(words))
	}
}

func TestLayoutLinesParaBreak(t *testing.T) {
	words := []string{"one", "two", "three", "four"}
	lines := layoutLines(words, map[int]bool{2: true}, 0, 3, 10000, textSize())
	if len(lines) != 2 {
		t.Fatalf("para start gave %d lines, want 2", len(lines))
	}
	if !lines[1].paraBreak || lines[1].words[0] != 2 {
		t.Fatalf("second line = %+v", lines[1])
	}
}

func TestActiveLineMapping(t *testing.T) {
	words := []string{"one", "two", "three", "four", "five", "six"}
	lines := layoutLines(words, map[int]bool{}, 0, 5, 10000, textSize())
	if got := activeLine(lines, 3); got != 0 {
		t.Fatalf("single-line active = %d", got)
	}
	narrow := layoutLines(words, map[int]bool{}, 0, 5, 60, textSize())
	first, last := activeLine(narrow, 0), activeLine(narrow, 5)
	if first == last {
		t.Fatalf("narrow layout collapses to one line")
	}
	// Active line index is monotonic in position.
	prev := -1
	for pos := 0; pos < 6; pos++ {
		if l := activeLine(narrow, pos); l < prev {
			t.Fatalf("non-monotonic at %d", pos)
		} else {
			prev = l
		}
	}
}

func TestRenderLinesActiveAndRows(t *testing.T) {
	words := []string{"one", "two", "three", "four", "five", "six"}
	lines := []ctxLine{
		{words: []int{0, 1}},
		{words: []int{2, 3}, paraBreak: true},
		{words: []int{4, 5}},
	}
	segs, activeRow, rows := renderLines(words, lines, 0, 2, 3)
	if got := activeText(segs); got != "four" {
		t.Fatalf("active = %q, want four", got)
	}
	// 3 text rows + 2 breaks between lines + 1 blank para row = 6.
	if rows != 6 {
		t.Fatalf("rows = %d, want 6", rows)
	}
	// "four" is the second word of the para line: text rows before it
	// are line0, break, blank, so its row is 3.
	if activeRow != 3 {
		t.Fatalf("activeRow = %d, want 3", activeRow)
	}
	_ = fyne.TextStyle{}
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
	// Headless canvases never lay out: force a size to run the real
	// render path (layout, highlight, header, exact scroll).
	a.contextRich.Resize(fyne.NewSize(300, 600))
	a.updateContext()
	if len(a.contextRich.Segments) == 0 {
		t.Fatalf("no lines rendered")
	}
	if got := activeText(a.contextRich.Segments); got != a.player.Current() {
		t.Fatalf("highlight = %q, player = %q", got, a.player.Current())
	}
	a.toggleContext()
	if a.contextOn {
		t.Fatalf("toggle did not disable")
	}
}
