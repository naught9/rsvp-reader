package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func nrgba(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func TestScandiDarkTokens(t *testing.T) {
	th := newScandiTheme()
	dark := theme.VariantDark

	if got := nrgba(th.Color(theme.ColorNameBackground, dark)); got != scandiCanvas {
		t.Fatalf("background = %#v, want near-black canvas", got)
	}
	if got := nrgba(th.Color(theme.ColorNamePrimary, dark)); got.R != 0xff || got.G != 0xff || got.B != 0xff {
		t.Fatalf("primary = %#v, want white ink", got)
	}
	fg := nrgba(th.Color(theme.ColorNameForegroundOnPrimary, dark))
	if fg.R > 0x20 || fg.G > 0x20 || fg.B > 0x20 {
		t.Fatalf("foreground-on-primary = %#v, want dark ink for contrast on the white primary", fg)
	}
	sep := nrgba(th.Color(theme.ColorNameSeparator, dark))
	if sep.A < 0x18 || sep.A > 0x2e {
		t.Fatalf("separator alpha = %#x, want a quiet ~12%% hairline", sep.A)
	}
	if sep.R != sep.G || sep.G != sep.B {
		t.Fatalf("separator = %#v, want neutral (no tint)", sep)
	}
	// Semantic colors must survive untouched.
	base := theme.DarkTheme()
	for _, name := range []fyne.ThemeColorName{
		theme.ColorNameError, theme.ColorNameWarning, theme.ColorNameSuccess, theme.ColorNameFocus,
	} {
		if nrgba(th.Color(name, dark)) != nrgba(base.Color(name, dark)) {
			t.Fatalf("semantic token %q was altered", name)
		}
	}
	// Light variant delegates entirely.
	for _, name := range []fyne.ThemeColorName{
		theme.ColorNameBackground, theme.ColorNamePrimary, theme.ColorNameSeparator,
	} {
		if nrgba(th.Color(name, theme.VariantLight)) != nrgba(base.Color(name, theme.VariantLight)) {
			t.Fatalf("light variant token %q was altered", name)
		}
	}
}
