package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// USER_MODE exposes all access errors through QueryException details:
// https://developer.salesforce.com/docs/platform/secure-coding/guide/secure-coding-access-control-protect-from-crud-fls-vulnerabilities.html
// This regression asserts permission-derived inclusion/exclusion, not pending
// native diagnostic text, ordering, or a version-specific complete map.
func TestUserModeProjectionAccessDetails(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Ledger__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Ledger__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
			"Id":        {APIName: "Id", Type: storage.FieldID},
			"Label__c":  {APIName: "Label__c", Type: storage.FieldString},
			"Text__c":   {APIName: "Text__c", Type: storage.FieldString},
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
			"Day__c":    {APIName: "Day__c", Type: storage.FieldDate},
		},
	}}
	const profileID = "00e000000000701"
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		profileID: {ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Projection probe")}},
	}}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"110000000000701": {Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Ledger__c"), "PermissionsRead": storage.BooleanValue(true),
		}},
	}}
	org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0FP000000000701": {Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Ledger__c"),
			"Field": storage.StringValue("Ledger__c.Amount__c"), "PermissionsRead": storage.BooleanValue(false),
		}},
		"0FP000000000702": {Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Ledger__c"),
			"Field": storage.StringValue("Ledger__c.Label__c"), "PermissionsRead": storage.BooleanValue(true),
		}},
		"0FP000000000703": {Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Ledger__c"),
			"Field": storage.StringValue("Ledger__c.Text__c"), "PermissionsRead": storage.BooleanValue(true),
		}},
		"0FP000000000704": {Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(profileID), "SObjectType": storage.StringValue("Ledger__c"),
			"Field": storage.StringValue("Ledger__c.Day__c"), "PermissionsRead": storage.BooleanValue(true),
		}},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.RunAsDepth = 1
	user := Object("User")
	user.Fields["Id"] = String("005000000000701")
	user.Fields["ProfileId"] = String(profileID)
	machine.testContext.CurrentUser = user
	machine.currentMethod.APIVersion = "63.0"
	query := soql.Query{
		Object: "Ledger__c", Fields: []string{"Amount__c", "Id", "Label__c", "Text__c"}, SecurityMode: "USER_MODE",
		Where: &soql.Condition{Field: "Id", Op: "IN"}, Order: []soql.OrderSpec{{Field: "Label__c", Nulls: "FIRST"}},
	}
	object, denied, _ := machine.userModeInaccessibleDeniedFields(query, "")
	if object != "Ledger__c" || !userModeProjectionContains(denied, "Amount__c") || userModeProjectionContains(denied, "Label__c") || userModeProjectionContains(denied, "Text__c") {
		t.Fatalf("permission-derived projected access details: object=%s fields=%v", object, denied)
	}
	query.Fields[3] = "Day__c"
	if _, denied, _ := machine.userModeInaccessibleDeniedFields(query, ""); !userModeProjectionContains(denied, "Amount__c") || userModeProjectionContains(denied, "Day__c") {
		t.Fatalf("mixed date/numeric projection details: %v", denied)
	}
	machine.currentMethod.APIVersion = "59.0"
	if err := machine.userModeInaccessibleDenial(query, ""); err != nil {
		t.Fatal("older flat-query API boundary expanded")
	}
	machine.currentMethod.APIVersion = "63.0"
	// A readable projection remains on the ordinary query path.
	permissions := org.Objects["FieldPermissions"]
	record := permissions.Records["0FP000000000701"]
	record.Fields["PermissionsRead"] = storage.BooleanValue(true)
	permissions.Records["0FP000000000701"] = record
	org.Objects["FieldPermissions"] = permissions
	if err := machine.userModeInaccessibleDenial(query, ""); err != nil {
		t.Fatalf("readable projection was denied: %v", err)
	}
	// The new admission does not expand the old two/three-field mixed shape,
	// permission scopes, relationship projections, or non-USER_MODE queries.
	query.Fields = []string{"Id", "Label__c", "Text__c"}
	if machine.userModeInaccessibleProjectionQuery(query) {
		t.Fatal("old field-count boundary changed")
	}
	query.Fields = []string{"Amount__c", "Id", "Label__c", "Parent.Name"}
	if machine.userModeInaccessibleProjectionQuery(query) {
		t.Fatal("relationship projection admitted")
	}
	query.Fields = []string{"Amount__c", "Id", "Label__c", "Text__c"}
	if err := machine.userModeInaccessibleDenial(query, "scoped"); err != nil {
		t.Fatal("uncaptured permission scope admitted")
	}
	query.SecurityMode = "SYSTEM_MODE"
	if err := machine.userModeInaccessibleDenial(query, ""); err != nil {
		t.Fatal("SYSTEM_MODE details admitted")
	}
}

func userModeProjectionContains(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}
