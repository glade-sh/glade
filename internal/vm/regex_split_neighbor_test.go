package vm

import (
	"reflect"
	"testing"
)

// These uncaptured shapes retain the pre-change behavior.
func TestRegexSplitUncapturedNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		limit int64
		want  []string
	}{
		{"ascii", "abc", 0, []string{"a", "b", "c"}},
		{"bmp", "aé", 0, []string{"a", "é"}},
		{"positiveLimit", "a😀b", 2, []string{"a", "😀b"}},
		{"negativeLimit", "😀b", -1, []string{"😀", "b", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := splitStringCharacters(tc.text, tc.limit); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
