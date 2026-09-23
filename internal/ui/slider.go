package ui

import (
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	slimTrackHeight = float32(4)
	slimThumbSize   = float32(16)
)

// SlimSlider is a quiet slider in the progress-hairline dialect: separator
// track, secondary-ink fill and thumb. Tap jumps, drag scrubs, steps snap.
type SlimSlider struct {
	widget.BaseWidget
	Min, Max, Step float64
	Value          float64
	OnChanged      func(float64)
	OnChangeEnded  func(float64)

	disabled bool
	hovered  bool
	focused  bool
}

// NewSlimSlider creates a slider over [min, max] with step snapping.
func NewSlimSlider(min, max, step float64) *SlimSlider {
	s := &SlimSlider{Min: min, Max: max, Step: step}
	s.ExtendBaseWidget(s)
	return s
}

// SetValue moves the thumb without firing OnChanged (for restores).
func (s *SlimSlider) SetValue(v float64) {
	v = s.clamp(v)
	if s.Value == v {
		return
	}
	s.Value = v
	s.Refresh()
}

// Enable makes the slider interactive.
func (s *SlimSlider) Enable() {
	s.disabled = false
	s.Refresh()
}

// Disable freezes the slider.
func (s *SlimSlider) Disable() {
	s.disabled = true
	s.Refresh()
}

// Disabled reports interactivity.
func (s *SlimSlider) Disabled() bool { return s.disabled }

// Tapped jumps the thumb to the tap.
func (s *SlimSlider) Tapped(e *fyne.PointEvent) {
	if s.disabled {
		return
	}
	s.setFromX(e.Position.X)
	s.endChange()
}

// Dragged scrubs the thumb.
func (s *SlimSlider) Dragged(e *fyne.DragEvent) {
	if s.disabled {
		return
	}
	s.setFromX(e.Position.X)
}

// DragEnd commits the scrub.
func (s *SlimSlider) DragEnd() {
	s.endChange()
}

// MouseIn tracks hover.
func (s *SlimSlider) MouseIn(*desktop.MouseEvent) {
	s.hovered = true
	s.Refresh()
}

// MouseMoved keeps hover alive during scrubs.
func (s *SlimSlider) MouseMoved(*desktop.MouseEvent) {
	if !s.hovered {
		s.hovered = true
		s.Refresh()
	}
}

// MouseOut ends hover.
func (s *SlimSlider) MouseOut() {
	s.hovered = false
	s.Refresh()
}

// FocusGained shows keyboard focus.
func (s *SlimSlider) FocusGained() {
	s.focused = true
	s.Refresh()
}

// FocusLost hides keyboard focus.
func (s *SlimSlider) FocusLost() {
	s.focused = false
	s.Refresh()
}

// Focused reports keyboard focus.
func (s *SlimSlider) Focused() bool { return s.focused }

// TypedKey is intentionally quiet: arrows stay global (speed/words),
// so focusing the slider never hijacks them.
func (s *SlimSlider) TypedKey(*fyne.KeyEvent) {}

// TypedRune is a no-op for focusability.
func (s *SlimSlider) TypedRune(rune) {}

func (s *SlimSlider) clamp(v float64) float64 {
	if v < s.Min {
		return s.Min
	}
	if v > s.Max {
		return s.Max
	}
	return v
}

func (s *SlimSlider) ratio() float64 {
	if s.Max <= s.Min {
		return 0
	}
	return (s.Value - s.Min) / (s.Max - s.Min)
}

func (s *SlimSlider) setFromX(x float32) {
	w := s.Size().Width
	inset := slimThumbSize / 2
	ratio := (float64(x) - float64(inset)) / (float64(w) - 2*float64(inset))
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	v := s.Min + ratio*(s.Max-s.Min)
	if s.Step > 0 {
		v = s.Min + math.Round((v-s.Min)/s.Step)*s.Step
	}
	v = s.clamp(v)
	if v == s.Value {
		return
	}
	s.Value = v
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(v)
	}
}

func (s *SlimSlider) endChange() {
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

type slimSliderRenderer struct {
	s     *SlimSlider
	track *canvas.Rectangle
	fill  *canvas.Rectangle
	thumb *canvas.Circle
}

func (s *SlimSlider) CreateRenderer() fyne.WidgetRenderer {
	r := &slimSliderRenderer{s: s}
	r.track = canvas.NewRectangle(theme.SeparatorColor())
	r.fill = canvas.NewRectangle(secondaryInk())
	r.thumb = &canvas.Circle{FillColor: secondaryInk()}
	return r
}

func (r *slimSliderRenderer) apply() {
	s := r.s
	r.track.FillColor = theme.SeparatorColor()
	if s.disabled {
		r.fill.FillColor = theme.DisabledColor()
		r.thumb.FillColor = theme.DisabledColor()
		return
	}
	r.fill.FillColor = secondaryInk()
	if s.hovered || s.focused {
		r.thumb.FillColor = theme.ForegroundColor()
	} else {
		r.thumb.FillColor = secondaryInk()
	}
}

func (r *slimSliderRenderer) MinSize() fyne.Size {
	return fyne.NewSize(60, 28)
}

func (r *slimSliderRenderer) Layout(size fyne.Size) {
	r.apply()
	cy := size.Height / 2
	r.track.Resize(fyne.NewSize(size.Width, slimTrackHeight))
	r.track.Move(fyne.NewPos(0, cy-slimTrackHeight/2))
	w := size.Width * float32(r.s.ratio())
	r.fill.Resize(fyne.NewSize(w, slimTrackHeight))
	r.fill.Move(fyne.NewPos(0, cy-slimTrackHeight/2))
	r.thumb.Resize(fyne.NewSize(slimThumbSize, slimThumbSize))
	r.thumb.Move(fyne.NewPos(w-slimThumbSize/2, cy-slimThumbSize/2))
}

func (r *slimSliderRenderer) Refresh() {
	r.apply()
	r.Layout(r.s.Size())
	canvas.Refresh(r.s)
}

func (r *slimSliderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill, r.thumb}
}

func (r *slimSliderRenderer) Destroy() {}

var (
	_ fyne.Tappable     = (*SlimSlider)(nil)
	_ fyne.Draggable    = (*SlimSlider)(nil)
	_ fyne.Focusable    = (*SlimSlider)(nil)
	_ fyne.Disableable  = (*SlimSlider)(nil)
	_ desktop.Hoverable = (*SlimSlider)(nil)
)
