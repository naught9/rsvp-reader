package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// contextRadius is the words shown on each side of the active word: the
// glance pane reads like a book page around the current position.
const contextRadius = 60

var (
	contextPlain = widget.RichTextStyle{Inline: true}
	contextBreak = widget.RichTextStyle{Inline: false}
	contextDim   = widget.RichTextStyle{Inline: true, ColorName: theme.ColorNameDisabled}
	// Active word: theme red only, echoing the ORP focal letter. No bold:
	// weight changes advance width and the line shivers as the highlight
	// moves.
	contextActive = widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNameError,
	}
)

// contextWindow bounds the glance window around pos, quantized to whole
// estimated lines: the anchor line holds pos, and radius words pad each
// side. Bounds move only when pos crosses a line, so text enters and
// leaves the pane line by line instead of jittering word by word.
func contextWindow(nwords, pos, wpl, radius int) (lo, hi int) {
	if nwords <= 0 {
		return 0, -1
	}
	if pos < 0 {
		pos = 0
	}
	if pos >= nwords {
		pos = nwords - 1
	}
	lineStart := (pos / wpl) * wpl
	lo = lineStart - radius
	if lo < 0 {
		lo = 0
	}
	hi = lineStart + wpl - 1 + radius
	if hi >= nwords {
		hi = nwords - 1
	}
	return lo, hi
}

// contextSegments renders words[lo..hi] with paragraph breaks, ellipses
// at truncated ends, and the active word styled. Returns the segment
// index of the active word and the active word's fraction of the window
// (for scroll positioning).
func contextSegments(words []string, starts []int, lo, hi, pos int) ([]widget.RichTextSegment, int, float32) {
	if len(words) == 0 || hi < lo {
		return nil, -1, 0
	}
	inWindow := map[int]bool{}
	for _, s := range starts {
		if s >= lo && s <= hi {
			inWindow[s] = true
		}
	}
	var segs []widget.RichTextSegment
	active := -1
	if lo > 0 {
		segs = append(segs, &widget.TextSegment{Style: contextDim, Text: "… "})
	}
	for i := lo; i <= hi; i++ {
		if inWindow[i] && i > lo {
			segs = append(segs, &widget.TextSegment{Style: contextBreak, Text: ""})
		}
		st := contextPlain
		if i == pos {
			st = contextActive
			active = len(segs)
		}
		segs = append(segs, &widget.TextSegment{Style: st, Text: words[i]})
		if i < hi {
			segs = append(segs, &widget.TextSegment{Style: contextPlain, Text: " "})
		}
	}
	if hi < len(words)-1 {
		segs = append(segs, &widget.TextSegment{Style: contextDim, Text: " …"})
	}
	var frac float32
	if hi > lo {
		frac = float32(pos-lo) / float32(hi-lo)
	}
	return segs, active, frac
}

// contextWPL estimates rendered words per line from the pane width, so
// window moves quantize to visual lines. Exactness is unnecessary — only
// the update cadence depends on it.
func (a *App) contextWPL() int {
	w := 0.0
	if a.contextScroll != nil {
		w = float64(a.contextScroll.Size().Width)
	}
	if w <= 0 {
		return 8
	}
	size := float64(theme.Size(theme.SizeNameText))
	wpl := int(w / (size * 0.55 * 6))
	if wpl < 4 {
		return 4
	}
	if wpl > 16 {
		return 16
	}
	return wpl
}

// buildContextPane creates the scrollable glance pane. Mirror gutters
// reserve the floating chrome's height (as in the tree pane) so text is
// never cut off at the top; spacers vertically center short content.
func (a *App) buildContextPane() {
	a.contextRich = widget.NewRichText()
	a.contextRich.Wrapping = fyne.TextWrapWord
	a.contextScroll = container.NewVScroll(
		container.NewVBox(layout.NewSpacer(), a.contextRich, layout.NewSpacer()))
	a.contextScroll.SetMinSize(fyne.NewSize(280, 0))
	a.contextPane = container.NewBorder(
		newMirrorSpacer(a.topWrap), newMirrorSpacer(a.bottomWrap),
		widget.NewSeparator(), nil,
		container.NewPadded(a.contextScroll))
	a.contextLine = -1
}

// updateContext keeps the glance window around the player's position.
// Segments rebuild every word so the highlight glides, but the window
// bounds and scroll hold until the active word crosses an estimated
// line — new text arrives line by line, not word by word.
func (a *App) updateContext() {
	if !a.contextOn || a.book == nil || a.player == nil || a.contextRich == nil {
		return
	}
	pos := a.player.Pos()
	wpl := a.contextWPL()
	line := pos / wpl
	lo, hi := contextWindow(len(a.book.Words), pos, wpl, contextRadius)
	segs, _, frac := contextSegments(a.book.Words, a.book.ParaStarts, lo, hi, pos)
	a.contextRich.Segments = segs
	a.contextRich.Refresh()
	if line == a.contextLine {
		return
	}
	a.contextLine = line
	// Center the active word: it sits ~frac through the content.
	a.contextScroll.Refresh()
	contentH := a.contextRich.MinSize().Height
	viewH := a.contextScroll.Size().Height
	if off := frac * (contentH - viewH); off > 0 {
		a.contextScroll.ScrollToOffset(fyne.NewPos(0, off))
	}
}

// toggleContext flips the glance pane, persists the choice, and rebuilds
// the reader center.
func (a *App) toggleContext() {
	a.contextOn = !a.contextOn
	a.contextLine = -1 // force re-anchor on reopen
	a.prefs().SetBool("contextPane", a.contextOn)
	a.buildReaderScreen()
	a.root.Objects[1] = a.readerScreen
	a.root.Refresh()
	if a.book != nil {
		a.showScreen(a.readerScreen)
	}
	a.refreshAll()
}
