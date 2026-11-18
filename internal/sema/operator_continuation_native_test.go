package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturedOperatorContinuation(t *testing.T) {
	const fixture = "testdata/operator_continuation"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 17 || strings.Join(rows[0], "|") != "case|api|route|outcome" {
		t.Fatalf("invalid native matrix: %d rows", len(rows))
	}
	seen := map[string]bool{}
	controls := map[string]string{"bad_compare": "Comparison arguments must be compatible types: Integer, Boolean", "bad_generic": "Illegal assignment from List<Integer> to List<String>"}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") || row[2] != "metadata-deploy" {
			t.Fatalf("invalid native row: %v", row)
		}
		name, api := row[0], row[1]
		key := name + "/" + api
		if seen[key] {
			t.Fatalf("duplicate row: %s", key)
		}
		seen[key] = true
		expected, rejection := controls[name]
		if (!rejection && row[3] != "accepted") || (rejection && !strings.Contains(row[3], expected)) {
			t.Fatalf("invalid native outcome: %v", row)
		}
		t.Run(key, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(fixture, name+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"OpProbe.cls": string(source)}, api)
			if result.HasErrors() != rejection {
				t.Fatalf("native outcome %q; diagnostics: %#v", row[3], result.Diagnostics)
			}
		})
	}
	if len(seen) != 16 {
		t.Fatalf("incomplete native observations: %d", len(seen))
	}
}
