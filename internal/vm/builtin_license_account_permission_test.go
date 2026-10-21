package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func builtinLicenseAccountActor(t *testing.T) *VM {
	t.Helper()
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	const profileID storage.ID = "00e000000000091"
	const licenseID storage.ID = "100000000000091"
	profiles := org.Objects["Profile"]
	profiles.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{
		"Name": storage.StringValue("Chatter Free User"), "UserType": storage.StringValue("CsnOnly"), "UserLicenseId": storage.IDValue(licenseID),
	}}
	org.Objects["Profile"] = profiles
	licenses := org.Objects["UserLicense"]
	licenses.Records[licenseID] = storage.Record{ID: licenseID, Object: "UserLicense", Fields: map[string]storage.Value{
		"Name": storage.StringValue("Chatter Free"), "LicenseDefinitionKey": storage.StringValue("CSN_User"),
	}}
	org.Objects["UserLicense"] = licenses
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "64.0"}
	actor := Object("User")
	actor.Fields["Id"] = String("005000000000091")
	actor.Fields["ProfileId"] = String(string(profileID))
	machine.executionUser = actor
	return machine
}

// The native API64 actor completes runAs but has no Account CRUD or read/write
// access to these fields, including the ordinary baseline system fields.
func TestBuiltinLicenseAccountDescribeDenial(t *testing.T) {
	machine := builtinLicenseAccountActor(t)
	for _, method := range []string{"isAccessible", "isCreateable", "isUpdateable", "isDeletable"} {
		t.Run(method, func(t *testing.T) {
			if machine.currentUserObjectPermission("Account", method) {
				t.Errorf("native actor denies Account %s", method)
			}
		})
	}
	for _, field := range []string{"Id", "Name", "AccountNumber", "BillingCity", "CreatedById", "Description", "Industry", "OwnerId", "Phone"} {
		for _, method := range []string{"isAccessible", "isCreateable", "isUpdateable"} {
			t.Run(field+"/"+method, func(t *testing.T) {
				if machine.currentUserFieldPermission("Account", field, method) {
					t.Errorf("native actor denies Account.%s %s", field, method)
				}
			})
		}
	}
}

// These are implementation-scope controls, not native permission grant claims.
func TestBuiltinLicenseAccountPermissionFallbackControls(t *testing.T) {
	machine := builtinLicenseAccountActor(t)
	if !machine.currentUserObjectPermission("Contact", "isAccessible") {
		t.Fatal("uncaptured object fallback changed")
	}
	licenses := machine.Org.Objects["UserLicense"]
	license := licenses.Records[storage.ID("100000000000091")]
	license.Fields["Name"] = storage.StringValue("Unobserved License")
	licenses.Records[license.ID] = license
	machine.Org.Objects["UserLicense"] = licenses
	if !machine.currentUserObjectPermission("Account", "isAccessible") || !machine.currentUserFieldPermission("Account", "Id", "isAccessible") {
		t.Fatal("unobserved license fallback changed")
	}
}

func TestBuiltinLicenseAccountExplicitGrantControl(t *testing.T) {
	machine := builtinLicenseAccountActor(t)
	const permissionSetID storage.ID = "0PS000000000091"
	assignments := machine.Org.Objects["PermissionSetAssignment"]
	assignmentID := storage.ID("0Pa000000000091")
	assignments.Records[assignmentID] = storage.Record{ID: assignmentID, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{
		"AssigneeId": storage.IDValue("005000000000091"), "PermissionSetId": storage.IDValue(permissionSetID),
	}}
	machine.Org.Objects["PermissionSetAssignment"] = assignments
	permissions := machine.Org.Objects["ObjectPermissions"]
	permissionID := storage.ID("110000000000091")
	permissions.Records[permissionID] = storage.Record{ID: permissionID, Object: "ObjectPermissions", Fields: map[string]storage.Value{
		"ParentId": storage.IDValue(permissionSetID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true),
	}}
	machine.Org.Objects["ObjectPermissions"] = permissions
	if !machine.currentUserObjectPermission("Account", "isAccessible") || !machine.currentUserFieldPermission("Account", "Id", "isAccessible") {
		t.Fatal("existing explicit grant lookup was bypassed")
	}
	// Preserve the existing baseline-system-field shortcut even when an
	// explicit field denial is present. Grant/denial interactions were not
	// observed in the native license packet and are not changed by this fix.
	for _, parentID := range []storage.ID{permissionSetID, "00e000000000091"} {
		fields := machine.Org.Objects["FieldPermissions"]
		fieldID := storage.ID("0FP000000000091")
		fields.Records[fieldID] = storage.Record{ID: fieldID, Object: "FieldPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(parentID), "SObjectType": storage.StringValue("Account"),
			"Field": storage.StringValue("Account.Id"), "PermissionsRead": storage.BooleanValue(false),
		}}
		machine.Org.Objects["FieldPermissions"] = fields
		if !machine.currentUserFieldPermission("Account", "Id", "isAccessible") {
			t.Fatalf("existing Id baseline shortcut changed with explicit field denial under %s", parentID)
		}
	}
}
