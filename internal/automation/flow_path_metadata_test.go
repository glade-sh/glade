package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowPathMetadataNativeDeployedLoaderAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []struct {
			family, flow, kind, name string
			immediate                bool
		}{
			{"platform_transaction_paths", "CaptureAsync", "async", "DelayedPath", true},
			{"platform_transaction_paths", "CaptureScheduled", "scheduled", "DelayedPath", true},
			{"scheduled_flow_path", "TimedParcelFuture", "scheduled", "FutureMark", false},
		} {
			t.Run(api+"/"+shape.flow, func(t *testing.T) {
				// Load the exact owned metadata whose deployment is recorded in
				// the associated native org.tsv and ORG-RUN.txt fixtures.
				path := filepath.Join("..", "apextest", "testdata", "conformance", shape.family, "project", "force-app", "main", "default", "flows", shape.flow+".flow-meta.xml")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				local := filepath.Join(t.TempDir(), shape.flow+".flow-meta.xml")
				if err := os.WriteFile(local, []byte(strings.ReplaceAll(string(raw), "62.0", api)), 0600); err != nil {
					t.Fatal(err)
				}
				flow, diagnostics, err := loadFlow(local)
				if err != nil {
					t.Fatal(err)
				}
				if len(flow.Rules) != 1 {
					t.Fatalf("native-deployed %s metadata dropped: rules=%d; diagnostics=%v", shape.kind, len(flow.Rules), diagnostics)
				}
				rule := flow.Rules[0]
				encoded, err := json.Marshal(rule)
				if err != nil {
					t.Fatal(err)
				}
				var projection struct {
					Paths []struct {
						Name, Kind, OffsetUnit, TimeSource, RecordField string
						OffsetNumber                                    float64
						Steps                                           []storage.FlowStep
						StepTargets                                     map[string][]storage.FlowStep
					}
				}
				if err := json.Unmarshal(encoded, &projection); err != nil {
					t.Fatal(err)
				}
				if len(projection.Paths) != 1 {
					t.Fatalf("native-deployed %s path discarded from serialized FlowRule: paths=%d", shape.kind, len(projection.Paths))
				}
				pathModel := projection.Paths[0]
				if pathModel.Name != shape.name || pathModel.Kind != shape.kind {
					t.Fatalf("path identity: name=%q kind=%q", pathModel.Name, pathModel.Kind)
				}
				if shape.kind == "scheduled" && (pathModel.OffsetNumber != 0 || pathModel.OffsetUnit != "Minutes" || pathModel.RecordField != "NextRun__c" || pathModel.TimeSource != "RecordField") {
					t.Fatalf("scheduled metadata lost: %+v", pathModel)
				}
				if !storage.FlowStepsHaveReachableEffects(pathModel.Steps, pathModel.StepTargets, nil) {
					t.Fatal("path connector lost its executable target graph")
				}
				if got := storage.FlowStepsHaveReachableEffects(rule.Steps, rule.StepTargets, nil); got != shape.immediate {
					t.Fatalf("immediate entry effects=%t, want %t", got, shape.immediate)
				}
			})
		}
	}
}
