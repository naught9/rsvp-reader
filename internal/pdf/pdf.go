package pdf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pdf "github.com/dslipak/pdf"

	"rsvp-reader/internal/doc"
)

// Safety limits so a malformed file cannot exhaust memory.
const (
	maxPDFBytes = 100 << 20
	maxPages    = 10000
	maxWords    = 2000000
)

// A page that never returns must not wedge the import: the underlying
// tokenizer has no EOF exit on truncated strings, so it spins forever
// at full CPU. Bound each page and skip the offender. (The stuck
// goroutine cannot be killed in-process; it dies with the app. Skipping
// keeps the other 699 pages readable instead of losing the book.)
var pageTimeout = 15 * time.Second

// maxStuckPages aborts the import when a file is broadly unreadable
// rather than paying the timeout per page for hundreds of pages.
const maxStuckPages = 10

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
	var skipped []int
	for p := 1; p <= n; p++ {
		words, stuck, err := extractPage(r, p)
		if stuck {
			skipped = append(skipped, p)
			if len(skipped) > maxStuckPages {
				return nil, fmt.Errorf("unsupported PDF: gave up after %d unreadable pages (starting at page %d)", len(skipped), skipped[0])
			}
			continue
		}
		if err != nil {
			continue // unreadable page: skip, keep the rest
		}
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
	if len(skipped) > 0 {
		d.TOCWarning += fmt.Sprintf(" Pages %s could not be read and were skipped.", formatPages(skipped))
	}
	if len(d.Words) == 0 {
		if len(skipped) > 0 {
			return nil, fmt.Errorf("unsupported content: no readable text found in this PDF (pages timed out while reading)")
		}
		return nil, fmt.Errorf("unsupported content: no readable text found in this PDF (it may be scanned images)")
	}
	return d, nil
}

type pageResult struct {
	words []string
	err   error
}

// extractPage reads one page with a watchdog. stuck=true means the
// library never returned; the page must be skipped.
func extractPage(r *pdf.Reader, page int) (words []string, stuck bool, err error) {
	ch := make(chan pageResult, 1)
	go func() {
		rows, err := r.Page(page).GetTextByRow()
		if err != nil {
			ch <- pageResult{err: err}
			return
		}
		var sb strings.Builder
		for _, row := range rows {
			for _, t := range row.Content {
				sb.WriteString(t.S)
				sb.WriteByte(' ')
			}
			sb.WriteByte('\n')
		}
		ch <- pageResult{words: strings.Fields(sb.String())}
	}()
	select {
	case res := <-ch:
		return res.words, false, res.err
	case <-time.After(pageTimeout):
		return nil, true, nil
	}
}

// formatPages renders [3] as "3" and [3 5 7 9] as "3, 5, 7, 9".
func formatPages(pages []int) string {
	if len(pages) > 6 {
		pages = pages[:6]
	}
	strs := make([]string, len(pages))
	for i, p := range pages {
		strs[i] = strconv.Itoa(p)
	}
	s := strings.Join(strs, ", ")
	return s
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
