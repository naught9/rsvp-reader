package doc

import (
	"rsvp-reader/internal/epub"
)

// FromEPUB adapts the structured EPUB parse result to a Document.
func FromEPUB(b *epub.Book) *Document {
	d := &Document{
		Fingerprint:  b.Fingerprint,
		Kind:         KindEPUB,
		Title:        b.Title,
		Creator:      b.Creator,
		TOCGenerated: b.TOCGenerated,
		TOCWarning:   b.TOCWarning,
	}
	d.Words = make([]string, len(b.Words))
	for i, w := range b.Words {
		d.Words[i] = w.Text
	}
	d.TOC = convertTOC(b.TOC)
	return d
}

func convertTOC(items []*epub.TOCItem) []*TOCItem {
	out := make([]*TOCItem, 0, len(items))
	for _, it := range items {
		c := &TOCItem{
			Label:       it.Label,
			StartWord:   it.StartWord,
			Unavailable: it.Unavailable,
			Ref:         it.TargetHref,
		}
		if c.Ref == "" {
			c.Ref = it.TargetPath
			if it.Fragment != "" {
				c.Ref += "#" + it.Fragment
			}
		}
		c.Children = convertTOC(it.Children)
		out = append(out, c)
	}
	return out
}
