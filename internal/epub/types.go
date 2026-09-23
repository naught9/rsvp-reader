package epub

// Book is the structured parse result: reading order (spine),
// navigation (TOC), and the displayed word sequence.
type Book struct {
	Fingerprint string
	Title       string
	Creator     string
	Language    string
	Identifier  string

	OPFPath string
	Spine   []SpineItem
	TOC     []*TOCItem

	// TOCGenerated is true when the entries were synthesized from the
	// spine because the publisher navigation was missing or unusable.
	TOCGenerated bool
	// TOCWarning explains a broken/partial TOC; empty when TOC is authoritative.
	TOCWarning string

	Words []Word

	anchors   map[string]int // "archive/path" or "archive/path#frag" -> word index
	nonLinear map[string][]Word
	pathSpine map[string]int
}

// SpineItem is one spine entry in reading order.
type SpineItem struct {
	ID        string
	Path      string
	Linear    bool
	StartWord int // inclusive; -1 when the item contributed no words
	EndWord   int // exclusive
}

// TOCItem is one navigation entry. StartWord is nil when the entry
// cannot start playback (unlinked parent, unavailable target, or
// anchor with no readable words after it).
type TOCItem struct {
	Label       string
	TargetPath  string
	Fragment    string
	TargetHref  string
	StartWord   *int
	Children    []*TOCItem
	Unavailable string // non-empty when shown as unavailable/disabled
}

// Word is one displayed token.
type Word struct {
	Text       string
	SpineIndex int
	Element    string
	ParaStart  bool
	SentEnd    bool
	NonLinear  bool
}

// AnchorIndex returns the first word at or after path#fragment.
// ok=false when the target has no readable words afterward or is unknown.
func (b *Book) AnchorIndex(path, frag string) (idx int, ok bool) {
	if b == nil {
		return 0, false
	}
	key := path
	if frag != "" {
		key += "#" + frag
	}
	idx, ok = b.anchors[key]
	if !ok {
		return 0, false
	}
	if idx >= len(b.Words) {
		return 0, false
	}
	return idx, true
}

// SectionForWord returns the deepest TOC entry whose resolved start is
// at or before wordIdx in reading order. Returns nil when none resolves.
func (b *Book) SectionForWord(wordIdx int) *TOCItem {
	if b == nil || wordIdx < 0 {
		return nil
	}
	var best *TOCItem
	var bestStart = -1
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
	walk(b.TOC)
	return best
}

// SeekTOC resolves a TOC entry to its start word index.
func (b *Book) SeekTOC(it *TOCItem) (int, bool) {
	if b == nil || it == nil || it.StartWord == nil {
		return 0, false
	}
	if *it.StartWord < 0 || *it.StartWord >= len(b.Words) {
		return 0, false
	}
	return *it.StartWord, true
}
