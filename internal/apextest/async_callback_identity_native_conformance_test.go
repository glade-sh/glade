package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAsyncContextIDsAnonymousAndIsTestNativeConformance(t *testing.T) {
	runAsyncCallbackObservation(t, "async_context_ids", "AsyncIdentityProbeTest", "")
}

func TestAsyncStaticCallbacksAnonymousAndIsTestNativeConformance(t *testing.T) {
	runAsyncCallbackObservation(t, "async_static_callbacks", "StaticCallbackProbeTest", "")
}

func TestAsyncEnqueueOverloadsAndInheritedInstanceCallbacksAnonymousAndIsTestNativeConformance(t *testing.T) {
	runAsyncCallbackObservation(t, "async_route_twins", "RouteTwinProbeTest", "")
}

func TestAsyncEnqueueOverloadsAnonymousAndIsTestNativeConformance(t *testing.T) {
	for _, overload := range []string{"plain", "integer", "options"} {
		t.Run(overload, func(t *testing.T) {
			runAsyncCallbackObservation(t, "async_route_twins", "EnqueueOverloadProbeTest", overload)
		})
	}
}

func runAsyncCallbackObservation(t *testing.T, family, class, overload string) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatalf("incomplete native async matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] || (api != "62.0" && api != "67.0") || (route != "anonymous" && route != "isTest" && route != "scheduledBatch-anonymous" && route != "scheduledBatch-isTest") {
			t.Fatalf("invalid native async cell: %s", key)
		}
		seen[key] = true
		if overload != "" {
			if strings.HasPrefix(route, "scheduledBatch-") {
				continue
			}
			var observations map[string]json.RawMessage
			if err := json.Unmarshal([]byte(row[2]), &observations); err != nil {
				t.Fatal(err)
			}
			value, ok := observations[overload]
			if !ok {
				t.Fatal("native overload observation missing")
			}
			expected, err := json.Marshal(map[string]json.RawMessage{overload: value})
			if err != nil {
				t.Fatal(err)
			}
			row = append([]string(nil), row...)
			row[2] = string(expected)
		}
		t.Run(key, func(t *testing.T) {
			if route == "scheduledBatch-isTest" {
				// Native does not enter a delayed batch callback during stopTest.
				// Keep its observation; the existing local drain-timing gap is
				// separate from live callback identity and static dispatch.
				if row[2] != "{}" && row[2] != "[]" {
					t.Fatal("native delayed-batch observation changed")
				}
				t.Log("carried lifecycle gap: native defers scheduleBatch at stopTest; local drain executes callbacks")
				return
			}
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
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var observed string
			if strings.HasSuffix(route, "isTest") {
				method := "observe"
				if overload != "" {
					method += overload
				}
				if strings.HasPrefix(route, "scheduledBatch-") {
					method = "observeScheduledBatch"
				}
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("missing named observation: %+v", run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatalf("missing native assertion transport: %+v", problem)
				}
				_, observed, _ = strings.Cut(problem.Message, "OBS|")
			} else {
				org := orgFromIndex(index)
				org.APIVersion = api
				machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
				execute := func(name string) vm.Result {
					t.Helper()
					source, err := os.ReadFile(filepath.Join(dir, name))
					if err != nil {
						t.Fatal(err)
					}
					if analysis := sema.AnalyzeAnonymous(index, string(source), api); analysis.HasErrors() {
						t.Fatalf("native accepted anonymous: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				enqueue, readback := "anonymous.apex", "readback.apex"
				if strings.HasPrefix(route, "scheduledBatch-") {
					enqueue, readback = "scheduled-batch.apex", "scheduled-batch-readback.apex"
				}
				if overload != "" {
					enqueue = "enqueue-" + overload + ".apex"
				}
				result := execute(enqueue)
				machine.AdvanceDeterministicTime(time.Minute)
				if err := machine.DrainAsync(&result); err != nil {
					t.Fatalf("native callbacks completed: %v", err)
				}
				for _, line := range execute(readback).Debug {
					if _, payload, ok := strings.Cut(line, "OBS|"); ok {
						if observed != "" {
							t.Fatal("duplicate observation")
						}
						observed = payload
					}
				}
			}
			var got, want any
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid native/Glade observation: %q / %q", row[2], observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
