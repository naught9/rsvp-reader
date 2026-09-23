package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gofont "github.com/go-text/typesetting/font"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"rsvp-reader/internal/store"
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

func TestExtractFaceFromCollection(t *testing.T) {
	// Finds a real .ttc/.dfont, extracts its first family, and proves the
	// result passes Fyne's exact gate (single-font ParseTTF).
	type candidate struct{ path, family string }
	var cand *candidate
	for _, dir := range FontDirs() {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if cand != nil || err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".ttc" && ext != ".dfont" {
				return nil
			}
			if fams := familiesInFile(path); len(fams) > 0 {
				cand = &candidate{path, fams[0]}
			}
			return nil
		})
		if cand != nil {
			break
		}
	}
	if cand == nil {
		t.Skip("no font collections on this machine")
	}
	single, err := ExtractFace(mustRead(t, cand.path), cand.family)
	if err != nil {
		t.Fatalf("extract %q: %v", cand.family, err)
	}
	if _, err := gofont.ParseTTF(bytes.NewReader(single)); err != nil {
		t.Fatalf("extracted face fails Fyne's loader gate: %v", err)
	}
	t.Logf("extracted %q from %s (%d -> %d bytes)", cand.family, cand.path,
		mustStat(t, cand.path), len(single))
}

func TestBadPersistedFontRecovers(t *testing.T) {
	fyneApp := fynetest.NewApp()
	db, _ := store.Open(t.TempDir())
	a := New(fyneApp, db, "")
	// Replay the crash loop: a persisted collection path must be rejected
	// and cleared, never installed as the theme font.
	a.prefs().SetString("readerFontPath", "/nonexistent/Helvetica.ttc")
	a.prefs().SetString("readerFontFamily", "Helvetica")
	a.loadSavedFont()
	if a.readerFont != nil {
		t.Fatalf("unloadable persisted font was installed")
	}
	if a.prefs().StringWithFallback("readerFontPath", "") != "" {
		t.Fatalf("bad persisted path was not cleared")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustStat(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
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
