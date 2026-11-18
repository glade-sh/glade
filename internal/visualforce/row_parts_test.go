package visualforce_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Row-split conformance tests run one top-level test per contiguous row range so
// the package shard planner can place the parts on different CI shards. The
// parts of a family cover every row exactly once per API, and a plain go test
// runs all of them. A family's ROW_RANGE variable (start:end, zero-based,
// half-open) is intersected with each part's range, so selecting rows still
// runs each of them once; a part with no selected row skips.

// rowPartRange returns the half-open row range of part (zero-based) out of
// parts, for count rows. Ranges of consecutive parts are contiguous and cover
// [0, count).
func rowPartRange(part, parts, count int) (int, int) {
	return part * count / parts, (part + 1) * count / parts
}

// rowSelection parses a start:end selection of count rows. An empty value
// selects every row.
func rowSelection(value string, count int) (int, int, error) {
	if value == "" {
		return 0, count, nil
	}
	first, last, found := strings.Cut(value, ":")
	if !found {
		return 0, 0, fmt.Errorf("row range %q must be start:end", value)
	}
	start, err := strconv.Atoi(first)
	if err != nil {
		return 0, 0, fmt.Errorf("row range %q start: %w", value, err)
	}
	end, err := strconv.Atoi(last)
	if err != nil {
		return 0, 0, fmt.Errorf("row range %q end: %w", value, err)
	}
	if start < 0 || end > count || start >= end {
		return 0, 0, fmt.Errorf("row range %q must satisfy 0 <= start < end <= %d", value, count)
	}
	return start, end, nil
}

// selectedPartRows intersects part's range with the optional selection. ok is
// false when no row is selected.
func selectedPartRows(part, parts, count int, selection string) (start, end int, ok bool, err error) {
	start, end = rowPartRange(part, parts, count)
	selectionStart, selectionEnd, err := rowSelection(selection, count)
	if err != nil {
		return 0, 0, false, err
	}
	start, end = max(start, selectionStart), min(end, selectionEnd)
	return start, end, start < end, nil
}

// rowPartReportPath inserts ".partNofM" before the extension so parts in one
// process do not truncate each other's report.
func rowPartReportPath(path string, part, parts int) string {
	extension := filepath.Ext(path)
	return fmt.Sprintf("%s.part%dof%d%s", strings.TrimSuffix(path, extension), part+1, parts, extension)
}

func TestRowPartsCoverEveryRowOnce(t *testing.T) {
	for _, parts := range []int{2, 3} {
		for _, count := range []int{1, 3, 8, 204, 249, 368} {
			for _, selection := range []string{"", "0:" + strconv.Itoa(count), "0:1", strconv.Itoa(count-1) + ":" + strconv.Itoa(count)} {
				wantStart, wantEnd, err := rowSelection(selection, count)
				if err != nil {
					continue
				}
				rows := make([]int, count)
				for part := range parts {
					start, end, ok, err := selectedPartRows(part, parts, count, selection)
					if err != nil || ok != (start < end) {
						t.Fatalf("parts %d count %d part %d %q: %d:%d ok=%v err=%v", parts, count, part, selection, start, end, ok, err)
					}
					if ok {
						for row := start; row < end; row++ {
							rows[row]++
						}
					}
				}
				for row, runs := range rows {
					if want := row >= wantStart && row < wantEnd; (runs == 1) != want || runs > 1 {
						t.Fatalf("parts %d count %d %q: row %d ran %d times, selected=%v", parts, count, selection, row, runs, want)
					}
				}
			}
		}
	}
	if _, _, _, err := selectedPartRows(0, 2, 204, "0:205"); err == nil {
		t.Fatal("an invalid selection must fail")
	}
}

func TestRowSelection(t *testing.T) {
	for _, tc := range []struct {
		input      string
		start, end int
		valid      bool
	}{
		{"", 0, 10, true}, {"0:1", 0, 1, true}, {"3:7", 3, 7, true}, {"0:10", 0, 10, true},
		{"0:0", 0, 0, false}, {"-1:2", 0, 0, false}, {"2:1", 0, 0, false}, {"0:11", 0, 0, false},
		{"1", 0, 0, false}, {" 0:1", 0, 0, false}, {"x:2", 0, 0, false},
	} {
		start, end, err := rowSelection(tc.input, 10)
		if (err == nil) != tc.valid || tc.valid && (start != tc.start || end != tc.end) {
			t.Fatalf("rowSelection(%q) = %d, %d, %v; want %d, %d, valid=%v", tc.input, start, end, err, tc.start, tc.end, tc.valid)
		}
	}
}
