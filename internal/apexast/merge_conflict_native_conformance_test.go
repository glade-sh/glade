package apexast

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// Deployment rejects the conflicted class before either execution route can run.
func TestMergeConflictNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "merge_conflict_markers")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		var source string
		switch {
		case row[1] == "Failed" && row[2] == "4" && row[3] == "1" && row[4] == "Expecting '}' but was: '<'":
			source = "MarkerProbe.cls"
		case row[1] == "Succeeded" && row[4] == "clean twin":
			source = "clean/MarkerProbe.cls"
		default:
			continue
		}
		key := row[0] + "/" + source
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(dir, source))
			if err != nil {
				t.Fatal(err)
			}
			parser := NewParser()
			for _, route := range []string{"fresh", "warm"} {
				t.Run(route, func(t *testing.T) {
					file := parser.ParseSource("MarkerProbe.cls", string(body))
					hasError, parseError := false, false
					for _, d := range file.Diagnostics {
						if d.Severity == diagnostic.Error {
							hasError = true
							if d.Code == "APEXPARSE001" {
								parseError = true
							}
						}
					}
					if row[1] == "Failed" && !parseError {
						t.Fatalf("native rejected; diagnostics: %v", file.Diagnostics)
					}
					if row[1] == "Succeeded" && hasError {
						t.Fatalf("native accepted; diagnostics: %v", file.Diagnostics)
					}
				})
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("native matrix has %d cells, want 4", len(seen))
	}
}
