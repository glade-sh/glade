package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The PE allowlist is a local execution boundary. Native PE05/PE12 pin the
// supported loop and chatter routes; nesting must not widen that boundary.
func TestPlatformEventFlowLoopActionAllowlistRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, action := range []string{"apex", "chatterPost", "emailSimple", "unsupported"} {
			for _, route := range []string{"top_level", "loop", "loop_decision", "loop_target", "nested_loop_target"} {
				t.Run(api+"/engine_delivery/"+route+"/"+action, func(t *testing.T) {
					step := storage.FlowStep{Kind: "action", Action: storage.FlowAction{Name: "Notify", ActionType: action}}
					rule := storage.FlowRule{TriggerType: "PlatformEvent"}
					switch route {
					case "loop_decision":
						step = storage.FlowStep{Kind: "decision", Branches: []storage.FlowBranch{{Steps: []storage.FlowStep{step}}}}
					case "loop_target", "nested_loop_target":
						rule.StepTargets = map[string][]storage.FlowStep{"body": {step}}
						step = storage.FlowStep{Kind: "reference", Target: "body"}
					}
					if route == "nested_loop_target" {
						step = storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{Name: "Inner", Steps: []storage.FlowStep{step}}}
					}
					if route != "top_level" {
						step = storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{Name: "Each", Steps: []storage.FlowStep{step}}}
					}
					want := action == "apex" || action == "chatterPost"
					if got := flowTriggeredStepsSafe(rule, []storage.FlowStep{step}, "platform-event"); got != want {
						t.Fatalf("action allowlist accepted=%t, want %t", got, want)
					}
				})
			}
		}
	}
}

func TestPlatformEventFlowLoopTargetValidationRoutes(t *testing.T) {
	for _, target := range []string{"missing", "recursive"} {
		t.Run("engine_delivery/loop/"+target, func(t *testing.T) {
			loop := storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{Name: "Each", Steps: []storage.FlowStep{{Kind: "reference", Target: target}}}}
			rule := storage.FlowRule{StepTargets: map[string][]storage.FlowStep{"recursive": {loop}}}
			if flowTriggeredStepsSafe(rule, []storage.FlowStep{loop}, "platform-event") {
				t.Fatal("invalid loop target bypassed the platform-event gate")
			}
		})
	}
}

func TestPlatformEventFlowLoopGateRejectsBeforeEffectsEngineRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"rule_steps", "selected_branch"} {
			t.Run(api+"/engine_delivery/"+route, func(t *testing.T) {
				org := testOrg()
				org.APIVersion = api
				steps := []storage.FlowStep{
					{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{Name: "Write", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "Must not be written"}}}},
					{Kind: "loop", Loop: storage.FlowLoop{Name: "Each", CollectionReference: "Empty", Steps: []storage.FlowStep{{Kind: "action", Action: storage.FlowAction{Name: "Mail", ActionType: "emailSimple"}}}}},
				}
				rule := storage.FlowRule{Name: "Boundary", Active: true, TriggerType: "PlatformEvent", Steps: steps}
				if route == "selected_branch" {
					rule.Steps = nil
					rule.Branches = []storage.FlowBranch{{Default: true, Steps: steps}}
				}
				org.Objects["Boundary__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Boundary__e", Metadata: map[string]string{"publishBehavior": "PublishImmediately"}, FlowRules: []storage.FlowRule{rule}}, Records: map[storage.ID]storage.Record{}}
				engine := NewEngine(&org)
				err := engine.ApplyPlatformEventFlows([]storage.Record{{Object: "Boundary__e"}})
				if err == nil || !strings.Contains(err.Error(), "unsupported side-effect steps") {
					t.Fatalf("loop gate error=%v", err)
				}
				if got := len(org.Objects["Account"].Records); got != 0 {
					t.Fatalf("rejected interview wrote %d records", got)
				}
			})
		}
	}
}
