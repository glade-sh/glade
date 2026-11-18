package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The captured standard-object custom-field DML rows use the test runner.
// An anonymous execution retains its prior SecurityException classification.
func TestStandardCustomFieldAnonymousPreservesDenial(t *testing.T) {
	org, user := permissionAugmentOrg(t)
	account := org.Objects["Account"]
	account.Definition.Fields["Hidden__c"] = storage.Field{APIName: "Hidden__c", Type: storage.FieldString}
	org.Objects["Account"] = account
	org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{ID: "110000000000998", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000998"), "SObjectType": storage.StringValue("Account"), "PermissionsCreate": storage.BooleanValue(true)}}
	assignSet(&org, "0Pa000000000998", "0PS000000000998")
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = user
	record := Object("Account")
	record.Fields["Hidden__c"] = String("value")
	_, err := machine.userModeDMLRecordAccess("insert", record, "isCreateable", "isCreateable", "")
	if err == nil || !strings.Contains(err.Error(), "Access to field 'Account.Hidden__c' denied") {
		t.Fatalf("anonymous denial = %v", err)
	}
}

// A test runner with a Standard User but no runAs is outside the captured route.
func TestStandardCustomFieldStandardUserWithoutRunAsPreservesDenial(t *testing.T) {
	org, user := permissionAugmentOrg(t)
	profile := org.Objects["Profile"]
	row := profile.Records["00e000000000002"]
	row.Fields["Name"] = storage.StringValue("Standard User")
	profile.Records[row.ID] = row
	org.Objects["Profile"] = profile
	account := org.Objects["Account"]
	account.Definition.Fields["Hidden__c"] = storage.Field{APIName: "Hidden__c", Type: storage.FieldString, DisplayType: "STRING", Length: 255}
	org.Objects["Account"] = account
	org.Objects["ObjectPermissions"].Records["110000000000998"] = storage.Record{ID: "110000000000998", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000998"), "SObjectType": storage.StringValue("Account"), "PermissionsCreate": storage.BooleanValue(true)}}
	assignSet(&org, "0Pa000000000998", "0PS000000000998")
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = user
	machine.currentMethod.APIVersion = "67.0"
	record := Object("Account")
	record.Fields["Hidden__c"] = String("value")
	_, err := machine.userModeDMLRecordAccess("insert", record, "isCreateable", "isCreateable", "")
	if err == nil || !strings.Contains(err.Error(), "Access to field 'Account.Hidden__c' denied") {
		t.Fatalf("uncaptured test-runner denial = %v", err)
	}
}
