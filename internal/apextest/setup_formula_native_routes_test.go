package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Replay native assertion transports. Each method uses a fresh @IsTest org.
func TestSetupFormulaNativeAutomationAndOrdering(t *testing.T) {
	for _, family := range []struct {
		name, class string
		methods     map[string]map[string]any
	}{
		{"setup_formula_automation", "SetupAutomationProbeTest", map[string]map[string]any{"actor": {"workflow": false, "flow": false}}},
		{"setup_formula_ordering", "SetupOrderingProbeTest", map[string]map[string]any{"actor": {"top": "Z", "child": "Z"}, "control": {"top": "A", "child": "A"}}},
		{"setup_formula_flow_types", "SetupFlowTypesProbeTest", map[string]map[string]any{"actor": {"numeric": true, "text": true}, "control": {"numeric": false, "text": false}}},
		{"setup_formula_flow_rhs", "SetupFlowTypesProbeTest", map[string]map[string]any{"actor": {"numeric": true, "text": true}, "control": {"numeric": false, "text": false}}},
		{"setup_formula_vm", "SetupFormulaVMProbeTest", map[string]map[string]any{"actor": {"defaultAfter": "actor", "defaultBefore": nil, "trigger": false, "recalc": false, "recalcSuccess": true}}},
		{"setup_formula_query_shapes", "SetupQueryShapesProbeTest", map[string]map[string]any{"actor": {"flowCriterion": true, "nested": false, "semijoin": float64(1), "aggregate": float64(4)}, "control": {"flowCriterion": false, "nested": true, "semijoin": float64(0), "aggregate": float64(2)}}},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			for method, want := range family.methods {
				t.Run(family.name+"/"+api+"/"+method, func(t *testing.T) {
					source := filepath.Join("testdata", "conformance", family.name, "project")
					root := t.TempDir()
					err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						data, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						rel, err := filepath.Rel(source, path)
						if err != nil {
							return err
						}
						writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", api))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{family.class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing assertion transport: %+v", run.Suites)
					}
					problem := run.Suites[0].Cases[0].Problem
					if !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("unexpected failure: %+v", problem)
					}
					_, payload, found := strings.Cut(problem.Message, "OBS|")
					var got map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v glade=%s", want, problem.Message)
					}
				})
			}
		}
	}
}
