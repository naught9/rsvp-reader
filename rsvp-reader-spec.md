# RSVP Reader — Product and Technical Specification

**Status:** Draft for review, 23 September 2026  
**Target:** macOS first; architecture should permit Windows and Linux later  
**Implementation:** Go desktop application using Fyne; no browser, web server, or JavaScript

## 1. Product goal

Build a local desktop reader that presents an EPUB one word at a time at a fixed focal point. A reader can open a book, browse its actual table of contents, choose a chapter or subsection, and begin at that location. The application is launched from a terminal with `go run ./cmd/rsvp` during development or a compiled executable later; launch opens a GUI window.

The reference project informs interaction design, but this is a new codebase. The defining difference is that importing a book preserves its reading order, navigation tree, and links from navigation entries to exact reading positions.

### Success criteria

1. A user opens a normal EPUB 2 or EPUB 3 file and sees its nested table of contents.
2. Choosing an entry that targets a heading inside an XHTML file starts at that heading, even when several entries share the same file.
3. Words display smoothly at the chosen pace with reliable pause, resume, and backward navigation.
4. Closing and reopening the same book restores the last word and reading settings.
5. All book content and reading state stay on the local machine.

## 2. Scope

### First release

- Open local `.epub` files through a file picker or an optional CLI path: `rsvp [book.epub]`.
- Support reflowable, text based EPUB 2 and EPUB 3 without DRM.
- Show title and author when present.
- Show a hierarchical, collapsible table of contents, including nested subsections.
- Select a TOC entry to position the reader at its exact target. Reading then continues through the remaining book in spine order.
- RSVP playback with fixed focal point, optional highlighted recognition letter, speed control, pause/resume, previous/next word, and progress display.
- Remember the last position and settings for each opened book; offer a short recent-books list.
- Work offline and require no account.

### Later, after the EPUB flow is reliable

- PDF and pasted text input, with navigation appropriate to those sources.
- More advanced pacing, such as configurable pauses at punctuation or paragraph boundaries.
- Search, annotations, sync, and packaging for distribution.
- Additional language-specific tokenization and right-to-left layout.

### Explicit limits of the first release

- DRM-protected or encrypted books cannot be opened.
- Image-only, fixed-layout, audio/video, and scripted EPUB content is outside the text reader's scope.
- Images, tables, footnotes, and mathematical layout are not reproduced in RSVP. Meaningful image alternative text may be included, but the app must not invent text for visual content.
- English and other left-to-right, space-separated text are the primary reading experience. Import must preserve UTF-8 text without corruption; high-quality pacing and focal highlighting for every writing system are later work.

## 3. Main user flow

1. Launch `rsvp` or `rsvp path/to/book.epub`.
2. If no book is open, show **Open EPUB** and a list of recent books.
3. Import occurs in the background. Show progress or a busy state and keep the window responsive.
4. Display book title, a collapsible contents panel, the current word area, and playback controls.
5. Select a TOC item. Playback pauses and the reader moves to the first word at that target. Show the selected section and its position before playback resumes.
6. Press Space or Play to read. Press Space again to pause. Adjust speed without resetting position.
7. Close the app; reopening the book resumes at the saved position.

If the TOC is missing or unusable, show generated entries from the EPUB spine where possible and label them **Sections**. Do not present generated entries as the publisher's table of contents. The book remains readable from its beginning if no usable navigation can be generated.

## 4. Interface

### Window layout

- **Top bar:** book title, Open EPUB action, and a button to show or hide contents.
- **Contents panel:** hierarchical entries with expand/collapse, current section indicator, and keyboard selection. Entries with no target can expand children but cannot start playback themselves.
- **Reader area:** one prominent word centered around a fixed recognition point, with minimal visual movement between words. The focal letter highlight can be disabled.
- **Bottom controls:** Play/Pause, previous/next word, speed, progress, and current section label.
- **Settings:** highlight on/off, font size, theme, and optional punctuation pacing. The initial theme is dark; a light option is available.

The contents panel can be hidden during reading without changing position. Resizing the window keeps the focal point stationary relative to the reader area.

### Keyboard behavior

| Key | Action |
| --- | --- |
| Space | Play or pause, except while typing into a text control |
| Left / Right | Previous / next word while paused |
| Up / Down | Increase / decrease speed in fixed steps when the reader area has focus |
| Escape | Leave focus mode or close an open dialog |
| Command+O on macOS | Open EPUB |

Controls must have visible labels and usable keyboard focus. Color is not the only cue for the focal point or selected section.

### States and errors

- **Empty:** clear action to open a book.
- **Loading:** responsive window and cancel option for a long import.
- **Ready/paused:** current word and section visible.
- **Playing:** minimal controls; selected TOC entry remains identifiable when contents are visible.
- **End of book:** stop on the last word and offer Restart or choose another section.
- **Invalid or unsupported file:** specific, nontechnical error and an Open Another File action. A malformed TOC should not prevent reading valid body text.
- **Missing recent file:** remove it from recent books after informing the user; retain no stale playback attempt.

## 5. EPUB import and navigation contract

### Reading order

1. Open the EPUB ZIP container and locate its package document through `META-INF/container.xml`.
2. Parse the package manifest and spine. The spine, not ZIP entry order or file names, defines sequential reading order.
3. Exclude spine items marked non-linear from automatic sequential playback, while allowing a valid TOC target to open one deliberately.
4. Parse each supported spine XHTML document in order. Extract readable body text while excluding scripts and styles. Preserve paragraph and heading boundaries in the internal model.
5. Apply explicit archive, resource-size, and total extracted-text limits so a malformed or compressed file cannot exhaust memory.

### Table of contents

- EPUB 3: parse the navigation document's `toc` tree.
- EPUB 2: parse the NCX `navMap` tree.
- Preserve label, nesting, source `href`, and fragment identifier for each entry. Resolve links relative to the navigation document, decode URLs, and normalize paths within the archive.
- A TOC entry can point to a whole content file or an element ID within it. Map that anchor to the first readable word at or after the element. When the element contains a heading, its first word is the start position.
- Multiple entries may point into one content file; they remain distinct selectable sections.
- If an anchor resolves to a valid document but has no readable words afterward, disable that entry with an explanation rather than silently jumping elsewhere.
- Unlinked parent labels remain visible as group headings. A link outside the supported spine is shown as unavailable rather than mapped to an unrelated position.
- Navigation order is displayed as supplied; playback order follows the spine.

### Internal model

The parser produces a structured `Book` rather than a flat string:

```text
Book { identity, metadata, spine[], toc[], words[] }
SpineItem { resourcePath, linear, startWord, endWord }
TOCItem { label, children[], targetPath, targetFragment, startWord? }
Word { text, spineItem, sourceElement, paragraphBoundary, sentenceBoundary }
```

`startWord` is a zero-based index in the book-wide word sequence. Source positions are retained during extraction so a TOC anchor is resolved before text normalization or tokenization loses the element boundary. The exact schema may evolve, but the distinction between reading order, navigation, and displayed words must remain.

### Tokenization and display

- Normalize whitespace for display without joining words across block boundaries incorrectly.
- Keep punctuation attached to the relevant word for intelligible playback.
- Calculate the highlighted recognition point on Unicode grapheme clusters rather than byte offsets; never split a multibyte character.
- Keep the focal letter at a fixed horizontal coordinate by separately measuring/drawing text before, at, and after it.
- Headings are read as part of the book; choosing a heading starts with its text.
- Provide a safe fallback when a section has symbols but no ordinary word tokens.

## 6. Playback rules

- Default: **300 words per minute**. Adjustable from **50 to 1,000 WPM**, in 25-WPM steps or direct numeric input.
- Playback displays one token at a time. The timer uses a monotonic clock and schedules the next word from the time the current word is shown.
- Pause freezes the current word. Resume holds that word for a full interval before advancing; it does not skip during app suspension or window inactivity.
- Selecting a TOC entry, opening a new book, or jumping manually pauses playback.
- Speed changes take effect on the next word.
- Progress shows current word / total words and percentage for the full book. The current section label is derived from the most recent resolved TOC target at or before the current position in reading order.
- A simple initial build may use constant word intervals. Punctuation-aware pacing is optional and must be clearly reflected in estimated time remaining if enabled.

## 7. Persistence and privacy

- Store settings and recent-book metadata in the application's local data directory; use Fyne preferences for small settings and a versioned JSON file for per-book progress.
- Identify a book by a content fingerprint, not just its path, so a replaced EPUB does not restore a position in a different text.
- Store the source path, fingerprint, last word index, selected TOC target, and updated time. Do not copy the book into the app's data directory in the first release.
- Save after pause, section change, and orderly close, with periodic saves during longer playback. Write state atomically so an interrupted write does not corrupt all progress.
- No telemetry, cloud sync, external assets, or network requests.

## 8. Implementation plan

### Stack

- **Language:** Go for parser, playback state, storage, and GUI.
- **GUI:** Fyne desktop window and widgets; custom reader display where stock text widgets do not maintain an exact focal coordinate.
- **EPUB:** Go standard library `archive/zip` and `encoding/xml`; `golang.org/x/net/html` for XHTML body traversal. Add a small Unicode grapheme library only if needed for correct focal display.
- **Dependencies:** keep the set small and pin versions in `go.mod`.
- **Launch:** `go run ./cmd/rsvp` in development; `go build ./cmd/rsvp` produces a CLI-launchable executable that opens the window.

### Suggested packages

```text
cmd/rsvp             CLI entry point and window startup
internal/epub        container, package, spine, EPUB 3 nav, EPUB 2 NCX
internal/reader      words, anchor mapping, playback state and timing
internal/store       settings, recent books, and progress
internal/ui          Fyne window, contents tree, reader canvas, controls
```

EPUB parsing and playback state should not import Fyne. Parsing runs off the GUI thread; UI updates from worker goroutines are dispatched through Fyne's `fyne.Do` mechanism. This lets us test the book/position contract without opening a window.

### Build phases and review gates

1. **Skeleton:** CLI launch opens a Fyne window; file picker works. Gate: macOS launch and quit are clean.
2. **EPUB model:** import spine text and EPUB 2/3 TOC with source anchors. Gate: fixture tests for nested TOC, same-file fragments, relative paths, and missing anchors.
3. **Reader:** display, timing, keyboard controls, speed, and section selection. Gate: manual reading session plus focused playback-state tests.
4. **Persistence and polish:** resume, recent books, errors, settings, responsive import. Gate: close/reopen the same book at a chosen subsection and verify exact resume; test malformed and large archives.

## 9. Acceptance scenarios

1. **EPUB 3 subsection:** two TOC entries target different IDs in one XHTML file. Choosing the second displays its heading's first word and continues from there.
2. **EPUB 2 NCX:** nested chapter entries display in hierarchy and each starts at the corresponding anchor.
3. **Unlinked parent:** expanding the parent reveals selectable children; selecting the parent does not jump to an arbitrary word.
4. **Reading order:** files whose names sort differently from the spine play in spine order.
5. **Resume:** pause halfway through a chapter, close, relaunch, and reopen; the same word and speed return.
6. **Bad navigation:** an EPUB with readable spine text but a broken TOC remains readable and explains the navigation problem.
7. **Unsupported content:** DRM/encrypted or image-only content produces an honest unsupported-content message rather than an empty reader.
8. **Timing:** pause, window inactivity, and system sleep do not cause a burst of skipped words on return.

## 10. Decisions to confirm before implementation

This draft assumes **macOS first**, **EPUB first**, **Go and Fyne throughout**, **offline storage**, and **reading onward to the end of the book after a selected section**. These choices keep the first build focused while delivering the TOC navigation that motivated the project.

Sources: [Fyne quick start](https://docs.fyne.io/started/quick/), [Fyne tree widget](https://docs.fyne.io/api/v2/widget/tree/), [Fyne file dialog](https://docs.fyne.io/api/v2/dialog/filedialog/), [Fyne goroutine guidance](https://docs.fyne.io/started/goroutines/), [EPUB 3.3 specification](https://www.w3.org/TR/epub-33/), [EPUBCheck EPUB 2 navigation note](https://www.w3.org/publishing/epubcheck/docs/messages/).
