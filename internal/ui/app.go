package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"rsvp-reader/internal/epub"
	"rsvp-reader/internal/reader"
	"rsvp-reader/internal/store"
)

// App wires the book model, playback state, and Fyne widgets.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	db      *store.Store

	book   *epub.Book
	player *reader.Player
	cliArg string

	uidToItem map[string]*epub.TOCItem
	itemToUID map[*epub.TOCItem]string
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

	tickTimer   *time.Timer
	importGen   int
	syncingTree bool
	contentsOn  bool
	lastSave    time.Time
}

// New builds the application. cliArg is an optional EPUB path.
func New(fyneApp fyne.App, db *store.Store, cliArg string) *App {
	a := &App{fyneApp: fyneApp, db: db, cliArg: cliArg, contentsOn: true, fontScanned: make(chan struct{})}
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
		a.openBook(a.cliArg)
	}
}

// ShowAndRun displays the window and runs the event loop.
func (a *App) ShowAndRun() {
	if a.cliArg != "" {
		a.openBook(a.cliArg)
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
	openBtn := NewPillButton("Open EPUB", a.showOpenDialog)
	toggleBtn := NewPillButton("Contents", a.toggleContents)
	settingsBtn := NewPillButton("Settings", a.showSettings)
	bar := container.NewBorder(nil, nil, nil,
		container.NewHBox(toggleBtn, pillGap(), settingsBtn, pillGap(), openBtn), a.titleLabel)
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
	if a.contentsOn {
		// Tree scrolls internally; do not wrap it in another scroller.
		return container.NewHSplit(a.tree, container.NewCenter(a.orp))
	}
	return container.NewCenter(a.orp)
}

func (a *App) buildReaderScreen() {
	a.topWrap = a.topBar()
	a.bottomWrap = a.bottomBar()
	// Content keeps permanent gutters mirroring the chrome, so the word
	// never moves when chrome hides. The chrome overlays its gutters;
	// the detector sits beneath everything.
	content := container.NewBorder(
		newMirrorSpacer(a.topWrap), newMirrorSpacer(a.bottomWrap),
		nil, nil, a.readerCenter())
	chrome := container.NewBorder(a.topWrap, a.bottomWrap, nil, nil)
	a.readerScreen = container.NewStack(a.detector, content, chrome)
	if a.chromeHidden && a.book != nil {
		a.topWrap.Hide()
		a.bottomWrap.Hide()
	}
}

func (a *App) buildLayout() {
	a.buildReaderScreen()

	emptyTitle := widget.NewLabel("RSVP Reader")
	emptyTitle.TextStyle = fyne.TextStyle{Bold: true}
	emptyOpen := NewPillButton("Open EPUB", a.showOpenDialog)
	emptyOpen.Bold = true
	a.emptyScreen = container.NewCenter(container.NewVBox(
		emptyTitle, widget.NewLabel("One word at a time, at a fixed focal point."),
		emptyOpen, widget.NewLabel("Recent books:"), a.recentBox,
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
	openItem := fyne.NewMenuItem("Open EPUB…", a.showOpenDialog)
	openItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper}
	a.win.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File", openItem)))
}

func (a *App) buildShortcuts() {
	c := a.win.Canvas()
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper},
		func(fyne.Shortcut) { a.showOpenDialog() })
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
		a.openBook(p)
	}, a.win)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".epub"}))
	fd.Show()
}

func (a *App) openBook(path string) {
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
	a.importGen++
	gen := a.importGen
	a.loadingLabel.SetText(fmt.Sprintf("Importing %s…", shortPath(path)))
	a.showScreen(a.loadingScreen)
	go func() {
		book, err := epub.OpenFile(path)
		fyne.Do(func() {
			if gen != a.importGen {
				return // cancelled or superseded
			}
			if err != nil {
				a.showScreen(a.bookOrEmpty())
				a.showOpenError(err)
				return
			}
			a.setBook(book, path)
			a.showScreen(a.readerScreen)
		})
	}()
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
		widget.NewLabel("EPUB 2/3 without DRM is supported; PDFs and fixed-layout books are not, yet."),
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

func (a *App) setBook(book *epub.Book, path string) {
	a.cancelTick()
	a.book = book
	words := make([]string, len(book.Words))
	for i, w := range book.Words {
		words[i] = w.Text
	}
	wpm := reader.SnapWPM(a.prefWPM())
	a.player = reader.NewPlayer(words, wpm)
	a.player.SectionAt = func(idx int) string {
		if s := book.SectionForWord(idx); s != nil {
			return s.Label
		}
		return ""
	}
	a.setWPM(wpm)
	a.buildTOCModel()
	a.titleLabel.SetText(bookTitle(book))
	if book.TOCWarning != "" {
		a.setStatus(book.TOCWarning)
	} else {
		a.setStatus("")
	}
	// Restore last position and settings for this fingerprint.
	if a.db != nil {
		if p, ok := a.db.Lookup(book.Fingerprint); ok {
			a.setWPM(reader.ClampWPM(p.WPM))
			a.player.Seek(p.WordIndex)
		}
		_ = a.db.Save(store.Progress{
			Fingerprint: book.Fingerprint, Path: path,
			Title: book.Title, Creator: book.Creator,
			WordIndex: a.player.Pos(), WPM: a.player.WPM(),
		})
		a.refreshRecent()
	}
	a.playBtn.Enable()
	a.prevBtn.Enable()
	a.nextBtn.Enable()
	a.playBtn.SetText("Play")
	a.chromeHidden = false
	a.refreshAll()
	a.poke() // chrome visible, melts after idle
}

func bookTitle(b *epub.Book) string {
	if b.Creator != "" {
		return fmt.Sprintf("%s — %s", b.Title, b.Creator)
	}
	return b.Title
}

// ---------- TOC tree ----------

func (a *App) buildTOCModel() {
	a.uidToItem = map[string]*epub.TOCItem{}
	a.itemToUID = map[*epub.TOCItem]string{}
	a.topUIDs = nil
	var assign func(items []*epub.TOCItem, prefix string)
	assign = func(items []*epub.TOCItem, prefix string) {
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
	a.saveProgress(it.TargetHref)
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
	now := time.Now()
	if a.player.Playing() {
		a.cancelTick()
		a.player.Pause()
		a.playBtn.SetText("Resume")
		a.saveProgress("")
		a.refreshAll()
		a.hideNow()
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
	a.poke()
}

func (a *App) wasResumed() bool { return a.lastSave.IsZero() == false || a.player.Pos() != 0 }

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
	a.hideNow()
}

// ---------- refresh ----------

func (a *App) refreshAll() {
	a.refreshWord()
	a.refreshProgress()
	a.syncTreeSelection()
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
			href = s.TargetHref
		}
	}
	_ = a.db.Save(store.Progress{
		Fingerprint: a.book.Fingerprint, Path: a.currentPath(),
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

func (a *App) onClose() {
	a.cancelTick()
	a.cancelHideTimer()
	a.saveProgress("")
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
		} else {
			label = fmt.Sprintf("%s (%s)", label, shortPath(p.Path))
		}
		path := p.Path
		a.recentBox.Add(NewPillButton(label, func() { a.openBook(path) }))
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
