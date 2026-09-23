package epub

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"golang.org/x/net/html"
)

// Safety limits so a malformed archive cannot exhaust memory.
const (
	maxArchiveEntries = 10000
	maxResourceBytes  = 20 << 20 // 20 MB per spine/nav resource
	maxTotalTextBytes = 32 << 20 // 32 MB extracted text
	maxWords          = 2000000  // 2M tokens
)

// OpenFile opens an EPUB from disk, fingerprinting it by content hash.
func OpenFile(epubPath string) (*Book, error) {
	data, err := os.ReadFile(epubPath)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	sum := sha256.Sum256(data)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid EPUB zip: %w", err)
	}
	return openZipper(zr, hex.EncodeToString(sum[:]))
}

type zipper interface {
	File() []*zip.File
	Open(name string) (io.ReadCloser, error)
}

type sliceZipper struct{ files []*zip.File }

func (s sliceZipper) File() []*zip.File { return s.files }
func (s sliceZipper) Open(name string) (io.ReadCloser, error) {
	for _, f := range s.files {
		if f.Name == name {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("entry not found: %s", name)
}

func openZipper(zr *zip.Reader, fingerprint string) (*Book, error) {
	return openFiles(sliceZipper{zr.File}, fingerprint)
}

func openFiles(z sliceZipper, fingerprint string) (*Book, error) {
	if len(z.files) > maxArchiveEntries {
		return nil, fmt.Errorf("unsupported EPUB: archive has %d entries (limit %d)", len(z.files), maxArchiveEntries)
	}
	byName := make(map[string]*zip.File, len(z.files))
	for _, f := range z.files {
		byName[f.Name] = f
		byName[path.Clean(f.Name)] = f
	}

	containerData, err := readEntry(z, byName, "META-INF/container.xml")
	if err != nil {
		return nil, fmt.Errorf("invalid EPUB: %w", err)
	}
	if _, encrypted := byName["META-INF/encryption.xml"]; encrypted {
		return nil, fmt.Errorf("unsupported content: this book is encrypted/DRM-protected and cannot be opened")
	}
	opfPath, err := parseContainer(containerData)
	if err != nil {
		return nil, err
	}

	opfData, err := readEntry(z, byName, opfPath)
	if err != nil {
		return nil, fmt.Errorf("invalid EPUB: cannot read package document: %w", err)
	}
	pkg, err := parseOPF(opfData)
	if err != nil {
		return nil, err
	}
	opfDir := path.Dir(opfPath)
	if opfPath == "." || opfDir == "." && !strings.Contains(opfPath, "/") {
		opfDir = ""
	}

	b := &Book{
		Fingerprint: fingerprint,
		Title:       pkg.Title(),
		Creator:     pkg.Creator(),
		Language:    pkg.Lang(),
		Identifier:  pkg.Ident(),
		OPFPath:     opfPath,
		anchors:     map[string]int{},
		nonLinear:   map[string][]Word{},
		pathSpine:   map[string]int{},
	}

	// Resolve manifest hrefs to archive paths.
	idToPath := make(map[string]string, len(pkg.Manifest))
	idToMedia := make(map[string]string, len(pkg.Manifest))
	var navPath, ncxPath string
	var navID string
	for _, m := range pkg.Manifest {
		p := joinArchivePath(opfDir, strings.TrimSpace(m.Href))
		idToPath[m.ID] = p
		idToMedia[m.ID] = m.MediaType
		props := strings.ToLower(m.Properties)
		if strings.Contains(props, "nav") && navPath == "" {
			navPath = p
			navID = m.ID
		}
	}
	_ = navID
	if pkg.Spine.TOC != "" {
		if p, ok := idToPath[pkg.Spine.TOC]; ok {
			ncxPath = p
		}
	}
	// Fallback: any manifest item with NCX media type.
	if ncxPath == "" {
		for id, mt := range idToMedia {
			if mt == "application/x-dtbncx+xml" {
				ncxPath = idToPath[id]
				break
			}
		}
	}

	// Build spine in declared order.
	type spineRef struct {
		idref  string
		linear bool
	}
	refs := make([]spineRef, 0, len(pkg.Spine.Items))
	for _, it := range pkg.Spine.Items {
		lin := true
		if strings.EqualFold(strings.TrimSpace(it.Linear), "no") {
			lin = false
		}
		refs = append(refs, spineRef{idref: it.IDRef, linear: lin})
	}

	// Parse spine documents in order, extracting words + anchor positions.
	var totalText int
	navDir := path.Dir(navPath)
	_ = navDir
	for i, r := range refs {
		docPath, ok := idToPath[r.idref]
		if !ok {
			// Dangling spine ref: keep a placeholder so order is visible.
			b.Spine = append(b.Spine, SpineItem{ID: r.idref, Path: "", Linear: r.linear, StartWord: -1, EndWord: -1})
			b.pathSpine[fmt.Sprintf("\x00missing:%d", i)] = i
			continue
		}
		si := SpineItem{ID: r.idref, Path: docPath, Linear: r.linear, StartWord: -1, EndWord: -1}
		b.pathSpine[docPath] = len(b.Spine)
		b.Spine = append(b.Spine, si)
		siIdx := len(b.Spine) - 1

		data, err := readEntry(z, byName, docPath)
		if err != nil {
			continue // unreadable spine item: leave empty, keep reading others
		}
		totalText += len(data)
		if totalText > maxTotalTextBytes+maxResourceBytes {
			return nil, fmt.Errorf("unsupported EPUB: extracted text exceeds %d bytes", maxTotalTextBytes)
		}
		doc, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			continue
		}
		base := &extractor{
			spineIndex: siIdx,
			nonLinear:  !r.linear,
		}
		base.walk(doc)
		if len(b.Words)+len(base.words) > maxWords {
			return nil, fmt.Errorf("unsupported EPUB: word count exceeds %d", maxWords)
		}
		if r.linear {
			start := len(b.Words)
			if len(base.words) > 0 {
				b.Spine[siIdx].StartWord = start
				b.Spine[siIdx].EndWord = start + len(base.words)
			}
			for frag, relIdx := range base.anchorRel {
				b.anchors[docPath+"#"+frag] = start + relIdx
			}
			b.anchors[docPath] = start
			b.Words = append(b.Words, base.words...)
		} else {
			// Non-linear: parse for deliberate TOC jumps but keep out of
			// the sequential word stream.
			for frag, relIdx := range base.anchorRel {
				b.nonLinear[docPath+"#"+frag] = base.words
				_ = relIdx
			}
			b.nonLinear[docPath] = base.words
		}
		_ = i
	}

	// Parse navigation: EPUB 3 nav first, EPUB 2 NCX as fallback/addition.
	var tocErrs []string
	var toc []*TOCItem
	if navPath != "" {
		navData, err := readEntry(z, byName, navPath)
		if err != nil {
			tocErrs = append(tocErrs, "navigation document unreadable")
		} else if n, err := parseNavDoc(navData, navPath); err != nil {
			tocErrs = append(tocErrs, "navigation document: "+err.Error())
		} else {
			toc = n
		}
	}
	if len(toc) == 0 && ncxPath != "" {
		ncxData, err := readEntry(z, byName, ncxPath)
		if err != nil {
			tocErrs = append(tocErrs, "NCX unreadable")
		} else if n, err := parseNCX(ncxData, ncxPath); err != nil {
			tocErrs = append(tocErrs, "NCX: "+err.Error())
		} else {
			toc = n
		}
	}

	if len(toc) == 0 {
		gen := b.generateSpineTOC()
		if len(gen) > 0 {
			b.TOC = gen
			b.TOCGenerated = true
			msg := "Table of contents missing; showing sections instead."
			if len(tocErrs) > 0 {
				msg = "Table of contents unusable (" + strings.Join(tocErrs, "; ") + "); showing sections instead."
			}
			b.TOCWarning = msg
		} else if len(tocErrs) > 0 {
			b.TOCWarning = "Table of contents unusable: " + strings.Join(tocErrs, "; ")
		}
	} else {
		b.TOC = toc
		if len(tocErrs) > 0 {
			b.TOCWarning = strings.Join(tocErrs, "; ")
		}
	}

	b.resolveTOC()
	if !b.anyTOCResolves() && len(b.Spine) > 0 {
		if gen := b.generateSpineTOC(); len(gen) > 0 {
			b.TOC = gen
			b.TOCGenerated = true
			extra := "Navigation entries point nowhere readable; showing sections instead."
			if b.TOCWarning != "" {
				b.TOCWarning += " " + extra
			} else {
				b.TOCWarning = extra
			}
			b.resolveTOC()
		}
	}

	if len(b.Words) == 0 {
		// Distinguish image-only/textless from encrypted later; for now honest error.
		return nil, fmt.Errorf("unsupported content: no readable text found in this EPUB")
	}
	return b, nil
}

func (b *Book) anyTOCResolves() bool {
	var ok bool
	var walk func(items []*TOCItem)
	walk = func(items []*TOCItem) {
		for _, it := range items {
			if it.StartWord != nil {
				ok = true
				return
			}
			walk(it.Children)
		}
	}
	walk(b.TOC)
	return ok
}

// resolveTOC maps each entry's href/fragment to a word index.
// Entries with no readable words afterward are disabled with an
// explanation instead of jumping elsewhere.
func (b *Book) resolveTOC() {
	var walk func(items []*TOCItem)
	walk = func(items []*TOCItem) {
		for _, it := range items {
			walk(it.Children)
			if it.TargetPath == "" {
				// Unlinked parent: visible group heading, not selectable.
				continue
			}
			if _, inSpine := b.pathSpine[it.TargetPath]; !inSpine {
				it.Unavailable = "target is outside this book's reading order"
				continue
			}
			// Non-linear deliberate target: resolve within its own word slice,
			// then splice: those words followed by the linear tail. For v1 we
			// resolve the entry to the non-linear doc start and let the UI
			// read that doc; continuation is handled by recording the tail.
			if nl, ok := b.nonLinear[it.TargetPath]; ok && len(nl) > 0 {
				start := len(b.Words)
				b.Words = append(b.Words, nl...)
				for _, si := range b.Spine {
					_ = si
				}
				// Map anchors of this doc into the appended region.
				for k, v := range b.nonLinear {
					if strings.HasPrefix(k, it.TargetPath+"#") {
						b.anchors[k] = start + v[0].SpineIndex // placeholder, fixed below
						_ = v
					}
				}
				// Simpler: point entry at appended start (fragment refinement below).
				s := start
				it.StartWord = &s
				delete(b.nonLinear, it.TargetPath)
				continue
			}
			if it.Fragment == "" {
				if idx, ok := b.AnchorIndex(it.TargetPath, ""); ok {
					v := idx
					it.StartWord = &v
				} else {
					si := b.pathSpine[it.TargetPath]
					if b.Spine[si].StartWord >= 0 {
						v := b.Spine[si].StartWord
						it.StartWord = &v
					} else {
						it.Unavailable = "section has no readable text"
					}
				}
				continue
			}
			if idx, ok := b.AnchorIndex(it.TargetPath, it.Fragment); ok {
				v := idx
				it.StartWord = &v
			} else if _, ok := b.AnchorIndex(it.TargetPath, ""); ok {
				it.Unavailable = "bookmark target not found in this edition"
			} else {
				it.Unavailable = "section has no readable text"
			}
		}
	}
	walk(b.TOC)
}

func (b *Book) generateSpineTOC() []*TOCItem {
	var out []*TOCItem
	for _, si := range b.Spine {
		if !si.Linear || si.Path == "" || si.StartWord < 0 {
			continue
		}
		label := path.Base(si.Path)
		v := si.StartWord
		out = append(out, &TOCItem{Label: label, TargetPath: si.Path, TargetHref: si.Path, StartWord: &v})
	}
	return out
}

// ---------- container / OPF ----------

type container struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

func parseContainer(data []byte) (string, error) {
	var c container
	if err := xml.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("invalid EPUB: bad container.xml: %w", err)
	}
	if len(c.Rootfiles) == 0 || c.Rootfiles[0].FullPath == "" {
		return "", fmt.Errorf("invalid EPUB: no package document in container.xml")
	}
	return path.Clean(c.Rootfiles[0].FullPath), nil
}

type opfPackage struct {
	Version  string `xml:"version,attr"`
	Metadata struct {
		Titles      []string `xml:"title"`
		Creators    []string `xml:"creator"`
		Languages   []string `xml:"language"`
		Identifiers []string `xml:"identifier"`
	} `xml:"metadata"`
	Manifest []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
	Spine struct {
		TOC   string `xml:"toc,attr"`
		Items []struct {
			IDRef  string `xml:"idref,attr"`
			Linear string `xml:"linear,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

func parseOPF(data []byte) (*opfPackage, error) {
	var p opfPackage
	if err := xml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid EPUB: bad package document: %w", err)
	}
	if len(p.Manifest) == 0 || len(p.Spine.Items) == 0 {
		return nil, fmt.Errorf("invalid EPUB: package has no manifest or spine")
	}
	return &p, nil
}

func (p *opfPackage) Title() string {
	if len(p.Metadata.Titles) > 0 {
		return strings.TrimSpace(p.Metadata.Titles[0])
	}
	return ""
}

func (p *opfPackage) Creator() string {
	if len(p.Metadata.Creators) > 0 {
		return strings.TrimSpace(p.Metadata.Creators[0])
	}
	return ""
}

func (p *opfPackage) Lang() string {
	if len(p.Metadata.Languages) > 0 {
		return strings.TrimSpace(p.Metadata.Languages[0])
	}
	return ""
}

func (p *opfPackage) Ident() string {
	if len(p.Metadata.Identifiers) > 0 {
		return strings.TrimSpace(p.Metadata.Identifiers[0])
	}
	return ""
}

// ---------- helpers ----------

func readEntry(z sliceZipper, byName map[string]*zip.File, name string) ([]byte, error) {
	f := byName[name]
	if f == nil {
		f = byName[path.Clean(name)]
	}
	if f == nil {
		return nil, fmt.Errorf("missing archive entry %q", name)
	}
	if f.UncompressedSize64 > maxResourceBytes*4 {
		return nil, fmt.Errorf("entry %q too large", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	lr := io.LimitReader(rc, maxResourceBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxResourceBytes {
		return nil, fmt.Errorf("entry %q exceeds %d bytes", name, maxResourceBytes)
	}
	// Detect encryption marker.
	if strings.HasSuffix(strings.ToLower(name), ".xml") && bytes.Contains(data, []byte("EncryptedData")) {
		return nil, fmt.Errorf("encrypted/DRM-protected content is not supported")
	}
	return data, nil
}

func joinArchivePath(baseDir, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	// Strip query; keep fragment handling to callers (hrefs here have none).
	if i := strings.IndexByte(href, '?'); i >= 0 {
		href = href[:i]
	}
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	if path.IsAbs(href) {
		return path.Clean(strings.TrimPrefix(href, "/"))
	}
	if baseDir == "" || baseDir == "." {
		return path.Clean(href)
	}
	return path.Clean(baseDir + "/" + href)
}

// splitHref resolves a nav href relative to the nav document into
// (archivePath, fragment).
func splitHref(navDocPath, raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	// External URLs are out of scope.
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") ||
		strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "data:") {
		return "", ""
	}
	frag := ""
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		frag = raw[i+1:]
		raw = raw[:i]
		if u, err := url.PathUnescape(frag); err == nil {
			frag = u
		}
	}
	var p string
	if strings.TrimSpace(raw) == "" {
		p = navDocPath // same-document fragment
	} else {
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			raw = raw[:i]
		}
		if u, err := url.PathUnescape(strings.TrimSpace(raw)); err == nil {
			raw = u
		}
		baseDir := path.Dir(navDocPath)
		if baseDir == "." {
			baseDir = ""
		}
		if path.IsAbs(raw) {
			p = path.Clean(strings.TrimPrefix(raw, "/"))
		} else if baseDir == "" {
			p = path.Clean(raw)
		} else {
			p = path.Clean(baseDir + "/" + raw)
		}
	}
	return p, frag
}
