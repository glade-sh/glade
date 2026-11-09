package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestR6FlowExecutionUserRevokedGrantDoesNotMatch(t *testing.T) {
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "PermissionSet", "PermissionSetAssignment"} {
		storage.EnsureStandardObject(&org, name)
	}
	object := org.Objects["Account"]
	object.Definition.FlowRules = []storage.FlowRule{{Name: "CheckPermission", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate", Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Name: "Granted", Criteria: []storage.WorkflowCriteriaItem{{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "true"}}, Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Messages: []storage.FlowCustomErrorMessage{{Message: "grant should be revoked"}}}}}}}}}}}
	org.Objects["Account"] = object
	setID := storage.ID("0PS000000000001")
	ps := storage.Record{ID: setID, Object: "PermissionSet", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))}}
	ps.System.IsDeleted = true
	sets := org.Objects["PermissionSet"]
	sets.Records[setID] = ps
	org.Objects["PermissionSet"] = sets
	assignments := org.Objects["PermissionSetAssignment"]
	assignments.Records["0Pa000000000001"] = storage.Record{ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue(setID)}}
	org.Objects["PermissionSetAssignment"] = assignments
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String(string(storage.DefaultUserID))
	engine := machine.newDMLEngine(nil)
	if err := engine.ApplyBeforeSaveFlows([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}}); err != nil {
		t.Fatalf("revoked grant selected branch: %v", err)
	}
}
