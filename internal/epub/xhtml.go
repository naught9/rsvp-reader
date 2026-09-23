package epub

import (
	"strings"

	"golang.org/x/net/html"
)

// extractor walks an XHTML body, emitting words while recording the
// word index of every element id (source anchor position).
type extractor struct {
	spineIndex int
	nonLinear  bool
	words      []Word
	anchorRel  map[string]int // fragment -> relative word index

	curElement string
	blockStart bool
	prevSent   bool
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
					e.anchorRel[id] = len(e.words)
				}
			}
			// Anchor tags use name= as fragment target in older EPUBs.
			if tag == "a" {
				if nm := attrOf(n, "name"); nm != "" {
					if _, exists := e.anchorRel[nm]; !exists {
						e.anchorRel[nm] = len(e.words)
					}
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
			} else if tag == "h1" || tag == "h2" || tag == "h3" {
				e.curElement = tag
			}
			if isHeading(tag) {
				e.curElement = tag
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			e.recurse(c, inBody)
		}
		if inBody && blockElements[tag] {
			e.blockStart = true
		}
		return
	}
	if n.Type == html.TextNode && inBody {
		e.emitText(n.Data, e.curElement)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.recurse(c, inBody)
	}
}

func (e *extractor) emitText(s, elem string) {
	for _, tok := range strings.Fields(s) {
		w := Word{
			Text:       tok,
			SpineIndex: e.spineIndex,
			Element:    elem,
			NonLinear:  e.nonLinear,
		}
		if e.blockStart || len(e.words) == 0 {
			w.ParaStart = true
			e.blockStart = false
		}
		if endsSentence(tok) {
			w.SentEnd = true
		}
		e.words = append(e.words, w)
	}
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
