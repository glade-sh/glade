package visualforce_test

import (
	"strconv"
	"testing"
)

// The parts must cover every row exactly once, with and without a selection.
func TestV09PartsCoverEveryRowOnce(t *testing.T) {
	for _, count := range []int{1, 3, 4, 278, 1000} {
		for _, selection := range []string{"", "0:" + strconv.Itoa(count), "0:1", strconv.Itoa(count-1) + ":" + strconv.Itoa(count)} {
			wantStart, wantEnd, err := v09RowRange(selection, count)
			if err != nil {
				continue
			}
			rows := make([]int, count)
			for part := 0; part < v09PartCount; part++ {
				start, end, ok, err := v09SelectedRows(part, count, selection)
				if err != nil || ok != (start < end) {
					t.Fatalf("count %d part %d %q: %d:%d ok=%v err=%v", count, part, selection, start, end, ok, err)
				}
				if ok {
					for row := start; row < end; row++ {
						rows[row]++
					}
				}
			}
			for row, runs := range rows {
				if want := row >= wantStart && row < wantEnd; (runs == 1) != want || runs > 1 {
					t.Fatalf("count %d %q: row %d ran %d times, selected=%v", count, selection, row, runs, want)
				}
			}
		}
	}
	if start, end := v09PartRange(0, 278); start != 0 {
		t.Fatalf("part 1 must hold row zero (control suites), got %d:%d", start, end)
	}
	if _, _, _, err := v09SelectedRows(0, 278, "0:279"); err == nil {
		t.Fatal("an invalid selection must fail")
	}
}

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
