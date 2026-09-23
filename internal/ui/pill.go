package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// PillButton is a quiet capsule-shaped button: a soft wash fill, word
// label, generous padding. Emphasis comes from label weight (Bold), never
// from competing fill colors.
type PillButton struct {
	widget.BaseWidget
	Text     string
	Bold     bool
	OnTapped func()

	disabled bool
	hovered  bool
	pressed  bool
	focused  bool
}

// NewPillButton creates a pill with a tap handler.
func NewPillButton(text string, onTapped func()) *PillButton {
	b := &PillButton{Text: text, OnTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetText changes the label.
func (b *PillButton) SetText(text string) {
	if b.Text == text {
		return
	}
	b.Text = text
	b.Refresh()
}

// Enable makes the button tappable.
func (b *PillButton) Enable() {
	b.disabled = false
	b.Refresh()
}

// Disable makes the button inert.
func (b *PillButton) Disable() {
	b.disabled = true
	b.Refresh()
}

// Disabled reports whether the button is inert.
func (b *PillButton) Disabled() bool { return b.disabled }

// Tapped activates the button.
func (b *PillButton) Tapped(*fyne.PointEvent) {
	if b.disabled || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

// MouseIn tracks hover.
func (b *PillButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}

// MouseMoved tracks hover.
func (b *PillButton) MouseMoved(*desktop.MouseEvent) {
	if !b.hovered {
		b.hovered = true
		b.Refresh()
	}
}

// MouseOut ends hover.
func (b *PillButton) MouseOut() {
	b.hovered = false
	b.Refresh()
}

// MouseDown tracks press.
func (b *PillButton) MouseDown(*desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.pressed = true
	b.Refresh()
}

// MouseUp ends press.
func (b *PillButton) MouseUp(*desktop.MouseEvent) {
	b.pressed = false
	b.Refresh()
}

// FocusGained shows keyboard focus.
func (b *PillButton) FocusGained() {
	b.focused = true
	b.Refresh()
}

// FocusLost hides keyboard focus.
func (b *PillButton) FocusLost() {
	b.focused = false
	b.Refresh()
}

// Focused reports keyboard focus.
func (b *PillButton) Focused() bool { return b.focused }

// TypedKey activates on Enter/Space for keyboard users.
func (b *PillButton) TypedKey(e *fyne.KeyEvent) {
	if e.Name == fyne.KeyReturn || e.Name == fyne.KeyEnter || e.Name == fyne.KeySpace {
		b.Tapped(nil)
	}
}

// TypedRune is a no-op for focusability.
func (b *PillButton) TypedRune(rune) {}

type pillRenderer struct {
	b     *PillButton
	bg    *canvas.Rectangle
	label *canvas.Text
}

func (b *PillButton) CreateRenderer() fyne.WidgetRenderer {
	r := &pillRenderer{b: b}
	r.bg = canvas.NewRectangle(theme.ButtonColor())
	r.label = canvas.NewText(b.Text, theme.ForegroundColor())
	r.apply()
	return r
}

func (r *pillRenderer) apply() {
	b := r.b
	r.label.Text = b.Text
	r.label.TextStyle = fyne.TextStyle{Bold: b.Bold}
	r.label.Alignment = fyne.TextAlignCenter
	switch {
	case b.disabled:
		r.bg.FillColor = theme.DisabledButtonColor()
		r.label.Color = theme.DisabledColor()
	case b.pressed:
		r.bg.FillColor = theme.PressedColor()
		r.label.Color = theme.ForegroundColor()
	case b.hovered || b.focused:
		r.bg.FillColor = theme.HoverColor()
		r.label.Color = theme.ForegroundColor()
	default:
		r.bg.FillColor = theme.ButtonColor()
		r.label.Color = theme.ForegroundColor()
	}
}

func (r *pillRenderer) MinSize() fyne.Size {
	r.apply()
	lm := r.label.MinSize()
	return fyne.NewSize(lm.Width+44, fyne.Max(lm.Height+20, 36))
}

func (r *pillRenderer) Layout(size fyne.Size) {
	r.apply()
	r.bg.Resize(size)
	r.bg.CornerRadius = size.Height / 2
	lm := r.label.MinSize()
	r.label.Resize(lm)
	r.label.Move(fyne.NewPos((size.Width-lm.Width)/2, (size.Height-lm.Height)/2))
}

func (r *pillRenderer) Refresh() {
	r.apply()
	r.Layout(r.b.Size())
	canvas.Refresh(r.b)
}

func (r *pillRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.label}
}

func (r *pillRenderer) Destroy() {}

var (
	_ fyne.Tappable     = (*PillButton)(nil)
	_ fyne.Focusable    = (*PillButton)(nil)
	_ desktop.Hoverable = (*PillButton)(nil)
	_ desktop.Mouseable = (*PillButton)(nil)
	_ fyne.Disableable  = (*PillButton)(nil)
)

// gap is a fixed transparent spacer for calm control rhythm.
type gap struct {
	widget.BaseWidget
	w, h float32
}

func (g *gap) CreateRenderer() fyne.WidgetRenderer {
	return &gapRenderer{g: g}
}

type gapRenderer struct {
	g *gap
}

func (r *gapRenderer) MinSize() fyne.Size { return fyne.NewSize(r.g.w, r.g.h) }
func (r *gapRenderer) Layout(fyne.Size)   {}
func (r *gapRenderer) Refresh()           {}
func (r *gapRenderer) Objects() []fyne.CanvasObject {
	return nil
}
func (r *gapRenderer) Destroy() {}

// pillGap is the calm fixed spacing between adjacent pill buttons.
func pillGap() fyne.CanvasObject {
	g := &gap{w: 12, h: 4}
	g.ExtendBaseWidget(g)
	return g
}
