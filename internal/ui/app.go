package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"rsvp-reader/internal/doc"
	"rsvp-reader/internal/epub"
	"rsvp-reader/internal/pdf"
	"rsvp-reader/internal/reader"
	"rsvp-reader/internal/store"
	"rsvp-reader/internal/wake"
)

// App wires the book model, playback state, and Fyne widgets.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	db      *store.Store

	book   *doc.Document
	player *reader.Player
	cliArg string

	uidToItem map[string]*doc.TOCItem
	itemToUID map[*doc.TOCItem]string
	topUIDs   []string

	titleLabel    *widget.Label
	tree          *widget.Tree
	orp           *ORPWidget
	playBtn       *PillButton
	prevBtn       *PillButton
	nextBtn       *PillButton
	wpmSlider     *SlimSlider
	wpmEntry      *widget.Entry
	progress      *SlimProgress
	progressLabel *widget.Label
	sectionLabel  *widget.Label
	statusLabel   *widget.Label
	emptyScreen   fyne.CanvasObject
	readerScreen  fyne.CanvasObject
	recentBox     *fyne.Container
	loadingBar    *widget.ProgressBarInfinite
	loadingLabel  *widget.Label
	loadingScreen fyne.CanvasObject
	root          *fyne.Container
	topWrap       *fyne.Container
	bottomWrap    *fyne.Container
	detector      *activityDetector
	hideTimer     *time.Timer
	chromeHidden  bool

	readerFont  fyne.Resource
	fontMu      sync.RWMutex
	fontList    []SystemFont
	fontScanned chan struct{}

	tickTimer       *time.Timer
	importGen       int
	syncingTree     bool
	contentsOn      bool
	contextOn       bool
	contextRich     *widget.RichText
	contextHead     *canvas.Text
	contextHeadText string
	contextScroll   *container.Scroll
	contextPane     fyne.CanvasObject
	ctxLines        []ctxLine
	ctxLo, ctxHi    int
	ctxWidth        float32
	ctxLine         int
	lastSave        time.Time
}

// New builds the application. cliArg is an optional EPUB path.
func New(fyneApp fyne.App, db *store.Store, cliArg string) *App {
	a := &App{fyneApp: fyneApp, db: db, cliArg: cliArg, contentsOn: true, fontScanned: make(chan struct{})}
	a.contextOn = a.prefs().BoolWithFallback("contextPane", false)
	a.win = fyneApp.NewWindow("RSVP Reader")
	a.loadSavedFont()
	a.applyThemePref()
	a.buildWidgets()
	a.buildLayout()
	a.buildMenu()
	a.buildShortcuts()
	a.win.SetCloseIntercept(a.onClose)
	a.win.Resize(fyne.NewSize(900, 650))
	go a.scanFonts()
	return a
}

// Show displays the window and opens the CLI book, if any.
func (a *App) Show() {
	a.win.Show()
	if a.cliArg != "" {
		a.openFile(a.cliArg)
	}
}

// ShowAndRun displays the window and runs the event loop.
func (a *App) ShowAndRun() {
	if a.cliArg != "" {
		a.openFile(a.cliArg)
	}
	a.win.ShowAndRun()
}

func (a *App) prefs() fyne.Preferences { return a.fyneApp.Preferences() }

func (a *App) prefWPM() int        { return a.prefs().IntWithFallback("wpm", reader.DefaultWPM) }
func (a *App) prefHighlight() bool { return a.prefs().BoolWithFallback("highlight", true) }
func (a *App) prefFontSize() float64 {
	return a.prefs().FloatWithFallback("fontSize", 64)
}

// ---------- construction ----------

func (a *App) buildWidgets() {
	a.titleLabel = widget.NewLabel("RSVP Reader")
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	a.orp = NewORPWidget()
	a.orp.Highlight = a.prefHighlight()
	a.orp.FontSize = float32(a.prefFontSize())

	a.playBtn = NewPillButton("Play", a.togglePlay)
	a.playBtn.Bold = true
	a.playBtn.Disable()

	prevBtn := NewPillButton("Previous", func() { a.step(-1) })
	nextBtn := NewPillButton("Next", func() { a.step(1) })
	prevBtn.Disable()
	nextBtn.Disable()
	a.prevBtn, a.nextBtn = prevBtn, nextBtn

	a.wpmEntry = widget.NewEntry()
	a.wpmEntry.SetText(fmt.Sprintf("%d", reader.SnapWPM(a.prefWPM())))
	a.wpmEntry.OnSubmitted = func(s string) {
		s = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(s), "wpm"))
		var v int
		if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
			a.wpmEntry.SetText(fmt.Sprintf("%d", a.currentWPM()))
			return
		}
		a.setWPM(v)
		a.saveProgress("")
		a.poke()
	}

	a.wpmSlider = NewSlimSlider(reader.MinWPM, reader.MaxWPM, reader.StepWPM)
	a.wpmSlider.OnChanged = func(v float64) {
		a.setWPM(reader.SnapWPM(int(v + 0.5)))
		a.poke()
	}
	a.wpmSlider.OnChangeEnded = func(float64) {
		a.saveProgress("")
	}
	a.wpmSlider.SetValue(float64(reader.SnapWPM(a.prefWPM())))

	a.progress = NewSlimProgress()
	a.progressLabel = widget.NewLabel("No book open")
	a.sectionLabel = widget.NewLabel("")
	a.sectionLabel.TextStyle = fyne.TextStyle{Bold: true}
	a.statusLabel = widget.NewLabel("")
	a.statusLabel.Wrapping = fyne.TextWrapWord
	a.statusLabel.Hide() // collapsed until there is something to say

	a.detector = newActivityDetector(a.poke, a.togglePlay)

	a.tree = widget.NewTree(
		func(uid string) []string { return a.childUIDs(uid) },
		func(uid string) bool { return a.isBranch(uid) },
		func(bool) fyne.CanvasObject { return widget.NewLabel("") },
		func(uid string, _ bool, o fyne.CanvasObject) {
			lbl := o.(*widget.Label)
			it := a.uidToItem[uid]
			if it == nil {
				lbl.SetText("")
				return
			}
			text := it.Label
			if it.StartWord == nil {
				if it.Unavailable != "" {
					text += " (unavailable)"
				} else if len(it.Children) == 0 {
					text += " (unavailable)"
				}
			}
			lbl.SetText(text)
		},
	)
	a.tree.OnSelected = a.onTOCSelected

	a.loadingBar = widget.NewProgressBarInfinite()
	a.loadingLabel = widget.NewLabel("Importing…")

	a.recentBox = container.NewVBox()
	a.refreshRecent()
}

func (a *App) topBar() *fyne.Container {
	openBtn := NewPillButton("Open file", a.showOpenDialog)
	toggleBtn := NewPillButton("Contents", a.toggleContents)
	contextBtn := NewPillButton("Context", a.toggleContext)
	settingsBtn := NewPillButton("Settings", a.showSettings)
	bar := container.NewBorder(nil, nil, nil,
		container.NewHBox(toggleBtn, pillGap(), contextBtn, pillGap(), settingsBtn, pillGap(), openBtn), a.titleLabel)
	return container.NewVBox(bar, widget.NewSeparator())
}

func (a *App) bottomBar() *fyne.Container {
	transport := container.NewHBox(a.prevBtn, pillGap(), a.playBtn, pillGap(), a.nextBtn)
	speedBox := container.NewHBox(widget.NewLabel("Speed"),
		container.NewGridWrap(fyne.NewSize(76, a.wpmEntry.MinSize().Height), a.wpmEntry))
	deck := container.NewBorder(nil, nil,
		container.NewHBox(transport, pillGap(), pillGap(), speedBox),
		nil, a.wpmSlider)
	return container.NewVBox(widget.NewSeparator(), a.sectionLabel, deck,
		a.progress, a.progressLabel, a.statusLabel)
}

// buildReaderScreen assembles the reader view and remembers the chrome
// wrappers so zen mode can hide them.

func (a *App) readerCenter() fyne.CanvasObject {
	center := fyne.CanvasObject(container.NewCenter(a.orp))
	if a.contextOn {
		if a.contextPane == nil {
			a.buildContextPane()
		}
		// Glance pane is the margin note: RSVP keeps the wide share.
		split := container.NewHSplit(center, a.contextPane)
		split.SetOffset(0.68)
		center = split
	}
	if a.contentsOn {
		// Tree scrolls internally; do not wrap it in another scroller.
		// The tree pane carries gutters mirroring the floating chrome so
		// rows never slide underneath it; the word pane spans the full
		// window height, so the word is always absolutely centered.
		treePane := container.NewBorder(
			newMirrorSpacer(a.topWrap), newMirrorSpacer(a.bottomWrap),
			nil, nil, a.tree)
		return container.NewHSplit(treePane, center)
	}
	return center
}

func (a *App) buildReaderScreen() {
	a.topWrap = a.topBar()
	a.bottomWrap = a.bottomBar()
	// Content fills the window so the word is absolutely centered; the
	// chrome floats over the tree pane's reserved gutters and the
	// detector sits beneath everything.
	chrome := container.NewBorder(a.topWrap, a.bottomWrap, nil, nil)
	a.readerScreen = container.NewStack(a.detector, a.readerCenter(), chrome)
	if a.chromeHidden && a.book != nil {
		a.topWrap.Hide()
		a.bottomWrap.Hide()
	}
}

func (a *App) buildLayout() {
	a.buildReaderScreen()

	emptyTitle := widget.NewLabel("RSVP Reader")
	emptyTitle.TextStyle = fyne.TextStyle{Bold: true}
	emptyOpen := NewPillButton("Open file", a.showOpenDialog)
	emptyOpen.Bold = true
	emptyPaste := NewPillButton("Paste text", a.showPasteDialog)
	a.emptyScreen = container.NewCenter(container.NewVBox(
		emptyTitle, widget.NewLabel("One word at a time, at a fixed focal point."),
		container.NewHBox(emptyOpen, pillGap(), emptyPaste), widget.NewLabel("Recent books:"), a.recentBox,
	))

	cancelBtn := NewPillButton("Cancel", a.cancelImport)
	a.loadingScreen = container.NewCenter(container.NewVBox(
		a.loadingLabel, a.loadingBar, cancelBtn,
	))

	stack := container.NewStack(a.emptyScreen, a.readerScreen, a.loadingScreen)
	a.showScreen(a.emptyScreen)
	a.root = stack
	a.win.SetContent(stack)
}

func (a *App) showScreen(s fyne.CanvasObject) {
	a.emptyScreen.Hide()
	a.readerScreen.Hide()
	a.loadingScreen.Hide()
	s.Show()
}

// ---------- menus, shortcuts, keys ----------

func (a *App) buildMenu() {
	openItem := fyne.NewMenuItem("Open file…", a.showOpenDialog)
	openItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper}
	a.win.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File", openItem)))
}

func (a *App) buildShortcuts() {
	c := a.win.Canvas()
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper},
		func(fyne.Shortcut) { a.showOpenDialog() })
	// Option+arrows jump a chunk. The driver dispatches Alt-modified
	// shortcuts but refuses Shift-modified ones outright, and Shift state
	// can't be tracked reliably (modifiers neither repeat nor report
	// release), so Shift chords degrade on hold. Option mirrors the
	// system word-jump binding; entries keep native keys via the guard.
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyLeft, Modifier: fyne.KeyModifierAlt},
		func(fyne.Shortcut) { a.onJump(-1) })
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyRight, Modifier: fyne.KeyModifierAlt},
		func(fyne.Shortcut) { a.onJump(1) })
	c.SetOnTypedKey(func(e *fyne.KeyEvent) {
		switch e.Name {
		case fyne.KeySpace:
			if _, ok := c.Focused().(*widget.Entry); ok {
				return
			}
			a.togglePlay()
		case fyne.KeyLeft:
			a.step(-1)
		case fyne.KeyRight:
			a.step(1)
		case fyne.KeyUp:
			a.bumpWPM(reader.StepWPM)
		case fyne.KeyDown:
			a.bumpWPM(-reader.StepWPM)
		case fyne.KeyC:
			if _, ok := c.Focused().(*widget.Entry); ok {
				return
			}
			a.toggleContext()
		case fyne.KeyEscape:
			c.Unfocus()
		}
	})
}

// ---------- book import ----------

func (a *App) showOpenDialog() {
	fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if rc == nil {
			return
		}
		p := rc.URI().Path()
		rc.Close()
		a.openFile(p)
	}, a.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".epub", ".pdf"}))
	// Fyne draws its own picker (no native NSOpenPanel integration
	// exists upstream); give it room to breathe instead. Resize must
	// come after Show: the inner window MinSize reads nil before that.
	fd.Show()
	fd.Resize(fyne.NewSize(780, 560))
}

// showPasteDialog collects plain text and starts a text session.
func (a *App) showPasteDialog() {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("Paste or type text to read…")
	body := container.NewGridWrap(fyne.NewSize(440, 240), entry)
	cancelBtn := NewPillButton("Cancel", nil)
	loadBtn := NewPillButton("Load text", nil)
	loadBtn.Bold = true
	buttons := container.NewHBox(layout.NewSpacer(), cancelBtn, pillGap(), loadBtn)
	d := dialog.NewCustomWithoutButtons("Paste text",
		container.NewVBox(body, buttons), a.win)
	cancelBtn.OnTapped = d.Hide
	loadBtn.OnTapped = func() {
		text := strings.TrimSpace(entry.Text)
		if len(strings.Fields(text)) == 0 {
			dialog.ShowInformation("Empty text", "Paste some text first.", a.win)
			return
		}
		d.Hide()
		a.openText(text)
	}
	d.Show()
}

func (a *App) openFile(path string) {
	if _, err := os.Stat(path); err != nil {
		// Missing recent file: inform, drop it, retain no stale attempt.
		if a.db != nil {
			for _, p := range a.db.ListRecent() {
				if p.Path == path {
					_ = a.db.Remove(p.Fingerprint)
				}
			}
			a.refreshRecent()
		}
		dialog.ShowInformation("File not found",
			fmt.Sprintf("Could not open %s. It was removed from recent books.", path), a.win)
		return
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub", ".pdf":
	default:
		a.showOpenError(fmt.Errorf("unsupported file type"))
		return
	}
	a.importGen++
	gen := a.importGen
	a.loadingLabel.SetText(fmt.Sprintf("Importing %s…", shortPath(path)))
	a.showScreen(a.loadingScreen)
	go func() {
		d, err := importFile(path)
		fyne.Do(func() {
			if gen != a.importGen {
				return // cancelled or superseded
			}
			if err != nil {
				a.showScreen(a.bookOrEmpty())
				a.showOpenError(err)
				return
			}
			a.setDocument(d, path)
			a.showScreen(a.readerScreen)
		})
	}()
}

// importFile routes a path to its source importer.
func importFile(path string) (*doc.Document, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".epub":
		b, err := epub.OpenFile(path)
		if err != nil {
			return nil, err
		}
		return doc.FromEPUB(b), nil
	case ".pdf":
		return pdf.OpenFile(path)
	default:
		return nil, fmt.Errorf("unsupported file type %q", filepath.Ext(path))
	}
}

// openText starts (or resumes) a pasted-text session.
func (a *App) openText(text string) {
	d := doc.FromText("Pasted text", text)
	if len(d.Words) == 0 {
		a.showOpenError(fmt.Errorf("no readable words in that text"))
		return
	}
	if a.db != nil {
		if err := a.db.SaveText(d.Fingerprint, text); err != nil {
			a.showOpenError(err)
			return
		}
	}
	a.setDocument(d, "")
	a.showScreen(a.readerScreen)
}

// openRecent reopens a recent entry: files by path, texts from storage.
func (a *App) openRecent(p store.Progress) {
	if p.Path != "" {
		a.openFile(p.Path)
		return
	}
	if a.db == nil {
		return
	}
	text, err := a.db.LoadText(p.Fingerprint)
	if err != nil {
		_ = a.db.Remove(p.Fingerprint)
		a.refreshRecent()
		dialog.ShowInformation("Text gone",
			"That pasted text is no longer stored. It was removed from recent books.", a.win)
		return
	}
	a.openText(text)
}

func (a *App) bookOrEmpty() fyne.CanvasObject {
	if a.book != nil {
		return a.readerScreen
	}
	return a.emptyScreen
}

func (a *App) cancelImport() {
	a.importGen++
	a.showScreen(a.bookOrEmpty())
}

func (a *App) showOpenError(err error) {
	msg := err.Error()
	content := container.NewVBox(
		widget.NewLabel("Could not open that file."),
		widget.NewLabel(msg),
		widget.NewLabel("EPUB 2/3 without DRM, plain PDFs, and pasted text are supported."),
	)
	d := dialog.NewCustom("Open failed", "Close", content, a.win)
	openAnother := widget.NewButton("Open Another File", func() {
		d.Hide()
		a.showOpenDialog()
	})
	content.Add(openAnother)
	d.Show()
}

func shortPath(p string) string {
	if len(p) > 60 {
		return "…" + p[len(p)-59:]
	}
	return p
}

func (a *App) setDocument(d *doc.Document, sourcePath string) {
	a.cancelTick()
	a.book = d
	wpm := reader.SnapWPM(a.prefWPM())
	a.player = reader.NewPlayer(d.Words, wpm)
	a.player.SectionAt = d.SectionLabel
	a.setWPM(wpm)
	a.buildTOCModel()
	a.titleLabel.SetText(docTitle(d))
	if d.TOCWarning != "" {
		a.setStatus(d.TOCWarning)
	} else {
		a.setStatus("")
	}
	// Restore last position and settings for this fingerprint.
	if a.db != nil {
		if p, ok := a.db.Lookup(d.Fingerprint); ok {
			a.setWPM(reader.ClampWPM(p.WPM))
			a.player.Seek(p.WordIndex)
		}
		_ = a.db.Save(store.Progress{
			Fingerprint: d.Fingerprint, Kind: d.Kind, Path: sourcePath,
			Title: d.Title, Creator: d.Creator,
			WordIndex: a.player.Pos(), WPM: a.player.WPM(),
		})
		a.refreshRecent()
	}
	a.playBtn.Enable()
	a.prevBtn.Enable()
	a.nextBtn.Enable()
	a.playBtn.SetText("Play")
	a.chromeHidden = false
	a.ctxLines = nil // new book re-anchors the glance pane
	a.refreshAll()
	a.poke()         // chrome visible, melts after idle
	a.syncWakeLock() // fresh documents open paused
}

func docTitle(d *doc.Document) string {
	if d.Creator != "" {
		return fmt.Sprintf("%s — %s", d.Title, d.Creator)
	}
	return d.Title
}

// ---------- TOC tree ----------

func (a *App) buildTOCModel() {
	a.uidToItem = map[string]*doc.TOCItem{}
	a.itemToUID = map[*doc.TOCItem]string{}
	a.topUIDs = nil
	var assign func(items []*doc.TOCItem, prefix string)
	assign = func(items []*doc.TOCItem, prefix string) {
		for i, it := range items {
			uid := fmt.Sprintf("%d", i)
			if prefix != "" {
				uid = prefix + "/" + uid
			}
			a.uidToItem[uid] = it
			a.itemToUID[it] = uid
			if prefix == "" {
				a.topUIDs = append(a.topUIDs, uid)
			}
			assign(it.Children, uid)
		}
	}
	if a.book != nil {
		assign(a.book.TOC, "")
	}
	a.tree.Refresh()
}

func (a *App) childUIDs(uid string) []string {
	if uid == "" {
		return a.topUIDs
	}
	it := a.uidToItem[uid]
	if it == nil {
		return nil
	}
	var out []string
	prefix := uid
	for i := range it.Children {
		out = append(out, fmt.Sprintf("%s/%d", prefix, i))
	}
	return out
}

func (a *App) isBranch(uid string) bool {
	// The tree walk starts at the root UID "" and only descends when
	// IsBranch reports true, so the root must count as a branch.
	if uid == "" {
		return true
	}
	it := a.uidToItem[uid]
	return it != nil && len(it.Children) > 0
}

func (a *App) onTOCSelected(uid string) {
	if a.syncingTree {
		return
	}
	it := a.uidToItem[uid]
	if it == nil || a.book == nil || a.player == nil {
		return
	}
	if it.StartWord == nil {
		if it.Unavailable != "" {
			a.setStatus(fmt.Sprintf("%s — unavailable: %s.", it.Label, it.Unavailable))
		} else if len(it.Children) > 0 {
			a.setStatus(fmt.Sprintf("%s is a group heading; expand it and choose a section.", it.Label))
		}
		return
	}
	idx, ok := a.book.SeekTOC(it)
	if !ok {
		a.setStatus(fmt.Sprintf("%s points nowhere readable.", it.Label))
		return
	}
	a.cancelTick()
	a.player.Seek(idx)
	a.playBtn.SetText("Play")
	a.setStatus(fmt.Sprintf("Section: %s", it.Label))
	a.refreshAll()
	a.saveProgress(it.Ref)
	a.syncWakeLock() // seeking pauses
}

func (a *App) toggleContents() {
	// HSplit has no collapse; swap the reader center with/without the tree.
	a.contentsOn = !a.contentsOn
	a.buildReaderScreen()
	a.root.Objects[1] = a.readerScreen
	a.root.Refresh()
	if a.book != nil {
		a.showScreen(a.readerScreen)
	}
	a.refreshAll()
}

// ---------- playback ----------

func (a *App) togglePlay() {
	if a.player == nil || a.book == nil {
		return
	}
	defer a.syncWakeLock() // held iff playing when we leave
	now := time.Now()
	if a.player.Playing() {
		a.cancelTick()
		a.player.Pause()
		a.playBtn.SetText("Resume")
		a.saveProgress("")
		a.refreshAll()
		a.revealChrome() // pause always shows and holds
		return
	}
	if a.player.Ended() {
		a.player.Restart()
		a.player.Play(now)
		a.playBtn.SetText("Pause")
		a.scheduleTick()
	} else {
		if a.player.Pos() == 0 && !a.wasResumed() {
			a.player.Play(now)
		} else {
			a.player.Resume(now)
		}
		a.playBtn.SetText("Pause")
		a.scheduleTick()
	}
	a.refreshAll()
	a.hideNow() // play always hides immediately
}

func (a *App) wasResumed() bool { return a.lastSave.IsZero() == false || a.player.Pos() != 0 }

// onJump moves a chunk, unless typing (entries keep native keys).
// Skipped when typing: entries keep their native keys.
func (a *App) onJump(dir int) {
	if _, ok := a.win.Canvas().Focused().(*widget.Entry); ok {
		return
	}
	a.jumpWords(dir * jumpChunk)
}

// jumpChunk is the Option+arrow jump distance in words.
const jumpChunk = 25

// jumpWords moves by a chunk, clamping at the ends. Like stepping, it
// pauses and reveals the chrome.
func (a *App) jumpWords(delta int) {
	if a.player == nil {
		return
	}
	a.cancelTick()
	a.player.Pause()
	target := a.player.Pos() + delta
	if target < 0 {
		target = 0
	}
	if target >= a.player.Len() {
		target = a.player.Len() - 1
	}
	a.player.Seek(target)
	if a.player.Ended() {
		a.playBtn.SetText("Restart")
		a.setStatus("End of book — Restart or choose another section.")
	} else {
		a.playBtn.SetText("Resume")
	}
	a.refreshAll()
	a.saveProgress("")
	a.revealChrome()
	a.syncWakeLock()
}

func (a *App) step(dir int) {
	if a.player == nil {
		return
	}
	wasPlaying := a.player.Playing()
	a.cancelTick()
	a.player.Pause()
	moved := false
	if dir < 0 {
		moved = a.player.StepPrev()
	} else {
		moved = a.player.StepNext()
	}
	if moved || !wasPlaying {
		a.playBtn.SetText("Resume")
	}
	a.refreshAll()
	if moved {
		a.saveProgress("")
	}
	a.revealChrome() // stepping pauses: paused chrome is shown and held
	a.syncWakeLock()
}

func (a *App) bumpWPM(delta int) {
	if a.player == nil {
		return
	}
	wpm := a.player.WPM()
	if delta > 0 {
		wpm += reader.StepWPM
	} else {
		wpm -= reader.StepWPM
	}
	a.setWPM(wpm)
	a.saveProgress("")
	a.poke()
}

func (a *App) scheduleTick() {
	a.cancelTick()
	if a.player == nil || !a.player.Playing() {
		return
	}
	d := time.Until(a.player.Deadline())
	if d < 0 {
		d = 0
	}
	a.tickTimer = time.AfterFunc(d, func() {
		advanced := a.player.Tick(time.Now())
		fyne.Do(func() {
			if a.player == nil {
				return
			}
			a.refreshAll()
			if advanced {
				a.maybePeriodicSave()
			}
			if a.player.Ended() {
				a.onEnded()
			} else {
				a.scheduleTick()
			}
		})
	})
}

func (a *App) cancelTick() {
	if a.tickTimer != nil {
		a.tickTimer.Stop()
		a.tickTimer = nil
	}
}

func (a *App) onEnded() {
	a.playBtn.SetText("Restart")
	a.setStatus("End of book — Restart or choose another section.")
	a.saveProgress("")
	a.revealChrome() // stopped reads as paused: shown and held
	a.syncWakeLock()
}

// ---------- refresh ----------

func (a *App) refreshAll() {
	a.refreshWord()
	a.refreshProgress()
	a.syncTreeSelection()
	a.updateContext()
}

func (a *App) refreshWord() {
	if a.player == nil {
		return
	}
	a.orp.SetWord(a.player.Current())
}

func (a *App) refreshProgress() {
	if a.player == nil || a.book == nil {
		return
	}
	pos, total, frac := a.player.Progress()
	pct := frac * 100
	a.progress.SetValue(frac)
	sec := a.player.Section()
	a.sectionLabel.SetText("Section: " + sec)
	a.progressLabel.SetText(fmt.Sprintf("%d / %d (%.1f%%) · %s left · %d WPM",
		pos+1, total, pct, a.player.TimeRemaining(), a.player.WPM()))
}

func (a *App) syncTreeSelection() {
	if a.player == nil || a.book == nil {
		return
	}
	sec := a.book.SectionForWord(a.player.Pos())
	if sec == nil {
		return
	}
	uid, ok := a.itemToUID[sec]
	if !ok {
		return
	}
	a.syncingTree = true
	a.tree.Select(uid)
	a.syncingTree = false
}

// currentWPM reports the live speed, falling back to the saved preference.
func (a *App) currentWPM() int {
	if a.player != nil {
		return a.player.WPM()
	}
	return reader.SnapWPM(a.prefWPM())
}

// setWPM applies an exact speed everywhere: player, slider position,
// numeric entry, and saved preference. Position never moves.
func (a *App) setWPM(wpm int) {
	wpm = reader.ClampWPM(wpm)
	if a.player != nil {
		a.player.SetWPM(wpm)
	}
	a.wpmSlider.SetValue(float64(wpm))
	a.wpmEntry.SetText(fmt.Sprintf("%d", wpm))
	a.prefs().SetInt("wpm", wpm)
	a.refreshProgress()
}

func (a *App) setStatus(s string) {
	if s == "" {
		a.statusLabel.Hide()
		return
	}
	a.statusLabel.SetText(s)
	a.statusLabel.Show()
}

// ---------- persistence ----------

func (a *App) saveProgress(tocHref string) {
	if a.db == nil || a.book == nil || a.player == nil {
		return
	}
	href := tocHref
	if href == "" {
		if s := a.book.SectionForWord(a.player.Pos()); s != nil {
			href = s.Ref
		}
	}
	_ = a.db.Save(store.Progress{
		Fingerprint: a.book.Fingerprint, Kind: a.book.Kind, Path: a.currentPath(),
		Title: a.book.Title, Creator: a.book.Creator,
		WordIndex: a.player.Pos(), WPM: a.player.WPM(), TOCHref: href,
	})
	a.lastSave = time.Now()
}

func (a *App) maybePeriodicSave() {
	if time.Since(a.lastSave) > 30*time.Second {
		a.saveProgress("")
	}
}

func (a *App) currentPath() string {
	if a.db == nil || a.book == nil {
		return ""
	}
	if p, ok := a.db.Lookup(a.book.Fingerprint); ok {
		return p.Path
	}
	return ""
}

// syncWakeLock holds the display awake exactly while playing.
func (a *App) syncWakeLock() {
	if a.player != nil && a.player.Playing() {
		wake.Hold()
	} else {
		wake.Release()
	}
}

func (a *App) onClose() {
	a.cancelTick()
	a.cancelHideTimer()
	a.saveProgress("")
	wake.Release()
	if a.chromeHidden {
		// Restore only what we hid: the hide count is system-global.
		setCursorVisible(true)
	}
	a.win.Close()
}

// ---------- recent books ----------

func (a *App) refreshRecent() {
	if a.recentBox == nil || a.db == nil {
		return
	}
	a.recentBox.Objects = nil
	for _, p := range a.db.ListRecent() {
		label := p.Title
		if label == "" {
			label = shortPath(p.Path)
		} else if p.Path != "" {
			label = fmt.Sprintf("%s (%s)", label, shortPath(p.Path))
		}
		entry := p
		a.recentBox.Add(NewPillButton(label, func() { a.openRecent(entry) }))
	}
	if len(a.recentBox.Objects) == 0 {
		a.recentBox.Add(widget.NewLabel("No recent books yet."))
	}
	a.recentBox.Refresh()
}

// ---------- settings ----------

const fontDefaultLabel = "System default"

func (a *App) showSettings() {
	highlight := widget.NewCheck("Highlight recognition letter", func(bool) {})
	highlight.SetChecked(a.orp.Highlight)
	fontSizeSlider := NewSlimSlider(28, 120, 4)
	fontSizeSlider.SetValue(float64(a.orp.FontSize))
	themeSel := widget.NewSelect([]string{"Dark", "Light"}, func(string) {})
	if a.prefs().StringWithFallback("theme", "dark") == "light" {
		themeSel.SetSelected("Light")
	} else {
		themeSel.SetSelected("Dark")
	}
	fontOpts, fontPaths := a.fontOptions()
	families := append([]string(nil), fontOpts[1:]...)
	pendingFont := ""
	if saved := a.prefs().StringWithFallback("readerFontPath", ""); saved != "" {
		for fam, path := range fontPaths {
			if path == saved {
				pendingFont = fam
				break
			}
		}
	}
	fontName := func(fam string) string {
		if fam == "" {
			return fontDefaultLabel
		}
		return fam
	}
	currentFontLabel := widget.NewLabel("Current: " + fontName(pendingFont))
	filtered := append([]string(nil), families...)
	fontSearch := widget.NewEntry()
	fontSearch.SetPlaceHolder("Search fonts")
	fontList := widget.NewList(
		func() int { return len(filtered) + 1 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id == 0 {
				o.(*widget.Label).SetText(fontDefaultLabel)
				return
			}
			o.(*widget.Label).SetText(filtered[id-1])
		},
	)
	fontList.OnSelected = func(id widget.ListItemID) {
		if id == 0 {
			pendingFont = ""
		} else {
			pendingFont = filtered[id-1]
		}
		currentFontLabel.SetText("Current: " + fontName(pendingFont))
	}
	fontSearch.OnChanged = func(s string) {
		q := strings.ToLower(strings.TrimSpace(s))
		filtered = filtered[:0]
		for _, f := range families {
			if q == "" || strings.Contains(strings.ToLower(f), q) {
				filtered = append(filtered, f)
			}
		}
		fontList.Refresh()
		fontList.UnselectAll()
		for i, f := range filtered {
			if f == pendingFont {
				fontList.Select(i + 1)
				break
			}
		}
		if pendingFont == "" {
			fontList.Select(0)
		}
	}
	select {
	case <-a.fontScanned:
	default:
		fontSearch.Disable() // scan still running; results land on reopen
	}
	fontPicker := container.NewVBox(
		currentFontLabel,
		fontSearch,
		container.NewGridWrap(fyne.NewSize(300, 170), fontList),
	)
	// Preselect the active font once rows exist.
	if pendingFont == "" {
		fontList.Select(0)
	} else {
		for i, f := range filtered {
			if f == pendingFont {
				fontList.Select(i + 1)
				break
			}
		}
	}
	form := widget.NewForm(
		widget.NewFormItem("Highlight", highlight),
		widget.NewFormItem("Font size", fontSizeSlider),
		widget.NewFormItem("Reader font", fontPicker),
		widget.NewFormItem("Theme", themeSel),
	)
	cancelBtn := NewPillButton("Cancel", nil)
	applyBtn := NewPillButton("Apply", nil)
	applyBtn.Bold = true
	buttons := container.NewHBox(layout.NewSpacer(), cancelBtn, pillGap(), applyBtn)
	d := dialog.NewCustomWithoutButtons("Settings",
		container.NewVBox(form, buttons), a.win)
	cancelBtn.OnTapped = d.Hide
	applyBtn.OnTapped = func() {
		a.orp.Highlight = highlight.Checked
		a.prefs().SetBool("highlight", highlight.Checked)
		a.orp.FontSize = float32(fontSizeSlider.Value)
		a.prefs().SetFloat("fontSize", fontSizeSlider.Value)
		a.applyReaderFont(pendingFont, fontPaths)
		if themeSel.Selected == "Light" {
			a.prefs().SetString("theme", "light")
		} else {
			a.prefs().SetString("theme", "dark")
		}
		a.applyThemePref()
		a.orp.Refresh()
		d.Hide()
	}
	d.Show()
}

// applyReaderFont loads the selected family (or restores the default),
// keeping the previous typeface when the file fails to load.
func (a *App) applyReaderFont(selected string, paths map[string]string) {
	if selected == "" || selected == fontDefaultLabel {
		a.readerFont = nil
		a.prefs().SetString("readerFontPath", "")
		a.prefs().SetString("readerFontFamily", "")
		return
	}
	path, ok := paths[selected]
	if !ok {
		return
	}
	data, err := LoadFontFace(path, selected)
	if err != nil {
		a.setStatus(fmt.Sprintf("Could not load %s; keeping the previous font.", selected))
		return
	}
	a.readerFont = fyne.NewStaticResource(filepath.Base(path), data)
	a.prefs().SetString("readerFontPath", path)
	a.prefs().SetString("readerFontFamily", selected)
}

func (a *App) applyThemePref() {
	if a.prefs().StringWithFallback("theme", "dark") == "light" {
		a.fyneApp.Settings().SetTheme(theme.LightTheme())
	} else {
		th := newScandiTheme()
		th.SetReaderFont(a.readerFont)
		a.fyneApp.Settings().SetTheme(th)
	}
}

// scanFonts inventories system fonts off the UI thread; settings reads
// the snapshot when opened.
func (a *App) scanFonts() {
	list := ScanSystemFonts(FontDirs())
	a.fontMu.Lock()
	a.fontList = list
	a.fontMu.Unlock()
	close(a.fontScanned)
	fyne.Do(a.reconcileSavedFont)
}

// reconcileSavedFont re-resolves the persisted family through the fresh
// scan. An older version may have stored a Bold/Italic file for the
// family; the scan now knows the Regular one, so the stored path is
// corrected once and the reader typeface follows.
func (a *App) reconcileSavedFont() {
	path := a.prefs().StringWithFallback("readerFontPath", "")
	family := a.prefs().StringWithFallback("readerFontFamily", "")
	if path == "" || family == "" {
		return
	}
	scanned := ""
	a.fontMu.RLock()
	for _, f := range a.fontList {
		if strings.EqualFold(f.Family, family) {
			scanned = f.Path
			break
		}
	}
	a.fontMu.RUnlock()
	if scanned == "" || scanned == path {
		return
	}
	data, err := LoadFontFace(scanned, family)
	if err != nil {
		return
	}
	a.readerFont = fyne.NewStaticResource(filepath.Base(scanned), data)
	a.prefs().SetString("readerFontPath", scanned)
	a.applyThemePref()
	a.orp.Refresh()
}

// loadSavedFont restores the persisted reader typeface, falling back to
// the bundled font when the file is gone or unloadable. Strict validation
// here is what breaks the crash loop a bad persisted font would cause.
func (a *App) loadSavedFont() {
	path := a.prefs().StringWithFallback("readerFontPath", "")
	if path == "" {
		return
	}
	family := a.prefs().StringWithFallback("readerFontFamily", "")
	data, err := LoadFontFace(path, family)
	if err != nil {
		a.prefs().SetString("readerFontPath", "")
		a.prefs().SetString("readerFontFamily", "")
		return
	}
	a.readerFont = fyne.NewStaticResource(filepath.Base(path), data)
}

// fontOptions snapshots the picker entries: default first, then families.
func (a *App) fontOptions() ([]string, map[string]string) {
	a.fontMu.RLock()
	defer a.fontMu.RUnlock()
	opts := []string{fontDefaultLabel}
	paths := map[string]string{}
	for _, f := range a.fontList {
		opts = append(opts, f.Family)
		paths[f.Family] = f.Path
	}
	return opts, paths
}
