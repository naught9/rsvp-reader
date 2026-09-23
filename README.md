# RSVP Reader

A local desktop speed-reader for EPUB books, written in Go. It presents a book one word at a time at a fixed focal point (Rapid Serial Visual Presentation), with the book's real table of contents driving navigation — pick a chapter or subsection and reading starts at that exact heading.

Built from scratch with [Fyne](https://fyne.io/). Interaction design informed by [thomaskolmans/rsvp-reading](https://github.com/thomaskolmans/rsvp-reading); the defining difference here is that import preserves the EPUB's spine order, navigation tree, and fragment anchors instead of flattening the book to plain text.

## Features

- Opens DRM-free EPUB 2 and EPUB 3 files, plain PDFs, and pasted text (file picker or `rsvp book.epub`)
- Hierarchical, collapsible table of contents; entries targeting headings inside a file start at that heading's first word (PDFs navigate by page, pasted text reads as one section)
- RSVP playback with Optimal Recognition Point (ORP) focal-letter highlight (toggleable), 50–1000 WPM in 25-WPM steps (default 300)
- Play/pause, previous/next word, progress (word count, %, time remaining), current-section label
- Remembers position, speed, and settings per book (keyed by content hash), plus a recent-books list
- Dark theme by default, light theme available; adjustable reader font size and a searchable system font picker for the reader typeface (TrueType, OpenType, and collections)
- Fully offline — no accounts, telemetry, or network requests

## Requirements

- Go 1.24+
- macOS first (Windows/Linux should work via Fyne, untested); Xcode command-line tools for the C toolchain Fyne needs

## Run it

```bash
go run ./cmd/rsvp [book.epub]
```

Or build a binary:

```bash
go build -o rsvp ./cmd/rsvp
./rsvp [book.epub]
```

## macOS app bundle

`go run` shows a generic `exec` icon in the dock. For the real thing with the app icon, package it (requires Xcode command-line tools):

```bash
go install fyne.io/tools/cmd/fyne@latest
fyne package -os darwin -src ./cmd/rsvp -icon Icon.png \
  -app-id com.rsvp.reader -name "RSVP Reader" -app-version 0.1.0 -app-build 1
```

Drag the resulting `RSVP Reader.app` into `/Applications` (or `~/Applications`).
First launch needs a right-click → Open, since the bundle is unsigned.
`FyneApp.toml` carries the bundle metadata; `Icon.png` is the 1024px source.

## Keyboard shortcuts

| Key | Action |
| --- | ------ |
| Space | Play / pause |
| Left / Right | Previous / next word (while paused) |
| Up / Down | Faster / slower in 25-WPM steps |
| ⌘O | Open EPUB |
| Esc | Unfocus / close dialog |

## How it works

```
cmd/rsvp          CLI entry point and window startup
internal/epub     EPUB import: container → package → spine → EPUB3 nav / EPUB2 NCX → words + anchor map
internal/reader   Playback state (no goroutines): constant-interval timing on a monotonic clock, ORP splitting
internal/store    Per-book progress as versioned JSON, keyed by SHA-256 of the file; atomic writes
internal/ui       Fyne window: contents tree, custom ORP canvas, controls, settings, import states
```

The parser produces a structured `Book { spine, toc, words }`: spine order defines playback, the TOC resolves each entry's `href` + fragment to the first readable word at or after the target element, and entries that point nowhere readable are shown as unavailable rather than jumping elsewhere. Non-linear spine items stay out of sequential playback. See [rsvp-reader-spec.md](rsvp-reader-spec.md) for the full product and import contract.

Playback advances at most one word per timer tick anchored to the previous display time, so sleep, suspension, or window inactivity can never cause a burst of skipped words. Resume re-shows the current word for a full interval.

## Development

```bash
go build ./...
go vet ./...
go test ./...
```

Tests include synthetic EPUB fixtures (same-file fragments, nested NCX, spine-vs-filename order, broken TOC, ghost anchors, non-linear spine) and headless Fyne UI tests. Real-book spot checks live behind the standard `go test` run in `internal/epub`.

## Limits (first release)

- No DRM/encrypted books; no image-only or fixed-layout EPUBs, and no scanned-image PDFs
- RSVP shows text only — images, tables, and math layout are not reproduced (image `alt` text may be included)
- English / left-to-right, space-separated text is the primary experience; UTF-8 is preserved throughout
- Pasted-text sessions are stored locally (up to 2 MB) so they resume and reopen like files
