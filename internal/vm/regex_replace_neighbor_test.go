package vm

import "testing"

// These uncaptured replacement shapes retain the origin/main results.
func TestRegexReplaceUncapturedNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name, input, pattern, replacement, want string
		all                                     bool
	}{
		{"ascii", "abc", "", "-", "-a-b-c-", true},
		{"bmp", "aéb", "x*", "-", "-a-é-b-", true},
		{"assertion", "😀a", "(?=.)", "!", "!😀!a", true},
		{"first", "a😀b", "", "-", "-a😀b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stringRegexReplace("String.replaceAll", tc.input, []Value{String(tc.pattern), String(tc.replacement)}, tc.all)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, err %v; want %q", got, err, tc.want)
			}
		})
	}
}
