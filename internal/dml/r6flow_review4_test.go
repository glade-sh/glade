package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
	"testing"
)

func TestR6FlowExpiringAssignmentRemainsUnknown(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))}}}}
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001"), "ExpirationDate": storage.StringValue("2000-01-01")}}}}
	engine := NewEngine(&org)
	_, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "true"})
	if !handled || supported {
		t.Fatalf("expiration was treated as an active grant: supported=%v handled=%v", supported, handled)
	}
}

func TestR6FlowFlatUnknownGlobalDoesNotSelectDefault(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Definition.FlowRules = []storage.FlowRule{{Name: "UnknownProfile", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Branches: []storage.FlowBranch{
		{Name: "Match", Criteria: []storage.WorkflowCriteriaItem{{Field: "$Profile.Name", Operation: "equals", Value: "System Administrator"}}, Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Messages: []storage.FlowCustomErrorMessage{{Message: "match"}}}}}},
		{Name: "Default", Default: true, Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Messages: []storage.FlowCustomErrorMessage{{Message: "wrong default"}}}}}},
	}}}
	org.Objects["Account"] = object
	engine := NewEngine(&org)
	err := engine.ApplyBeforeSaveFlows([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
	if err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("unknown context selected default: %v", err)
	}
}

func TestR6FlowBeforeSaveLoopRequiresAssignedRecordCollection(t *testing.T) {
	loop := storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{CollectionReference: "Items", IterationOrder: "Asc", Steps: []storage.FlowStep{{Kind: "customError"}}}}
	rule := storage.FlowRule{TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Variables: []storage.FlowVariable{{Name: "Items", DataType: "SObject", ObjectType: "Account", IsCollection: true}}, Steps: []storage.FlowStep{loop}}
	if flowTriggeredStepsSafe(rule, rule.Steps, "before-save") {
		t.Fatal("unassigned record collection admitted")
	}
}
