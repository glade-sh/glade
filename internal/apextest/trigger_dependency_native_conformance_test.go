package apextest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// The source-unit oracle is a check-only deployment. Partial execution of an
// unrelated test is Glade's policy; it does not claim a failed deploy can run.
func TestTriggerDependencyNativeConformance(t *testing.T) {
	rows := sourceBoundaryRows(t, "road131")
	if len(rows) != 12 {
		t.Fatalf("native rows=%d, want 12", len(rows))
	}
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, row := range rows {
		t.Run(row.API+"/"+row.ID+"/"+row.Route, func(t *testing.T) {
			index := dependencyGateIndex(t, dependencyGateCell{API: row.API, Classes: row.Classes, Triggers: row.Triggers})
			switch row.Route {
			case "checkOnly":
				var native struct {
					Success  bool
					Failures []struct {
						FullName, ComponentType, Problem string
						LineNumber                       int
					}
				}
				if err := json.Unmarshal(row.Native, &native); err != nil {
					t.Fatal(err)
				}
				want := map[string]testreport.CompileProblem{}
				for _, failure := range native.Failures {
					if failure.ComponentType != "ApexClass" && failure.ComponentType != "ApexTrigger" {
						t.Fatalf("non-Apex capture failure: %+v", failure)
					}
					if _, exists := want[failure.FullName]; exists {
						t.Fatalf("multiple native problems for %s", failure.FullName)
					}
					want[failure.FullName] = testreport.CompileProblem{Line: failure.LineNumber, Message: failure.Problem}
				}
				if native.Success != (len(want) == 0) {
					t.Fatal("native deployment status disagrees with its Apex failures")
				}
				check := func(entries []testreport.CompileExclusion) {
					t.Helper()
					if len(entries) != len(want) {
						t.Errorf("native excluded %d units; Glade excluded %d: %+v", len(want), len(entries), entries)
					}
					for _, entry := range entries {
						expected, exists := want[entry.ClassName]
						if !exists || entry.Line != expected.Line || entry.Message != expected.Message {
							t.Errorf("%s native %+v; Glade line=%d message=%q", entry.ClassName, expected, entry.Line, entry.Message)
						}
						if len(entry.Problems) > 0 && (len(entry.Problems) != 1 || entry.Problems[0] != expected) {
							t.Errorf("%s native has one problem %+v; Glade problems=%+v", entry.ClassName, expected, entry.Problems)
						}
					}
					for name := range want {
						found := false
						for _, entry := range entries {
							found = found || entry.ClassName == name
						}
						if !found {
							t.Errorf("missing native exclusion %s: %+v", name, want[name])
						}
					}
				}
				gate := dependencyGateExclusions(t, index)
				check(gate.excluded)
				for _, phase := range []string{"cold", "warm"} {
					t.Run(phase, func(t *testing.T) {
						run := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"R131IndependentTest"}, Parallelism: 1}, nil)
						check(run.ExcludedClasses)
						if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.CompileErrors != 0 {
							t.Fatalf("unrelated test must run under partial execution: %+v", run)
						}
					})
				}
			case "anonymous":
				var native struct{ Compiled, Success bool }
				if err := json.Unmarshal(row.Native, &native); err != nil || !native.Compiled || !native.Success {
					t.Fatalf("missing successful native anonymous answer: %s", row.Native)
				}
				base := conformanceSchemaBase(t, dependencyGateIndex(t, dependencyGateCell{API: row.API}), row.API)
				runtime := conformanceSchemaRequestRuntime(t, index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: base.orgSource})
				program, err := vm.CompileAnonymousWithOptions(row.Source, vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				for _, phase := range []string{"cold", "warm"} {
					if _, err := runner.execute(program); err != nil {
						t.Fatalf("%s native anonymous DML succeeds; Glade: %v", phase, err)
					}
				}
			case "isTest":
				var native []struct{ MethodName, Outcome string }
				if err := json.Unmarshal(row.Native, &native); err != nil || len(native) != 1 || native[0].MethodName != "observe" || native[0].Outcome != "Pass" {
					t.Fatalf("missing successful native named answer: %s", row.Native)
				}
				for _, phase := range []string{"cold", "warm"} {
					run := Run(index, Options{SelectedClasses: []string{row.ClassName}, SelectedMethod: "observe", Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || len(run.ExcludedClasses) != 0 {
						t.Fatalf("%s native named DML passes; Glade: %+v", phase, run)
					}
				}
			default:
				t.Fatalf("uncaptured route %q", strings.TrimSpace(row.Route))
			}
		})
	}
}
