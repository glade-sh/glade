package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Permission grants are synthetic fixtures; expected values follow the native
// permission formula rows in supplemental/contexts/org.tsv.
func TestFormulaPermissionWorkflowPrincipal(t *testing.T) {
	org := testOrg()
	grants := permissionRegexOrg(true, "Bypass")
	for _, name := range []string{"User", "PermissionSet", "PermissionSetAssignment", "SetupEntityAccess", "CustomPermission"} {
		org.Objects[name] = grants.Objects[name]
	}
	account := org.Objects["Account"]
	account.Definition.Fields["Granted__c"] = storage.Field{APIName: "Granted__c", Type: storage.FieldBoolean}
	account.Definition.WorkflowRules = []storage.WorkflowRule{{Name: "Grant", Active: true, FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Copy", Field: "Granted__c", Formula: "$Permission.Bypass"}}}}
	org.Objects["Account"] = account
	engine := NewEngine(&org)
	engine.UserID = permTestUser
	result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A")}}})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("insert: %+v", result)
	}
	if got := org.Objects["Account"].Records[result[0].ID].Fields["Granted__c"]; got.Kind != storage.ValueBoolean || !got.Boolean {
		t.Errorf("workflow principal lost: %+v", got)
	}
}
