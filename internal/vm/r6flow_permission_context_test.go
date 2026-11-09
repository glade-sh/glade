package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestR6FlowRespectsExecutionUserPermission(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	object := org.Objects["Account"]
	object.Definition.FlowRules = []storage.FlowRule{{Name: "CheckPermission", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate", Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{{Name: "Granted", Criteria: []storage.WorkflowCriteriaItem{{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "true"}}, Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Messages: []storage.FlowCustomErrorMessage{{Message: "granted"}}}}}}}}}}}
	org.Objects["Account"] = object
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String(string(storage.DefaultUserID))
	machine.executionUser.Fields["Permissions"] = Value{Kind: ValueList, List: []Value{String("R6_Bypass")}}
	engine := machine.newDMLEngine(nil)
	err := engine.ApplyBeforeSaveFlows([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
	if err == nil || !strings.Contains(err.Error(), "granted") {
		t.Fatalf("execution user grant was lost: %v", err)
	}
}
