package compile_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Row-split conformance tests run one top-level test per part so the package
// shard planner can place the parts on different CI shards. The parts of a
// family cover every row exactly once per API, and a plain go test runs all of
// them. A family's ROW_RANGE variable (start:end, zero-based, half-open) is
// intersected with each part, so selecting rows still runs each of them once;
// a part with no selected row skips.
//
// The fixtures list their rows by kind (compile rows, then browser rows), so
// part p takes every row whose index is p modulo the part count. A contiguous
// range would hand all the browser rows to one part.

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

// selectedPartRows returns the indices of count rows that part (zero-based) of
// parts replays, limited to the optional selection. The result is empty when
// no row is selected.
func selectedPartRows(part, parts, count int, selection string) ([]int, error) {
	start, end, err := rowSelection(selection, count)
	if err != nil {
		return nil, err
	}
	var rows []int
	for index := start; index < end; index++ {
		if index%parts == part {
			rows = append(rows, index)
		}
	}
	return rows, nil
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
				runs := make([]int, count)
				for part := range parts {
					rows, err := selectedPartRows(part, parts, count, selection)
					if err != nil {
						t.Fatalf("parts %d count %d part %d %q: %v", parts, count, part, selection, err)
					}
					for _, row := range rows {
						runs[row]++
					}
				}
				for row, n := range runs {
					if want := row >= wantStart && row < wantEnd; (n == 1) != want || n > 1 {
						t.Fatalf("parts %d count %d %q: row %d ran %d times, selected=%v", parts, count, selection, row, n, want)
					}
				}
			}
		}
	}
	if _, err := selectedPartRows(0, 2, 204, "0:205"); err == nil {
		t.Fatal("an invalid selection must fail")
	}
}

// The browser rows follow the compile rows in the fixtures, so every part must
// get about the same number of each kind.
func TestRowPartsInterleaveRowKinds(t *testing.T) {
	const compileRows, browserRows = 94, 110
	for part := range 2 {
		rows, err := selectedPartRows(part, 2, compileRows+browserRows, "")
		if err != nil {
			t.Fatal(err)
		}
		compileCount := 0
		for _, row := range rows {
			if row < compileRows {
				compileCount++
			}
		}
		if browserCount := len(rows) - compileCount; compileCount != compileRows/2 || browserCount != browserRows/2 {
			t.Fatalf("part %d holds %d compile and %d browser rows", part, compileCount, browserCount)
		}
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
