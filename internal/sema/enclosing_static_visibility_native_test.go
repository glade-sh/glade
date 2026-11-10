package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Replays native compile-time visibility and overload controls. Runtime values
// have separate anonymous rows and require a linked VM replay.
func TestCapturedEnclosingStaticVisibility(t *testing.T) {
	const fixture = "testdata/enclosing_static_visibility"
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
	if len(rows) != 31 || strings.Join(rows[0], ",") != "case,api,route,outcome" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid native row: %v", row)
		}
		if row[2] != "metadata-deploy" {
			continue
		}
		name, api, outcome := row[0], row[1], row[3]
		key := name + "/" + api
		if seen[key] || (api != "62.0" && api != "67.0") || strings.Contains(outcome, "CAPTURE_ERROR") {
			t.Fatalf("invalid native observation: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			paths, err := filepath.Glob(filepath.Join(fixture, "cases", name, api, "*.cls"))
			if err != nil || len(paths) == 0 {
				t.Fatalf("missing sources: %v", err)
			}
			files := map[string]string{}
			for _, path := range paths {
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				files[filepath.Base(path)] = string(body)
			}
			analysis := analyzeDeclarationProjectWithAPIVersion(t, files, api)
			rejected := outcome != "accepted"
			if analysis.HasErrors() != rejected {
				t.Fatalf("native=%q; local diagnostics=%v", outcome, analysis.Diagnostics)
			}
			if !rejected {
				return
			}
			want := ""
			switch {
			case strings.Contains(outcome, "Ambiguous method signature"):
				want = "ambiguous"
			case strings.Contains(outcome, "protected methods cannot be static"):
				want = "protected"
			case strings.Contains(outcome, "Method is not visible"):
				want = "private"
			case strings.Contains(outcome, "Method does not exist"):
				want = "value"
			default:
				t.Fatalf("unknown native rejection: %q", outcome)
			}
			found := false
			for _, diagnostic := range analysis.Diagnostics {
				if strings.Contains(strings.ToLower(diagnostic.Message+" "+diagnostic.NativeMessage), want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("native rejection %q lacked local %q diagnostic: %v", outcome, want, analysis.Diagnostics)
			}
		})
	}
	if len(seen) != 24 {
		t.Fatalf("native deployments covered %d cells, want 24", len(seen))
	}
}
