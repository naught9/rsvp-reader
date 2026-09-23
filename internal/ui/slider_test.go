package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func TestSlimSliderTapAndDrag(t *testing.T) {
	s := NewSlimSlider(50, 1000, 25)
	s.Resize(fyne.NewSize(200, 28))
	var changed []float64
	var ended []float64
	s.OnChanged = func(v float64) { changed = append(changed, v) }
	s.OnChangeEnded = func(v float64) { ended = append(ended, v) }

	s.Tapped(&fyne.PointEvent{Position: fyne.NewPos(8, 14)}) // far left
	if s.Value != 50 {
		t.Fatalf("tap left = %v, want min", s.Value)
	}
	s.Tapped(&fyne.PointEvent{Position: fyne.NewPos(192, 14)}) // far right
	if s.Value != 1000 {
		t.Fatalf("tap right = %v, want max", s.Value)
	}
	if len(ended) != 2 {
		t.Fatalf("tap must commit, ended = %d", len(ended))
	}
	// Drag scrubs without committing until release.
	n := len(ended)
	s.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(100, 14)}})
	if len(ended) != n {
		t.Fatalf("drag must not commit early")
	}
	if s.Value < 400 || s.Value > 650 || int(s.Value)%25 != 0 {
		t.Fatalf("drag mid = %v, want snapped mid-range 25-step", s.Value)
	}
	s.DragEnd()
	if len(ended) != n+1 {
		t.Fatalf("release must commit once")
	}
	// Disabled ignores everything.
	s.Disable()
	before := s.Value
	s.Tapped(&fyne.PointEvent{Position: fyne.NewPos(192, 14)})
	s.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(192, 14)}})
	if s.Value != before {
		t.Fatalf("disabled slider moved to %v", s.Value)
	}
	_ = changed
}

func TestSelectionTokenMonochrome(t *testing.T) {
	th := newScandiTheme()
	got := nrgba(th.Color(theme.ColorNameSelection, theme.VariantDark))
	if got.R != got.G || got.G != got.B {
		t.Fatalf("selection = %#v, want neutral wash, not blue", got)
	}
	if got.A < 0x18 || got.A > 0x30 {
		t.Fatalf("selection alpha = %#x, want quiet wash", got.A)
	}
}
