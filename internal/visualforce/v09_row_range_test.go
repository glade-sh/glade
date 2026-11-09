package visualforce_test

import "testing"

func TestV09RowRange(t *testing.T) {
	for _, tc := range []struct {
		input      string
		start, end int
		valid      bool
	}{
		{"", 0, 278, true},
		{"0:1", 0, 1, true},
		{"1:277", 1, 277, true},
		{"277:278", 277, 278, true},
		{"0:278", 0, 278, true},
		{"0:0", 0, 0, false},
		{"-1:2", 0, 0, false},
		{"2:1", 0, 0, false},
		{"0:279", 0, 0, false},
		{"1", 0, 0, false},
		{" 0:1", 0, 0, false},
		{"x:2", 0, 0, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			start, end, err := v09RowRange(tc.input, 278)
			if (err == nil) != tc.valid || tc.valid && (start != tc.start || end != tc.end) {
				t.Fatalf("v09RowRange(%q) = %d, %d, %v; want %d, %d, valid=%v", tc.input, start, end, err, tc.start, tc.end, tc.valid)
			}
		})
	}
}
