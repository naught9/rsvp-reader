package reader

import (
	"testing"
	"time"
)

func TestORPTable(t *testing.T) {
	cases := map[string]int{
		"a":            0,
		"the":          0,
		"test":         1,
		"hello":        1,
		"reading":      2,
		"presents":     2,
		"presentation": 3,
	}
	for w, want := range cases {
		if got := ORPIndex(w); got != want {
			t.Errorf("ORPIndex(%q) = %d, want %d", w, got, want)
		}
	}
}

func TestORPSkipsLeadingPunctuation(t *testing.T) {
	// Ref behavior: getActualORPIndex('"hello') lands on 'e'.
	if got := string([]rune("\"hello")[ORPRuneIndex("\"hello")]); got != "e" {
		t.Fatalf("leading-quote ORP = %q, want %q", got, "e")
	}
}

func TestORPMultibyteSafe(t *testing.T) {
	words := []string{"Ará", "“hello”", "日本語テスト", "🎉party", "God’s", "SMÉAGOL"}
	for _, w := range words {
		b, f, a := SplitForDisplay(w)
		if b+f+a != w {
			t.Fatalf("SplitForDisplay(%q) rejoins to %q", w, b+f+a)
		}
		if f == "" {
			t.Fatalf("SplitForDisplay(%q) has empty focal", w)
		}
	}
	// Focal of a CJK word must be a full rune, not a split byte.
	_, f, _ := SplitForDisplay("日本語")
	if len([]rune(f)) != 1 {
		t.Fatalf("CJK focal = %q, want single rune", f)
	}
}

func TestInterval(t *testing.T) {
	if Interval(300) != 200*time.Millisecond {
		t.Fatalf("300 WPM interval = %v, want 200ms", Interval(300))
	}
	if Interval(0) != Interval(MinWPM) || Interval(99999) != Interval(MaxWPM) {
		t.Fatalf("Interval does not clamp")
	}
}

func TestPlayPauseResumeHold(t *testing.T) {
	p := NewPlayer([]string{"a", "b", "c"}, 600) // 100ms/word
	now := time.Now()
	p.Play(now)
	if !p.Playing() || p.Current() != "a" {
		t.Fatalf("after Play: playing=%v current=%q", p.Playing(), p.Current())
	}
	// Before deadline: no advance.
	if p.Tick(now.Add(50 * time.Millisecond)) {
		t.Fatalf("advanced before deadline")
	}
	// At deadline: one advance.
	if !p.Tick(now.Add(100*time.Millisecond)) || p.Current() != "b" {
		t.Fatalf("did not advance at deadline, current=%q", p.Current())
	}
	p.Pause()
	if p.Playing() {
		t.Fatalf("Pause did not stop")
	}
	// Resume holds current word a full interval: ticking at 50ms must not advance.
	p.Resume(now)
	if p.Tick(now.Add(50*time.Millisecond)) || p.Current() != "b" {
		t.Fatalf("Resume skipped the held word")
	}
	if !p.Tick(now.Add(100*time.Millisecond)) || p.Current() != "c" {
		t.Fatalf("Resume did not advance after full interval")
	}
}

func TestNoBurstAfterSleep(t *testing.T) {
	p := NewPlayer([]string{"a", "b", "c", "d"}, 600)
	now := time.Now()
	p.Play(now)
	// Simulate 10s of system sleep: exactly one advance, deadline re-anchored.
	if !p.Tick(now.Add(10 * time.Second)) {
		t.Fatalf("expected single advance after gap")
	}
	if p.Current() != "b" {
		t.Fatalf("after gap current=%q, want %q (no burst)", p.Current(), "b")
	}
	if p.Tick(now.Add(10*time.Second + 50*time.Millisecond)) {
		t.Fatalf("burst: advanced again immediately after gap")
	}
}

func TestEndOfBook(t *testing.T) {
	p := NewPlayer([]string{"a", "b"}, 600)
	now := time.Now()
	p.Play(now)
	p.Tick(now.Add(100 * time.Millisecond)) // -> b
	if p.Tick(now.Add(200 * time.Millisecond)) {
		t.Fatalf("Tick past last word reported advance")
	}
	if !p.Ended() || p.Playing() || p.Current() != "b" {
		t.Fatalf("end state: ended=%v playing=%v current=%q", p.Ended(), p.Playing(), p.Current())
	}
	// Play again restarts from top.
	p.Play(now)
	if p.Current() != "a" || p.Ended() {
		t.Fatalf("replay did not restart")
	}
}

func TestSeekPausesAndSteps(t *testing.T) {
	p := NewPlayer([]string{"a", "b", "c", "d"}, 300)
	now := time.Now()
	p.Play(now)
	p.Seek(2)
	if p.Playing() || p.Current() != "c" {
		t.Fatalf("Seek must pause at target")
	}
	p.StepPrev()
	if p.Current() != "b" {
		t.Fatalf("StepPrev = %q", p.Current())
	}
	p.StepNext()
	p.StepNext()
	if p.Current() != "d" {
		t.Fatalf("StepNext = %q", p.Current())
	}
	if p.StepNext() {
		t.Fatalf("StepNext past end reported true")
	}
	p.SeekFraction(0.5)
	if p.Current() != "b" {
		t.Fatalf("SeekFraction(0.5) of 4 = %q, want b", p.Current())
	}
}

func TestWPMStepsAndClamp(t *testing.T) {
	p := NewPlayer([]string{"a"}, 300)
	p.StepUp()
	if p.WPM() != 325 {
		t.Fatalf("StepUp = %d", p.WPM())
	}
	p.StepDown()
	p.StepDown()
	if p.WPM() != 275 {
		t.Fatalf("StepDown x2 = %d", p.WPM())
	}
	p.SetWPM(5)
	if p.WPM() != MinWPM {
		t.Fatalf("SetWPM(5) = %d", p.WPM())
	}
	p.SetWPM(5000)
	if p.WPM() != MaxWPM {
		t.Fatalf("SetWPM(5000) = %d", p.WPM())
	}
	// Speed change must not move position.
	p2 := NewPlayer([]string{"a", "b", "c"}, 300)
	p2.Seek(1)
	p2.SetWPM(600)
	if p2.Pos() != 1 {
		t.Fatalf("SetWPM moved position to %d", p2.Pos())
	}
}

func TestTimeRemaining(t *testing.T) {
	p := NewPlayer(make([]string, 301), 300)
	if got := p.TimeRemaining(); got != "1:00" {
		t.Fatalf("TimeRemaining = %q, want 1:00", got)
	}
}

func TestTrailingPause(t *testing.T) {
	major := []string{"word.", "Really?", "Stop!", "so\u2026", "wait...", "end.\u201d", "go.')", "."}
	for _, w := range major {
		if got := TrailingPause(w); got != 2 {
			t.Errorf("TrailingPause(%q) = %d, want 2", w, got)
		}
	}
	minor := []string{"word,", "say:", "pause;", "yes,\u201d"}
	for _, w := range minor {
		if got := TrailingPause(w); got != 1 {
			t.Errorf("TrailingPause(%q) = %d, want 1", w, got)
		}
	}
	none := []string{"word", "don't", "3.14", "v2.0", "e.g.", "U.S.", "J.", "12,109", "well-known", "dogs'", "\u2014", "this-", ""}
	for _, w := range none {
		if got := TrailingPause(w); got != 0 {
			t.Errorf("TrailingPause(%q) = %d, want 0", w, got)
		}
	}
	if !EndsSentence("word.") || EndsSentence("word,") {
		t.Errorf("EndsSentence compatibility")
	}
}

func TestTickSentenceBeat(t *testing.T) {
	now := time.Now()
	p := NewPlayer([]string{"one", "two.", "three"}, 600) // 100ms beat
	p.SentencePause = true
	p.Play(now)
	if !p.Tick(now.Add(100*time.Millisecond)) || p.Current() != "two." {
		t.Fatalf("advance to %q", p.Current())
	}
	// "two." lingers two extra beats: deadline three intervals out.
	if d := p.Deadline().Sub(now); d != 400*time.Millisecond {
		t.Fatalf("deadline = %v, want 400ms", d)
	}
	if p.Tick(now.Add(350 * time.Millisecond)) {
		t.Fatalf("advanced mid-beat")
	}
	r := NewPlayer([]string{"one", "two,", "three"}, 600)
	r.SentencePause = true
	r.Play(now)
	r.Tick(now.Add(100 * time.Millisecond))
	// Clause mark: one extra beat, two intervals out.
	if d := r.Deadline().Sub(now); d != 300*time.Millisecond {
		t.Fatalf("clause deadline = %v, want 300ms", d)
	}
	p.SentencePause = false
	q := NewPlayer([]string{"one", "two.", "three"}, 600)
	q.Play(now)
	q.Tick(now.Add(100 * time.Millisecond))
	if d := q.Deadline().Sub(now); d != 200*time.Millisecond {
		t.Fatalf("no-pause deadline = %v, want 200ms", d)
	}
}
