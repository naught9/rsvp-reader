package vocab

import (
	"testing"
)

func TestRarityBeats(t *testing.T) {
	if got := RarityBeats("the"); got != 0 {
		t.Errorf("the = %d, want 0", got)
	}
	if got := RarityBeats("radio"); got != 0 {
		t.Errorf("radio = %d, want 0", got)
	}
	if got := RarityBeats("prizes"); got != 0 {
		t.Errorf("prizes = %d, want 0", got)
	}
	// Mid band breathes once (absinthe ≈ 2.7 in the table).
	if got := RarityBeats("absinthe"); got != 1 {
		t.Errorf("absinthe = %d, want 1", got)
	}
	// Invented words miss the table: the New Sun case.
	for _, w := range []string{"fuligin", "cacogen", "Severian", "fuligin-", "FULIGIN"} {
		if got := RarityBeats(w); got < 2 {
			t.Errorf("%s = %d, want >= 2", w, got)
		}
	}
	// Guards: digits, abbreviations, and plain punctuation never slow.
	for _, w := range []string{"1945", "3.14", "12,109", "—", "", "e.g.", "Mr.", "U.S."} {
		if got := RarityBeats(w); got != 0 {
			t.Errorf("%s = %d, want 0", w, got)
		}
	}
	// Punctuation rides along without changing the verdict.
	for _, w := range []string{"said,", "it.", "her.", "don\u2019t", "\u201cI", "word?"} {
		if got := RarityBeats(w); got != 0 {
			t.Errorf("%s = %d, want 0", w, got)
		}
	}
	// Very long words earn a length beat on top of the rarity beats.
	if got := RarityBeats("pneumonoultramicroscopicsilicovolcanoconiosis"); got != 3 {
		t.Errorf("long unknown word = %d, want 3", got)
	}
}

func TestTableLoaded(t *testing.T) {
	load()
	if len(zipfByW) < 49000 {
		t.Fatalf("table holds %d words", len(zipfByW))
	}
	if z, ok := ZipfOf("the"); !ok || z < 7 || z > 8.5 {
		t.Fatalf("the zipf = %v %v", z, ok)
	}
	if _, ok := ZipfOf("fuligin"); ok {
		t.Fatalf("fuligin should miss the table")
	}
}
