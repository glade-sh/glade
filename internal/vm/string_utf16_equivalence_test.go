package vm

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

// The optimization must preserve the runtime's existing UTF-16 semantics,
// including replacement of malformed UTF-8 and signed hash overflow.
func TestStringUTF16AndHashEquivalence(t *testing.T) {
	inputs := []string{"", "ASCII\x00z", "éKΣ", "e\u0301😀𐐷", "\xff\xc0\xe2\x82", strings.Repeat("𐐷Abé", 1000)}
	for _, input := range inputs {
		units := utf16.Encode([]rune(input))
		if got := apexStringUTF16Units(input); !reflect.DeepEqual(got, units) {
			t.Fatalf("units for %q differ: %v versus %v", input, got, units)
		}
		var want int32
		for _, unit := range units {
			want = 31*want + int32(unit)
		}
		if got := javaStringHashCode(input); got != want {
			t.Fatalf("hash for %q: got %d, want %d", input, got, want)
		}
	}
}
