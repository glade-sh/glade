package automation

import (
	"encoding/xml"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/storage"
)

// The captured oversized index must never wrap into an existing condition on
// any caller of the shared parser. The adjacent oversized index also wraps to
// a valid condition in the old parser; 1 and 2 are the valid boundary controls.
var flowLogicRouteCases = []struct {
	name  string
	logic string
	valid bool
}{
	{"first", "1 OR 2", true},
	{"last", "2 OR 2", true},
	{"overflow_first", "18446744073709551617 OR 2", false},
	{"overflow_last", "18446744073709551618 OR 2", false},
}

func TestFlowLogicOverflowDecisionRoutes(t *testing.T) {
	for _, tc := range flowLogicRouteCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, route := range []string{"flat", "routed", "step"} {
				t.Run(route, func(t *testing.T) {
					raw := flowLogicRouteFixture(tc.logic)
					if route == "flat" {
						raw.Decisions[0].Rules[0].Connector = flowConnectorXML{}
						raw.Decisions[0].DefaultConnector = flowConnectorXML{}
						raw.Start.Connector = flowConnectorXML{}
						rule, diagnostics := flowLogicRouteLoad(t, raw)
						if !tc.valid {
							flowLogicRouteDiagnostic(t, diagnostics, "condition logic")
							if len(rule.Criteria) != 0 || rule.Formula != "" {
								t.Fatalf("overflow retained a flat decision condition: %#v", rule)
							}
							return
						}
						if len(diagnostics) != 0 || rule.Formula != flowLogicRouteFormula(tc.name) {
							t.Fatalf("valid flat decision: formula=%q diagnostics=%v", rule.Formula, diagnostics)
						}
						return
					}
					var branches []storage.FlowBranch
					var diagnostics []diagnostic.Diagnostic
					if route == "routed" {
						branches, diagnostics = modeledFlowDecisionBranches("owned-flow", "LogicRoutes", raw, nil, nil)
					} else {
						branches, diagnostics = modeledFlowStepDecisionBranches(raw.Decisions[0], raw, nil, nil, nil)
					}
					if !tc.valid {
						if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "condition logic") {
							t.Fatalf("overflow diagnostics=%v", diagnostics)
						}
						if route == "routed" && diagnostics[0].Code != "GLADEAUTO002" {
							t.Fatalf("routed overflow diagnostic=%v", diagnostics[0])
						}
						if len(branches) != 1 || !branches[0].Default || branches[0].Name != "Route_Default" {
							t.Fatalf("overflow retained a decision branch: %#v", branches)
						}
						return
					}
					if len(diagnostics) != 0 || len(branches) != 3 || !branches[2].Default {
						t.Fatalf("valid decision: branches=%#v diagnostics=%v", branches, diagnostics)
					}
					firstValue := "Alpha"
					if tc.name == "last" {
						firstValue = "Beta"
					}
					for i, want := range []string{firstValue, "Beta"} {
						if len(branches[i].Criteria) != 1 || branches[i].Criteria[0].Field != "Name" || branches[i].Criteria[0].Value != want {
							t.Fatalf("valid branch %d=%#v", i, branches[i])
						}
					}
				})
			}
		})
	}
}

func TestFlowLogicOverflowGraphCollection(t *testing.T) {
	for _, tc := range flowLogicRouteCases {
		for _, withDefault := range []bool{false, true} {
			name := "without_default"
			if withDefault {
				name = "with_default"
			}
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				raw := flowLogicRouteFixture(tc.logic)
				if !withDefault {
					raw.Decisions[0].DefaultConnector = flowConnectorXML{}
				}
				sources := collectFlowGraphSources(raw)
				if !tc.valid && !withDefault {
					if sources != nil {
						t.Fatalf("overflow without default retained graph: %#v", sources)
					}
					return
				}
				if sources == nil {
					t.Fatal("missing graph for supported/default route")
				}
				want := map[string]int{}
				if withDefault {
					want["unmatched"] = 1
				}
				if tc.valid {
					want["matched"] = 2
				}
				if !reflect.DeepEqual(sources.weights["route"], want) {
					t.Fatalf("decision edges=%v, want %v", sources.weights["route"], want)
				}
			})
		}
	}
}

func TestFlowLogicOverflowEntryFormulaRoutes(t *testing.T) {
	for _, route := range []struct {
		name, trigger, event string
		changed              bool
	}{
		{"before_update", "RecordBeforeSave", "Update", false},
		{"before_changed_update", "RecordBeforeSave", "Update", true},
		{"after_changed_update", "RecordAfterSave", "Update", true},
		{"before_create_and_update", "RecordBeforeSave", "CreateAndUpdate", false},
		{"after_create_and_update", "RecordAfterSave", "CreateAndUpdate", false},
	} {
		for _, tc := range flowLogicRouteCases {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				raw := flowLogicRouteFixture(tc.logic)
				raw.Decisions = nil
				raw.Start.Connector.TargetReference = "Matched"
				raw.Start.TriggerType = route.trigger
				raw.Start.RecordTriggerType = route.event
				raw.Start.DoesRequireRecordChangedToMeetCriteria = route.changed
				raw.Start.FilterLogic = tc.logic
				raw.Start.Filters = []flowFilterXML{
					{Field: "Name", Operator: "EqualTo", Value: flowValueXML{StringValue: "Alpha"}},
					{Field: "Name", Operator: "EqualTo", Value: flowValueXML{StringValue: "Beta"}},
				}
				rule, diagnostics := flowLogicRouteLoad(t, raw)
				if !tc.valid {
					flowLogicRouteDiagnostic(t, diagnostics, "start filter logic")
					if rule.Formula != "" || rule.CoreEntryFilterLogic || len(rule.Criteria) != 2 || len(rule.CriteriaGroups) != 0 {
						t.Fatalf("overflow did not retain AND entry fallback: %#v", rule)
					}
					return
				}
				if len(diagnostics) != 0 || !rule.CoreEntryFilterLogic || len(rule.Criteria) != 0 || rule.Formula != flowLogicRouteFormula(tc.name) {
					t.Fatalf("valid entry formula: rule=%#v diagnostics=%v", rule, diagnostics)
				}
			})
		}
	}
}

func flowLogicRouteFormula(name string) string {
	if name == "last" {
		return `OR(Name = "Beta",Name = "Beta")`
	}
	return `OR(Name = "Alpha",Name = "Beta")`
}

func flowLogicRouteFixture(logic string) flowXML {
	return flowXML{
		APIVersion: "67.0", ProcessType: "AutoLaunchedFlow", Status: "Active",
		Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Connector: flowConnectorXML{TargetReference: "Route"}},
		Decisions: []flowDecisionXML{{Name: "Route", DefaultConnector: flowConnectorXML{TargetReference: "Unmatched"}, Rules: []flowDecisionRuleXML{{
			Name: "Match", ConditionLogic: logic, Connector: flowConnectorXML{TargetReference: "Matched"},
			Conditions: []flowConditionXML{
				{LeftValueReference: "$Record.Name", Operator: "EqualTo", RightValue: flowValueXML{StringValue: "Alpha"}},
				{LeftValueReference: "$Record.Name", Operator: "EqualTo", RightValue: flowValueXML{StringValue: "Beta"}},
			},
		}}}},
		Assignments: []flowAssignmentXML{
			{Name: "Matched", Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{StringValue: "matched"}}}},
			{Name: "Unmatched", Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{StringValue: "unmatched"}}}},
		},
	}
}

func flowLogicRouteLoad(t *testing.T, raw flowXML) (storage.FlowRule, []diagnostic.Diagnostic) {
	t.Helper()
	encoded, err := xml.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(encoded), "<flowXML>", `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">`, 1)
	source = strings.Replace(source, "</flowXML>", "</Flow>", 1)
	path := filepath.Join(t.TempDir(), "LogicRoutes.flow-meta.xml")
	writeWorkflowTestFile(t, path, source)
	flow, diagnostics, err := loadFlow(path)
	if err != nil || len(flow.Rules) != 1 {
		t.Fatalf("load: error=%v rules=%#v diagnostics=%v", err, flow.Rules, diagnostics)
	}
	return flow.Rules[0], diagnostics
}

func flowLogicRouteDiagnostic(t *testing.T, diagnostics []diagnostic.Diagnostic, message string) {
	t.Helper()
	if len(diagnostics) != 1 || diagnostics[0].Code != "GLADEAUTO002" || !strings.Contains(diagnostics[0].Message, message) {
		t.Fatalf("overflow diagnostics=%v", diagnostics)
	}
}
