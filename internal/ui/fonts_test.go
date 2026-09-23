package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func TestScanSkipsGarbage(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "broken.ttf"), []byte("not a font at all"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "empty.otf"), []byte{}, 0o644)
	if got := ScanSystemFonts([]string{dir}); len(got) != 0 {
		t.Fatalf("garbage scan = %v, want empty", got)
	}
	if got := ScanSystemFonts([]string{filepath.Join(dir, "missing")}); len(got) != 0 {
		t.Fatalf("missing dir scan = %v, want empty", got)
	}
}

func TestScanFindsRealFont(t *testing.T) {
	// Opportunistic: validates the happy path wherever fonts exist.
	var found []SystemFont
	for _, dir := range FontDirs() {
		if found = ScanSystemFonts([]string{dir}); len(found) > 0 {
			break
		}
	}
	// Fall back to a full scan before giving up.
	if len(found) == 0 {
		found = ScanSystemFonts(FontDirs())
	}
	if len(found) == 0 {
		t.Skip("no loadable system fonts on this machine")
	}
	t.Logf("first families: %q %q %q", found[0].Family,
		familyAt(found, 1), familyAt(found, 2))
	for _, f := range found {
		if f.Family == "" || f.Path == "" {
			t.Fatalf("incomplete entry: %+v", f)
		}
	}
	// Every listed file must reload.
	for _, f := range found[:min(5, len(found))] {
		if _, err := LoadFontResource(f.Path); err != nil {
			t.Fatalf("listed font %q failed to reload: %v", f.Family, err)
		}
	}
}

func familyAt(fs []SystemFont, i int) string {
	if i < len(fs) {
		return fs[i].Family
	}
	return ""
}

func TestThemeReaderFontSlot(t *testing.T) {
	th := newScandiTheme()
	mono := fyne.TextStyle{Monospace: true}
	base := theme.DarkTheme()
	if th.Font(mono) != base.Font(mono) {
		t.Fatalf("unset slot must delegate to the bundled font")
	}
	custom := fyne.NewStaticResource("test.ttf", []byte{1, 2, 3})
	th.SetReaderFont(custom)
	if th.Font(mono) != custom {
		t.Fatalf("monospace slot must serve the reader typeface")
	}
	if th.Font(fyne.TextStyle{}) != base.Font(fyne.TextStyle{}) {
		t.Fatalf("interface chrome must keep the bundled font")
	}
	th.SetReaderFont(nil)
	if th.Font(mono) != base.Font(mono) {
		t.Fatalf("clearing must restore the bundled font")
	}
}
