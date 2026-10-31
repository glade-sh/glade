package automation

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Variable targets have no native row: their formula modeling must match the
// ordinary shipping normalization, including within a captured mixed element.
func TestFlowVariableAssignmentFormulaUsesOrdinaryMap(t *testing.T) {
	for _, target := range []string{"Output", "Result"} {
		t.Run(target, func(t *testing.T) {
			assignment := flowAssignmentXML{Name: "Mixed", Items: []flowAssignmentItemXML{
				{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Combined"}},
				{AssignToReference: target, Operator: "Assign", Value: flowValueXML{ElementReference: "Combined"}},
			}}
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: "Site", DataType: "String"}, {Name: target, DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "Combined", Expression: `{!$Record.Site} & {!Site}`}}, Assignments: []flowAssignmentXML{assignment}}
			vars := flowVariableMap(raw.Variables)
			ordinary := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
			raw.frameFormulas = flowFrameFormulaMap(raw, vars)
			branch, diagnostics := flowPopulateBranchBody(storage.FlowBranch{}, "Mixed", "", raw, ordinary, vars, make(map[string]bool))
			if len(diagnostics) != 0 || len(branch.Steps) != 2 {
				t.Fatalf("steps=%#v diagnostics=%v", branch.Steps, diagnostics)
			}
			if got := branch.Steps[1].Assignment.Formula; got != ordinary["combined"] {
				t.Fatalf("variable formula %q, shipping formula %q", got, ordinary["combined"])
			}
			if got := branch.Steps[0].FieldUpdates[0].Formula; got == ordinary["combined"] {
				t.Fatalf("captured record-field formula lost distinct binding: %q", got)
			}
		})
	}
}

func TestFlowVariableOnlyAssignmentDoesNotEnableFramedGraph(t *testing.T) {
	for _, target := range []string{"Output", "Result"} {
		raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: "Site", DataType: "String"}, {Name: target, DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "Combined", Expression: `{!$Record.Site} & {!Site}`}}, Assignments: []flowAssignmentXML{{Name: "OnlyVariable", Items: []flowAssignmentItemXML{{AssignToReference: target, Operator: "Assign", Value: flowValueXML{ElementReference: "Combined"}}}}}}
		if got := flowFrameFormulaMap(raw, flowVariableMap(raw.Variables)); got != nil {
			t.Errorf("%s enabled framed graph: %#v", target, got)
		}
	}
}

func TestFlowOtherAssignmentOperatorsUseOrdinaryMap(t *testing.T) {
	for _, operator := range []string{"Add", "AssignCount"} {
		t.Run(operator, func(t *testing.T) {
			assignment := flowAssignmentXML{Name: "Mixed", Items: []flowAssignmentItemXML{
				{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Combined"}},
				{AssignToReference: "Output", Operator: operator, Value: flowValueXML{ElementReference: "Combined"}},
			}}
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: "Site", DataType: "String"}, {Name: "Output", DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "Combined", Expression: `{!$Record.Site} & {!Site}`}}, Assignments: []flowAssignmentXML{assignment}}
			vars := flowVariableMap(raw.Variables)
			ordinary := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
			raw.frameFormulas = flowFrameFormulaMap(raw, vars)
			branch, diagnostics := flowPopulateBranchBody(storage.FlowBranch{}, "Mixed", "", raw, ordinary, vars, make(map[string]bool))
			if len(diagnostics) != 0 || len(branch.Steps) != 2 {
				t.Fatalf("steps=%#v diagnostics=%v", branch.Steps, diagnostics)
			}
			if got := branch.Steps[1].Assignment.Formula; got != ordinary["combined"] {
				t.Fatalf("%s formula %q, shipping formula %q", operator, got, ordinary["combined"])
			}
		})
	}
}

func TestFlowOtherRecordAssignmentOperatorsDoNotActivateBindings(t *testing.T) {
	for _, operator := range []string{"Subtract", "AddItem"} {
		t.Run(operator, func(t *testing.T) {
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: "Site", DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "Combined", Expression: `{!$Record.Site} & {!Site}`}}, Assignments: []flowAssignmentXML{{Name: "Other", Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: operator, Value: flowValueXML{ElementReference: "Combined"}}}}}}
			if got := flowFrameFormulaMap(raw, flowVariableMap(raw.Variables)); got != nil {
				t.Fatalf("%s enabled uncaptured bindings: %#v", operator, got)
			}
		})
	}
}

func TestFlowOtherRecordOperatorInMixedElementUsesOrdinaryFormula(t *testing.T) {
	for _, operator := range []string{"Subtract", "AddItem"} {
		t.Run(operator, func(t *testing.T) {
			assignment := flowAssignmentXML{Name: "Mixed", Items: []flowAssignmentItemXML{
				{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Combined"}},
				{AssignToReference: "$Record.BillingCity", Operator: operator, Value: flowValueXML{ElementReference: "Combined"}},
			}}
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: "Site", DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "Combined", Expression: `{!$Record.Site} & {!Site}`}}, Assignments: []flowAssignmentXML{assignment}}
			vars := flowVariableMap(raw.Variables)
			ordinary := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
			raw.frameFormulas = flowFrameFormulaMap(raw, vars)
			branch, diagnostics := flowPopulateBranchBody(storage.FlowBranch{}, "Mixed", "", raw, ordinary, vars, make(map[string]bool))
			if len(diagnostics) != 0 || len(branch.Steps) != 2 || len(branch.Steps[1].FieldUpdates) != 1 {
				t.Fatalf("steps=%#v diagnostics=%v", branch.Steps, diagnostics)
			}
			update := branch.Steps[1].FieldUpdates[0]
			if update.Formula != ordinary["combined"] || update.FlowFrameFormula {
				t.Fatalf("%s selected uncaptured formula: %#v", operator, update)
			}
		})
	}
}
