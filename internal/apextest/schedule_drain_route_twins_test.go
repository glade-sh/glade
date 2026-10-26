package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Reuse the unchanged native project and all sixteen observations, including
// schedule_drain/org.tsv:2,7,10,15 for the batch-finish/queueable regressions.
func TestScheduleDrainNamedAndCachedParallelRunnerNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/schedule_drain"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("native rows: count=%d error=%v", len(rows), err)
	}
	classes := map[string]string{}
	files, err := filepath.Glob(filepath.Join(fixture, "project/force-app/main/default/classes/*.cls"))
	if err != nil || len(files) != 5 {
		t.Fatalf("captured classes: count=%d error=%v", len(files), err)
	}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		classes[strings.TrimSuffix(filepath.Base(path), ".cls")] = string(body)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, classes, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			want := map[string]string{}
			for _, row := range rows[1:] {
				if row[0] == api {
					want[row[1]] = row[2]
				}
			}
			check := func(t *testing.T, method, message string) {
				t.Helper()
				_, payload, found := strings.Cut(message, "OBSERVED|"+method+"|")
				var got, expected map[string]int
				if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(want[method]), &expected) != nil {
					t.Fatalf("invalid observation for %s: %s", method, message)
				}
				if !reflect.DeepEqual(got, expected) {
					t.Errorf("native=%s; Glade=%s", want[method], payload)
				}
			}
			t.Run("named_isTest", func(t *testing.T) {
				cases := Discover(index, Options{SelectedClasses: []string{"ScheduleDrainObservationTest"}})
				if len(cases) != len(want) {
					t.Fatalf("discovered %d tests; want %d", len(cases), len(want))
				}
				compiled, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				org := orgFromIndex(index)
				org.APIVersion = api
				base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, test := range cases {
					t.Run(test.MethodName, func(t *testing.T) {
						key := testCaseKey(test)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("native executes: method=%v program=%v", methodErrors[key], programErrors[key])
						}
						machine := base.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(compiled[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteInClass(programs[key], test.ClassName)
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("missing assertion transport: %v", err)
						}
						check(t, test.MethodName, fault.ExceptionMessage())
					})
				}
			})
			// A repeated project run exercises cached compilation and concurrent
			// test VMs; each observation still owns its database and pending jobs.
			for _, route := range []string{"runner_parallel_cold", "runner_parallel_cached"} {
				t.Run(route, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"ScheduleDrainObservationTest"}, Parallelism: 2})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != len(want) {
						t.Fatalf("incomplete observations: %+v", run.Summary())
					}
					for _, item := range run.Suites[0].Cases {
						t.Run(item.MethodName, func(t *testing.T) {
							if item.Problem == nil || !strings.HasSuffix(item.Problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", item)
							}
							check(t, item.MethodName, item.Problem.Message)
						})
					}
				})
			}
		})
	}
}
