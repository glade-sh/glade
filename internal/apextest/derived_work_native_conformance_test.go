package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestDerivedCollectionTraversalNativeAnonymousNamedAndRunnerRoutes(t *testing.T) {
	runDerivedWorkNativeRoutes(t, "derived_collection_traversal", "DerivedTraversalProbe", 11)
}

func TestFinalizerDeferredWorkNativeAnonymousNamedAndRunnerRoutes(t *testing.T) {
	runDerivedWorkNativeRoutes(t, "finalizer_deferred_work", "DeferredWorkProbe", 10)
}

func TestMetadataMasterLabelNativeAnonymousNamedAndRunnerRoutes(t *testing.T) {
	runDerivedWorkNativeRoutes(t, "metadata_master_label", "MetadataLabelProbe", 4)
}

// Replay each invocation independently, including native exception results.
func runDerivedWorkNativeRoutes(t *testing.T, family, class string, count int) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	for _, route := range []string{"anonymous", "named"} {
		file := "org.tsv"
		if route == "anonymous" {
			file = "anonymous.tsv"
		}
		raw, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(raw)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		raw.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1+2*count || strings.Join(rows[0], ",") != "source_api,method,observation" {
			t.Fatalf("incomplete %s/%s observations", family, route)
		}
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(route+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join(dir, "project")
				err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
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
					writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				seen := map[string]bool{}
				for _, row := range rows[1:] {
					if row[0] != api {
						continue
					}
					method := row[1]
					if seen[method] {
						t.Fatal("duplicate observation", method)
					}
					seen[method] = true
					t.Run(method, func(t *testing.T) {
						var got map[string]any
						if route == "named" {
							run := Run(index, Options{SelectedClasses: []string{class + "Test"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("missing runner case: %+v", run.Summary())
							}
							problem := run.Suites[0].Cases[0].Problem
							if problem == nil {
								t.Fatal("missing observation failure")
							}
							got = derivedWorkObservation(t, problem.Type, problem.Message, method)
						} else {
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							program, err := vm.CompileAnonymousWithOptions("System.debug('OBSERVED|"+method+"|'+JSON.serialize("+class+"."+method+"()));", vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := runner.newMachine().Execute(program)
							if err != nil {
								got = derivedWorkObservation(t, "", err.Error(), method)
							} else {
								for _, line := range result.Debug {
									if strings.Contains(line, "OBSERVED|"+method+"|") {
										got = derivedWorkObservation(t, "", line, method)
									}
								}
							}
						}
						var want map[string]any
						if json.Unmarshal([]byte(row[2]), &want) != nil {
							t.Fatal("invalid native JSON")
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%s; Glade=%v", row[2], got)
						}
					})
				}
				if len(seen) != count {
					t.Fatal("missing API rows")
				}
			})
		}
	}
}

func derivedWorkObservation(t *testing.T, kind, message, method string) map[string]any {
	t.Helper()
	if _, payload, ok := strings.Cut(message, "OBSERVED|"+method+"|"); ok {
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if kind == "" {
		for _, name := range []string{"System.FinalException", "System.TypeException", "System.QueryException"} {
			if _, tail, ok := strings.Cut(message, name+": "); ok {
				kind = name
				message = strings.SplitN(tail, "\n", 2)[0]
				break
			}
		}
	}
	return map[string]any{"exception": kind, "message": message}
}
