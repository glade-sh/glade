package automation

import "testing"

func TestFlowFormulaElementMapExcludesUnrelatedMerges(t *testing.T) {
	raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"},
		Formulas:     []flowFormulaXML{{Name: "Variable", Expression: "{!Site}"}, {Name: "RecordOnly", Expression: "{!$Record.Name}"}},
		Assignments:  []flowAssignmentXML{{Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Variable"}}}}},
		CustomErrors: []flowCustomErrorXML{{Messages: []flowCustomErrorMessageXML{{ErrorMessage: "{!RecordOnly}"}}}},
	}
	got := flowFrameFormulaMap(raw, map[string]flowVariableXML{"site": {Name: "Site", DataType: "String"}})
	if _, ok := got["recordonly"]; ok {
		t.Fatal("unrelated Custom Error formula enabled")
	}
}

func TestFlowFormulaElementMapDoesNotActivateRecordOnlyElement(t *testing.T) {
	raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"},
		Formulas:     []flowFormulaXML{{Name: "RecordOnly", Expression: "{!$Record.Site}"}},
		CustomErrors: []flowCustomErrorXML{{Messages: []flowCustomErrorMessageXML{{ErrorMessage: "{!RecordOnly}"}}}},
	}
	got := flowFrameFormulaMap(raw, map[string]flowVariableXML{"site": {Name: "Site", DataType: "String"}})
	if got != nil {
		t.Fatalf("record-only element enabled formula bindings: %#v", got)
	}
}

func TestFlowFormulaElementMapKeepsSeparateAssignmentOrdinary(t *testing.T) {
	raw := flowXML{
		Start:     flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"},
		Variables: []flowVariableXML{{Name: "Site", DataType: "String"}},
		Formulas:  []flowFormulaXML{{Name: "Variable", Expression: "{!Site}"}, {Name: "RecordOnly", Expression: "{!$Record.Site}"}},
		Assignments: []flowAssignmentXML{
			{Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Variable"}}, {AssignToReference: "$Record.BillingCity", Operator: "Assign", Value: flowValueXML{ElementReference: "RecordOnly"}}}},
			{Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "RecordOnly"}}}},
		},
	}
	raw.frameFormulas = flowFrameFormulaMap(raw, flowVariableMap(raw.Variables))
	ordinary := flowFormulaMapWithConstants(raw.Formulas, raw.Constants)
	active := flowAssignmentElementFormulas(raw, ordinary, raw.Assignments[0])
	separate := flowAssignmentElementFormulas(raw, ordinary, raw.Assignments[1])
	if active["recordonly"] == ordinary["recordonly"] || separate["recordonly"] != ordinary["recordonly"] {
		t.Fatalf("active=%q separate=%q ordinary=%q", active["recordonly"], separate["recordonly"], ordinary["recordonly"])
	}
}
