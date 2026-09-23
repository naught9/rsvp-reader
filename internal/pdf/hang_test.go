package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildRawPDF assembles objects: 1 catalog, 2 pages, then (page, content)
// pairs, then the font. Offsets computed as we go.
func buildRawPDF(t *testing.T, streams []string) string {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("%PDF-1.4\n")
	var offsets []int
	emit := func(body string) int {
		offsets = append(offsets, sb.Len())
		n := len(offsets)
		fmt.Fprintf(&sb, "%d 0 obj\n%s\nendobj\n", n, body)
		return n
	}
	kids := []string{}
	for i := range streams {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i*2))
	}
	emit("<< /Type /Catalog /Pages 2 0 R >>")
	emit(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(streams)))
	fontNum := 3 + len(streams)*2
	for i, s := range streams {
		contentNum := 4 + i*2
		emit(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>", contentNum, fontNum))
		emit(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(s), s))
	}
	emit("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	xref := sb.Len()
	fmt.Fprintf(&sb, "xref\n0 %d\n", len(offsets)+1)
	sb.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&sb, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&sb, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	p := filepath.Join(t.TempDir(), "probe.pdf")
	if err := os.WriteFile(p, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestStuckPageSkipped replays the Iliad incident: a content stream ending
// mid-string hangs the tokenizer forever. The import must bound the page,
// skip it with a warning, and keep the readable pages.
func TestStuckPageSkipped(t *testing.T) {
	old := pageTimeout
	pageTimeout = 300 * time.Millisecond
	defer func() { pageTimeout = old }()

	p := buildRawPDF(t, []string{
		"BT /F1 12 Tf 72 720 Td (Readable page one) Tj ET",
		"BT /F1 12 Tf 72 720 Td (Hung (unterminated",
	})
	d, err := OpenFile(p)
	if err != nil {
		t.Fatalf("import failed instead of skipping: %v", err)
	}
	if len(d.Words) != 3 || d.Words[0] != "Readable" {
		t.Fatalf("words = %q", d.Words)
	}
	if len(d.TOC) != 1 || d.TOC[0].Label != "Page 1" {
		t.Fatalf("TOC = %+v", d.TOC)
	}
	if !strings.Contains(d.TOCWarning, "2 could not be read") || !strings.Contains(d.TOCWarning, "skipped") {
		t.Fatalf("warning = %q, must name the skipped page", d.TOCWarning)
	}
}
