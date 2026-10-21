package automation

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// These fixtures preserve the existing local compiler's path-visited behavior.
// They do not assert Salesforce behavior for cyclic or overlapping Flow graphs.
func TestPathVisitedFlowTargetsStayCompact(t *testing.T) {
	for _, shape := range []string{"cycle", "loop-overlap", "sibling-loops"} {
		t.Run(shape, func(t *testing.T) {
			var sizes []int
			for _, depth := range []int{3, 5} {
				raw := pathVisitedGrowthFixture(shape, depth)
				if newFlowTargetGraph(raw) != nil {
					t.Fatal("fixture unexpectedly accepted by strict graph compiler")
				}
				rule := pathVisitedLoadRule(t, raw)
				pathVisitedRequireGeneric(t, rule)
				org := storage.NewOrgState()
				org.Metadata.Flows = []storage.FlowRule{rule}
				encoded, err := json.Marshal(org.Clone().Metadata.Flows)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("%d decisions: %d cloned JSON bytes", depth, len(encoded))
				if len(sizes) > 0 && len(encoded) > 3*sizes[0] {
					t.Fatalf("two additional shared decisions multiplied path model: %d -> %d bytes", sizes[0], len(encoded))
				}
				sizes = append(sizes, len(encoded))
			}
		})
	}
}

func TestPathVisitedFlowTargetsMatchLegacy(t *testing.T) {
	for _, shape := range []string{
		"self-return", "empty-chain", "empty-decision", "unsupported-empty-rule",
		"effectful-cycle", "error-outcome", "loop-overlap", "sibling-loops", "nested-loops",
	} {
		t.Run(shape, func(t *testing.T) {
			raw := pathVisitedControlFixture(shape)
			pathVisitedPrependSharedDecisions(&raw)
			normal := pathVisitedLoadRule(t, raw)
			pathVisitedRequireGeneric(t, normal)
			legacy := pathVisitedLegacyRule(t, raw, normal)
			for mask := 0; mask < 8; mask++ {
				for _, count := range []int{0, 1, 2} {
					for _, roundtrip := range []bool{false, true} {
						label := fmt.Sprintf("mask=%d/items=%d/json=%t", mask, count, roundtrip)
						t.Run(label, func(t *testing.T) {
							want := pathVisitedObserve(t, legacy, mask, count, roundtrip)
							got := pathVisitedObserve(t, normal, mask, count, roundtrip)
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("path behavior changed\nnormal: %#v\nlegacy: %#v", got, want)
							}
							if shape == "self-return" || shape == "empty-chain" || shape == "empty-decision" || shape == "unsupported-empty-rule" {
								if len(want.Actions) != 2 || !strings.HasPrefix(want.Actions[0], "Default|") {
									t.Fatalf("legacy empty outcome no longer reaches default: %#v", want)
								}
							}
							if shape == "effectful-cycle" && mask&1 != 0 && mask&4 == 0 {
								if len(want.Actions) != 1 || !strings.HasPrefix(want.Actions[0], "Effect|") {
									t.Fatalf("legacy effectful cycle did not truncate once: %#v", want)
								}
							}
						})
					}
				}
			}
		})
	}
}

func TestPathVisitedFlowEmptyStartKeepsRoutedFallback(t *testing.T) {
	raw := pathVisitedBase("Empty")
	raw.Start.Object = "Account"
	raw.Start.TriggerType = "RecordBeforeSave"
	raw.Start.RecordTriggerType = "Create"
	raw.Assignments = []flowAssignmentXML{
		{Name: "Empty", Connector: pathVisitedConnector("EmptyJoin")},
		{Name: "Marker", Items: []flowAssignmentItemXML{{AssignToReference: "Carrier.Description", Operator: "Assign", Value: flowValueXML{StringValue: "unused"}}}},
		{Name: "Matched", Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{StringValue: "matched"}}}},
		{Name: "Default", Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{StringValue: "default"}}}},
	}
	raw.ActionCalls = nil
	raw.Decisions = []flowDecisionXML{
		{Name: "Disconnected", Rules: []flowDecisionRuleXML{{Name: "Match", Conditions: []flowConditionXML{{
			LeftValueReference: "$Record.Name", Operator: "EqualTo", RightValue: flowValueXML{StringValue: "match"},
		}}, Connector: pathVisitedConnector("Matched")}}, DefaultConnector: pathVisitedConnector("Default")},
		{Name: "EmptyJoin", Rules: []flowDecisionRuleXML{{Name: "Continue", Conditions: []flowConditionXML{{
			LeftValueReference: "$Record.Name", Operator: "EqualTo", RightValue: flowValueXML{StringValue: "join"},
		}}, Connector: pathVisitedConnector("EmptySink")}}, DefaultConnector: pathVisitedConnector("EmptySink")},
		{Name: "EmptySink", DefaultConnector: pathVisitedConnector("EmptySink")},
	}
	normal := pathVisitedLoadRule(t, raw)
	pathVisitedRequireGeneric(t, normal)
	legacy := pathVisitedLegacyRule(t, raw, normal)
	for _, rule := range []storage.FlowRule{normal, legacy} {
		if len(rule.Steps) != 0 || len(rule.Branches) == 0 {
			t.Fatalf("empty ordered entry suppressed routed fallback: steps=%d branches=%d", len(rule.Steps), len(rule.Branches))
		}
	}
	for _, name := range []string{"match", "other"} {
		for _, roundtrip := range []bool{false, true} {
			var results []string
			for _, rule := range []storage.FlowRule{legacy, normal} {
				org := storage.NewOrgState()
				org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "Account", Fields: map[string]storage.Field{
						"Name": {APIName: "Name", Type: storage.FieldString}, "Description": {APIName: "Description", Type: storage.FieldString},
					}, FlowRules: []storage.FlowRule{pathVisitedRoundTrip(t, rule, roundtrip)},
				}}
				records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}}
				engine := dml.NewEngine(&org)
				if err := engine.ApplyBeforeSaveFlows(records); err != nil {
					t.Fatal(err)
				}
				results = append(results, records[0].Fields["Description"].String)
			}
			if results[0] != results[1] || results[0] == "" {
				t.Fatalf("routed fallback result: name=%s json=%t legacy=%q normal=%q", name, roundtrip, results[0], results[1])
			}
		}
	}
}

type pathVisitedObservation struct {
	Actions []string
	Output  dml.FlowInterviewOutput
	Error   string
}

// The behavior controls must exercise the path-aware representation rather than
// silently passing because their source shape stayed on the legacy compiler.
func pathVisitedRequireGeneric(t *testing.T, rule storage.FlowRule) {
	t.Helper()
	if len(rule.StepTargets) == 0 {
		t.Fatal("fixture did not select compact path targets")
	}
	entries := append([]storage.FlowStep(nil), rule.Steps...)
	for _, branch := range rule.Branches {
		entries = append(entries, branch.Steps...)
	}
	for _, step := range entries {
		if step.Kind == "reference" && step.VisitKey != "" {
			return
		}
	}
	t.Fatal("fixture has no path-aware execution entry")
}

func pathVisitedObserve(t *testing.T, rule storage.FlowRule, mask, count int, roundtrip bool) pathVisitedObservation {
	t.Helper()
	org := storage.NewOrgState()
	engine := dml.NewEngine(&org)
	var observed pathVisitedObservation
	engine.FlowActionInvokerWithContext = func(action storage.FlowAction, _ storage.Record, context dml.FlowActionContext) error {
		encoded, err := json.Marshal(struct {
			Scalars map[string]storage.Value
			Records map[string]storage.Record
		}{context.Scalars, context.Records})
		if err != nil {
			return err
		}
		observed.Actions = append(observed.Actions, action.Name+"|"+string(encoded))
		if action.Name == "Effect" && mask&4 != 0 {
			return fmt.Errorf("owned effect failure")
		}
		return nil
	}
	items := []storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("First")}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Second")}},
	}
	input := dml.FlowInterviewInput{
		Scalars:     map[string]storage.Value{"A": storage.BooleanValue(mask&1 != 0), "B": storage.BooleanValue(mask&2 != 0), "C": storage.BooleanValue(mask&4 != 0)},
		Records:     map[string]storage.Record{"Carrier": {Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Carrier")}}},
		Collections: map[string][]storage.Record{"Items": items[:count]},
	}
	var err error
	observed.Output, err = engine.RunAutolaunchedFlowWithOutput(pathVisitedRoundTrip(t, rule, roundtrip), input)
	if err != nil {
		observed.Error = err.Error()
	}
	return observed
}

func pathVisitedLoadRule(t *testing.T, raw flowXML) storage.FlowRule {
	t.Helper()
	encoded, err := xml.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(encoded), "<flowXML>", `<Flow xmlns="http://soap.sforce.com/2006/04/metadata">`, 1)
	text = strings.Replace(text, "</flowXML>", "</Flow>", 1)
	path := filepath.Join(t.TempDir(), "PathVisited.flow-meta.xml")
	writeWorkflowTestFile(t, path, text)
	flow, diagnostics, err := loadFlow(path)
	if err != nil || len(diagnostics) != 0 || len(flow.Rules) != 1 {
		t.Fatalf("owned path fixture: error=%v diagnostics=%v rules=%d", err, diagnostics, len(flow.Rules))
	}
	return flow.Rules[0]
}

func pathVisitedLegacyRule(t *testing.T, raw flowXML, header storage.FlowRule) storage.FlowRule {
	t.Helper()
	raw.graph = nil
	raw.graphStop = ""
	formulas, variables := flowFormulaMap(raw.Formulas), flowVariableMap(raw.Variables)
	compiled, diagnostics := flowPopulateBranchFromTarget(storage.FlowBranch{}, raw.Start.Connector.TargetReference, raw, formulas, variables)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	header.StepTargets = nil
	header.Steps = compiled.Steps
	header.Branches = nil
	if len(header.Steps) == 0 {
		branches, diagnostics := modeledFlowDecisionBranches("owned-path", header.Name, raw, formulas, variables)
		if len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		header.Branches = branches
	}
	return header
}

func pathVisitedRoundTrip(t *testing.T, rule storage.FlowRule, enabled bool) storage.FlowRule {
	t.Helper()
	if !enabled {
		return rule
	}
	encoded, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	var decoded storage.FlowRule
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func pathVisitedBase(start string) flowXML {
	raw := flowXML{APIVersion: "67.0", ProcessType: "AutoLaunchedFlow", Status: "Active", Start: flowStartXML{Connector: pathVisitedConnector(start)}}
	for _, name := range []string{"A", "B", "C"} {
		raw.Variables = append(raw.Variables, flowVariableXML{Name: name, DataType: "Boolean", IsInput: true})
	}
	raw.Variables = append(raw.Variables,
		flowVariableXML{Name: "Carrier", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true},
		flowVariableXML{Name: "Items", DataType: "SObject", ObjectType: "Account", IsCollection: true, IsInput: true})
	raw.Assignments = []flowAssignmentXML{{Name: "Finish", Items: []flowAssignmentItemXML{{AssignToReference: "Carrier.Description", Operator: "Assign", Value: flowValueXML{StringValue: "done"}}}, Connector: pathVisitedConnector("End")}}
	raw.ActionCalls = []flowActionCallXML{pathVisitedAction("End", "")}
	return raw
}

func pathVisitedGrowthFixture(shape string, depth int) flowXML {
	raw := pathVisitedBase("Route0")
	for i := 0; i < depth; i++ {
		next := "Finish"
		if i+1 < depth {
			next = fmt.Sprint("Route", i+1)
		}
		rule := pathVisitedOutcome("Match", "A", next)
		rule.ConditionLogic = "or"
		rule.Conditions = append(rule.Conditions, pathVisitedOutcome("Unused", "B", "").Conditions...)
		raw.Decisions = append(raw.Decisions, flowDecisionXML{Name: fmt.Sprint("Route", i), Rules: []flowDecisionRuleXML{rule}, DefaultConnector: pathVisitedConnector(next)})
	}
	switch shape {
	case "cycle":
		raw.ActionCalls[0].Connector = pathVisitedConnector("Route0")
	case "loop-overlap":
		raw.Start.Connector = pathVisitedConnector("Repeat")
		raw.Loops = []flowLoopXML{pathVisitedLoop("Repeat", "Route0", "End", "Item")}
		raw.ActionCalls[0].Connector = pathVisitedConnector("Repeat")
	case "sibling-loops":
		raw.Start.Connector = pathVisitedConnector("FirstLoop")
		raw.Loops = []flowLoopXML{pathVisitedLoop("FirstLoop", "Route0", "SecondLoop", "Item"), pathVisitedLoop("SecondLoop", "Route0", "After", "Item")}
		raw.ActionCalls = append(raw.ActionCalls, pathVisitedAction("After", ""))
	}
	return raw
}

func pathVisitedControlFixture(shape string) flowXML {
	if shape == "loop-overlap" || shape == "sibling-loops" {
		return pathVisitedGrowthFixture(shape, 2)
	}
	raw := pathVisitedBase("Entry")
	entry := flowDecisionXML{Name: "Entry", Rules: []flowDecisionRuleXML{pathVisitedOutcome("Selected", "A", "Entry")}, DefaultConnector: pathVisitedConnector("Default")}
	raw.Decisions = []flowDecisionXML{entry}
	raw.ActionCalls = append(raw.ActionCalls, pathVisitedAction("Default", "Finish"))
	switch shape {
	case "self-return":
	case "empty-chain":
		raw.Decisions[0].Rules[0].Connector = pathVisitedConnector("EmptyOne")
		raw.Assignments = append(raw.Assignments, flowAssignmentXML{Name: "EmptyOne", Connector: pathVisitedConnector("EmptyTwo")}, flowAssignmentXML{Name: "EmptyTwo", Connector: pathVisitedConnector("Entry")})
	case "empty-decision":
		raw.Decisions[0].Rules[0].Connector = pathVisitedConnector("EmptyDecision")
		raw.Decisions = append(raw.Decisions, flowDecisionXML{Name: "EmptyDecision", DefaultConnector: pathVisitedConnector("Entry")})
	case "unsupported-empty-rule":
		raw.Decisions[0].Rules[0].Conditions[0].Operator = "UnmodeledOperator"
	case "effectful-cycle":
		raw.Decisions[0].Rules[0].Connector = pathVisitedConnector("Effect")
		raw.ActionCalls = append(raw.ActionCalls, pathVisitedAction("Effect", "Entry"))
	case "error-outcome":
		raw.Decisions[0].Rules[0].Connector = pathVisitedConnector("Abort")
		raw.CustomErrors = []flowCustomErrorXML{{Name: "Abort", Messages: []flowCustomErrorMessageXML{{Message: "owned failure"}}}}
		raw.Decisions = append(raw.Decisions, flowDecisionXML{Name: "DisconnectedCycle", DefaultConnector: pathVisitedConnector("DisconnectedCycle")})
	case "nested-loops":
		raw.Start.Connector = pathVisitedConnector("Outer")
		raw.Decisions[0].DefaultConnector = pathVisitedConnector("Effect")
		raw.Loops = []flowLoopXML{pathVisitedLoop("Outer", "Inner", "Finish", "OuterItem"), pathVisitedLoop("Inner", "Entry", "Bridge", "InnerItem")}
		raw.ActionCalls = append(raw.ActionCalls, pathVisitedAction("Effect", "Inner"), pathVisitedAction("Bridge", "Outer"))
	}
	return raw
}

func pathVisitedPrependSharedDecisions(raw *flowXML) {
	for _, name := range []string{"PreludeTwo", "PreludeOne"} {
		next := raw.Start.Connector.TargetReference
		outcome := pathVisitedOutcome("Continue", "A", next)
		outcome.ConditionLogic = "or"
		outcome.Conditions = append(outcome.Conditions, pathVisitedOutcome("Other", "B", "").Conditions...)
		raw.Decisions = append(raw.Decisions, flowDecisionXML{
			Name: name, Rules: []flowDecisionRuleXML{outcome}, DefaultConnector: pathVisitedConnector(next),
		})
		raw.Start.Connector = pathVisitedConnector(name)
	}
}

func pathVisitedConnector(target string) flowConnectorXML {
	return flowConnectorXML{TargetReference: target}
}

func pathVisitedOutcome(name, input, target string) flowDecisionRuleXML {
	return flowDecisionRuleXML{Name: name, Connector: pathVisitedConnector(target), Conditions: []flowConditionXML{{
		LeftValueReference: input, Operator: "EqualTo", RightValue: flowValueXML{BooleanValue: "true"},
	}}}
}

func pathVisitedAction(name, target string) flowActionCallXML {
	return flowActionCallXML{Name: name, ActionType: "apex", ActionName: "Notice.run", Connector: pathVisitedConnector(target)}
}

func pathVisitedLoop(name, body, after, item string) flowLoopXML {
	return flowLoopXML{Name: name, CollectionReference: "Items", IterationOrder: "Asc", AssignNextValueToReference: item,
		NextValueConnector: pathVisitedConnector(body), NoMoreValuesConnector: pathVisitedConnector(after)}
}
