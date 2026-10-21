package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestMetadataDependencyNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "metadata_dependency_shapes")
	var cases []struct {
		ID, Type, Field, Class, Expected string
	}
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 5 {
		t.Fatalf("invalid native cases: %v", err)
	}
	copyTree := func(root, source string) {
		t.Helper()
		if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(root, name), string(body))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, state := range []string{"absent", "empty", "complete"} {
			for _, route := range []string{"anonymous", "named"} {
				t.Run(api+"/"+state+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					if err := os.MkdirAll(filepath.Join(root, "force-app"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					if state != "absent" {
						copyTree(filepath.Join(root, "force-app", "main", "default"), filepath.Join(dir, "metadata", state))
					}
					if route == "named" {
						for _, c := range cases {
							body, err := os.ReadFile(filepath.Join(dir, "sources", c.Class+".cls"))
							if err != nil {
								t.Fatal(err)
							}
							writeFile(t, filepath.Join(root, "force-app", "classes", c.Class+".cls"), string(body))
						}
						if state == "complete" {
							body, err := os.ReadFile(filepath.Join(dir, "sources", "MeadowShapeTest.cls"))
							if err != nil {
								t.Fatal(err)
							}
							writeFile(t, filepath.Join(root, "force-app", "classes", "MeadowShapeTest.cls"), string(body))
						}
					}
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
					file := "org.tsv"
					if route == "named" {
						file = "named.tsv"
					}
					f, err := os.Open(filepath.Join(dir, "api"+api[:2], state, file))
					if err != nil {
						t.Fatal(err)
					}
					defer f.Close()
					reader := csv.NewReader(f)
					reader.Comma = '\t'
					reader.LazyQuotes = true
					rows, err := reader.ReadAll()
					if err != nil || len(rows) != len(cases)+1 || strings.Join(rows[0], ",") != "id,outcome,value" {
						t.Fatalf("incomplete native observations: %v", err)
					}
					for i, c := range cases {
						t.Run(c.ID, func(t *testing.T) {
							row := rows[i+1]
							if len(row) != 3 || row[0] != c.ID || row[2] == "" {
								t.Fatal("invalid native row", row)
							}
							diags := diagnostics
							body, err := os.ReadFile(filepath.Join(dir, "sources", c.ID+".apex"))
							if err != nil {
								t.Fatal(err)
							}
							if route == "anonymous" {
								diags = sema.AnalyzeAnonymous(index, string(body), api).Diagnostics
							}
							var errors []diagnostic.Diagnostic
							for _, d := range diags {
								if d.Severity == diagnostic.Error && (route == "anonymous" || strings.EqualFold(filepath.Base(d.File), c.Class+".cls")) {
									errors = append(errors, d)
								}
							}
							if row[1] == "compile-rejected" {
								// These rows establish rejection, while preserving the native
								// diagnostic separately; wording equivalence is not asserted.
								if state == "complete" || len(errors) == 0 {
									t.Fatalf("native rejected <%s>; Glade diagnostics=%v", row[2], diags)
								}
								t.Logf("native rejected <%s>; Glade rejected <%s>", row[2], errors[0].Message)
								return
							}
							if row[1] != "accepted" || state != "complete" || row[2] != c.Expected {
								t.Fatal("unexpected native outcome", row)
							}
							if index.HasErrors() || analysis.HasErrors() || len(errors) > 0 {
								t.Fatalf("native accepted; Glade diagnostics=%v", diags)
							}
							if route == "named" {
								source, err := os.ReadFile(filepath.Join(dir, "sources", "MeadowShapeTest.cls"))
								if err != nil || !strings.Contains(string(source), "assertEquals('"+row[2]+"', "+c.Class+".observe())") {
									t.Fatal("native named assertion does not match row", err)
								}
								run := Run(index, Options{SelectedClasses: []string{"MeadowShapeTest"}, SelectedMethod: c.ID, NoDiskCache: true, Parallelism: 1})
								if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
									t.Fatalf("expected one named test: %+v", run.Summary())
								}
								result := run.Suites[0].Cases[0]
								if result.Problem != nil || result.Status != "pass" {
									t.Fatalf("native test passed; Glade=%+v", result)
								}
								return
							}
							program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							result, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program)
							if err != nil {
								t.Fatal(err)
							}
							for _, line := range result.Debug {
								if strings.Contains(line, "NATIVE|"+row[2]) {
									return
								}
							}
							t.Fatalf("native=%s; Glade debug=%v", row[2], result.Debug)
						})
					}
				})
			}
		}
	}
}
