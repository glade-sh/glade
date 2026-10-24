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

func TestScheduledDetailAbortNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/scheduled_detail"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	classes := map[string]string{}
	runFacts := read("RUN-FACTS.txt")
	for _, name := range []string{"WillowDetailProbe", "WillowDetailProbeTest"} {
		source := read("project/force-app/main/default/classes/" + name + ".cls")
		classes[name] = source
		if !strings.Contains(runFacts, fmt.Sprintf("%s.cls_source_sha256=%x\n", name, sha256.Sum256([]byte(source)))) {
			t.Fatalf("native capture does not bind %s source", name)
		}
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 || strings.Join(rows[0], ",") != "source_api,route,case,observation" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "istest") {
			t.Fatalf("invalid native row %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
	}
	for _, api := range []string{"62.0", "67.0"} {
		index := nativeBoundaryIndex(t, api, classes, nil)
		if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native classes compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
		}
		for _, row := range rows[1:] {
			if row[0] != api {
				continue
			}
			t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
				var want map[string]any
				if err := json.Unmarshal([]byte(row[3]), &want); err != nil || len(want) != 22 {
					t.Fatalf("invalid native observation: %v", err)
				}
				payload := ""
				if row[1] == "anonymous" {
					source := read(row[2] + ".apex")
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					result, err := runner.execute(program)
					var fault *vm.RuntimeError
					if errors.As(err, &fault) {
						if row[2] == "queuedSchedule" && fault.Type == "UnsupportedFeature" && fault.Message == `unsupported call "System.abortJob local async scheduling surface"` {
							t.Skip("native anonymous abort captured; local scheduled Apex abort outside test context remains unsupported")
						}
						if row[2] == "queuedBatch" && fault.Type == "System.StringException" && fault.ExceptionMessage() == "Job does not exist or is already aborted." {
							t.Skip("native anonymous abort captured; local scheduled batch abort outside test context has no AsyncApexJob record")
						}
					}
					if err != nil || len(result.Debug) != 1 {
						t.Fatalf("missing anonymous observation: %v %v", err, result.Debug)
					}
					_, payload, _ = strings.Cut(result.Debug[0], "OBS|")
				} else {
					// Run loads the synthetic project and exercises the actual test runner,
					// including stopTest draining and test isolation on the recorded routes.
					run := Run(index, Options{SelectedClasses: []string{"WillowDetailProbeTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("missing project test: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing assertion transport: %+v", problem)
					}
					_, payload, _ = strings.Cut(problem.Message, "OBS|")
				}
				var got map[string]any
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid Glade observation %q: %v", payload, err)
				}
				for name, expected := range want {
					t.Run(name, func(t *testing.T) {
						if !reflect.DeepEqual(got[name], expected) {
							t.Errorf("native=%v; Glade=%v", expected, got[name])
						}
					})
				}
				if len(got) != len(want) {
					t.Errorf("native has %d observation cells; Glade has %d", len(want), len(got))
				}
			})
		}
	}
}
