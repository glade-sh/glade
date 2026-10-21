package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned user_field_permission_fallback captures Standard User missing-grant
// update denial. Keep explicit object/field grants and denials independent;
// qualified names remain the existing local namespace-resolution control.
func TestCustomUpdatePermissionPreservesMissingMetadataDefault(t *testing.T) {
	for _, objectName := range []string{"TriadParcel__c", "pkg__TriadParcel__c"} {
		t.Run(objectName, func(t *testing.T) {
			org := testDataOrg()
			org.Objects[objectName] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: objectName, Fields: map[string]storage.Field{
					"ParcelKey__c": {APIName: "ParcelKey__c", Type: storage.FieldString},
				},
			}}
			machine := New(nil)
			machine.SetOrg(&org)
			profile := Object("Profile")
			profile.Fields["Name"] = String("Standard User")
			user := Object("User")
			user.Fields["Id"] = String("005000000000099")
			user.Fields["ProfileId"] = String("00e000000000099")
			user.Fields["Profile"] = profile
			machine.executionUser = user
			if machine.currentUserObjectPermission(objectName, "isUpdateable") ||
				machine.currentUserFieldPermission(objectName, "ParcelKey__c", "isUpdateable") {
				t.Fatal("missing grants allowed an update")
			}
			// Explicit object denies and field denies keep their precedence.
			objects := machine.Org.Objects["ObjectPermissions"]
			objects.Records = map[storage.ID]storage.Record{"110000000000099": {
				ID: "110000000000099", Object: "ObjectPermissions", Fields: map[string]storage.Value{
					"ParentId": storage.IDValue("00e000000000099"), "SObjectType": storage.StringValue(objectName),
					"PermissionsEdit": storage.BooleanValue(false),
				},
			}}
			machine.Org.Objects["ObjectPermissions"] = objects
			if machine.currentUserObjectPermission(objectName, "isUpdateable") {
				t.Fatal("explicit object denial was ignored")
			}
			record := objects.Records["110000000000099"]
			record.Fields["PermissionsEdit"] = storage.BooleanValue(true)
			objects.Records[record.ID] = record
			machine.Org.Objects["ObjectPermissions"] = objects
			fields := machine.Org.Objects["FieldPermissions"]
			fields.Records = map[storage.ID]storage.Record{"111000000000099": {
				ID: "111000000000099", Object: "FieldPermissions", Fields: map[string]storage.Value{
					"ParentId": storage.IDValue("00e000000000099"), "SObjectType": storage.StringValue(objectName),
					"Field": storage.StringValue(objectName + ".ParcelKey__c"), "PermissionsEdit": storage.BooleanValue(false),
				},
			}}
			machine.Org.Objects["FieldPermissions"] = fields
			if !machine.currentUserObjectPermission(objectName, "isUpdateable") ||
				machine.currentUserFieldPermission(objectName, "ParcelKey__c", "isUpdateable") {
				t.Fatal("explicit field denial lost its independent precedence")
			}
			field := fields.Records["111000000000099"]
			field.Fields["PermissionsEdit"] = storage.BooleanValue(true)
			fields.Records[field.ID] = field
			if !machine.currentUserFieldPermission(objectName, "ParcelKey__c", "isUpdateable") {
				t.Fatal("explicit field grant was denied")
			}
		})
	}
}

func TestCustomSettingManagementGrantUsesActorAndCanonicalID(t *testing.T) {
	org := testDataOrg()
	setID := "0PS000000000099"
	sets := org.Objects["PermissionSet"]
	sets.Records = map[storage.ID]storage.Record{storage.ID(setID): {
		ID: storage.ID(setID), Object: "PermissionSet", Fields: map[string]storage.Value{
			"PermissionsCustomizeApplication": storage.BooleanValue(true),
		},
	}}
	org.Objects["PermissionSet"] = sets
	assignments := org.Objects["PermissionSetAssignment"]
	assignment := storage.Record{ID: "0Pa000000000099", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{
		"AssigneeId": storage.IDValue("005000000000098"), "PermissionSetId": storage.IDValue(storage.ID(apexIDTo18(setID))),
	}}
	assignments.Records = map[storage.ID]storage.Record{assignment.ID: assignment}
	org.Objects["PermissionSetAssignment"] = assignments
	machine := New(nil)
	machine.SetOrg(&org)
	user := Object("User")
	user.Fields["Id"] = String("005000000000099")
	if machine.customSettingManagementPermission(user, "") {
		t.Fatal("another actor's grant leaked")
	}
	assignment.Fields["AssigneeId"] = storage.IDValue("005000000000099")
	assignments.Records[assignment.ID] = assignment
	machine.Org.Objects["PermissionSetAssignment"] = assignments
	if !machine.customSettingManagementPermission(user, "") {
		t.Fatal("assigned grant with equivalent 18-character Id was missed")
	}
	delete(assignments.Records, assignment.ID)
	if !machine.customSettingManagementPermission(user, apexIDTo18(setID)) {
		t.Fatal("profile-owned grant with equivalent Id was missed")
	}
}
