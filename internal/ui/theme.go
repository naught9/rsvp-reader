package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Scandinavian theme: a near-black canvas with a neutral white ink ladder
// in the dark variant, mirrored as soft black ink on paper white in the
// light variant, per the scandinavian-design system. Semantic colors
// (error, warning, success, focus) are left exactly as Fyne ships them.
var (
	scandiCanvas     = color.NRGBA{R: 0x0a, G: 0x0a, B: 0x0a, A: 0xff}
	scandiInk        = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	scandiOnPrimary  = color.NRGBA{R: 0x0a, G: 0x0a, B: 0x0a, A: 0xff}
	scandiButton     = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x17} // ~9%
	scandiHover      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x17} // ~9%
	scandiPressed    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x24} // ~14%
	scandiSeparator  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x1f} // ~12%
	scandiSelected   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x24} // ~14%
	scandiInputBG    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0d} // ~5%
	scandiInputEdge  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x2e} // ~18%
	scandiMuted      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x5c} // ~36%
	scandiScrollBar  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x40} // ~25%
	scandiMenuBG     = color.NRGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xff}
	scandiOverlayBG  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x70} // ~44% scrim
	scandiDisabledBl = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0d} // ~5%

	scandiPaper        = color.NRGBA{R: 0xfa, G: 0xfa, B: 0xf9, A: 0xff}
	scandiInkDark      = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	scandiOnLight      = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	scandiButtonLt     = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x17} // ~9%
	scandiHoverLt      = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x17} // ~9%
	scandiPressedLt    = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x24} // ~14%
	scandiSeparatorLt  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x1f} // ~12%
	scandiSelectedLt   = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x24} // ~14%
	scandiInputBGLt    = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0d} // ~5%
	scandiInputEdgeLt  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x2e} // ~18%
	scandiMutedLt      = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x8c} // ~55%
	scandiScrollBarLt  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x40} // ~25%
	scandiMenuBGLt     = color.NRGBA{R: 0xf2, G: 0xf2, B: 0xf1, A: 0xff}
	scandiDisabledBlLt = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0d} // ~5%
	// Focus guides are fully opaque (overlapping translucent hairlines
	// double up at intersections). Values match the old 12% separator
	// over each canvas: dark #0a0a0a + 12% white ≈ #262626, paper
	// #fafaf9 + 12% black ≈ #dededb.
	scandiGuide   = color.NRGBA{R: 0x26, G: 0x26, B: 0x26, A: 0xff}
	scandiGuideLt = color.NRGBA{R: 0xde, G: 0xde, B: 0xdb, A: 0xff}
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
	lightBase  fyne.Theme
	readerFont fyne.Resource
}

func newScandiTheme() *scandiTheme {
	return &scandiTheme{base: theme.DarkTheme(), lightBase: theme.LightTheme()}
}

// SetReaderFont installs the user-selected reader typeface (nil restores
// the bundled font).
func (t *scandiTheme) SetReaderFont(r fyne.Resource) {
	t.readerFont = r
}

func (t *scandiTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// The object stays coherent under either variant: SetTheme preserves
	// the system variant, so a dark-only table would render dark rows on
	// a light surface (or vice versa) whenever the two disagree.
	if variant == theme.VariantLight {
		switch name {
		case theme.ColorNameBackground:
			return scandiPaper
		case theme.ColorNameForeground:
			return scandiInkDark
		case theme.ColorNamePrimary:
			return scandiInkDark
		case theme.ColorNameForegroundOnPrimary:
			return scandiOnLight
		case theme.ColorNameButton:
			return scandiButtonLt
		case theme.ColorNameDisabledButton:
			return scandiDisabledBlLt
		case theme.ColorNameDisabled:
			return scandiMutedLt
		case theme.ColorNameHover:
			return scandiHoverLt
		case theme.ColorNamePressed:
			return scandiPressedLt
		case theme.ColorNameSeparator:
			return scandiSeparatorLt
		case theme.ColorNameSelection:
			return scandiSelectedLt
		case theme.ColorNameScrollBar:
			return scandiScrollBarLt
		case theme.ColorNameInputBackground:
			return scandiInputBGLt
		case theme.ColorNameInputBorder:
			return scandiInputEdgeLt
		case theme.ColorNamePlaceHolder:
			return scandiMutedLt
		case theme.ColorNameMenuBackground:
			return scandiMenuBGLt
		case theme.ColorNameOverlayBackground:
			return scandiOverlayBG
		}
		return t.lightBase.Color(name, variant)
	}
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
		case theme.ColorNameSelection:
			return scandiSelected
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

// focusGuideColor is the opaque focal-guide ink for the current variant.
func focusGuideColor() color.Color {
	if fyne.CurrentApp() != nil &&
		fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantLight {
		return scandiGuideLt
	}
	return scandiGuide
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
