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

func TestStyleScore(t *testing.T) {
	for _, s := range []string{"", "Regular", "Roman", "normal", "Plain", "Book"} {
		if styleScore(s) != 0 {
			t.Errorf("styleScore(%q) != 0", s)
		}
	}
	for _, s := range []string{"Bold", "Italic", "Bold Italic", "Black", "Light", "Oblique"} {
		if styleScore(s) != 2 {
			t.Errorf("styleScore(%q) != 2", s)
		}
	}
}

func TestScanPrefersRegular(t *testing.T) {
	// A Bold file sorting first lexically must not win the family.
	bold, reg := realFont(t, "Arial Bold.ttf"), realFont(t, "Arial.ttf")
	if bold == "" || reg == "" {
		t.Skip("Arial pair not on this machine")
	}
	dir := t.TempDir()
	copyFile(t, bold, dir+"/a.ttf") // sorts first, must lose
	copyFile(t, reg, dir+"/z.ttf")
	found := ScanSystemFonts([]string{dir})
	if len(found) != 1 || found[0].Family != "Arial" {
		t.Fatalf("scan = %+v", found)
	}
	if found[0].Path != dir+"/z.ttf" {
		t.Fatalf("family maps to %q, want the Regular file", found[0].Path)
	}
}

func TestExtractFacePrefersRegular(t *testing.T) {
	path := realCollection(t)
	data := mustRead(t, path)
	fams := familiesInFile(path)
	if len(fams) == 0 {
		t.Skip("collection names nothing")
	}
	single, err := ExtractFace(data, fams[0])
	if err != nil {
		t.Fatalf("extract %q: %v", fams[0], err)
	}
	tmp := t.TempDir() + "/face.ttf"
	if err := os.WriteFile(tmp, single, 0o644); err != nil {
		t.Fatal(err)
	}
	faces := facesInFile(tmp)
	if len(faces) != 1 {
		t.Fatalf("extracted %d faces", len(faces))
	}
	if got := styleScore(faces[0].subfamily); got != 0 {
		t.Fatalf("picked subfamily %q scores %d, want Regular", faces[0].subfamily, got)
	}
}

func realFont(t *testing.T, base string) string {
	t.Helper()
	for _, dir := range FontDirs() {
		p := filepath.Join(dir, base)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func realCollection(t *testing.T) string {
	t.Helper()
	for _, dir := range FontDirs() {
		var found string
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if found != "" || err != nil || d.IsDir() {
				return nil
			}
			if ext := strings.ToLower(filepath.Ext(path)); ext == ".ttc" || ext == ".dfont" {
				found = path
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	t.Skip("no font collections on this machine")
	return ""
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
