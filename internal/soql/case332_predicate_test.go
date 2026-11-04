package soql

import "testing"

func TestLikeAlternatingInputsPreserveEscapes(t *testing.T) {
	for _, tc := range []struct {
		text, pattern string
		want          bool
	}{
		{"abc", "a%", true}, {"", "a%", false}, {"a", "a%", true},
		{"a_b", `a\_b`, true}, {"acb", `a\_b`, false},
		{"a%b", `a\%b`, true}, {"abc", `a\%b`, false},
		{"Mixed", "m_x%", true}, {"ab", "___", false},
	} {
		if got := matchLikePattern(tc.text, tc.pattern); got != tc.want {
			t.Errorf("matchLikePattern(%q, %q) = %v; want %v", tc.text, tc.pattern, got, tc.want)
		}
	}
}
