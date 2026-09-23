package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SlimProgress is a quiet 4px progress hairline: a separator track with a
// secondary-ink fill. It replaces the chunky high-contrast stock bar.
type SlimProgress struct {
	widget.BaseWidget
	Value float64 // 0..1
}

// NewSlimProgress creates the progress hairline.
func NewSlimProgress() *SlimProgress {
	p := &SlimProgress{}
	p.ExtendBaseWidget(p)
	return p
}

// SetValue updates the fraction, clamped to 0..1.
func (p *SlimProgress) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	if p.Value == v {
		return
	}
	p.Value = v
	p.Refresh()
}

type slimRenderer struct {
	p     *SlimProgress
	track *canvas.Rectangle
	fill  *canvas.Rectangle
}

func (p *SlimProgress) CreateRenderer() fyne.WidgetRenderer {
	r := &slimRenderer{p: p}
	r.track = canvas.NewRectangle(theme.SeparatorColor())
	r.fill = canvas.NewRectangle(secondaryInk())
	return r
}

func (r *slimRenderer) apply() {
	r.track.FillColor = theme.SeparatorColor()
	r.fill.FillColor = secondaryInk()
}

func (r *slimRenderer) MinSize() fyne.Size {
	return fyne.NewSize(40, 4)
}

func (r *slimRenderer) Layout(size fyne.Size) {
	r.apply()
	h := float32(4)
	y := (size.Height - h) / 2
	r.track.Resize(fyne.NewSize(size.Width, h))
	r.track.Move(fyne.NewPos(0, y))
	w := size.Width * float32(r.p.Value)
	if w < 0 {
		w = 0
	}
	if w > size.Width {
		w = size.Width
	}
	r.fill.Resize(fyne.NewSize(w, h))
	r.fill.Move(fyne.NewPos(0, y))
}

func (r *slimRenderer) Refresh() {
	r.apply()
	r.Layout(r.p.Size())
	canvas.Refresh(r.p)
}

func (r *slimRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill}
}

func (r *slimRenderer) Destroy() {}
