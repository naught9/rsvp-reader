package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"strings"

	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"rsvp-reader/internal/doc"
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

func TestLayoutCovers(t *testing.T) {
	if layoutCovers(600, 100, 500, 40, 2) {
		t.Fatalf("starved top with room to grow holds")
	}
	if layoutCovers(600, 100, 500, 40, 37) {
		t.Fatalf("starved bottom with room to grow holds")
	}
	if !layoutCovers(600, 0, 500, 40, 2) {
		t.Fatalf("book start should excuse a short top")
	}
	if !layoutCovers(600, 100, 599, 40, 37) {
		t.Fatalf("book end should excuse a short bottom")
	}
	if !layoutCovers(600, 100, 500, 40, 20) {
		t.Fatalf("covered middle should hold")
	}
}

// TestBelowContextNeverDrains walks a long book through the real update
// path: below-context must hold a full radius until the book end makes
// it impossible. Regression: the layout used to exhaust mid-page, the
// active line sank to the bottom, and a new batch jumped in.
func TestBelowContextNeverDrains(t *testing.T) {
	a, _ := newTestApp(t)
	var paras []string
	for p := 0; p < 30; p++ {
		var ws []string
		for i := 0; i < 20; i++ {
			ws = append(ws, "w")
		}
		paras = append(paras, strings.Join(ws, " "))
	}
	a.setDocument(doc.FromText("long", strings.Join(paras, "\n\n")), "long")
	a.toggleContext()
	a.contextRich.Resize(fyne.NewSize(300, 600))
	nwords := len(a.book.Words)
	for pos := 0; pos < nwords; pos += 7 {
		a.player.Seek(pos)
		a.updateContext()
		line := activeLine(a.ctxLines, pos)
		if !layoutCovers(nwords, a.ctxLo, a.ctxHi, len(a.ctxLines), line) {
			below := len(a.ctxLines) - 1 - line
			t.Fatalf("pos %d: layout starved, %d lines below", pos, below)
		}
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

func TestContextPaneShrinkable(t *testing.T) {
	a, _ := newTestApp(t)
	a.toggleContext()
	// A vertical-only scroll floors its width at the content MinSize,
	// which with wrapping off is the longest laid line: the divider
	// could never shrink past the current lines. Both directions ignore
	// content size, so only the explicit floor below applies.
	if a.contextScroll.Direction == container.ScrollVerticalOnly {
		t.Fatalf("pane scroll is vertical-only: longest line locks the splitter")
	}
	if w := a.contextScroll.MinSize().Width; w > contextMinWidth+2*16 {
		t.Fatalf("pane floor = %vpx, blocks shrinking", w)
	}
}

func TestRewrapOnShrink(t *testing.T) {
	a, _ := newTestApp(t)
	a.toggleContext()
	a.updateContext()
	laidOut := a.ctxWidth
	// Simulate a divider shrink leaving a wide layout behind: the next
	// refresh must re-anchor instead of keeping the stale wrap.
	a.ctxWidth = laidOut + 500
	a.rewrapIfNeeded(0)
	// Re-anchored near the true width (headless sizes wobble a few px
	// between refreshes), not 500px away on the stale layout.
	if d := a.ctxWidth - laidOut; d < -8 || d > 8 {
		t.Fatalf("shrink kept width %.0f, want re-anchor near %.0f", a.ctxWidth, laidOut)
	}
	// Steady width: no rebuild (same layout object kept working).
	before := a.ctxLines
	a.rewrapIfNeeded(0)
	if len(a.ctxLines) != len(before) {
		t.Fatalf("steady width rebuilt the layout")
	}
}

func TestContextUsesReaderFontSlot(t *testing.T) {
	// The pane renders the reader typeface through the theme's Monospace
	// slot (shared with the ORP display); measuring must use the same
	// style or the wrap drifts from what's drawn.
	for name, st := range map[string]fyne.TextStyle{
		"body": contextBodyStyle().TextStyle, "active": contextActive.TextStyle,
	} {
		if !st.Monospace {
			t.Fatalf("%s style left the reader font slot", name)
		}
	}
	// Dark keeps the quiet dimmed body; light takes full ink, where the
	// dimmed rung is illegible.
	if got := contextBodyStyle().ColorName; got != theme.ColorNameDisabled {
		t.Fatalf("dark body = %q, want the quiet rung", got)
	}
	if !ctxMeasureStyle.Monospace {
		t.Fatalf("layout measures outside the reader font slot")
	}
}

func TestRenderLinesEllipses(t *testing.T) {
	words := []string{"a", "b", "c", "d", "e"}
	lines := []ctxLine{{words: []int{1, 2, 3}}}
	segs, _, _ := renderLines(words, lines, 0, 0, 2)
	first := segs[0].(*widget.TextSegment)
	last := segs[len(segs)-1].(*widget.TextSegment)
	if first.Text != "… " || last.Text != " …" {
		t.Fatalf("truncated ends = %q / %q", first.Text, last.Text)
	}
	full := []ctxLine{{words: []int{0, 1}}, {words: []int{2, 3, 4}}}
	segs, _, _ = renderLines(words, full, 0, 1, 2)
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && (ts.Text == "… " || ts.Text == " …") {
			t.Fatalf("untruncated window shows ellipsis")
		}
	}
}
