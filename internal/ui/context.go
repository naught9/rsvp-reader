package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The glance pane owns its lines: text is wrapped here with MeasureText
// and rendered with wrapping off, so line breaks, the active line, and
// scroll offsets are exact — never estimated from the renderer's layout.
const (
	// contextLayoutRadius bounds the words laid out around the position
	// to find lines; contextLineRadius is the lines shown each side of
	// the active one.
	contextLayoutRadius = 90
	contextLineRadius   = 8
)

var (
	// Quiet body copy; the active word alone carries color.
	contextPlain = widget.RichTextStyle{Inline: true, ColorName: theme.ColorNameDisabled}
	contextBreak = widget.RichTextStyle{Inline: false}
	contextDim   = widget.RichTextStyle{Inline: true, ColorName: theme.ColorNameDisabled}
	// Active word: theme red only, echoing the ORP focal letter. No bold:
	// weight changes advance width and the line shivers as it moves.
	contextActive = widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNameError,
	}
)

// ctxLine is one laid-out row: word indices plus whether a paragraph
// break opens before it (rendered as a blank row).
type ctxLine struct {
	words     []int
	paraBreak bool
}

// layoutLines wraps words[lo..hi] to maxWidth at textSize, breaking
// words greedily and forcing a break at each paragraph start.
func layoutLines(words []string, starts map[int]bool, lo, hi int, maxWidth, textSize float32) []ctxLine {
	spaceW := fyne.MeasureText(" ", textSize, fyne.TextStyle{}).Width
	var lines []ctxLine
	cur := ctxLine{}
	curW := float32(0)
	flush := func() {
		if len(cur.words) > 0 {
			lines = append(lines, cur)
			cur = ctxLine{}
			curW = 0
		}
	}
	for i := lo; i <= hi; i++ {
		if starts[i] && len(cur.words) > 0 {
			flush()
			cur.paraBreak = true
		}
		w := fyne.MeasureText(words[i], textSize, fyne.TextStyle{}).Width
		if len(cur.words) > 0 {
			w += spaceW
		}
		if len(cur.words) > 0 && curW+w > maxWidth {
			flush()
		}
		cur.words = append(cur.words, i)
		curW += w
	}
	flush()
	return lines
}

// activeLine returns the laid-out line holding pos, or -1.
func activeLine(lines []ctxLine, pos int) int {
	for li, ln := range lines {
		for _, wi := range ln.words {
			if wi == pos {
				return li
			}
		}
	}
	return -1
}

// renderLines builds segments for lines[lo..hi] of the layout. Returns
// the row index holding pos and the total row count (blank paragraph
// rows included) for exact scroll positioning.
func renderLines(words []string, lines []ctxLine, lo, hi, pos int) ([]widget.RichTextSegment, int, int) {
	var segs []widget.RichTextSegment
	activeRow, rows := -1, 0
	for li := lo; li <= hi; li++ {
		if li > lo {
			segs = append(segs, &widget.TextSegment{Style: contextBreak, Text: ""})
			rows++
		}
		if lines[li].paraBreak {
			segs = append(segs, &widget.TextSegment{Style: contextBreak, Text: " "})
			rows++
		}
		for vi, wi := range lines[li].words {
			st := contextPlain
			if wi == pos {
				st = contextActive
				activeRow = rows
			}
			segs = append(segs, &widget.TextSegment{Style: st, Text: words[wi]})
			if vi < len(lines[li].words)-1 {
				segs = append(segs, &widget.TextSegment{Style: contextPlain, Text: " "})
			}
		}
		rows++
	}
	return segs, activeRow, rows
}

// buildContextPane creates the glance pane: a quiet section header over
// the scrollable lines. Mirror gutters reserve the floating chrome's
// height (as in the tree pane); spacers vertically center short content.
func (a *App) buildContextPane() {
	a.contextRich = widget.NewRichText()
	a.contextRich.Wrapping = fyne.TextWrapOff
	a.contextHead = canvas.NewText("", theme.Color(theme.ColorNameForeground))
	a.contextHead.Alignment = fyne.TextAlignCenter
	a.contextHead.TextSize = theme.Size(theme.SizeNameCaptionText)
	a.contextScroll = container.NewVScroll(
		container.NewVBox(layout.NewSpacer(), a.contextRich, layout.NewSpacer()))
	a.contextScroll.SetMinSize(fyne.NewSize(280, 0))
	body := container.NewBorder(
		container.NewVBox(a.contextHead, widget.NewSeparator()),
		nil, widget.NewSeparator(), nil,
		container.NewPadded(a.contextScroll))
	a.contextPane = container.NewBorder(
		newMirrorSpacer(a.topWrap), newMirrorSpacer(a.bottomWrap),
		nil, nil, body)
	a.ctxLines = nil
}

// updateContext keeps the glance window on the player's line. The layout
// rebuilds only when the position leaves it or the width moves; the
// highlight glides within a stable layout word by word, and the scroll
// recenters exactly on line changes.
func (a *App) updateContext() {
	if !a.contextOn || a.book == nil || a.player == nil || a.contextRich == nil {
		return
	}
	textSize := theme.Size(theme.SizeNameText)
	width := a.contextRich.Size().Width - 2*theme.Padding()
	if width < 50 {
		return // not laid out yet; next refresh fits
	}
	pos := a.player.Pos()
	if a.ctxLines == nil || pos < a.ctxLo || pos > a.ctxHi ||
		width < a.ctxWidth-2 || width > a.ctxWidth+2 {
		lo := pos - contextLayoutRadius
		if lo < 0 {
			lo = 0
		}
		hi := pos + contextLayoutRadius
		if hi >= len(a.book.Words) {
			hi = len(a.book.Words) - 1
		}
		starts := map[int]bool{}
		for _, s := range a.book.ParaStarts {
			if s >= lo && s <= hi {
				starts[s] = true
			}
		}
		a.ctxLines = layoutLines(a.book.Words, starts, lo, hi, width, textSize)
		a.ctxLo, a.ctxHi, a.ctxWidth = lo, hi, width
		a.ctxLine = -1
	}
	line := activeLine(a.ctxLines, pos)
	if line < 0 {
		return
	}
	lo := line - contextLineRadius
	if lo < 0 {
		lo = 0
	}
	hi := line + contextLineRadius
	if hi >= len(a.ctxLines) {
		hi = len(a.ctxLines) - 1
	}
	segs, activeRow, rows := renderLines(a.book.Words, a.ctxLines, lo, hi, pos)
	a.contextRich.Segments = segs
	a.contextRich.Refresh()
	a.syncContextHead()
	if line == a.ctxLine {
		return // same line: highlight glides, layout and scroll hold
	}
	a.ctxLine = line
	a.contextScroll.Refresh()
	contentH := a.contextRich.MinSize().Height
	viewH := a.contextScroll.Size().Height
	if contentH <= viewH || rows <= 0 {
		a.contextScroll.ScrollToTop()
		return
	}
	rowH := contentH / float32(rows)
	off := float32(activeRow)*rowH + rowH/2 - viewH/2
	if off < 0 {
		off = 0
	}
	if max := contentH - viewH; off > max {
		off = max
	}
	a.contextScroll.ScrollToOffset(fyne.NewPos(0, off))
}

// syncContextHead names the current section above the lines; untouched
// unless it changed, so the header never flickers mid-section.
func (a *App) syncContextHead() {
	label := ""
	if sec := a.book.SectionForWord(a.player.Pos()); sec != nil {
		label = sec.Label
	}
	if label == a.contextHeadText {
		return
	}
	a.contextHeadText = label
	a.contextHead.Text = label
	a.contextHead.Refresh()
}

// toggleContext flips the glance pane, persists the choice, and rebuilds
// the reader center.
func (a *App) toggleContext() {
	a.contextOn = !a.contextOn
	a.ctxLines = nil // force re-anchor on reopen
	a.prefs().SetBool("contextPane", a.contextOn)
	a.buildReaderScreen()
	a.root.Objects[1] = a.readerScreen
	a.root.Refresh()
	if a.book != nil {
		a.showScreen(a.readerScreen)
	}
	a.refreshAll()
}
