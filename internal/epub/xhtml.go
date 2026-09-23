package epub

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"

	"rsvp-reader/internal/tokens"
)

// Marginalia pruning.
//
// Printed-book artifacts leak into EPUB body text as words: line numbers
// in verse ("Sing to me" preceded by "1"), page-number anchors, and the
// like. Two conservative rules remove them:
//
//  1. Digit anchors: an <a> carrying an id/name whose entire text is
//     digits ("<a id="filepos123">47</a>"). An id exists to be linked to;
//     a purely numeric link target is a positional marker, and the anchor
//     itself is retained for navigation — only its digit text is dropped.
//     Destinations with real link text, and content links (<a href>),
//     are never touched.
//
//  2. Sequential bare digits: a bare all-digit token opening a block is a
//     line-number *candidate*. Candidates are dropped only when they form
//     an increasing run of 3+ within one document (steps of 1..25 allow
//     partially numbered verse). Isolated numbers — years, counts,
//     "Chapter 1" (not bare), numbered lists ("1." has punctuation) —
//     survive, because pruning must err toward keeping real content.
//
// Dropping rewrites indices, so anchor positions are remapped onto the
// surviving stream and paragraph/sentence flags transfer forward.
type extractor struct {
	spineIndex int
	nonLinear  bool
	words      []Word
	anchorRel  map[string]int // fragment -> relative word index

	curElement string
	blockStart bool

	// Mutable working state; words/out resolved in finish().
	raw         []rawWord
	digitAnchor *digitAnchorSpan
}

type rawWord struct {
	Word
	// drop classifies prunable tokens: anchorDigit always drops;
	// bareDigit drops only inside a qualifying run.
	drop     dropKind
	numValue int
}

type dropKind uint8

const (
	keep dropKind = iota
	anchorDigit
	bareDigit
)

type digitAnchorSpan struct {
	eligible bool
	hasDigit bool
	startIdx int
	depth    int
}

var skipElements = map[string]bool{
	"script": true, "style": true, "noscript": true,
	"template": true, "head": true, "title": true, "meta": true, "link": true,
}

var blockElements = map[string]bool{
	"p": true, "div": true, "section": true, "article": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "ul": true, "ol": true, "blockquote": true, "pre": true,
	"tr": true, "header": true, "footer": true, "figure": true, "figcaption": true,
	"dt": true, "dd": true, "br": true,
}

func (e *extractor) walk(n *html.Node) {
	if e.anchorRel == nil {
		e.anchorRel = map[string]int{}
	}
	e.recurse(n, false)
	e.finish()
}

func (e *extractor) recurse(n *html.Node, inBody bool) {
	if n.Type == html.ElementNode {
		tag := strings.ToLower(n.Data)
		if tag == "body" {
			inBody = true
		}
		if inBody && skipElements[tag] {
			return
		}
		if inBody {
			if id := attrOf(n, "id"); id != "" {
				if _, exists := e.anchorRel[id]; !exists {
					e.anchorRel[id] = len(e.raw)
				}
			}
			// Anchor tags use name= as fragment target in older EPUBs.
			if tag == "a" {
				if nm := attrOf(n, "name"); nm != "" {
					if _, exists := e.anchorRel[nm]; !exists {
						e.anchorRel[nm] = len(e.raw)
					}
				}
				if attrOf(n, "id") != "" || attrOf(n, "name") != "" {
					e.enterDigitAnchor()
				}
			}
			if tag == "img" {
				alt := strings.TrimSpace(attrOf(n, "alt"))
				if alt != "" {
					e.emitText(alt, "img")
				}
			}
			if blockElements[tag] {
				e.blockStart = true
				if tag != "br" {
					e.curElement = tag
				}
			}
			if isHeading(tag) {
				e.curElement = tag
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			e.recurse(c, inBody)
		}
		if inBody {
			if tag == "a" && e.digitAnchor != nil {
				e.exitDigitAnchor()
			}
			if blockElements[tag] {
				e.blockStart = true
			}
		}
		return
	}
	if n.Type == html.TextNode && inBody {
		e.emitText(n.Data, e.curElement)
		if e.digitAnchor != nil {
			e.digitAnchor.observe(n.Data)
		}
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.recurse(c, inBody)
	}
}

// enterDigitAnchor begins tracking a potential digit-only id/name anchor.
// Nested anchors are invalid HTML; the outer span governs.
func (e *extractor) enterDigitAnchor() {
	if e.digitAnchor == nil {
		e.digitAnchor = &digitAnchorSpan{eligible: true, startIdx: len(e.raw)}
	}
	e.digitAnchor.depth++
}

func (e *extractor) exitDigitAnchor() {
	sp := e.digitAnchor
	sp.depth--
	if sp.depth > 0 {
		return
	}
	e.digitAnchor = nil
	if sp.eligible && sp.hasDigit {
		for i := sp.startIdx; i < len(e.raw); i++ {
			e.raw[i].drop = anchorDigit
		}
	}
}

func (sp *digitAnchorSpan) observe(text string) {
	hasDigit := false
	for _, r := range text {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsSpace(r):
			// Marginalia padding.
		default:
			sp.eligible = false
			return
		}
	}
	if hasDigit {
		sp.hasDigit = true
	}
}

func (e *extractor) emitText(s, elem string) {
	fields := tokens.SplitWords(s)
	blockOpen := e.blockStart
	for _, tok := range fields {
		atBoundary := blockOpen || len(e.raw) == 0
		w := rawWord{Word: Word{
			Text:       tok,
			SpineIndex: e.spineIndex,
			Element:    elem,
			NonLinear:  e.nonLinear,
		}}
		if atBoundary {
			w.ParaStart = true
			blockOpen = false
		}
		if endsSentence(tok) {
			w.SentEnd = true
		}
		if v, ok := bareDigitValue(tok); ok && atBoundary {
			// All-digit token opening a block: line-number candidate.
			w.drop = bareDigit
			w.numValue = v
		}
		e.raw = append(e.raw, w)
	}
	// A whitespace-only stretch preserves the boundary for the next
	// real token; emitted words consume it.
	if len(fields) > 0 {
		e.blockStart = false
	}
}

// bareDigitValue reports whether tok is all (unicode) digits.
func bareDigitValue(tok string) (int, bool) {
	if tok == "" {
		return 0, false
	}
	v := 0
	digits := 0
	for _, r := range tok {
		if r < '0' || r > '9' {
			if !unicode.IsDigit(r) {
				return 0, false
			}
			// Non-ASCII digits: candidate, value approximate.
			if digits >= 9 {
				return 0, true
			}
			v = v*10 + int(r%10)
			digits++
			continue
		}
		if digits >= 9 {
			return v, true
		}
		v = v*10 + int(r-'0')
		digits++
	}
	if digits == 0 {
		return 0, false
	}
	return v, true
}

// runStep bounds the gaps a line-number run may skip (partially numbered
// verse) without merging unrelated sequences.
const runStep = 25

// minRun is the shortest qualifying increasing run. Shorter runs are kept:
// an isolated pair like "1939 … 1940" is more likely content than marginalia.
const minRun = 3

// finish resolves pruning: digit anchors always drop; bare candidates drop
// inside qualifying runs. Surviving words keep stable order, anchors remap,
// and boundary flags transfer forward across drops.
func (e *extractor) finish() {
	drop := make([]bool, len(e.raw))
	for i, w := range e.raw {
		if w.drop == anchorDigit {
			drop[i] = true
		}
	}
	// Bare-digit runs, per document.
	run := []int{}
	flush := func() {
		if len(run) >= minRun {
			for _, i := range run {
				drop[i] = true
			}
		}
		run = run[:0]
	}
	prev := -1
	for i, w := range e.raw {
		if w.drop != bareDigit || drop[i] {
			continue
		}
		if prev >= 0 && w.numValue > prev && w.numValue-prev <= runStep {
			run = append(run, i)
		} else {
			flush()
			run = append(run, i)
		}
		prev = w.numValue
	}
	flush()

	kept := make([]Word, 0, len(e.raw))
	// Total drops before each position, for anchor remap.
	droppedBefore := make([]int, len(e.raw)+1)
	for i := range e.raw {
		droppedBefore[i+1] = droppedBefore[i]
		if drop[i] {
			droppedBefore[i+1]++
		}
	}
	var carryPara, carrySent bool
	for i, w := range e.raw {
		if drop[i] {
			carryPara = carryPara || w.ParaStart
			carrySent = carrySent || w.SentEnd
			continue
		}
		w.ParaStart = w.ParaStart || carryPara
		w.SentEnd = w.SentEnd || carrySent
		carryPara, carrySent = false, false
		w.drop = keep
		kept = append(kept, w.Word)
	}
	// Remap anchors onto survivors; anchors past the final word point
	// nowhere in this document and are removed rather than leaking into
	// whatever follows in the book.
	for frag, rel := range e.anchorRel {
		if rel < 0 {
			delete(e.anchorRel, frag)
			continue
		}
		if rel > len(e.raw) {
			rel = len(e.raw)
		}
		mapped := rel - droppedBefore[rel]
		if mapped >= len(kept) {
			delete(e.anchorRel, frag)
			continue
		}
		e.anchorRel[frag] = mapped
	}
	e.words = kept
}

func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func isHeading(tag string) bool {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}

func endsSentence(tok string) bool {
	if tok == "" {
		return false
	}
	r := []rune(tok)
	last := r[len(r)-1]
	switch last {
	case '.', '!', '?', '…', '。', '！', '？':
		return true
	}
	return false
}
