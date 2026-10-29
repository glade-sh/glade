package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// These owned observations distinguish runtime behavior from assumptions in
// application tests: record identity, activity logging and active traversal.
func TestResidualRuntimeBoundariesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/residual_runtime_boundaries"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 31 || strings.Join(rows[0], "\t") != "source_api\troute\tmethod\tobservation" {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GLADE_HOME", filepath.Join(root, ".glade"))
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
					t.Fatalf("captured source changed: %s", name)
				}
				rendered := strings.ReplaceAll(string(body), "62.0", api)
				if !strings.Contains(string(capture), fmt.Sprintf("source_api_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256([]byte(rendered)))) {
					t.Fatalf("missing native deployment input: %s/%s", api, name)
				}
				writeFile(t, filepath.Join(root, name), rendered)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native deployed; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			anonymous := map[string]string{}
			sources, err := os.ReadFile(filepath.Join(fixture, "anonymous.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(sources)), "\n")[1:] {
				method, source, ok := strings.Cut(line, "\t")
				if !ok || anonymous[method] != "" {
					t.Fatal("invalid anonymous source")
				}
				if !strings.Contains(string(capture), fmt.Sprintf("anonymous_api_sha256:%s:%s=%x\n", api, method, sha256.Sum256([]byte(source+"\n")))) {
					t.Fatalf("missing anonymous input: %s/%s", api, method)
				}
				anonymous[method] = source
			}
			for _, temperature := range []string{"cold", "warm"} {
				run := Run(index, Options{SelectedClasses: []string{"ResidualBoundaryTest"}, Parallelism: 1})
				cases := map[string]testreport.Case{}
				for _, suite := range run.Suites {
					for _, test := range suite.Cases {
						cases[test.MethodName] = test
					}
				}
				if len(cases) != 8 {
					t.Fatalf("named observations: %+v", run.Summary())
				}
				seen := map[string]bool{}
				for _, row := range rows[1:] {
					if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") {
						t.Fatal("invalid native row")
					}
					if row[0] != api {
						continue
					}
					route, method := row[1], row[2]
					key := route + "/" + method
					if seen[key] {
						t.Fatal("duplicate native row")
					}
					seen[key] = true
					t.Run(temperature+"/"+key, func(t *testing.T) {
						var message, exceptionType string
						if route == "isTest" {
							test, ok := cases[method]
							if !ok || test.Problem == nil {
								t.Fatalf("missing observation: %+v", test)
							}
							message, exceptionType = test.Problem.Message, test.Problem.Type
						} else {
							source := anonymous[method]
							if source == "" {
								t.Fatal("missing anonymous source")
							}
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							_, err = runner.execute(program)
							var runtimeErr *vm.RuntimeError
							if !errors.As(err, &runtimeErr) {
								t.Fatalf("missing native observation: %v", err)
							}
							message, exceptionType = runtimeErr.ExceptionMessage(), userModeInaccessibleExceptionType(runtimeErr.Type)
						}
						var got map[string]any
						if _, payload, found := strings.Cut(message, "BOUNDARY|"); found {
							if exceptionType != "System.AssertException" {
								t.Fatal("invalid observation transport")
							}
							var value any
							if err := json.Unmarshal([]byte(payload), &value); err != nil {
								t.Fatal(err)
							}
							got = map[string]any{"value": value}
						} else {
							got = map[string]any{"exceptionType": exceptionType, "message": strings.TrimSpace(message)}
						}
						var want map[string]any
						if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%v; Glade=%v", want, got)
						}
					})
				}
				if len(seen) != 15 {
					t.Fatalf("missing native routes: %d", len(seen))
				}
			}
		})
	}
}
