package ui

import (
	"math"
	"sync/atomic"

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
	// contextLineRadius is the lines shown each side of the active one;
	// contextPrefetch is the extra lines laid out beyond that before the
	// layout re-anchors, so below-context never drains mid-page.
	contextLineRadius = 8
	contextPrefetch   = 4
	// contextMinWidth floors the pane so the splitter can shrink it well
	// below the default; lines simply wrap narrower. (A 280px floor is
	// what used to block shrinking the pane at all.)
	contextMinWidth = 140
)

// layoutCovers reports whether the layout holds the active line with a
// full radius of lines on each side, excusing sides the book edge makes
// impossible to grow.
func layoutCovers(nwords, ctxLo, ctxHi, nlines, line int) bool {
	if line < 0 {
		return false
	}
	need := contextLineRadius + contextPrefetch
	if line < need && ctxLo > 0 {
		return false
	}
	if line > nlines-1-need && ctxHi < nwords-1 {
		return false
	}
	return true
}

// anchorLayout lays out words around pos until the active line holds a
// full radius of lines on each growable side. Word counts can't predict
// line counts (narrow words, paragraph breaks), so the window extends
// until coverage holds or the book edge stops it.
func anchorLayout(words []string, starts map[int]bool, pos, nwords int, width, textSize float32) ([]ctxLine, int, int) {
	need := contextLineRadius + contextPrefetch
	const chunk = 120
	lo, hi := pos-chunk, pos+chunk
	for {
		if lo < 0 {
			lo = 0
		}
		if hi >= nwords {
			hi = nwords - 1
		}
		lines := layoutLines(words, starts, lo, hi, width, textSize)
		line := activeLine(lines, pos)
		grew := false
		if line >= 0 {
			if line < need && lo > 0 {
				lo -= chunk
				grew = true
			}
			if line > len(lines)-1-need && hi < nwords-1 {
				hi += chunk
				grew = true
			}
		}
		if !grew {
			return lines, lo, hi
		}
	}
}

// ctxMeasureStyle must match the segment styles below: the layout
// measures with the same font it renders, or the wrap drifts.
var ctxMeasureStyle = fyne.TextStyle{Monospace: true}

// Quiet body copy; the active word alone carries color. Monospace
// renders the reader typeface: the theme carries the user's font in
// that slot (shared with the ORP display).
var contextBreak = widget.RichTextStyle{Inline: false}

// contextBodyStyle is the quiet body ink, resolved per variant at render
// time: dimmed white on the near-black canvas, full ink on light surfaces
// where the dimmed rung falls below legibility.
func contextBodyStyle() widget.RichTextStyle {
	name := theme.ColorNameDisabled
	if fyne.CurrentApp() != nil &&
		fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantLight {
		name = theme.ColorNameForeground
	}
	return widget.RichTextStyle{Inline: true, ColorName: name, TextStyle: ctxMeasureStyle}
}

var (
	// Active word: theme red only, echoing the ORP focal letter. No bold:
	// weight changes advance width and the line shivers as it moves.
	contextActive = widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNameError,
		TextStyle: ctxMeasureStyle,
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
	spaceW := fyne.MeasureText(" ", textSize, ctxMeasureStyle).Width
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
		w := fyne.MeasureText(words[i], textSize, ctxMeasureStyle).Width
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
	if first := lines[lo].words; len(first) > 0 && first[0] > 0 {
		segs = append(segs, &widget.TextSegment{Style: contextBodyStyle(), Text: "… "})
	}
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
			st := contextBodyStyle()
			if wi == pos {
				st = contextActive
				activeRow = rows
			}
			segs = append(segs, &widget.TextSegment{Style: st, Text: words[wi]})
			if vi < len(lines[li].words)-1 {
				segs = append(segs, &widget.TextSegment{Style: contextBodyStyle(), Text: " "})
			}
		}
		rows++
	}
	if last := lines[hi].words; len(last) > 0 && last[len(last)-1] < len(words)-1 {
		segs = append(segs, &widget.TextSegment{Style: contextBodyStyle(), Text: " …"})
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
	// Both scroll directions: a vertical-only scroll floors its width at
	// the content's MinSize, which with wrapping off is the longest laid
	// line — the divider could never shrink past the current lines. Both
	// directions ignore content size, so the SetMinSize floor below holds.
	// Lines fit the width by construction, so no horizontal bar appears
	// in steady state.
	a.contextScroll = container.NewScroll(
		container.NewVBox(layout.NewSpacer(), a.contextRich, layout.NewSpacer()))
	a.contextScroll.SetMinSize(fyne.NewSize(contextMinWidth, 0))
	// The watcher re-anchors the wrap whenever a divider drag (or any
	// resize) moves the pane width: without it, shrinking only clips the
	// stale lines because nothing else refreshes while paused.
	a.contextWrap = container.New(newRewrapLayout(a.onPaneWidth), a.contextScroll)
	body := container.NewBorder(
		container.NewVBox(a.contextHead, widget.NewSeparator()),
		nil, widget.NewSeparator(), nil,
		container.NewPadded(a.contextWrap))
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
	nwords := len(a.book.Words)
	line := -1
	// A moved width re-anchors: lines wrapped for another width are
	// wrong, and keeping them would freeze the pane. Tolerance absorbs
	// sub-pixel noise; real resizes rebuild around the same position,
	// so the visible window holds still.
	widthMoved := width < a.ctxWidth-8 || width > a.ctxWidth+8
	if a.ctxLines == nil || pos < a.ctxLo || pos > a.ctxHi || widthMoved {
		a.ctxLines = nil
	} else if l := activeLine(a.ctxLines, pos); layoutCovers(nwords, a.ctxLo, a.ctxHi, len(a.ctxLines), l) {
		// Reuse the layout unless the active line is starved of lines
		// on a side that can still grow; book edges hold by necessity.
		line = l
	} else {
		a.ctxLines = nil
	}
	if a.ctxLines == nil {
		starts := map[int]bool{}
		for _, s := range a.book.ParaStarts {
			starts[s] = true
		}
		a.ctxLines, a.ctxLo, a.ctxHi = anchorLayout(a.book.Words, starts, pos, nwords, width, textSize)
		a.ctxWidth = width
		a.ctxLine = -1
		line = activeLine(a.ctxLines, pos)
	}
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

// rewrapLayout is a pass-through layout reporting its width on every
// arrange, so divider drags re-anchor the pane's line wrap.
type rewrapLayout struct{ onWidth func(float32) }

func newRewrapLayout(onWidth func(float32)) rewrapLayout { return rewrapLayout{onWidth} }

func (l rewrapLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) > 0 {
		objs[0].Resize(size)
	}
	l.onWidth(size.Width)
}

func (l rewrapLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objs[0].MinSize()
}

// onPaneWidth re-anchors the wrap once the pane width actually moves;
// layout runs off the main thread, so the refresh marshals over.
func (a *App) onPaneWidth(w float32) {
	if !a.contextOn || a.book == nil {
		return
	}
	last := math.Float32frombits(atomic.LoadUint32(&a.ctxWatchW))
	if math.Abs(float64(w-last)) < 4 {
		return
	}
	atomic.StoreUint32(&a.ctxWatchW, math.Float32bits(w))
	fyne.Do(func() { a.rewrapIfNeeded(w) })
}

// rewrapIfNeeded refreshes once resizes leave the laid-out width
// behind. Widths are compared at the RichText basis updateContext uses;
// w (the watcher's width) only trips the dispatch guard above.
func (a *App) rewrapIfNeeded(w float32) {
	if !a.contextOn || a.book == nil || a.contextRich == nil {
		return
	}
	rw := a.contextRich.Size().Width - 2*theme.Padding()
	if rw < a.ctxWidth-8 || rw > a.ctxWidth+8 {
		a.updateContext()
	}
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
