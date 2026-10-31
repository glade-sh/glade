package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
)

// A v15 runtime can retain after-save assignment formula markers from before
// the captured-route narrowing. A fresh source digest does not make it safe.
func TestFlowFormulaFrameRejectLegacyDiskRuntime(t *testing.T) {
	const fixture = "testdata/conformance/flow_formula_frame"
	const legacyABI = "apextest-runtime-v15"
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			key, built, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
			if err != nil {
				t.Fatal(err)
			}
			org := built.restored.CloneOrg()
			account := org.Objects["Account"]
			var marked, errors int
			clearUpdates := func(updates []storage.WorkflowFieldUpdate) {
				for i := range updates {
					if updates[i].FlowFrameFormula {
						marked++
						updates[i].FlowFrameFormula = false
					}
				}
			}
			clearError := func(ce *storage.FlowCustomError) {
				for i := range ce.Messages {
					if len(ce.Messages[i].Formulas) != 0 {
						errors++
						ce.Messages[i].Formulas = nil
					}
				}
			}
			var clearSteps func([]storage.FlowStep)
			clearSteps = func(steps []storage.FlowStep) {
				for i := range steps {
					clearUpdates(steps[i].FieldUpdates)
					clearUpdates(steps[i].RecordCreate.InputAssignments)
					clearError(&steps[i].CustomError)
					clearSteps(steps[i].FaultBranch)
					for j := range steps[i].Branches {
						clearSteps(steps[i].Branches[j].Steps)
					}
				}
			}
			for i := range account.Definition.FlowRules {
				rule := &account.Definition.FlowRules[i]
				clearUpdates(rule.FieldUpdates)
				for j := range rule.RecordCreates {
					clearUpdates(rule.RecordCreates[j].InputAssignments)
				}
				for j := range rule.CustomErrors {
					clearError(&rule.CustomErrors[j])
				}
				clearSteps(rule.Steps)
				for _, steps := range rule.StepTargets {
					clearSteps(steps)
				}
			}
			org.Objects["Account"] = account
			if marked == 0 || errors == 0 {
				t.Fatalf("fixture did not remove captured fields: updates=%d errors=%d", marked, errors)
			}
			input, err := validatedDiskRuntimeInput(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			entry, err := startupcache.NewEntryWithValidatedInput(input, org, startupcache.CompiledRuntime{Methods: built.Methods, Classes: built.Classes, Triggers: built.Triggers, PageNames: built.PageNames})
			if err != nil {
				t.Fatal(err)
			}
			entry.RuntimeABI, entry.RuntimeKey = legacyABI, string(key)
			if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
				t.Fatal(err)
			}
			legacy, err := startupcache.ReadFreshRuntimeWithValidatedInput(root, startupcache.SubdirTest, startupcache.Version, legacyABI, string(key), input)
			if err != nil || legacy == nil {
				t.Fatalf("legacy fixture is not otherwise fresh: entry=%v err=%v", legacy != nil, err)
			}
			if _, ok := runtimeCacheEntryFromStartup(*legacy); !ok {
				t.Fatal("legacy fixture has invalid runtime structure")
			}
			InvalidateRuntimeCaches()
			if _, ok := tryLoadDiskRuntimeWithSourceDigests(index, nil, key); ok {
				t.Fatal("accepted v15 runtime containing stale Flow models")
			}
			if _, ok := tryLoadDiskRuntimeWithPerf(index, nil, key, newRunPerfCounters(true)); ok {
				t.Fatal("perf reader accepted v15 stale Flow models")
			}
			if _, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), true); err != nil {
				t.Fatal(err)
			}
			persisted, err := startupcache.Read(root, startupcache.SubdirTest)
			if err != nil || persisted == nil || persisted.RuntimeABI != testRuntimeCacheABI || persisted.RuntimeABI == legacyABI {
				t.Fatalf("rebuilt runtime cache = %#v, err=%v", persisted, err)
			}
			want := map[string]any{}
			for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 4)
				if len(cells) == 4 && cells[0] == api && cells[1] == "istest" {
					var value any
					if err := json.Unmarshal([]byte(cells[3]), &value); err != nil {
						t.Fatal(err)
					}
					want[cells[2]] = value
				}
			}
			if len(want) != 5 {
				t.Fatalf("native rows = %d, want 5", len(want))
			}
			InvalidateRuntimeCaches()
			ResetPerfCounters()
			t.Cleanup(ResetPerfCounters)
			run := Run(index, Options{SelectedClasses: []string{"FFFrameProbeTest"}, SelectedMethod: "capture", RuntimeRESTAPIVersion: "67.0", PerfCounters: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("selection=%+v problem=%s", run.Summary(), firstRunProblem(run))
			}
			problem := run.Suites[0].Cases[0].Problem
			if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
				t.Fatalf("missing observation: %+v", problem)
			}
			checkFlowFollowupsPayload(t, problem.Message, "FFFRAME|", want)
			if phases := SnapshotPerfCounters().Phases; phases.DiskCacheHits != 1 || phases.CacheMisses != 0 {
				t.Fatalf("warm runtime phases=%#v", phases)
			}
		})
	}
}
