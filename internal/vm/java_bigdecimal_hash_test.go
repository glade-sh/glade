package vm

import "testing"

// Values follow java.math.BigDecimal.hashCode: 31*unscaledHash + scale. The
// scale-0 row (1 -> 31) is pinned by native surf_collections_strings cs10.
func TestJavaBigDecimalHashCode(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int32
	}{
		{"1", 31},
		{"0", 0},
		{"-1", -31},
		{"1.50", 31*150 + 2},
		{"1E+3", 31*1 - 3},
		{"4294967296", 31*(1*31+0) + 0},
	} {
		if got := javaBigDecimalHashCode(Value{Kind: ValueDecimal, Text: tc.text}); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.text, got, tc.want)
		}
	}
}
