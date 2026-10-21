package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Reuse the owned sharing fixture's explicit CRUD/FLS grants without any
// ViewAll/ModifyAll grant, and give each actor one private record.
func recordSharingControlMachine(t *testing.T, model string) *VM {
	t.Helper()
	org := stripInaccessibleTestOrg()
	org.APIVersion = "68.0"
	profile := org.Objects["Profile"]
	actor := profile.Records["00e000000000002"]
	actor.Fields["Name"] = storage.StringValue("Standard User")
	profile.Records[actor.ID] = actor
	org.Objects["Profile"] = profile
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.RunAsDepth = 1
	state := privateExecutionPolicyOrg().Objects["Widget__c"]
	state.Definition.SharingModel = model
	state.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	for id, record := range state.Records {
		record.Fields["Amount__c"] = storage.DecimalValue("10")
		state.Records[id] = record
	}
	machine.Org.Objects["Widget__c"] = state
	permissions := machine.Org.Objects["ObjectPermissions"]
	for id, row := range permissions.Records {
		row.Fields["SObjectType"] = storage.StringValue("Widget__c")
		permissions.Records[id] = row
	}
	machine.Org.Objects["ObjectPermissions"] = permissions
	fields := machine.Org.Objects["FieldPermissions"]
	fields.Records = map[storage.ID]storage.Record{}
	for i, name := range []string{"Name", "Amount__c", "OwnerId"} {
		id := storage.ID(fmt.Sprintf("0FP00000000000%d", i+1))
		fields.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Widget__c"), "Field": storage.StringValue("Widget__c." + name), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	machine.Org.Objects["FieldPermissions"] = fields
	machine.SetOrg(machine.Org)
	if machine.currentUserID() != "005000000000002" || machine.currentUserBypassesRecordSharing() || machine.currentUserObjectRecordGrant("Widget__c", "PermissionsViewAllRecords", "PermissionsModifyAllRecords") {
		t.Fatal("actor identity or bypass grant drift")
	}
	users := machine.Org.Objects["User"]
	if users.Definition.APIName == "" {
		users.Definition.APIName = "User"
	}
	if users.Definition.KeyPrefix == "" {
		users.Definition.KeyPrefix = "005"
	}
	if users.Records == nil {
		users.Records = map[storage.ID]storage.Record{}
	}
	// Keep each stored owner reference valid while exercising share access.
	for _, id := range []storage.ID{"005000000000001", "005000000000002"} {
		users.Records[id] = storage.Record{
			ID: id, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(id)},
		}
	}
	machine.Org.Objects["User"] = users
	machine.SetOrg(machine.Org)
	if !machine.currentUserObjectPermission("Widget__c", "isAccessible") || !machine.currentUserObjectPermission("Widget__c", "isUpdateable") || !machine.currentUserFieldPermission("Widget__c", "Amount__c", "isAccessible") {
		t.Fatal("explicit object/field grants missing")
	}
	return machine
}
