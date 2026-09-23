package doc

import (
	"testing"
)

func TestFromText(t *testing.T) {
	d := FromText("Pasted text", "hello world foo")
	if d.Kind != KindText || len(d.Words) != 3 {
		t.Fatalf("doc = %+v", d)
	}
	if d.Fingerprint == "" {
		t.Fatalf("missing fingerprint")
	}
	if len(d.TOC) != 1 || d.TOC[0].StartWord == nil || *d.TOC[0].StartWord != 0 {
		t.Fatalf("single section TOC = %+v", d.TOC)
	}
	if got := d.SectionLabel(2); got != "Pasted text" {
		t.Fatalf("section = %q", got)
	}
	// Same text fingerprints identically (resume works).
	if FromText("Pasted text", "hello world foo").Fingerprint != d.Fingerprint {
		t.Fatalf("fingerprint unstable")
	}
	// Empty paste: readable nothing, no sections.
	e := FromText("Pasted text", "   \n ")
	if len(e.Words) != 0 || len(e.TOC) != 0 {
		t.Fatalf("empty paste = %d words %d toc", len(e.Words), len(e.TOC))
	}
}

func TestSectionForWordDeepest(t *testing.T) {
	s0, s1, s2 := 0, 5, 8
	d := &Document{Words: make([]string, 12)}
	d.TOC = []*TOCItem{
		{Label: "A", StartWord: &s0, Children: []*TOCItem{
			{Label: "A.1", StartWord: &s1},
		}},
		{Label: "B", StartWord: &s2},
		{Label: "Ghost"}, // unresolvable: never selected
	}
	if got := d.SectionLabel(6); got != "A.1" {
		t.Fatalf("nested section = %q", got)
	}
	if got := d.SectionLabel(10); got != "B" {
		t.Fatalf("later section = %q", got)
	}
	if idx, ok := d.SeekTOC(d.TOC[2]); ok || idx != 0 {
		t.Fatalf("ghost seek = %d %v", idx, ok)
	}
}
