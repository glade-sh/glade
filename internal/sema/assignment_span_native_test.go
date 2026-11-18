package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturedAssignmentSpanControls(t *testing.T) {
	const fixture = "testdata/assignment_span_controls"
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
	controls := map[string]string{"if_bad": "Illegal assignment from Integer to String", "for_bad": "Illegal assignment from Integer to String"}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") || row[2] != "metadata-deploy" {
			t.Fatalf("invalid native row: %v", row)
		}
		name, api := row[0], row[1]
		key := name + "/" + api
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		expected, reject := controls[name]
		if (!reject && row[3] != "accepted") || (reject && !strings.Contains(row[3], expected)) {
			t.Fatalf("invalid native outcome: %v", row)
		}
		t.Run(key, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixture, name+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			const prefix = "public class SpanProbe { public static Integer run() { "
			body, ok := strings.CutPrefix(source, prefix)
			if !ok {
				t.Fatal("fixture missing body prefix")
			}
			body, ok = strings.CutSuffix(body, " } }\n")
			if !ok {
				t.Fatal("fixture missing body suffix")
			}
			want := strings.Count(body, "Integer value")
			got := 0
			for _, m := range findSemaLocalDeclMatches(body) {
				if body[m[4]:m[5]] == "value" && strings.TrimSpace(body[m[2]:m[3]]) == "Integer" {
					got++
				}
			}
			if got != want {
				t.Fatalf("captured locals found %d, want %d", got, want)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"SpanProbe.cls": source}, api)
			if result.HasErrors() != reject {
				t.Fatalf("native outcome %q; local diagnostics %#v", row[3], result.Diagnostics)
			}
		})
	}
	if len(seen) != 16 {
		t.Fatalf("incomplete native matrix: %d", len(seen))
	}
}
