package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"

	"rsvp-reader/internal/tokens"
)

// Kinds of readable sources.
const (
	KindEPUB = "epub"
	KindPDF  = "pdf"
	KindText = "text"
)

// TOCItem is one navigation entry. StartWord is nil when the entry cannot
// start playback (group heading, unavailable target, or empty section).
type TOCItem struct {
	Label       string
	StartWord   *int
	Children    []*TOCItem
	Unavailable string
	// Ref identifies the target for persistence (href, page marker, ...).
	Ref string
}

// Document is the source-agnostic reading model: an identity, a word
// stream in reading order, and a navigation tree over it.
type Document struct {
	Fingerprint string
	Kind        string
	Title       string
	Creator     string
	Words       []string

	TOC []*TOCItem
	// TOCGenerated marks entries synthesized from pages/sections rather
	// than supplied by the publisher.
	TOCGenerated bool
	TOCWarning   string
	// ParaStarts holds sorted word indices that open a paragraph. Always
	// contains 0 when Words is non-empty. Drives the context pane.
	ParaStarts []int
}

// NormalizeParaStarts sorts, dedupes, and anchors paragraph starts at 0.
func NormalizeParaStarts(starts []int, nwords int) []int {
	seen := map[int]bool{}
	out := []int{0}
	seen[0] = true
	for _, s := range starts {
		if s < 0 || s >= nwords || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Ints(out)
	if nwords == 0 {
		return nil
	}
	return out
}

// or before wordIdx. Returns nil when none resolves.
func (d *Document) SectionForWord(wordIdx int) *TOCItem {
	if d == nil || wordIdx < 0 {
		return nil
	}
	var best *TOCItem
	bestStart := -1
	var walk func(items []*TOCItem)
	walk = func(items []*TOCItem) {
		for _, it := range items {
			if it.StartWord != nil && *it.StartWord <= wordIdx && *it.StartWord > bestStart {
				bestStart = *it.StartWord
				best = it
			}
			walk(it.Children)
		}
	}
	walk(d.TOC)
	return best
}

// SeekTOC resolves an entry to its start word.
func (d *Document) SeekTOC(it *TOCItem) (int, bool) {
	if d == nil || it == nil || it.StartWord == nil {
		return 0, false
	}
	if *it.StartWord < 0 || *it.StartWord >= len(d.Words) {
		return 0, false
	}
	return *it.StartWord, true
}

// SectionLabel returns the section name for a word, or "" when unknown.
func (d *Document) SectionLabel(wordIdx int) string {
	if s := d.SectionForWord(wordIdx); s != nil {
		return s.Label
	}
	return ""
}

// FromText builds a document from pasted or loaded plain text: one
// section spanning the whole stream.
var blankLine = regexp.MustCompile(`\n[ \t\r]*\n`)

func FromText(title, text string) *Document {
	// Blank-line separated blocks are paragraphs; tokenization per block
	// matches whole-text tokenization, only the starts are recorded.
	var words []string
	var starts []int
	for _, para := range blankLine.Split(text, -1) {
		pw := tokens.SplitWords(para)
		if len(pw) == 0 {
			continue
		}
		starts = append(starts, len(words))
		words = append(words, pw...)
	}
	fp := sha256.Sum256([]byte(text))
	d := &Document{
		Fingerprint: hex.EncodeToString(fp[:]),
		Kind:        KindText,
		Title:       title,
		Words:       words,
		ParaStarts:  NormalizeParaStarts(starts, len(words)),
	}
	if len(words) > 0 {
		start := 0
		d.TOC = []*TOCItem{{Label: title, StartWord: &start, Ref: "text"}}
	}
	return d
}
