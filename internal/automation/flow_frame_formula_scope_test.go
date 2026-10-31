package automation

import "testing"

func TestFlowFrameFormulaMapCapturedScalarScope(t *testing.T) {
	for _, tc := range []struct {
		name, datatype string
		want           bool
	}{
		{"Site", "String", true},
		{"NumberOfEmployees", "Number", false},
		{"Flag", "Boolean", false},
	} {
		t.Run(tc.datatype, func(t *testing.T) {
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}, Formulas: []flowFormulaXML{{Name: "Value", Expression: "{!" + tc.name + "}"}}, Assignments: []flowAssignmentXML{{Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Value"}}}}}}
			variables := map[string]flowVariableXML{tc.name: {Name: tc.name, DataType: tc.datatype}}
			got := flowFrameFormulaMap(raw, variables) != nil
			if got != tc.want {
				t.Fatalf("frame map=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlowFrameFormulaMapCapturedTriggerScope(t *testing.T) {
	for _, tc := range []struct {
		name, trigger, mode string
		want                bool
	}{
		{"before_create", "RecordBeforeSave", "Create", true},
		{"after_create", "RecordAfterSave", "Create", false},
		{"update", "RecordBeforeSave", "Update", false},
		{"create_and_update", "RecordBeforeSave", "CreateAndUpdate", false},
		{"autolaunched", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: tc.trigger, RecordTriggerType: tc.mode}, Formulas: []flowFormulaXML{{Name: "Value", Expression: "{!Site}"}}, Assignments: []flowAssignmentXML{{Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "Value"}}}}}}
			vars := map[string]flowVariableXML{"site": {Name: "Site", DataType: "String"}}
			if got := flowFrameFormulaMap(raw, vars) != nil; got != tc.want {
				t.Fatalf("frame map=%v, want %v", got, tc.want)
			}
		})
	}
}
