package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPlatformEventFlowCreateUserResourceNativeMetadataRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/platform_event_flow_elements/project/force-app/main/default/flows/ElementSubscriber.flow-meta.xml"
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ElementSubscriber.flow-meta.xml")
			if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(source), "62.0", api)), 0600); err != nil {
				t.Fatal(err)
			}
			flow, diagnostics, err := loadFlow(path)
			if err != nil || len(diagnostics) != 0 || len(flow.Rules) != 1 {
				t.Fatalf("native flow accepted: %v %v; rules=%d", err, diagnostics, len(flow.Rules))
			}
			rule := flow.Rules[0]
			if len(rule.Variables) != 0 {
				t.Fatal("a global resource became a public interview variable")
			}
			check := func(t *testing.T, create storage.FlowRecordCreate) {
				t.Helper()
				for _, input := range create.InputAssignments {
					if input.Field == "Actor__c" {
						if input.SourceField != "$User.Id" {
							t.Fatalf("native current-user input lost in %s: %+v", create.Name, input)
						}
						return
					}
				}
				t.Fatalf("missing native current-user input in %s", create.Name)
			}
			if len(rule.RecordCreates) != 2 {
				t.Fatalf("create nodes=%d, want 2", len(rule.RecordCreates))
			}
			for _, create := range rule.RecordCreates {
				check(t, create)
			}
			found := 0
			var walk func(*testing.T, []storage.FlowStep)
			walk = func(t *testing.T, steps []storage.FlowStep) {
				for _, step := range steps {
					if step.Kind == "recordCreate" {
						check(t, step.RecordCreate)
						found++
					}
					for _, branch := range step.Branches {
						walk(t, branch.Steps)
					}
				}
			}
			walk(t, rule.Steps)
			for _, steps := range rule.StepTargets {
				walk(t, steps)
			}
			if found < 2 {
				t.Fatalf("executable create nodes=%d, want at least 2", found)
			}
			t.Run("serialized_record_inputs", func(t *testing.T) {
				raw, err := json.Marshal(rule)
				if err != nil {
					t.Fatal(err)
				}
				var restored storage.FlowRule
				if err := json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				if len(restored.RecordCreates) != len(rule.RecordCreates) {
					t.Fatal("serialized model lost create nodes")
				}
				for _, create := range restored.RecordCreates {
					check(t, create)
				}
				walk(t, restored.Steps)
				for _, steps := range restored.StepTargets {
					walk(t, steps)
				}
			})
		})
	}
}
