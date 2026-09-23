package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
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
}

// SectionForWord returns the deepest TOC entry whose resolved start is at
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
func FromText(title, text string) *Document {
	words := strings.Fields(text)
	fp := sha256.Sum256([]byte(text))
	d := &Document{
		Fingerprint: hex.EncodeToString(fp[:]),
		Kind:        KindText,
		Title:       title,
		Words:       words,
	}
	if len(words) > 0 {
		start := 0
		d.TOC = []*TOCItem{{Label: title, StartWord: &start, Ref: "text"}}
	}
	return d
}
