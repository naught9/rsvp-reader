package tokens

import (
	"strings"
)

// Hyphen characters split in EPUB prose. SOFT HYPHEN is handled
// separately: it is an invisible formatting hint, not content.
var hyphenRunes = map[rune]bool{
	'\u002D': true, // HYPHEN-MINUS
	'\u2010': true, // HYPHEN
	'\u2011': true, // NON-BREAKING HYPHEN
	'\u2012': true, // FIGURE DASH
	'\u2013': true, // EN DASH
	'\u2014': true, // EM DASH
	'\u2015': true, // HORIZONTAL BAR
	'\u2212': true, // MINUS SIGN
}

const softHyphen = "­"

// IsHyphen reports whether r is a word-splitting hyphen character.
func IsHyphen(r rune) bool { return hyphenRunes[r] }

// SplitWords tokenizes text for serial display: whitespace split, then
// hyphen compounds break with the mark kept on the left part, so
// "this-word" reads as "this-" then "word". Soft hyphens are stripped:
// they mark print line-break opportunities inside words, and splitting
// there would shred the word.
func SplitWords(text string) []string {
	var out []string
	for _, field := range strings.Fields(text) {
		field = strings.ReplaceAll(field, softHyphen, "")
		if field == "" {
			continue
		}
		out = append(out, splitHyphens(field)...)
	}
	return out
}

// splitHyphens breaks one whitespace-delimited token on hyphen marks.
// The mark stays with the preceding part; a leading mark becomes its own
// token and a trailing mark stays attached. Tokens without marks, lone
// marks, and digits pass through untouched.
func splitHyphens(tok string) []string {
	if !strings.ContainsFunc(tok, IsHyphen) {
		return []string{tok}
	}
	var out []string
	var cur strings.Builder
	for _, r := range tok {
		if !IsHyphen(r) {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() == 0 {
			// Leading or repeated mark: extend a lone-mark token,
			// else start one.
			if n := len(out); n > 0 && allMarks(out[n-1]) {
				out[n-1] += string(r)
			} else {
				out = append(out, string(r))
			}
			continue
		}
		out = append(out, cur.String()+string(r))
		cur.Reset()
	}
	if rest := cur.String(); rest != "" {
		out = append(out, rest)
	}
	if len(out) == 0 {
		return []string{tok}
	}
	return out
}

func allMarks(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !IsHyphen(r) {
			return false
		}
	}
	return true
}
