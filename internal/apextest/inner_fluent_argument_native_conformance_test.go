package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the owned scratch org rows, including the collision rejection twin.
func TestInnerFluentArgumentNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/inner_fluent_argument"
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 || strings.Join(rows[0], ",") != "case,api,route,compiled,success,outcome" {
		t.Fatalf("native table: %d rows: %v", len(rows), err)
	}
	seen := map[string]bool{}
	for line, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid row %d: %q", line+2, row)
		}
		id, api, route := row[0], row[1], row[2]
		key := id + "/" + api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate row %s", key)
		}
		seen[key] = true
		if (api != "62.0" && api != "67.0") || (route != "anonymous" && route != "isTest") {
			t.Fatalf("unexpected row %s", key)
		}
		t.Run(key, func(t *testing.T) {
			classes := map[string]string{"ReceiverFixture": read("ReceiverFixture.cls")}
			testClass, method := "ReceiverFixtureTest", "exercise"
			if id == "boundary" {
				method = "boundary"
			}
			if id == "reject" {
				testClass = "RejectFixtureTest"
			}
			if id == "collision_reject" {
				testClass = "CollisionFixtureTest"
			}
			if route == "isTest" {
				classes[testClass] = read(testClass + ".cls")
			}
			index := nativeBoundaryIndex(t, api, classes, nil)
			diagnostics := append([]diagnostic.Diagnostic(nil), index.Diagnostics...)
			var prepared innerShapeAnonymous
			if route == "anonymous" {
				prepared = innerShapePrepareAnonymous(t, api, index, read(id+".apex"))
				diagnostics = append(diagnostics, prepared.diagnostics...)
			} else {
				diagnostics = append(diagnostics, sema.Analyze(index).Diagnostics...)
			}
			var errors []string
			for _, d := range diagnostics {
				if d.Severity == diagnostic.Error {
					msg := d.NativeMessage
					if msg == "" {
						msg = d.Message
					}
					errors = append(errors, msg)
				}
			}
			compiled := len(errors) == 0
			if compiled != (row[3] == "true") {
				t.Fatalf("native compiled=%s, errors=%q", row[3], errors)
			}
			if !compiled {
				found := false
				for _, msg := range errors {
					if msg == row[5] {
						found = true
					}
				}
				if !found {
					t.Fatalf("native rejection %q; errors=%q", row[5], errors)
				}
				return
			}
			if row[4] != "true" || row[5] != "pass" {
				t.Fatalf("unexpected native success row: %q", row)
			}
			if route == "isTest" {
				InvalidateRuntimeCaches()
				for pass, temperature := range []string{"cold", "memory", "disk"} {
					t.Run(temperature, func(t *testing.T) {
						if pass == 2 {
							InvalidateRuntimeCaches()
						}
						ResetPerfCounters()
						result := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: method, Parallelism: 1, PerfCounters: true})
						if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
							t.Fatalf("summary: %+v", summary)
						}
						phases := SnapshotPerfCounters().Phases
						if pass == 1 && phases.MemoryCacheHits != 1 {
							t.Fatal("warm runner missed memory cache")
						}
						if pass == 2 && phases.DiskCacheHits != 1 {
							t.Fatal("restored runner missed disk cache")
						}
					})
				}
				return
			}
			program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(prepared.merged, prepared.body, api)})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(prepared.merged)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, temperature := range []string{"cold", "warm"} {
				t.Run(temperature, func(t *testing.T) {
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
