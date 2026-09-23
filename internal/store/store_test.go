package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := Progress{Fingerprint: "abc123", Path: "/books/a.epub", Title: "A", WordIndex: 42, WPM: 300}
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Lookup("abc123")
	if !ok || got.WordIndex != 42 || got.WPM != 300 || got.Title != "A" {
		t.Fatalf("lookup = %+v %v", got, ok)
	}
	// Reopen: durable.
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok = s2.Lookup("abc123")
	if !ok || got.WordIndex != 42 {
		t.Fatalf("reopen lookup = %+v %v", got, ok)
	}
	// Valid JSON on disk.
	raw, _ := os.ReadFile(filepath.Join(dir, "progress.json"))
	if len(raw) == 0 || raw[0] != '{' {
		t.Fatalf("progress.json not JSON")
	}
}

func TestRecentOrderingAndCap(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	for i := 0; i < MaxRecent+3; i++ {
		fp := string(rune('a'+i)) + "fp"
		if err := s.Save(Progress{Fingerprint: fp, Path: fp}); err != nil {
			t.Fatal(err)
		}
	}
	rec := s.ListRecent()
	if len(rec) != MaxRecent {
		t.Fatalf("recent = %d, want %d", len(rec), MaxRecent)
	}
	if rec[0].Fingerprint == "" || rec[0].Path == "" {
		t.Fatalf("newest entry missing")
	}
	// Re-saving an old fingerprint moves it to front.
	old := rec[MaxRecent-1].Fingerprint
	if err := s.Save(Progress{Fingerprint: old, Path: old}); err != nil {
		t.Fatal(err)
	}
	if s.ListRecent()[0].Fingerprint != old {
		t.Fatalf("re-save did not refresh recency")
	}
}

func TestRemoveAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	_ = s.Save(Progress{Fingerprint: "x", Path: "x"})
	if err := s.Remove("x"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Lookup("x"); ok {
		t.Fatalf("removed entry still present")
	}
	// Corrupt file starts fresh, keeps backup.
	os.WriteFile(filepath.Join(dir, "progress.json"), []byte("{nope"), 0o644)
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.ListRecent()) != 0 {
		t.Fatalf("corrupt store should start empty")
	}
}

func TestTextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	if err := s.SaveText("fp1", "hello pasted world"); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadText("fp1")
	if err != nil || got != "hello pasted world" {
		t.Fatalf("text = %q %v", got, err)
	}
	if _, err := s.LoadText("missing"); err == nil {
		t.Fatalf("missing text loaded without error")
	}
	big := make([]byte, MaxTextBytes+1)
	if err := s.SaveText("big", string(big)); err == nil {
		t.Fatalf("oversize text accepted")
	}
}

func TestThemeRoundTrip(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := s.GetTheme(); got != "" {
		t.Fatalf("fresh theme = %q", got)
	}
	if err := s.SetTheme(ThemeLight); err != nil {
		t.Fatal(err)
	}
	if got := s.GetTheme(); got != ThemeLight {
		t.Fatalf("theme = %q", got)
	}
	// Survives reopen; nil store is safe.
	s2, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.GetTheme(); got != ThemeLight {
		t.Fatalf("reopened theme = %q", got)
	}
	var nilStore *Store
	if got := nilStore.GetTheme(); got != "" {
		t.Fatalf("nil theme = %q", got)
	}
	if got := GetThemeOr(nil, "x"); got != "x" {
		t.Fatalf("nil fallback = %q", got)
	}
}
