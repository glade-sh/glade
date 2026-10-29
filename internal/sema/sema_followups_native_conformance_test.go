package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func readSemaFollowupRows(t *testing.T, path string, columns int) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.FieldsPerRecord = '\t', columns
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("%s: rows=%d err=%v", path, len(rows), err)
	}
	return rows[1:]
}

// The parent and child are separate top-level types so private access cannot
// accidentally pass through a shared enclosing class.
func TestExceptionConstructorVisibilityNativeConformance(t *testing.T) {
	const dir = "../../testdata/conformance/exception_constructor_visibility"
	cases := map[string][]string{}
	for _, row := range readSemaFollowupRows(t, filepath.Join(dir, "cases.tsv"), 3) {
		cases[row[0]] = row[1:]
	}
	seen := map[string]bool{}
	for _, row := range readSemaFollowupRows(t, filepath.Join(dir, "org.tsv"), 5) {
		if row[2] != "named" {
			continue
		}
		key := row[0] + "/" + row[1]
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native key %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			files, ok := cases[row[1]]
			if !ok {
				t.Fatalf("missing source: %s", row[1])
			}
			sources := map[string]string{}
			for _, name := range files {
				source, err := os.ReadFile(filepath.Join(dir, "sources", name))
				if err != nil {
					t.Fatal(err)
				}
				sources[name] = string(source)
			}
			result := analyzeDeclarationProjectWithAPIVersion(t, sources, row[0])
			if result.HasErrors() != (row[3] == "rejected") {
				t.Fatalf("native=%s %s; Glade=%+v", row[3], row[4], result.Diagnostics)
			}
			if strings.Contains(row[4], "System exception constructor already defined") && !declarationDiagnosticMatching(result, "System exception constructor already defined: void <init>()") {
				t.Fatalf("missing native exception-constructor rejection: %+v", result.Diagnostics)
			}
			if row[1] == "privateparam_custom_args_super" {
				matched := false
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error && d.Code == "GLADESEMA010" && strings.Contains(d.Message, `cannot access private constructor "super"`) {
						matched = true
					}
				}
				if !matched {
					t.Fatalf("missing private super constructor visibility rejection: %+v", result.Diagnostics)
				}
			}
			if row[3] != "accepted" && row[3] != "rejected" {
				t.Fatalf("invalid outcome: %s", row[3])
			}
		})
	}
	if len(seen) != 2*len(cases) {
		t.Fatalf("native coverage=%d, want %d", len(seen), 2*len(cases))
	}
}

func TestNamespaceStaticCallNativeConformance(t *testing.T) {
	const dir = "../../testdata/conformance/namespace_absence"
	cases := map[string]string{}
	for _, row := range readSemaFollowupRows(t, filepath.Join(dir, "native-cases.tsv"), 2) {
		source, err := os.ReadFile(filepath.Join(dir, row[1]))
		if err != nil {
			t.Fatal(err)
		}
		cases[row[0]] = string(source)
	}
	seen := map[string]bool{}
	for _, row := range readSemaFollowupRows(t, filepath.Join(dir, "org.tsv"), 5) {
		key := strings.Join(row[:3], "/")
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native key %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			source, ok := cases[row[1]]
			if !ok {
				t.Fatalf("missing source: %s", row[1])
			}
			var diagnostics []diagnostic.Diagnostic
			switch row[2] {
			case "anonymous":
				diagnostics = AnalyzeAnonymous(typesys.Index{}, source, row[0]).Diagnostics
			case "named":
				diagnostics = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QualifiedNativeProbe.cls": "public class QualifiedNativeProbe { public static void observe() { " + source + " } }"}, row[0]).Diagnostics
			case "isTest":
				diagnostics = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QualifiedNativeProbeTest.cls": "@IsTest private class QualifiedNativeProbeTest { @IsTest static void observe() { " + source + " } }"}, row[0]).Diagnostics
			default:
				t.Fatalf("unknown route: %s", row[2])
			}
			rejected := false
			matched := false
			for _, d := range diagnostics {
				if d.Severity != diagnostic.Error {
					continue
				}
				rejected = true
				matched = matched || d.NativeMessage == row[4]
			}
			if rejected != (row[3] == "rejected") || (rejected && !matched) {
				t.Fatalf("native=%s %q; Glade=%+v", row[3], row[4], diagnostics)
			}
		})
	}
	if len(seen) != 2*3*len(cases) {
		t.Fatalf("native coverage=%d, want %d", len(seen), 2*3*len(cases))
	}
}
