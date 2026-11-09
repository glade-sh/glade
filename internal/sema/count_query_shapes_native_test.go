package sema

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// Native rows (testdata/count_query_shapes/org.tsv, API 62.0 and 67.0): a
// scalar COUNT() query is an Integer. It cannot be indexed, assigned to a
// non-numeric type or iterated, and COUNT() cannot be combined with GROUP BY.
func TestCountQueryShapesNativeRows(t *testing.T) {
	t.Parallel()
	const dir = "testdata/count_query_shapes"
	// These rows are rejected on the native line with Glade's general wording
	// for the same rule (initializer, return, loop and parser diagnostics).
	localWording := map[string]bool{
		"SnA4": true, "SnB1": true, "SnB4": true, "SnB5": true, "SnB8": true,
		"SnB10": true, "SnB11": true, "SnB12": true, "SnC1": true, "SnC2": true,
		"SnC3": true, "SnC4": true, "SnC5": true, "SnD4": true,
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	type nativeRow struct {
		outcome, problem string
		line, column     int
	}
	first := map[string]nativeRow{}
	for i, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		cols := strings.Split(line, "\t")
		if i == 0 {
			continue
		}
		if len(cols) != 6 {
			t.Fatalf("org.tsv row %d: %q", i, line)
		}
		key := cols[0] + "/" + cols[1]
		if _, seen := first[key]; seen {
			continue
		}
		row := nativeRow{outcome: cols[2], problem: cols[5]}
		if row.outcome == "rejected" {
			row.line, _ = strconv.Atoi(cols[3])
			row.column, _ = strconv.Atoi(cols[4])
		}
		first[key] = row
	}
	if len(first) != 60 {
		t.Fatalf("native classes = %d, want 30 per API", len(first))
	}
	for key, row := range first {
		api, class, _ := strings.Cut(key, "/")
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(filepath.Join(dir, "classes", class+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{class + ".cls": string(source)}, api)
			var errs []diagnostic.Diagnostic
			for _, item := range result.Diagnostics {
				if item.Severity == diagnostic.Error {
					errs = append(errs, item)
				}
			}
			if row.outcome == "accepted" {
				if len(errs) != 0 {
					t.Fatalf("native compiles; got %#v", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("native rejects %d:%d %q; Glade reports nothing", row.line, row.column, row.problem)
			}
			if class != "SnD4" && len(errs) != 1 {
				t.Fatalf("want one diagnostic like native, got %#v", errs)
			}
			got := errs[0]
			if got.Range == nil || got.Range.Start.Line != row.line {
				t.Fatalf("line: got %#v, native %d", got.Range, row.line)
			}
			if localWording[class] {
				return
			}
			if got.Message != row.problem && got.NativeMessage != row.problem {
				t.Fatalf("message %q (native %q), want %q", got.Message, got.NativeMessage, row.problem)
			}
			if strings.HasPrefix(row.problem, "COUNT() cannot be used with GROUP BY") && got.Range.Start.Column != row.column {
				t.Fatalf("column %d, native %d", got.Range.Start.Column, row.column)
			}
		})
	}
}
