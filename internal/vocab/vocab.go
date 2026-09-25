package vocab

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"strconv"
	"strings"
	"sync"

	"rsvp-reader/internal/tokens"
)

//go:embed data/en_top50k.tsv.gz
var freqGzip []byte

var (
	loadOnce sync.Once
	zipfByW  map[string]float32
)

// Zipf tiers for RarityBeats, calibrated to wordfreq's English scale.
// Familiar words read at full speed; the 2.5–3 band (roughly once per
// million down to the table floor) breathes one beat; below that —
// effectively absent from the table — earns two.
const (
	zipfFamiliar = 3.0
	zipfRare     = 2.5
	// maxWordLen earns a length beat beyond this; digits never slow.
	longWordLen = 10
)

func load() {
	loadOnce.Do(func() {
		zipfByW = map[string]float32{}
		gz, err := gzip.NewReader(bytes.NewReader(freqGzip))
		if err != nil {
			return
		}
		defer gz.Close()
		sc := bufio.NewScanner(gz)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			word, zstr, found := strings.Cut(sc.Text(), "\t")
			if !found || word == "" {
				continue
			}
			z, err := strconv.ParseFloat(zstr, 32)
			if err != nil {
				continue
			}
			zipfByW[word] = float32(z)
		}
	})
}

// ZipfOf returns the table Zipf value and whether the word is listed.
// Normalizes like RarityBeats; exported for tests and spot checks.
func ZipfOf(word string) (float32, bool) {
	load()
	z, ok := zipfByW[normalize(word)]
	return z, ok
}

// RarityBeats is the extra display beats an unfamiliar word earns: 0 at
// Zipf 3 and up, 1 down to Zipf 2.5, 2 below that or absent from the
// table (invented words land here) — plus a length beat for very long
// words. Guards: digits, abbreviation remnants, and hyphen-split
// fragments' trailing marks never slow.
func RarityBeats(word string) int {
	load()
	w := normalize(word)
	if w == "" || strings.ContainsAny(w, "0123456789") {
		return 0
	}
	if strings.Contains(w, ".") {
		return 0 // abbreviation remnant ("e.g", "U.S")
	}
	beats := 0
	if z, ok := zipfByW[w]; !ok || z < zipfRare {
		beats = 2
	} else if z < zipfFamiliar {
		beats = 1
	}
	if len([]rune(w)) > longWordLen {
		beats++
	}
	return beats
}

// normalize lowercases, folds curly apostrophes, and strips surrounding
// punctuation ("said," looks up "said"; "\u201cI" looks up "i"), so only
// the word itself meets the table. Hyphen-split trailing marks go too
// ("fuligin-" looks up "fuligin").
func normalize(word string) string {
	w := strings.ToLower(word)
	w = strings.ReplaceAll(w, "\u2019", "'")
	w = strings.ReplaceAll(w, "\u2018", "'")
	w = strings.TrimLeftFunc(w, isOpener)
	w = strings.TrimRightFunc(w, isCloser)
	return w
}

func isOpener(r rune) bool {
	return strings.ContainsRune("\"'\u2018\u2019\u201c\u201d\u00ab\u00bb\u00bf\u00a1([{", r)
}

func isCloser(r rune) bool {
	return tokens.IsHyphen(r) || strings.ContainsRune("\"'\u2018\u2019\u201c\u201d\u00ab\u00bb.,!?:;\u2026)]}", r)
}
