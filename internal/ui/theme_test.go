package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"rsvp-reader/internal/store"
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
	// Light variant carries the paper/ink mirror (see
	// TestScandiCoherentBothVariants); only semantics delegate.
	lightBase := theme.LightTheme()
	for _, name := range []fyne.ThemeColorName{
		theme.ColorNameError, theme.ColorNameWarning, theme.ColorNameSuccess, theme.ColorNameFocus,
	} {
		if nrgba(th.Color(name, theme.VariantLight)) != nrgba(lightBase.Color(name, theme.VariantLight)) {
			t.Fatalf("light semantic token %q was altered", name)
		}
	}
}

func TestScandiCoherentBothVariants(t *testing.T) {
	th := newScandiTheme()
	bgD := th.Color(theme.ColorNameBackground, theme.VariantDark)
	bgL := th.Color(theme.ColorNameBackground, theme.VariantLight)
	if bgD == bgL {
		t.Fatalf("background identical across variants")
	}
	fgD := th.Color(theme.ColorNameForeground, theme.VariantDark)
	fgL := th.Color(theme.ColorNameForeground, theme.VariantLight)
	if fgD == fgL {
		t.Fatalf("foreground identical across variants")
	}
	// Light rows are light: paper background, dark ink.
	lr, lg, lb, _ := bgL.RGBA()
	fr, _, _, _ := fgL.RGBA()
	if lr < 0xf000 || lg < 0xf000 || lb < 0xf000 {
		t.Fatalf("light background too dark: %v", bgL)
	}
	if fr > 0x4000 {
		t.Fatalf("light foreground too faint: %v", fgL)
	}
	// Semantics still delegate per variant.
	if th.Color(theme.ColorNameError, theme.VariantLight) != theme.LightTheme().Color(theme.ColorNameError, theme.VariantLight) {
		t.Fatalf("light error diverges from stock")
	}
}

func TestThemeChoiceMapping(t *testing.T) {
	if themeChoiceToStore("Dark") != store.ThemeDark || themeChoiceToStore("Light") != store.ThemeLight {
		t.Fatalf("choice mapping")
	}
	if themeChoiceToStore("Follow System") != store.ThemeSystem || themeChoiceToStore("??") != store.ThemeSystem {
		t.Fatalf("default mapping")
	}
	if themeStoreToLabel(store.ThemeDark) != "Dark" || themeStoreToLabel("") != "Follow System" {
		t.Fatalf("label mapping")
	}
	if env, ok := ThemeEnvOverride(store.ThemeLight); !ok || env != "light" {
		t.Fatalf("light env")
	}
	if env, ok := ThemeEnvOverride(store.ThemeDark); !ok || env != "dark" {
		t.Fatalf("dark env")
	}
	if _, ok := ThemeEnvOverride(store.ThemeSystem); ok {
		t.Fatalf("system must not force the environment")
	}
	if _, ok := ThemeEnvOverride(""); ok {
		t.Fatalf("unset must not force the environment")
	}
}
