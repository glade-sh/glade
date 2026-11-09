package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestR6FlowMalformedGrantMetadataIsUnknown(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		missingParent, missingName bool
	}{{"missingParent", true, false}, {"missingName", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")}}}}
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001"}}}
			access := map[string]storage.Value{"SetupEntityId": storage.IDValue("0CP000000000001")}
			if !tc.missingParent {
				access["ParentId"] = storage.IDValue("0PS000000000001")
			}
			org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0J0000000000001": {ID: "0J0000000000001", Fields: access}}}
			name := map[string]storage.Value{}
			if !tc.missingName {
				name["DeveloperName"] = storage.StringValue("R6_Bypass")
			}
			org.Objects["CustomPermission"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0CP000000000001": {ID: "0CP000000000001", Fields: name}}}
			engine := NewEngine(&org)
			_, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "false"})
			if !handled || supported {
				t.Fatalf("malformed metadata treated as denial: supported=%v handled=%v", supported, handled)
			}
		})
	}
}

func TestR6FlowBeforeSaveUncapturedLoopBodyIsUnsupported(t *testing.T) {
	for _, kind := range []string{"decision", "recordLookup", "loop", "fieldUpdate"} {
		t.Run(kind, func(t *testing.T) {
			assignment := storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "Items", Operator: "Assign", SourceField: "Records"}}
			loop := storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{CollectionReference: "Items", IterationOrder: "Asc", Steps: []storage.FlowStep{{Kind: kind}}}}
			rule := storage.FlowRule{TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Variables: []storage.FlowVariable{{Name: "Items", DataType: "SObject", ObjectType: "Account", IsCollection: true}}, Steps: []storage.FlowStep{assignment, loop}}
			if flowTriggeredStepsSafe(rule, rule.Steps, "before-save") {
				t.Fatalf("uncaptured %s body admitted", kind)
			}
		})
	}
}
