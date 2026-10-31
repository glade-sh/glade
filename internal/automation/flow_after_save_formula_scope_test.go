package automation

import "testing"

// Before-save assignment and after-save creation are captured; after-save
// assignment retains the shipping formula selection even for shared resources.
func TestFlowAfterSaveAssignmentKeepsOrdinaryFormulaSelection(t *testing.T) {
	for _, name := range []string{"Site", "Description"} {
		for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
			t.Run(name+"/"+trigger, func(t *testing.T) {
				assignment := flowAssignmentXML{Items: []flowAssignmentItemXML{{AssignToReference: "$Record.Description", Operator: "Assign", Value: flowValueXML{ElementReference: "F"}}}}
				create := flowRecordCreateXML{Fields: []flowFieldAssignmentXML{{Field: "Description", Value: flowValueXML{ElementReference: "F"}}}}
				raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: trigger, RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: name, DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "F", Expression: "{!" + name + "}"}}, Assignments: []flowAssignmentXML{assignment}, RecordCreates: []flowRecordCreateXML{create}}
				raw.frameFormulas = flowFrameFormulaMap(raw, flowVariableMap(raw.Variables))
				selected := flowAssignmentElementSelection(raw, assignment)
				if (len(selected) != 0) != (trigger == "RecordBeforeSave") {
					t.Fatalf("assignment selection=%v", selected)
				}
				if got := len(flowCreateElementSelection(raw, create)) != 0; got != (trigger == "RecordAfterSave") {
					t.Fatalf("create selection=%v", got)
				}
			})
		}
	}
}

func TestFlowAfterSaveCustomErrorDoesNotEnableFrameFormulas(t *testing.T) {
	for _, name := range []string{"Site", "Description"} {
		raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordAfterSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: name, DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "F", Expression: "{!" + name + "}"}}, CustomErrors: []flowCustomErrorXML{{Messages: []flowCustomErrorMessageXML{{ErrorMessage: "{!F}"}}}}}
		if got := flowFrameFormulaMap(raw, flowVariableMap(raw.Variables)); len(got) != 0 {
			t.Errorf("%s enabled uncaptured custom error map: %v", name, got)
		}
	}
}

func TestFlowAfterSaveCustomErrorSharingCreateFormulaRemainsOrdinary(t *testing.T) {
	for _, name := range []string{"Site", "Description"} {
		create := flowRecordCreateXML{Fields: []flowFieldAssignmentXML{{Field: "Description", Value: flowValueXML{ElementReference: "F"}}}}
		raw := flowXML{Start: flowStartXML{Object: "Account", TriggerType: "RecordAfterSave", RecordTriggerType: "Create"}, Variables: []flowVariableXML{{Name: name, DataType: "String"}}, Formulas: []flowFormulaXML{{Name: "F", Expression: "{!" + name + "}"}}, RecordCreates: []flowRecordCreateXML{create}, CustomErrors: []flowCustomErrorXML{{Messages: []flowCustomErrorMessageXML{{ErrorMessage: "{!F}"}}}}}
		raw.frameFormulas = flowFrameFormulaMap(raw, flowVariableMap(raw.Variables))
		if len(flowCreateElementSelection(raw, create)) == 0 || len(flowCustomErrorFrameFormulas(raw)) != 0 {
			t.Errorf("%s: create=%v custom error=%v", name, flowCreateElementSelection(raw, create), flowCustomErrorFrameFormulas(raw))
		}
	}
}
