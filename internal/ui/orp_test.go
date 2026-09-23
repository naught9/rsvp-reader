package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

func TestFocusRules(t *testing.T) {
	a, _ := newTestApp(t) // scandi theme active: full font set
	w := a.orp
	w.SetWord("word")
	r := w.CreateRenderer().(*orpRenderer)
	size := fyne.NewSize(800, 600)
	r.Layout(size)
	midY := size.Height / 2
	// Rules span the pane at the ticks' outer ends, touching them, in
	// the separator color.
	rules := map[string]*canvas.Rectangle{"top": r.ruleTop, "bottom": r.ruleBottom}
	wantY := map[string]float32{"top": midY - 90 - 1, "bottom": midY + 90 - 1}
	for name, rule := range rules {
		if rule.Size().Width != size.Width || rule.Position().X != 0 {
			t.Fatalf("%s rule does not span the pane: pos %v size %v", name, rule.Position(), rule.Size())
		}
		if rule.Position().Y != wantY[name] {
			t.Fatalf("%s rule y = %v, want %v", name, rule.Position().Y, wantY[name])
		}
		if rule.FillColor != theme.SeparatorColor() {
			t.Fatalf("%s rule color diverges from the separators", name)
		}
	}
	// Touching: top tick starts at its rule, bottom tick ends at its rule.
	if r.guideTop.Position().Y != midY-90 {
		t.Fatalf("top tick does not meet its rule")
	}
	if r.guideBottom.Position().Y+r.guideBottom.Size().Height != midY+90 {
		t.Fatalf("bottom tick does not meet its rule")
	}
}
