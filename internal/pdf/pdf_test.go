package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalPDF builds a valid two-page PDF with uncompressed content.
func minimalPDF(t *testing.T) string {
	t.Helper()
	contents1 := "BT /F1 18 Tf 72 720 Td (Hello world page one) Tj ET"
	contents2 := "BT /F1 18 Tf 72 720 Td (Second page text here) Tj ET"
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 5 0 R /Resources << /Font << /F1 7 0 R >> >> >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 6 0 R /Resources << /Font << /F1 7 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contents1), contents1),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contents2), contents2),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var sb strings.Builder
	sb.WriteString("%PDF-1.4\n")
	offsets := []int{}
	for i, body := range objs {
		offsets = append(offsets, sb.Len())
		fmt.Fprintf(&sb, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := sb.Len()
	fmt.Fprintf(&sb, "xref\n0 %d\n", len(objs)+1)
	sb.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&sb, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&sb, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	p := filepath.Join(t.TempDir(), "two-pages.pdf")
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpenTwoPages(t *testing.T) {
	d, err := OpenFile(minimalPDF(t))
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "pdf" || d.Title != "two-pages" {
		t.Fatalf("doc = kind %q title %q", d.Kind, d.Title)
	}
	if !d.TOCGenerated || d.TOCWarning == "" {
		t.Fatalf("pages must be labeled generated with an explanation")
	}
	if len(d.TOC) != 2 || d.TOC[0].Label != "Page 1" || d.TOC[1].Label != "Page 2" {
		t.Fatalf("page TOC = %+v", d.TOC)
	}
	want := []string{"Hello", "world", "page", "one", "Second", "page", "text", "here"}
	if len(d.Words) != len(want) {
		t.Fatalf("words = %q", d.Words)
	}
	for i, w := range want {
		if d.Words[i] != w {
			t.Fatalf("words = %q, want %q", d.Words, want)
		}
	}
	if got := d.SectionLabel(0); got != "Page 1" {
		t.Fatalf("section(0) = %q", got)
	}
	if got := d.SectionLabel(4); got != "Page 2" {
		t.Fatalf("section(4) = %q", got)
	}
	if idx, ok := d.SeekTOC(d.TOC[1]); !ok || idx != 4 {
		t.Fatalf("seek page 2 = %d %v", idx, ok)
	}
}

func TestOpenGarbage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "junk.pdf")
	os.WriteFile(p, []byte("this is not a pdf"), 0o644)
	if _, err := OpenFile(p); err == nil {
		t.Fatalf("garbage PDF opened without error")
	}
}

func TestPageStartsBreakParagraphs(t *testing.T) {
	d, err := OpenFile(minimalPDF(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.ParaStarts) != 2 || d.ParaStarts[0] != 0 || d.ParaStarts[1] != 4 {
		t.Fatalf("starts = %v, want [0 4]", d.ParaStarts)
	}
}
