package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestScheduledDetailTransactionNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/scheduled_detail_transactions"
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	classes := map[string]string{}
	facts := read("RUN-FACTS.txt")
	for _, name := range []string{"CedarTransactionProbe", "CedarTransactionProbeTest"} {
		source := read("project/force-app/main/default/classes/" + name + ".cls")
		classes[name] = source
		if !strings.Contains(facts, fmt.Sprintf("%s.cls_source_sha256=%x\n", name, sha256.Sum256([]byte(source)))) {
			t.Fatalf("unbound native source: %s", name)
		}
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 || strings.Join(rows[0], ",") != "source_api,route,case,observation" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "istest") || (row[2] != "schedule" && row[2] != "scheduleBatch") {
			t.Fatalf("invalid row %v", row)
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
			t.Fatalf("synthetic project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
		}
		for _, row := range rows[1:] {
			if row[0] != api {
				continue
			}
			t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
				var want map[string]any
				if err := json.Unmarshal([]byte(row[3]), &want); err != nil || len(want) != 9 {
					t.Fatalf("invalid native observation: %v", err)
				}
				payload := ""
				if row[1] == "anonymous" {
					org := orgFromIndex(index)
					org.APIVersion = api
					// The request runtime is cloned anew for each transaction; only org storage survives.
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
					for _, phase := range []string{"First", "Second"} {
						program, err := vm.CompileAnonymousWithOptions(read(row[2]+phase+".apex"), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := runner.newMachine()
						machine.SetOrg(&org)
						result, err := machine.Execute(program)
						if err != nil {
							t.Fatalf("%s transaction: %v", phase, err)
						}
						if phase == "Second" {
							if len(result.Debug) != 1 {
								t.Fatalf("missing observation: %v", result.Debug)
							}
							_, payload, _ = strings.Cut(result.Debug[0], "OBS|")
						}
					}
				} else {
					run := Run(index, Options{SelectedClasses: []string{"CedarTransactionProbeTest"}, SelectedMethod: row[2] + "SameTransaction", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("missing project test: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing observation transport: %+v", problem)
					}
					_, payload, _ = strings.Cut(problem.Message, "OBS|")
				}
				var got map[string]any
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid observation %q: %v", payload, err)
				}
				for name, expected := range want {
					if !reflect.DeepEqual(got[name], expected) {
						t.Errorf("%s: native=%v; Glade=%v", name, expected, got[name])
					}
				}
				if len(got) != len(want) {
					t.Errorf("native has %d cells; Glade has %d", len(want), len(got))
				}
			})
		}
	}
}
