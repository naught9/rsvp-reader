package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Scandinavian dark theme: a near-black canvas with a neutral white ink
// ladder built from alpha, per the scandinavian-design system. Semantic
// colors (error, warning, success, focus) and the light variant are left
// exactly as Fyne ships them.
var (
	scandiCanvas     = color.NRGBA{R: 0x0a, G: 0x0a, B: 0x0a, A: 0xff}
	scandiInk        = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	scandiOnPrimary  = color.NRGBA{R: 0x0a, G: 0x0a, B: 0x0a, A: 0xff}
	scandiButton     = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x17} // ~9%
	scandiHover      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x17} // ~9%
	scandiPressed    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x24} // ~14%
	scandiSeparator  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x1f} // ~12%
	scandiInputBG    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0d} // ~5%
	scandiInputEdge  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x2e} // ~18%
	scandiMuted      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x5c} // ~36%
	scandiScrollBar  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x40} // ~25%
	scandiMenuBG     = color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff}
	scandiOverlayBG  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x70} // ~44% scrim
	scandiDisabledBl = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0d} // ~5%
)

// scandiTheme wraps the stock dark theme, overriding only the neutral
// chrome tokens. Everything else (fonts, icons, sizes, semantic colors)
// delegates, so accessibility states keep their shipped prominence.
//
// The reader font rides the Monospace slot: the ORP display is the only
// monospace consumer, so a user-picked reader typeface changes the book
// text without touching interface chrome.
type scandiTheme struct {
	base       fyne.Theme
	readerFont fyne.Resource
}

func newScandiTheme() *scandiTheme {
	return &scandiTheme{base: theme.DarkTheme()}
}

// SetReaderFont installs the user-selected reader typeface (nil restores
// the bundled font).
func (t *scandiTheme) SetReaderFont(r fyne.Resource) {
	t.readerFont = r
}

func (t *scandiTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if variant == theme.VariantDark {
		switch name {
		case theme.ColorNameBackground:
			return scandiCanvas
		case theme.ColorNameForeground:
			return scandiInk
		case theme.ColorNamePrimary:
			return scandiInk
		case theme.ColorNameForegroundOnPrimary:
			return scandiOnPrimary
		case theme.ColorNameButton:
			return scandiButton
		case theme.ColorNameDisabledButton:
			return scandiDisabledBl
		case theme.ColorNameDisabled:
			return scandiMuted
		case theme.ColorNameHover:
			return scandiHover
		case theme.ColorNamePressed:
			return scandiPressed
		case theme.ColorNameSeparator:
			return scandiSeparator
		case theme.ColorNameScrollBar:
			return scandiScrollBar
		case theme.ColorNameInputBackground:
			return scandiInputBG
		case theme.ColorNameInputBorder:
			return scandiInputEdge
		case theme.ColorNamePlaceHolder:
			return scandiMuted
		case theme.ColorNameMenuBackground:
			return scandiMenuBG
		case theme.ColorNameOverlayBackground:
			return scandiOverlayBG
		}
	}
	return t.base.Color(name, variant)
}

func (t *scandiTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace && t.readerFont != nil {
		return t.readerFont
	}
	return t.base.Font(style)
}

func (t *scandiTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *scandiTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}

// secondaryInk returns the supporting-ink rung for the current variant:
// soft white on the near-black canvas, soft black on light surfaces.
func secondaryInk() color.Color {
	if fyne.CurrentApp() != nil &&
		fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantLight {
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x8f} // ~56%
	}
	return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x8f} // ~56%
}
