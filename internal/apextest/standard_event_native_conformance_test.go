package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func runStandardEventNativeRows(t *testing.T, family string) {
	t.Helper()
	dir := filepath.Join("testdata/conformance", family)
	var rows []struct{ ID, Body string }
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named"} {
			file := "org.tsv"
			if route == "named" {
				file = "named.tsv"
			}
			raw, err = os.ReadFile(filepath.Join(dir, "api"+api[:2], file))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(lines) != len(rows)+1 {
				t.Fatal("incomplete native rows")
			}
			for i, row := range rows {
				fields := strings.SplitN(lines[i+1], "\t", 3)
				if len(fields) != 3 || fields[0] != row.ID {
					t.Fatal("invalid native row")
				}
				t.Run(api+"/"+route+"/"+row.ID, func(t *testing.T) {
					root := t.TempDir()
					p := project.Project{Root: root, SourceAPIVersion: api}
					if route == "named" {
						path := filepath.Join(root, "MeadowCompileProbe.cls")
						writeFile(t, path, "public class MeadowCompileProbe { public static void observe() { "+row.Body+" } }")
						p.ApexFiles = []string{path}
					}
					index := typesys.Build(p, schema.Schema{})
					diagnostics := append([]diagnostic.Diagnostic{}, index.Diagnostics...)
					if route == "named" {
						diagnostics = append(diagnostics, sema.Analyze(index).Diagnostics...)
					} else {
						diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, row.Body, api).Diagnostics...)
					}
					primary := ""
					for _, d := range diagnostics {
						if d.Severity == diagnostic.Error {
							primary = d.NativeMessage
							if primary == "" {
								primary = d.Message
							}
							break
						}
					}
					if fields[1] == "compile-rejected" {
						if primary != fields[2] {
							t.Fatalf("native <%s>; Glade <%s>; diagnostics=%v", fields[2], primary, diagnostics)
						}
						return
					}
					if fields[1] != "accepted" {
						t.Fatalf("unanswered native row %v", fields)
					}
					if primary != "" {
						t.Fatalf("native accepted; Glade rejected: %v", diagnostics)
					}
					// Named rows for assignment edges capture compilation only.
					if route == "named" && family == "standard_assignment_edges" {
						return
					}
					source := row.Body
					if route == "named" {
						source = "MeadowCompileProbe.observe();"
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					result, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if fields[2] != "accepted" {
						want := "NATIVE|" + fields[2]
						found := false
						for _, line := range result.Debug {
							if strings.Contains(line, want) {
								found = true
							}
						}
						if !found {
							t.Fatalf("native <%s>; Glade debug %v", want, result.Debug)
						}
					}
				})
			}
		}
	}
}
