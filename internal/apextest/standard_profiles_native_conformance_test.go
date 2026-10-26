package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the same singleton queries through anonymous execution and the project
// test runner. The named helper and @IsTest wrapper retain their source API.
func TestStandardProfilesNativeConformance(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/standard_profiles"
	for _, shape := range []string{"developer", "communities"} {
		t.Run(shape, func(t *testing.T) {
			native := filepath.Join(fixture, "native", shape)
			body, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, fact := range []string{"shape=" + shape, "rows=12", "native_complete=true"} {
				if !strings.Contains("\n"+string(body), "\n"+fact+"\n") {
					t.Fatalf("missing capture fact %s", fact)
				}
			}
			for _, name := range []string{"StandardProfileProbe", "StandardProfileProbeTest"} {
				source, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes", name+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(body), fmt.Sprintf("%s.cls_sha256=%x\n", name, sha256.Sum256(source))) {
					t.Fatalf("capture detached from %s source", name)
				}
			}
			file, err := os.Open(filepath.Join(native, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			reader := csv.NewReader(file)
			reader.Comma, reader.LazyQuotes = '\t', true
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 13 || strings.Join(rows[0], ",") != "shape,source_api,route,method,observation" {
				t.Fatalf("incomplete native observations: %v", err)
			}
			seen := map[string]bool{}
			for _, api := range []string{"62.0", "67.0"} {
				t.Run(api, func(t *testing.T) {
					root := t.TempDir()
					project := filepath.Join(fixture, "project")
					err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						body, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						name, err := filepath.Rel(project, path)
						if err == nil {
							writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
						}
						return err
					})
					if err != nil {
						t.Fatal(err)
					}
					if shape == "communities" {
						writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Communities]\n")
					}
					index := loadTestIndex(t, root)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					for _, row := range rows[1:] {
						if row[0] != shape || row[1] != api {
							continue
						}
						key := strings.Join(row[:4], "/")
						if seen[key] || (row[3] != "profiles" && row[3] != "missing" && row[3] != "boundary") {
							t.Fatal("invalid native observation", key)
						}
						seen[key] = true
						t.Run(row[2]+"/"+row[3], func(t *testing.T) {
							var message string
							if row[2] == "isTest" {
								report := Run(index, Options{SelectedClasses: []string{"StandardProfileProbeTest"}, SelectedMethod: row[3], NoDiskCache: true, Parallelism: 1})
								if report.Summary().Total != 1 || len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 {
									t.Fatalf("test selection: %+v", report.Summary())
								}
								result := report.Suites[0].Cases[0]
								if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
									t.Fatalf("missing assertion transport: %+v", result)
								}
								message = result.Problem.Message
							} else if row[2] == "anonymous" {
								source := "System.assert(false,'OBSERVED|" + row[3] + "|'+JSON.serialize(StandardProfileProbe." + row[3] + "()));"
								if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
									t.Fatal(analysis.Diagnostics)
								}
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								org := orgFromIndex(index)
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
								_, err = runner.newMachine().Execute(program)
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
									t.Fatalf("missing assertion transport: %v", err)
								}
								message = runtimeErr.ExceptionMessage()
							} else {
								t.Fatal("invalid route", row[2])
							}
							_, payload, found := strings.Cut(message, "OBSERVED|"+row[3]+"|")
							var got, want map[string]string
							if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[4]), &want) != nil {
								t.Fatalf("invalid observation: %s", message)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("native=%s; Glade=%s", row[4], payload)
							}
						})
					}
				})
			}
			if len(seen) != 12 {
				t.Fatalf("observations=%d; want 12", len(seen))
			}
		})
	}
}
