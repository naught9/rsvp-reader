package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"rsvp-reader/internal/reader"
)

// ORPWidget displays one word around a fixed focal coordinate. The text
// before, the focal letter, and the text after are measured and drawn
// separately so the recognition point never moves between words.
type ORPWidget struct {
	widget.BaseWidget
	Before, Focal, After string
	Highlight            bool
	FontSize             float32
	Placeholder          string
}

var focalRed = color.NRGBA{R: 0xff, G: 0x44, B: 0x44, A: 0xff}

// guideThickness is the single width for focal ticks and rules.
const guideThickness float32 = 2

// NewORPWidget creates the reader display with a default size.
func NewORPWidget() *ORPWidget {
	w := &ORPWidget{FontSize: 64, Highlight: true, Placeholder: "Open a book to begin"}
	w.ExtendBaseWidget(w)
	return w
}

// SetWord splits word on rune boundaries and repaints.
func (w *ORPWidget) SetWord(word string) {
	b, f, a := reader.SplitForDisplay(word)
	if b == w.Before && f == w.Focal && a == w.After {
		return
	}
	w.Before, w.Focal, w.After = b, f, a
	w.Refresh()
}

type orpRenderer struct {
	w           *ORPWidget
	before      *canvas.Text
	focal       *canvas.Text
	after       *canvas.Text
	placeholder *canvas.Text
	guideTop    *canvas.Rectangle
	guideBottom *canvas.Rectangle
	ruleTop     *canvas.Rectangle
	ruleBottom  *canvas.Rectangle
}

func (w *ORPWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &orpRenderer{w: w}
	r.before = canvas.NewText("", theme.ForegroundColor())
	r.focal = canvas.NewText("", focalRed)
	r.after = canvas.NewText("", theme.ForegroundColor())
	r.placeholder = canvas.NewText(w.Placeholder, theme.DisabledColor())
	r.guideTop = canvas.NewRectangle(focusGuideColor())
	r.guideBottom = canvas.NewRectangle(focusGuideColor())
	// Focus rules: full-width horizontals meeting the focal ticks
	// end-to-end, framing the word band. Same opaque guide ink.
	r.ruleTop = canvas.NewRectangle(focusGuideColor())
	r.ruleBottom = canvas.NewRectangle(focusGuideColor())
	for _, t := range []*canvas.Text{r.before, r.focal, r.after, r.placeholder} {
		t.TextStyle = fyne.TextStyle{Monospace: true}
	}
	r.focal.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	r.Refresh()
	return r
}

func (r *orpRenderer) apply() {
	w := r.w
	size := w.FontSize
	if size <= 0 {
		size = 64
	}
	fg := theme.ForegroundColor()
	for _, t := range []*canvas.Text{r.before, r.after, r.placeholder} {
		t.Color = fg
		t.TextSize = size
		if t == r.placeholder {
			t.Color = theme.DisabledColor()
			t.TextSize = 20
		}
	}
	r.before.Text = w.Before
	r.after.Text = w.After
	r.focal.Text = w.Focal
	r.focal.TextSize = size
	if w.Highlight && w.Focal != "" {
		r.focal.Color = focalRed
		r.focal.TextStyle = fyne.TextStyle{Monospace: true, Bold: true} // shape cue, not color-only
	} else {
		r.focal.Color = fg
		r.focal.TextStyle = fyne.TextStyle{Monospace: true}
	}
	// Guide ink re-resolves every refresh: a theme switch strands any
	// color snapshotted at creation (the old light-mode invisibility).
	gc := focusGuideColor()
	r.guideTop.FillColor, r.guideBottom.FillColor = gc, gc
	r.ruleTop.FillColor, r.ruleBottom.FillColor = gc, gc
	r.placeholder.Text = w.Placeholder
	if w.Focal == "" && w.Before == "" && w.After == "" {
		r.placeholder.Show()
	} else {
		r.placeholder.Hide()
	}
}

func (r *orpRenderer) MinSize() fyne.Size {
	r.apply()
	w := r.before.MinSize().Width + r.focal.MinSize().Width + r.after.MinSize().Width + 80
	h := r.focal.MinSize().Height + 140
	ph := r.placeholder.MinSize()
	if ph.Width+80 > w {
		w = ph.Width + 80
	}
	if h < 220 {
		h = 220
	}
	return fyne.NewSize(w, h)
}

func (r *orpRenderer) Layout(size fyne.Size) {
	r.apply()
	cx := size.Width / 2
	midY := size.Height / 2

	// Focal guides: fixed markers above/below the recognition point.
	// Ticks and rules share one thickness so intersections stay even.
	gw, gh := guideThickness, float32(36)
	r.guideTop.Resize(fyne.NewSize(gw, gh))
	r.guideTop.Move(fyne.NewPos(cx-gw/2, midY-90))
	r.guideBottom.Resize(fyne.NewSize(gw, gh))
	r.guideBottom.Move(fyne.NewPos(cx-gw/2, midY+54))
	// Rules span the pane at the ticks' outer ends, touching them.
	rh := guideThickness
	r.ruleTop.Resize(fyne.NewSize(size.Width, rh))
	r.ruleTop.Move(fyne.NewPos(0, midY-90-rh/2))
	r.ruleBottom.Resize(fyne.NewSize(size.Width, rh))
	r.ruleBottom.Move(fyne.NewPos(0, midY+90-rh/2))

	if r.placeholder.Visible() {
		r.placeholder.Resize(r.placeholder.MinSize())
		ps := r.placeholder.Size()
		r.placeholder.Move(fyne.NewPos(cx-ps.Width/2, midY-ps.Height/2))
		r.before.Hide()
		r.focal.Hide()
		r.after.Hide()
		return
	}
	r.before.Show()
	r.focal.Show()
	r.after.Show()
	r.placeholder.Hide()

	fs := r.focal.MinSize()
	bs := r.before.MinSize()
	y := midY - fs.Height/2
	r.focal.Resize(fs)
	r.focal.Move(fyne.NewPos(cx-fs.Width/2, y))
	r.before.Resize(bs)
	r.before.Move(fyne.NewPos(cx-fs.Width/2-bs.Width, y))
	as := r.after.MinSize()
	r.after.Resize(as)
	r.after.Move(fyne.NewPos(cx+fs.Width/2, y))
}

func (r *orpRenderer) Refresh() {
	r.apply()
	r.Layout(r.w.Size())
	canvas.Refresh(r.w)
}

func (r *orpRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.guideTop, r.guideBottom, r.ruleTop, r.ruleBottom, r.before, r.focal, r.after, r.placeholder}
}

func (r *orpRenderer) Destroy() {}
