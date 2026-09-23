package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// contextWindowSize bounds the glance pane: a tail of recent words ending
// at the active one, so the highlight is always visible and refreshes
// stay cheap at any WPM.
const contextWindowSize = 60

var (
	contextPlain  = widget.RichTextStyle{Inline: true}
	contextBreak  = widget.RichTextStyle{Inline: false}
	contextDim    = widget.RichTextStyle{Inline: true, ColorName: theme.ColorNameDisabled}
	contextActive = widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNamePrimary,
		TextStyle: fyne.TextStyle{Bold: true},
	}
)

// contextSegments builds the glance window: up to size words ending at
// pos, with paragraph breaks and a leading ellipsis when truncated. The
// returned active index marks the segment holding the current word.
func contextSegments(words []string, starts []int, pos, size int) ([]widget.RichTextSegment, int) {
	if len(words) == 0 || pos < 0 {
		return nil, -1
	}
	if pos >= len(words) {
		pos = len(words) - 1
	}
	lo := pos - size + 1
	if lo < 0 {
		lo = 0
	}
	inWindow := map[int]bool{}
	for _, s := range starts {
		if s >= lo && s <= pos {
			inWindow[s] = true
		}
	}
	var segs []widget.RichTextSegment
	active := -1
	if lo > 0 {
		segs = append(segs, &widget.TextSegment{Style: contextDim, Text: "… "})
	}
	for i := lo; i <= pos; i++ {
		if inWindow[i] && i > lo {
			segs = append(segs, &widget.TextSegment{Style: contextBreak, Text: ""})
		}
		st := contextPlain
		if i == pos {
			st = contextActive
			active = len(segs)
		}
		segs = append(segs, &widget.TextSegment{Style: st, Text: words[i]})
		if i < pos {
			segs = append(segs, &widget.TextSegment{Style: contextPlain, Text: " "})
		}
	}
	return segs, active
}

// buildContextPane creates the scrollable glance pane; content arrives via
// updateContext on every refresh.
func (a *App) buildContextPane() {
	a.contextRich = widget.NewRichText()
	a.contextRich.Wrapping = fyne.TextWrapWord
	a.contextScroll = container.NewVScroll(a.contextRich)
	a.contextScroll.SetMinSize(fyne.NewSize(280, 0))
	pad := container.NewBorder(nil, nil, widget.NewSeparator(), nil, a.contextScroll)
	a.contextPane = container.NewPadded(pad)
}

// updateContext rebuilds the glance window around the player's position
// and pins the active word to the bottom. No-op unless the pane is shown.
func (a *App) updateContext() {
	if !a.contextOn || a.book == nil || a.player == nil || a.contextRich == nil {
		return
	}
	segs, _ := contextSegments(a.book.Words, a.book.ParaStarts, a.player.Pos(), contextWindowSize)
	a.contextRich.Segments = segs
	a.contextRich.Refresh()
	a.contextScroll.ScrollToBottom()
}

// toggleContext flips the glance pane, persists the choice, and rebuilds
// the reader center.
func (a *App) toggleContext() {
	a.contextOn = !a.contextOn
	a.prefs().SetBool("contextPane", a.contextOn)
	a.buildReaderScreen()
	a.root.Objects[1] = a.readerScreen
	a.root.Refresh()
	if a.book != nil {
		a.showScreen(a.readerScreen)
	}
	a.refreshAll()
}
