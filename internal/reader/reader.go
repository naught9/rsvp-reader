package reader

import (
	"fmt"
	"strings"

	"rsvp-reader/internal/vocab"
	"time"
	"unicode"
)

// Speed limits per spec: 50-1000 WPM in 25-WPM steps, default 300.
const (
	DefaultWPM = 300
	MinWPM     = 50
	MaxWPM     = 1000
	StepWPM    = 25
)

// ClampWPM constrains v to the valid range.
func ClampWPM(v int) int {
	if v < MinWPM {
		return MinWPM
	}
	if v > MaxWPM {
		return MaxWPM
	}
	return v
}

// SnapWPM rounds v to the nearest 25-WPM step, then clamps.
func SnapWPM(v int) int {
	return ClampWPM(((v + StepWPM/2) / StepWPM) * StepWPM)
}

// Interval is the constant per-word display duration at wpm.
func Interval(wpm int) time.Duration {
	wpm = ClampWPM(wpm)
	return time.Minute / time.Duration(wpm)
}

// ---------- Optimal Recognition Point ----------

// ORPIndex returns the letter-offset (counting only letters) of the focal
// letter, following the reference project's table:
//
//	1-3 letters: 1st, 4-5: 2nd, 6-9: 3rd, 10-12: 4th, 13+: log2-based.
func ORPIndex(word string) int {
	n := 0
	for _, r := range word {
		if unicode.IsLetter(r) {
			n++
		}
	}
	switch {
	case n <= 3:
		return 0
	case n <= 5:
		return 1
	case n <= 9:
		return 2
	case n <= 12:
		return 3
	default:
		return ilog2(n-1) + 1
	}
}

func ilog2(n int) int {
	l := 0
	for n >>= 1; n > 0; n >>= 1 {
		l++
	}
	return l
}

// ORPRuneIndex converts the letter-offset to a rune index, skipping leading
// non-letters (e.g. opening quotes) so the highlight lands on a letter.
func ORPRuneIndex(word string) int {
	runes := []rune(word)
	if len(runes) == 0 {
		return 0
	}
	target := ORPIndex(word)
	letters := 0
	for i, r := range runes {
		if unicode.IsLetter(r) {
			if letters == target {
				return i
			}
			letters++
		}
	}
	return len(runes) - 1
}

// SplitForDisplay splits word into (before, focal, after) on rune
// boundaries; it never splits a multibyte character.
func SplitForDisplay(word string) (before, focal, after string) {
	runes := []rune(word)
	if len(runes) == 0 {
		return "", "", ""
	}
	i := ORPRuneIndex(word)
	return string(runes[:i]), string(runes[i : i+1]), string(runes[i+1:])
}

// ---------- Player ----------

// Player is pure playback state: no goroutines, no timers. The UI drives it
// by calling Tick when its timer fires, passing the monotonic now.
// Advancing at most one word per Tick guarantees a sleep/inactivity gap can
// never cause a burst of skipped words.
type Player struct {
	words    []string
	pos      int
	playing  bool
	ended    bool
	wpm      int
	deadline time.Time
	// SentencePause lingers extra beats on punctuation (clause one,
	// sentence two), aiding comprehension.
	SentencePause bool
	// RareWordPause lingers extra beats on uncommon, rare, and invented
	// words (vocab tiers 1-2 plus a length beat), so unknown words can
	// actually be read.
	RareWordPause bool

	// SectionAt maps a word index to its section label; wired by the UI
	// from the book's TOC. Nil means no labels.
	SectionAt func(idx int) string
}

// NewPlayer creates a paused player over words (copying the slice header only).
func NewPlayer(words []string, wpm int) *Player {
	return &Player{words: words, wpm: ClampWPM(wpm)}
}

func (p *Player) Len() int            { return len(p.words) }
func (p *Player) Pos() int            { return p.pos }
func (p *Player) Playing() bool       { return p.playing }
func (p *Player) Ended() bool         { return p.ended }
func (p *Player) WPM() int            { return p.wpm }
func (p *Player) Deadline() time.Time { return p.deadline }

// Current returns the word under the focal point ("" when empty).
func (p *Player) Current() string {
	if len(p.words) == 0 || p.pos < 0 || p.pos >= len(p.words) {
		return ""
	}
	return p.words[p.pos]
}

// Play starts (or restarts from the top when at the end) and schedules the
// first advance one full interval out.
func (p *Player) Play(now time.Time) {
	if len(p.words) == 0 {
		return
	}
	if p.ended || p.pos >= len(p.words) {
		p.pos = 0
		p.ended = false
	}
	p.playing = true
	p.deadline = now.Add(Interval(p.wpm))
}

// Pause freezes the current word.
func (p *Player) Pause() { p.playing = false }

// Resume re-shows the current word for a full interval before advancing;
// it never skips during suspension or window inactivity.
func (p *Player) Resume(now time.Time) {
	if len(p.words) == 0 || p.ended {
		return
	}
	p.playing = true
	p.deadline = now.Add(Interval(p.wpm))
}

// Tick reports whether the display should advance. When it returns
// advanced=true the caller should repaint; Current() is the word to show.
// A long gap (sleep, suspension) advances exactly one word and re-anchors
// the deadline to now, so playback resumes at pace without bursting.
func (p *Player) Tick(now time.Time) (advanced bool) {
	if !p.playing || len(p.words) == 0 || p.ended {
		return false
	}
	if now.Before(p.deadline) {
		return false
	}
	if p.pos+1 >= len(p.words) {
		p.playing = false
		p.ended = true
		return false
	}
	p.pos++
	p.deadline = now.Add(Interval(p.wpm))
	extra := 0
	if p.SentencePause {
		// Clause marks breathe one beat, sentence marks two.
		extra += TrailingPause(p.words[p.pos])
	}
	if p.RareWordPause {
		extra += vocab.RarityBeats(p.words[p.pos])
	}
	if extra > maxExtraBeats {
		extra = maxExtraBeats
	}
	if extra > 0 {
		p.deadline = p.deadline.Add(time.Duration(extra) * Interval(p.wpm))
	}
	return true
}

// maxExtraBeats caps stacked punctuation + rarity beats: the worst case
// (a rare sentence-final word) lingers ~1s at 300 WPM, never stalls.
const maxExtraBeats = 4

// TrailingPause is the extra beats a word earns from its final mark: 2
// for sentence ends (. ! ? ellipsis), 1 for clause marks (, : ;), 0
// otherwise. Closing quotes and brackets are ignored. Abbreviations are
// left alone as best-effort: words with interior periods (e.g., U.S.),
// digits (3.14), or a lone initial (J.) earn no sentence beat. "Mr."
// still breathes; the alternative is a dictionary, deliberately out of
// scope.
func TrailingPause(word string) int {
	w := strings.TrimRight(word, "\"'\u2019\u201d)]}")
	if w == "" {
		return 0
	}
	last := w[len(w)-1]
	if last == ',' || last == ':' || last == ';' {
		return 1
	}
	if last != '.' && last != '!' && last != '?' && !strings.HasSuffix(w, "\u2026") {
		return 0
	}
	if strings.HasSuffix(w, "..") {
		return 2 // ellipsis ("...") always breathes deep
	}
	body := strings.TrimSuffix(w, "\u2026")
	if body == "" {
		return 2 // a bare mark still breathes
	}
	body = body[:len(body)-1]
	if strings.ContainsAny(body, "0123456789") {
		return 0
	}
	if strings.Contains(body, ".") {
		return 0
	}
	if len([]rune(body)) == 1 && last == '.' {
		return 0
	}
	if last == '.' && commonAbbrev[strings.ToLower(body)] {
		return 0
	}
	return 2
}

// commonAbbrev holds titles that end in a period but never end a
// sentence ("Dr." lingers two beats without this). Deliberately tight:
// ordinary words are excluded, so a real sentence end never loses its
// beat to this list.
var commonAbbrev = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true,
	"jr": true, "sr": true, "prof": true, "rev": true,
	"st": true, "gen": true, "col": true, "capt": true,
	"lt": true, "sgt": true, "gov": true, "pres": true,
}

// EndsSentence reports whether a word closes a sentence.
func EndsSentence(word string) bool { return TrailingPause(word) == 2 }

// StepNext moves one word forward while paused. Returns false at the end.
func (p *Player) StepNext() bool {
	if len(p.words) == 0 || p.pos+1 >= len(p.words) {
		return false
	}
	p.pos++
	p.ended = false
	return true
}

// StepPrev moves one word back. Returns false at the start.
func (p *Player) StepPrev() bool {
	if len(p.words) == 0 || p.pos <= 0 {
		return false
	}
	p.pos--
	p.ended = false
	return true
}

// Seek jumps to idx and pauses, per spec (TOC selection pauses playback).
func (p *Player) Seek(idx int) {
	if len(p.words) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(p.words) {
		idx = len(p.words) - 1
	}
	p.pos = idx
	p.playing = false
	p.ended = idx == len(p.words)-1
}

// SeekFraction jumps to frac (0..1) of the book and pauses.
func (p *Player) SeekFraction(frac float64) {
	if len(p.words) == 0 {
		return
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	p.Seek(int(frac * float64(len(p.words)-1)))
}

// SetWPM changes speed; it takes effect on the next scheduled word and
// never resets the current position.
func (p *Player) SetWPM(wpm int) { p.wpm = ClampWPM(wpm) }

// StepUp/StepDown adjust speed in fixed 25-WPM steps.
func (p *Player) StepUp()   { p.wpm = ClampWPM(p.wpm + StepWPM) }
func (p *Player) StepDown() { p.wpm = ClampWPM(p.wpm - StepWPM) }

// Restart stops and returns to the first word, staying paused.
func (p *Player) Restart() {
	p.pos = 0
	p.playing = false
	p.ended = false
}

// Progress returns (current, total, fraction).
func (p *Player) Progress() (int, int, float64) {
	n := len(p.words)
	if n == 0 {
		return 0, 0, 0
	}
	return p.pos, n, float64(p.pos+1) / float64(n)
}

// Section returns the current section label, or "" when unknown.
func (p *Player) Section() string {
	if p.SectionAt == nil {
		return ""
	}
	return p.SectionAt(p.pos)
}

// TimeRemaining formats the estimated remaining time as M:SS at the
// current constant pace.
func (p *Player) TimeRemaining() string {
	if len(p.words) == 0 || p.wpm <= 0 {
		return "0:00"
	}
	remaining := len(p.words) - p.pos - 1
	if remaining < 0 {
		remaining = 0
	}
	secs := (remaining*60 + p.wpm - 1) / p.wpm // ceil
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}
