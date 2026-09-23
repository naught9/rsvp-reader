package pdf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pdf "github.com/dslipak/pdf"

	"rsvp-reader/internal/doc"
)

// Safety limits so a malformed file cannot exhaust memory.
const (
	maxPDFBytes = 100 << 20
	maxPages    = 10000
	maxWords    = 2000000
)

// OpenFile extracts a PDF's reading order (page by page) into a Document
// whose navigation is the page list.
func OpenFile(path string) (*doc.Document, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	if info.Size() > maxPDFBytes {
		return nil, fmt.Errorf("unsupported PDF: file exceeds %d bytes", maxPDFBytes)
	}
	fp, err := fingerprint(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	r, err := pdf.Open(path)
	if err != nil {
		return nil, pdfError(err)
	}
	n := r.NumPage()
	if n > maxPages {
		return nil, fmt.Errorf("unsupported PDF: %d pages exceeds %d", n, maxPages)
	}
	d := &doc.Document{
		Fingerprint:  fp,
		Kind:         doc.KindPDF,
		Title:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		TOCGenerated: true,
		TOCWarning:   "This PDF has no chapter list; showing pages instead.",
	}
	for p := 1; p <= n; p++ {
		rows, err := r.Page(p).GetTextByRow()
		if err != nil {
			continue // unreadable page: skip, keep the rest
		}
		var sb strings.Builder
		for _, row := range rows {
			for _, t := range row.Content {
				sb.WriteString(t.S)
				sb.WriteByte(' ')
			}
			sb.WriteByte('\n')
		}
		words := strings.Fields(sb.String())
		if len(words) == 0 {
			continue // blank or image-only page
		}
		if len(d.Words)+len(words) > maxWords {
			return nil, fmt.Errorf("unsupported PDF: word count exceeds %d", maxWords)
		}
		start := len(d.Words)
		d.Words = append(d.Words, words...)
		label := fmt.Sprintf("Page %d", p)
		ref := fmt.Sprintf("page:%d", p)
		d.TOC = append(d.TOC, &doc.TOCItem{Label: label, StartWord: &start, Ref: ref})
	}
	if len(d.Words) == 0 {
		return nil, fmt.Errorf("unsupported content: no readable text found in this PDF (it may be scanned images)")
	}
	return d, nil
}

func fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func pdfError(err error) error {
	msg := strings.ToLower(err.Error())
	for _, hint := range []string{"decrypt", "encrypt", "password", "permission"} {
		if strings.Contains(msg, hint) {
			return fmt.Errorf("unsupported content: this PDF is encrypted and cannot be opened")
		}
	}
	return fmt.Errorf("invalid PDF: %w", err)
}
