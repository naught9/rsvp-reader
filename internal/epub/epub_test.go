package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// buildEPUB assembles a minimal EPUB zip from a map of archive path -> content.
func buildEPUB(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func openBytes(t *testing.T, data []byte) *Book {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := openZipper(zr, "test-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const containerXML = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

// TestEPUB3SameFileFragments: two TOC entries target different IDs in one
// XHTML file; choosing the second must start at its heading's first word.
func TestEPUB3SameFileFragments(t *testing.T) {
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Frag Test</dc:title><dc:creator>Tester</dc:creator><dc:language>en</dc:language></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/><item id="ch2" href="ch2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="ch1"/><itemref idref="ch2"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="ch1.xhtml">Chapter One</a></li><li><a href="ch2.xhtml#sec-a">Section A</a></li><li><a href="ch2.xhtml#sec-b">Section B</a></li></ol></nav></body></html>`,
		"OEBPS/ch1.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><h1>Chapter One</h1><p>Alpha beta gamma.</p></body></html>`,
		"OEBPS/ch2.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="sec-a">Section A head</h1><p>Delta epsilon.</p><h2 id="sec-b">Section B head</h2><p>Zeta eta theta.</p></body></html>`,
	})
	b := openBytes(t, data)
	if b.Title != "Frag Test" || b.Creator != "Tester" {
		t.Fatalf("metadata: title=%q creator=%q", b.Title, b.Creator)
	}
	if len(b.TOC) != 3 {
		t.Fatalf("top-level TOC entries = %d, want 3", len(b.TOC))
	}
	secB := b.TOC[2]
	if secB.StartWord == nil {
		t.Fatalf("Section B did not resolve: unav=%q", secB.Unavailable)
	}
	if got := b.Words[*secB.StartWord].Text; got != "Section" {
		t.Fatalf("Section B starts at %q, want heading first word %q", got, "Section")
	}
	// Section A must resolve to an earlier, distinct position.
	secA := b.TOC[1]
	if secA.StartWord == nil || *secA.StartWord >= *secB.StartWord {
		t.Fatalf("Section A start=%v Section B start=%v, want A<B", secA.StartWord, secB.StartWord)
	}
	// Reading continues in spine order: words after Section B head.
	found := false
	for i := *secB.StartWord; i < len(b.Words); i++ {
		if b.Words[i].Text == "Zeta" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("words after Section B do not continue in spine order")
	}
	// Section label derivation mid-book.
	if s := b.SectionForWord(*secB.StartWord + 1); s == nil || s.Label != "Section B" {
		t.Fatalf("SectionForWord mid-B = %v", s)
	}
}

// TestEPUB2NCXNested: nested navPoints with relative paths and unlinked parent.
func TestEPUB2NCXNested(t *testing.T) {
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>NCX Test</dc:title></metadata><manifest><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="c1" href="text/c1.html" media-type="application/xhtml+xml"/><item id="c2" href="text/c2.html" media-type="application/xhtml+xml"/></manifest><spine toc="ncx"><itemref idref="c1"/><itemref idref="c2"/></spine></package>`,
		"OEBPS/toc.ncx":          `<?xml version="1.0"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>NCX Test</text></docTitle><navMap><navPoint id="p1" playOrder="1"><navLabel><text>Part One</text></navLabel><content src="text/c1.html"/><navPoint id="p1a" playOrder="2"><navLabel><text>Chapter 1</text></navLabel><content src="text/c1.html#ch1"/></navPoint></navPoint><navPoint id="p2" playOrder="3"><navLabel><text>Group</text></navLabel><navPoint id="p2a" playOrder="4"><navLabel><text>Chapter 2</text></navLabel><content src="text/c2.html"/></navPoint></navPoint></navMap></ncx>`,
		"OEBPS/text/c1.html":     `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><h1 id="ch1">Chapter One text</h1><p>one two</p></body></html>`,
		"OEBPS/text/c2.html":     `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>three four</p></body></html>`,
	})
	b := openBytes(t, data)
	if len(b.TOC) != 2 {
		t.Fatalf("top-level TOC = %d, want 2", len(b.TOC))
	}
	// Unlinked parent "Group" stays visible but cannot start playback.
	group := b.TOC[1]
	if group.StartWord != nil {
		t.Fatalf("unlinked parent resolved to %d, want nil", *group.StartWord)
	}
	if len(group.Children) != 1 {
		t.Fatalf("group children = %d, want 1", len(group.Children))
	}
	ch2 := group.Children[0]
	if ch2.StartWord == nil {
		t.Fatalf("Chapter 2 unresolved: %q", ch2.Unavailable)
	}
	// Relative href with fragment resolves inside c1.
	ch1 := b.TOC[0].Children[0]
	if ch1.StartWord == nil {
		t.Fatalf("Chapter 1 unresolved: %q", ch1.Unavailable)
	}
	if got := b.Words[*ch1.StartWord].Text; got != "Chapter" {
		t.Fatalf("Chapter 1 starts at %q, want %q", got, "Chapter")
	}
}

// TestSpineOrderBeatsFilenames: files whose names sort differently from
// the spine must play in spine order.
func TestSpineOrderBeatsFilenames(t *testing.T) {
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Order</dc:title></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="z" href="z-last.html" media-type="application/xhtml+xml"/><item id="a" href="a-first.html" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="a"/><itemref idref="z"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="a-first.html">First</a></li><li><a href="z-last.html">Last</a></li></ol></nav></body></html>`,
		"OEBPS/z-last.html":      `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>zee words here</p></body></html>`,
		"OEBPS/a-first.html":     `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>aye words here</p></body></html>`,
	})
	b := openBytes(t, data)
	if len(b.Words) < 6 {
		t.Fatalf("words = %d, want >= 6", len(b.Words))
	}
	if b.Words[0].Text != "aye" || b.Words[3].Text != "zee" {
		t.Fatalf("spine order violated: %q %q %q %q", b.Words[0].Text, b.Words[1].Text, b.Words[3].Text, b.Words[4].Text)
	}
}

// TestBrokenTOCStillReadable: readable spine + broken TOC stays readable
// with an explanation, and does not present generated entries as the
// publisher's TOC.
func TestBrokenTOCStillReadable(t *testing.T) {
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Broken</dc:title></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="c" href="c.html" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>No nav element here at all.</p></body></html>`,
		"OEBPS/c.html":           `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>readable body text remains</p></body></html>`,
	})
	b := openBytes(t, data)
	if len(b.Words) != 4 {
		t.Fatalf("words = %d, want 4", len(b.Words))
	}
	if !b.TOCGenerated {
		t.Fatalf("expected generated sections fallback")
	}
	if b.TOCWarning == "" {
		t.Fatalf("expected TOC warning explanation")
	}
}

// TestMissingAnchorDisabled: an anchor that resolves to a valid document
// but has no readable words afterward disables the entry.
func TestMissingAnchorDisabled(t *testing.T) {
	opf := `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Anchor</dc:title></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="c" href="c.html" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c"/></spine></package>`
	nav := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="c.html#ghost">Ghost</a></li><li><a href="c.html">Whole</a></li></ol></nav></body></html>`
	doc := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>hello world</p></body></html>`
	data := buildEPUB(t, map[string]string{
		"mimetype": "application/epub+zip", "META-INF/container.xml": containerXML,
		"OEBPS/content.opf": opf, "OEBPS/nav.xhtml": nav, "OEBPS/c.html": doc,
	})
	b := openBytes(t, data)
	ghost := b.TOC[0]
	if ghost.StartWord != nil || ghost.Unavailable == "" {
		t.Fatalf("ghost anchor: start=%v unav=%q, want disabled with reason", ghost.StartWord, ghost.Unavailable)
	}
	if b.TOC[1].StartWord == nil {
		t.Fatalf("whole-file entry should resolve")
	}
}

// TestNonLinearExcluded: non-linear spine items stay out of sequential
// playback.
func TestNonLinearExcluded(t *testing.T) {
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>NL</dc:title></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="cover" href="cover.html" media-type="application/xhtml+xml"/><item id="c" href="c.html" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="cover" linear="no"/><itemref idref="c"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="c.html">Body</a></li></ol></nav></body></html>`,
		"OEBPS/cover.html":       `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>cover image page</p></body></html>`,
		"OEBPS/c.html":           `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>main text here</p></body></html>`,
	})
	b := openBytes(t, data)
	for _, w := range b.Words {
		if w.Text == "cover" {
			t.Fatalf("non-linear cover words leaked into sequential stream")
		}
	}
	if b.Words[0].Text != "main" {
		t.Fatalf("first word = %q, want %q", b.Words[0].Text, "main")
	}
}

func TestRealBooksSpotCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-book check in short mode")
	}
	for _, f := range []string{"../../epubs/10540559.epub", "../../epubs/7986754.epub"} {
		b, err := OpenFile(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(b.Words) < 1000 || len(b.TOC) == 0 {
			t.Fatalf("%s: words=%d toc=%d", f, len(b.Words), len(b.TOC))
		}
		if b.Title == "" {
			t.Fatalf("%s: empty title", f)
		}
		fmt.Printf("%s: %d words, %d top-level TOC\n", f, len(b.Words), len(b.TOC))
	}
}

// TestLineNumberPruning covers the Odyssey pattern: digit id-anchors,
// bare sequential digits, and the content that must survive (years,
// chapter numbers, numbered lists, short runs).
func TestLineNumberPruning(t *testing.T) {
	verse := `<p class="v"><span><a id="l1">1</a> Sing to me of the man</span></p>` +
		`<p class="v"><span><a id="l2">2</a> driven off course</span></p>` +
		`<p class="v"><span><a id="l3">3</a> the hallowed heights</span></p>` +
		`<p class="v"><span>4 many cities he saw</span></p>` +
		`<p class="v"><span>5 many pains he suffered</span></p>` +
		`<p class="v"><span>6 fighting to save his life</span></p>` +
		`<p class="v"><span>In 1945 the fleet returned</span></p>` +
		`<h2><span>Chapter 1</span></h2>` +
		`<p class="v"><span>7 the wine-dark sea</span></p>` +
		`<p class="v"><span>8 and back again</span></p>`
	lists := `<ol><li>Preheat the oven</li><li>Mix the flour</li></ol>`
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Prune</dc:title></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="v" href="v.xhtml" media-type="application/xhtml+xml"/><item id="l" href="l.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="v"/><itemref idref="l"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="v.xhtml#l2">Second line</a></li><li><a href="l.xhtml">Lists</a></li></ol></nav></body></html>`,
		"OEBPS/v.xhtml":          `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body>` + verse + `</body></html>`,
		"OEBPS/l.xhtml":          `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body>` + lists + `</body></html>`,
	})
	b := openBytes(t, data)
	var texts []string
	for _, w := range b.Words {
		texts = append(texts, w.Text)
	}
	want := []string{"Sing", "to", "me", "of", "the", "man",
		"driven", "off", "course", "the", "hallowed", "heights",
		"many", "cities", "he", "saw", "many", "pains", "he", "suffered",
		"fighting", "to", "save", "his", "life",
		"In", "1945", "the", "fleet", "returned",
		"Chapter", "1",
		"the", "wine-", "dark", "sea", "and", "back", "again",
		"Preheat", "the", "oven", "Mix", "the", "flour"}
	if len(texts) != len(want) {
		t.Fatalf("words = %q, want %q", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Fatalf("words = %q, want %q", texts, want)
		}
	}
	// The digit anchor still navigates: "Second line" starts at "driven".
	if b.TOC[0].StartWord == nil {
		t.Fatalf("TOC past digit anchor lost: %q", b.TOC[0].Unavailable)
	}
	if got := b.Words[*b.TOC[0].StartWord].Text; got != "driven" {
		t.Fatalf("anchor resolves to %q, want driven", got)
	}
	// Paragraph boundary transfers across the dropped number.
	for i, w := range b.Words {
		if w.Text == "Sing" && !w.ParaStart {
			t.Fatalf("word after dropped number lost its paragraph boundary (idx %d)", i)
		}
	}
}

// TestShortDigitRunsKept ensures isolated pairs never prune.
func TestShortDigitRunsKept(t *testing.T) {
	data := buildEPUB(t, map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": containerXML,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Pairs</dc:title></metadata><manifest><item id="c" href="c.html" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c"/></spine></package>`,
		"OEBPS/c.html":           `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>7 samurai stood</p><p>8 banners flew</p><p>In 1939 tensions rose</p></body></html>`,
	})
	b := openBytes(t, data)
	var texts []string
	for _, w := range b.Words {
		texts = append(texts, w.Text)
	}
	joined := strings.Join(texts, " ")
	for _, keep := range []string{"7", "8", "1939"} {
		if !strings.Contains(joined, keep) {
			t.Fatalf("isolated number %q wrongly pruned from %q", keep, joined)
		}
	}
}
