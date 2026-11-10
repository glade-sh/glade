package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay the installed provider from captured source. The unavailable alias
// remains in the index as an independent negative control.
func TestUnlockedSubscriberNestedTypeNativeControls(t *testing.T) {
	const root = "../apextest/testdata/conformance/namespace_unlocked_subscriber"
	for _, state := range []string{"installed", "absent"} {
		f, err := os.Open(filepath.Join(root, state, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		f.Close()
		if err != nil || len(rows) != 25 {
			t.Fatalf("%s native rows: %d, %v", state, len(rows), err)
		}
		for _, row := range rows[1:] {
			if len(row) != 7 {
				t.Fatalf("malformed native row: %q", row)
			}
			id, api, route, outcome, source := row[0], row[1], row[2], row[3], row[6]
			t.Run(state+"/"+api+"/"+route+"/"+id, func(t *testing.T) {
				path := filepath.Join(root, state, filepath.FromSlash(source))
				code, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				projectRoot := t.TempDir()
				p := project.Project{Root: projectRoot, SourceAPIVersion: api}
				if state == "installed" {
					p.ApexFiles = append(p.ApexFiles, unlockedCapturedProvider(t, projectRoot))
				}
				if route == "istest-checkonly" {
					path = filepath.Join(projectRoot, filepath.Base(path))
					writeSemaFile(t, path, string(code))
					p.ApexFiles = append(p.ApexFiles, path)
				}
				index := typesys.Build(p, schema.Schema{})
				if index.HasErrors() {
					t.Fatalf("index: %v", index.Diagnostics)
				}
				if state == "installed" {
					index.Dependencies = []typesys.DependencyInfo{{Namespace: "v036-unlocked", Status: "missing"}}
				}
				var ds []diagnostic.Diagnostic
				switch route {
				case "anonymous":
					ds = AnalyzeAnonymous(index, string(code), api).Diagnostics
				case "istest-checkonly":
					ds = Analyze(index).Diagnostics
				default:
					t.Fatalf("unexpected route %q", route)
				}
				var errors []string
				matchedNative := false
				for _, d := range ds {
					if d.Severity == diagnostic.Error {
						errors = append(errors, d.Code+": "+d.Message)
						if d.NativeMessage == row[5] || d.Message == row[5] {
							matchedNative = true
						}
					}
				}
				if outcome == "COMPILED" && len(errors) != 0 {
					t.Fatalf("native compiles, offline rejects: %s", strings.Join(errors, "; "))
				}
				if outcome == "COMPILE_ERROR" && !matchedNative {
					t.Fatalf("native diagnostic %q not found in offline errors: %v", row[5], errors)
				}
			})
		}
	}
}
