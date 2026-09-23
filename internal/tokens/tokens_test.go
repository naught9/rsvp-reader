package tokens

import (
	"reflect"
	"testing"
)

func TestSplitWords(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"this-word", []string{"this-", "word"}},
		{"state-of-the-art", []string{"state-", "of-", "the-", "art"}},
		{"e-mail", []string{"e-", "mail"}},
		{"word\u2014word", []string{"word\u2014", "word"}}, // em dash
		{"word\u2013word", []string{"word\u2013", "word"}}, // en dash
		{"word\u2010word", []string{"word\u2010", "word"}}, // hyphen
		{"word\u2011word", []string{"word\u2011", "word"}}, // non-breaking hyphen
		{"word\u2012word", []string{"word\u2012", "word"}}, // figure dash
		{"word\u2212word", []string{"word\u2212", "word"}}, // minus sign
		{"\u2014said", []string{"\u2014", "said"}},         // leading mark stands alone
		{"word\u2014", []string{"word\u2014"}},             // trailing mark stays
		{"well--known", []string{"well-", "-", "known"}},   // double mark
		{"don't stop", []string{"don't", "stop"}},          // apostrophe untouched
		{"1945 12,109", []string{"1945", "12,109"}},        // digits untouched
		{"a-b", []string{"a-", "b"}},
		{"-", []string{"-"}},                                       // lone mark passes through
		{"en\u00ADvi\u00ADron\u00ADment", []string{"environment"}}, // soft hyphens stripped
		{"Blood/Brain", []string{"Blood/Brain"}},                   // slash untouched
		{"  spaced   out  ", []string{"spaced", "out"}},
	}
	for _, c := range cases {
		if got := SplitWords(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("SplitWords(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
