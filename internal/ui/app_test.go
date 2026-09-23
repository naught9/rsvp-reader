package ui

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	fynetest "fyne.io/fyne/v2/test"

	"rsvp-reader/internal/doc"
	"rsvp-reader/internal/epub"
	"rsvp-reader/internal/store"
)

func writeTestEPUB(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>UI Test Book</dc:title><dc:creator>Tester</dc:creator></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`,
		"OEBPS/nav.xhtml":        `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol><li><a href="c1.xhtml">Start</a><ol><li><a href="c1.xhtml#mid">Middle</a></li></ol></li><li><a href="c2.xhtml">End</a></li></ol></nav></body></html>`,
		"OEBPS/c1.xhtml":         `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>one two three</p><h2 id="mid">Middle head</h2><p>four five</p></body></html>`,
		"OEBPS/c2.xhtml":         `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>six seven eight nine</p></body></html>`,
	}
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
	p := filepath.Join(t.TempDir(), "test.epub")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestApp(t *testing.T) (*App, string) {
	t.Helper()
	fyneApp := fynetest.NewApp()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	epubPath := writeTestEPUB(t)
	book, err := epub.OpenFile(epubPath)
	if err != nil {
		t.Fatal(err)
	}
	a := New(fyneApp, db, "")
	a.setDocument(doc.FromEPUB(book), epubPath)
	return a, epubPath
}

func TestTreeModelWalkableFromRoot(t *testing.T) {
	a, _ := newTestApp(t)
	// Replicate widget.Tree.walk: start at root "", descend only into branches.
	var visited []string
	var walk func(uid string)
	walk = func(uid string) {
		if a.isBranch(uid) {
			if uid != "" {
				visited = append(visited, uid)
			}
			for _, c := range a.childUIDs(uid) {
				walk(c)
			}
		} else if uid != "" {
			visited = append(visited, uid)
		}
	}
	walk("")
	// Test book TOC: Start(+Middle child) + End = 3 nodes.
	if len(visited) != 3 {
		t.Fatalf("tree walk visited %d nodes %v, want 3", len(visited), visited)
	}
	if got := a.childUIDs(""); len(got) != 2 {
		t.Fatalf("root children = %d, want 2", len(got))
	}
	if !a.isBranch("") || !a.isBranch("0") || a.isBranch("0/0") {
		t.Fatalf("branch flags wrong: root=%v 0=%v 0/0=%v",
			a.isBranch(""), a.isBranch("0"), a.isBranch("0/0"))
	}
}

func TestSetBookAndTOCSeek(t *testing.T) {
	a, _ := newTestApp(t)
	if a.player == nil || a.player.Len() != 11 {
		t.Fatalf("words = %v", a.player)
	}
	if len(a.topUIDs) != 2 {
		t.Fatalf("top TOC = %d, want 2", len(a.topUIDs))
	}
	// Nested entry resolves to the heading's first word.
	mid := a.uidToItem["0/0"]
	if mid == nil || mid.StartWord == nil {
		t.Fatalf("nested TOC unresolved")
	}
	a.onTOCSelected("0/0")
	if got := a.player.Current(); got != "Middle" {
		t.Fatalf("after TOC select current = %q, want Middle", got)
	}
	if a.player.Playing() {
		t.Fatalf("TOC selection must pause playback")
	}
	if got := a.sectionLabel.Text; got != "Section: Middle" {
		t.Fatalf("section label = %q", got)
	}
}

func TestPlayPauseStepAndSpeed(t *testing.T) {
	a, _ := newTestApp(t)
	a.togglePlay()
	if !a.player.Playing() {
		t.Fatalf("toggle did not start playback")
	}
	a.togglePlay()
	if a.player.Playing() || a.playBtn.Text != "Resume" {
		t.Fatalf("second toggle did not pause")
	}
	a.step(1)
	if a.player.Current() != "two" {
		t.Fatalf("step = %q, want two", a.player.Current())
	}
	before := a.player.WPM()
	a.bumpWPM(25)
	if a.player.WPM() != before+25 {
		t.Fatalf("bumpWPM = %d", a.player.WPM())
	}
}

func TestResumePersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	fyneApp := fynetest.NewApp()
	db, _ := store.Open(dir)
	epubPath := writeTestEPUB(t)
	book, _ := epub.OpenFile(epubPath)

	a1 := New(fyneApp, db, "")
	a1.setDocument(doc.FromEPUB(book), epubPath)
	a1.onTOCSelected("1") // "End" -> word "six"
	a1.bumpWPM(25)

	a2 := New(fyneApp, db, "")
	a2.setDocument(doc.FromEPUB(book), epubPath)
	if got := a2.player.Current(); got != "six" {
		t.Fatalf("reopen current = %q, want six", got)
	}
	if a2.player.WPM() == 300 {
		t.Fatalf("reopen did not restore saved speed")
	}
	if len(a2.db.ListRecent()) != 1 {
		t.Fatalf("recent list = %d, want 1", len(a2.db.ListRecent()))
	}
}

func TestEndedState(t *testing.T) {
	a, _ := newTestApp(t)
	last := a.player.Len() - 1
	a.player.Seek(last - 1)
	a.player.Play(time.Now())
	a.player.Tick(time.Now().Add(time.Hour)) // advances exactly one word
	if a.player.Current() != "nine" {
		t.Fatalf("current = %q, want last word", a.player.Current())
	}
	a.player.Tick(time.Now().Add(2 * time.Hour)) // stops on the last word
	if !a.player.Ended() {
		t.Fatalf("expected end of book")
	}
	a.onEnded()
	a.refreshAll()
	if a.playBtn.Text != "Restart" {
		t.Fatalf("end-of-book button = %q", a.playBtn.Text)
	}
}

func TestPasteTextSessionResumes(t *testing.T) {
	dir := t.TempDir()
	fyneApp := fynetest.NewApp()
	db, _ := store.Open(dir)
	a1 := New(fyneApp, db, "")
	a1.openText("alpha beta gamma delta epsilon")
	if a1.player.Len() != 5 {
		t.Fatalf("words = %d", a1.player.Len())
	}
	if got := a1.player.Section(); got != "Pasted text" {
		t.Fatalf("section = %q", got)
	}
	a1.player.Seek(3)
	a1.saveProgress("")

	a2 := New(fyneApp, db, "")
	a2.openText("alpha beta gamma delta epsilon")
	if got := a2.player.Current(); got != "delta" {
		t.Fatalf("reopen current = %q, want delta", got)
	}
}

func TestRecentTextReopensFromStore(t *testing.T) {
	dir := t.TempDir()
	fyneApp := fynetest.NewApp()
	db, _ := store.Open(dir)
	a1 := New(fyneApp, db, "")
	a1.openText("one two three")
	rec := db.ListRecent()
	if len(rec) != 1 || rec[0].Path != "" {
		t.Fatalf("recent = %+v", rec)
	}
	a2 := New(fyneApp, db, "")
	a2.openRecent(rec[0])
	if a2.player.Len() != 3 || a2.player.Current() != "one" {
		t.Fatalf("reopened text session broken")
	}
}

func TestImportFileRejectsUnknownType(t *testing.T) {
	if _, err := importFile("/tmp/notes.txt"); err == nil {
		t.Fatalf("unknown extension imported without error")
	}
}

func TestShowOpenDialogDoesNotCrash(t *testing.T) {
	// Resize-before-Show nil-derefs the inner dialog window.
	a, _ := newTestApp(t)
	a.showOpenDialog()
}

func TestJumpWordsChunk(t *testing.T) {
	a, _ := newTestApp(t) // 11 words
	a.togglePlay()
	a.jumpWords(25) // overshoots: clamps to the end
	if a.player.Current() != "nine" {
		t.Fatalf("jump forward = %q, want last word", a.player.Current())
	}
	if a.player.Playing() {
		t.Fatalf("jump must pause")
	}
	if a.playBtn.Text != "Restart" {
		t.Fatalf("jump to end button = %q, want Restart", a.playBtn.Text)
	}
	a.jumpWords(-25) // undershoots: clamps to the start
	if a.player.Current() != "one" {
		t.Fatalf("jump back = %q, want first word", a.player.Current())
	}
	if a.playBtn.Text != "Resume" {
		t.Fatalf("jump mid-book button = %q, want Resume", a.playBtn.Text)
	}
	a.cancelTick()
}

func TestOnJumpChunk(t *testing.T) {
	a, _ := newTestApp(t) // 11 words
	a.onJump(1)           // jumps to the end, paused
	if a.player.Current() != "nine" {
		t.Fatalf("option+right = %q, want last word", a.player.Current())
	}
	if a.player.Playing() {
		t.Fatalf("jump must pause")
	}
	a.cancelTick()
	a.onJump(-1) // jumps to the start
	if a.player.Current() != "one" {
		t.Fatalf("option+left = %q, want first word", a.player.Current())
	}
	a.cancelTick()
}

func TestSentencePausePref(t *testing.T) {
	a, path := newTestApp(t)
	if !a.player.SentencePause {
		t.Fatalf("sentence pause should default on")
	}
	a.prefs().SetBool("sentencePause", false)
	a.setDocument(a.book, path)
	if a.player.SentencePause {
		t.Fatalf("player should follow the pref")
	}
	a.cancelTick()
}

func TestPanesRememberedDefaultClosed(t *testing.T) {
	a, _ := newTestApp(t)
	if a.contentsOn || a.contextOn {
		t.Fatalf("panes should default closed (contents=%v context=%v)", a.contentsOn, a.contextOn)
	}
	a.toggleContents()
	if !a.prefs().BoolWithFallback("contentsPane", false) {
		t.Fatalf("contents choice not persisted")
	}
	a.toggleContents()
	if a.prefs().BoolWithFallback("contentsPane", true) {
		t.Fatalf("contents close not persisted")
	}
	a.cancelTick()
}
