package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowScalarTextDecisionPlainTriggerAndInterviewLowering(t *testing.T) {
	const project = "../dml/testdata/conformance/flow_text_prefix_suffix/project/force-app/main/default/flows"
	entries, err := os.ReadDir(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, entry := range entries {
			t.Run(api+"/"+entry.Name(), func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(project, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), entry.Name())
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0600); err != nil {
					t.Fatal(err)
				}
				flow, diagnostics, err := loadFlow(path)
				if err != nil || len(diagnostics) != 0 || len(flow.Rules) != 1 {
					t.Fatalf("load: %v diagnostics=%v rules=%d", err, diagnostics, len(flow.Rules))
				}
				rule := flow.Rules[0]
				if len(rule.Steps) == 0 || len(rule.Branches) != 0 {
					t.Fatalf("scalar decision needs framed execution; steps=%d legacy branches=%d", len(rule.Steps), len(rule.Branches))
				}
			})
		}
	}
}

func TestFlowScalarPrefixSuffixPlainTriggerGraphSelectionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, operator, dataType, reference string
		collection, want                    bool
	}{
		{name: "prefix_scalar", operator: "StartsWith", dataType: "String", reference: "InputText", want: true},
		{name: "suffix_scalar", operator: "EndsWith", dataType: "String", reference: "InputText", want: true},
		{name: "trimmed_case", operator: " startsWITH ", dataType: "string", reference: " inputTEXT ", want: true},
		{name: "record_field", operator: "StartsWith", dataType: "String", reference: "$Record.Name"},
		{name: "collection", operator: "StartsWith", dataType: "String", reference: "InputText", collection: true},
		{name: "number", operator: "StartsWith", dataType: "Number", reference: "InputText"},
		{name: "unsupported_operator", operator: "DoesNotContain", dataType: "String", reference: "InputText"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := flowXML{ProcessType: "AutoLaunchedFlow", Start: flowStartXML{Object: "Account", TriggerType: "RecordAfterSave", Connector: flowConnectorXML{TargetReference: "Choose"}}, Variables: []flowVariableXML{{Name: "InputText", DataType: tc.dataType, IsCollection: tc.collection}}, Decisions: []flowDecisionXML{{Rules: []flowDecisionRuleXML{{Conditions: []flowConditionXML{{LeftValueReference: tc.reference, Operator: tc.operator, RightValue: flowValueXML{StringValue: "world"}}}}}}}}
			if got := flowRequiresOrderedGraph(raw); got != tc.want {
				t.Fatalf("ordered graph=%t; want %t", got, tc.want)
			}
		})
	}
}

func TestFlowScalarPrefixSuffixPlainTriggerLaterBranchNeedsFrame(t *testing.T) {
	for _, operator := range []string{"StartsWith", "EndsWith"} {
		t.Run(operator, func(t *testing.T) {
			raw := flowXML{ProcessType: "AutoLaunchedFlow", Start: flowStartXML{Object: "Account", TriggerType: "RecordAfterSave"}, Variables: []flowVariableXML{{Name: "InputText", DataType: "String"}}, Decisions: []flowDecisionXML{{Rules: []flowDecisionRuleXML{
				{Conditions: []flowConditionXML{{LeftValueReference: "$Record.Name", Operator: "EqualTo", RightValue: flowValueXML{StringValue: "other"}}}},
				{Conditions: []flowConditionXML{{LeftValueReference: "InputText", Operator: operator, RightValue: flowValueXML{StringValue: "World"}}}},
			}}}}
			if !flowRequiresOrderedGraph(raw) {
				t.Fatal("a scalar text predicate after a record predicate still requires a frame")
			}
		})
	}
}
