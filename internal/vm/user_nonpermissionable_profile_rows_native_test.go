package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// invocable_record_followups/org.tsv:10 and :19 (userFields, API 62.0 and
// 67.0): under a custom profile that lists User field permissions, the
// non-permissionable User.ProfileId and User.UserRoleId describe as not
// accessible while Name, Email, Phone, Alias, IsActive, LastName and
// ManagerId stay readable. The non-permissionable rule must not outrank the
// profile's field-permission listing for the object.
func TestUserNonPermissionableFieldsUnderProfileFieldRowsNative(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureDeterministicPlatformData(&org)
	const profileID storage.ID = "00e000000000990"
	const userID storage.ID = "005000000000990"
	const setID storage.ID = "0PS000000000990"
	profiles := org.Objects["Profile"]
	profiles.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic Reader"), "UserType": storage.StringValue("Standard"), "UserLicenseId": storage.IDValue("100000000000001")}}
	org.Objects["Profile"] = profiles
	sets := org.Objects["PermissionSet"]
	sets.Records[setID] = storage.Record{ID: setID, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("X" + string(profileID)), "Label": storage.StringValue(string(profileID)), "Type": storage.StringValue("Profile"), "IsCustom": storage.BooleanValue(true), "IsOwnedByProfile": storage.BooleanValue(true), "ProfileId": storage.IDValue(profileID)}}
	org.Objects["PermissionSet"] = sets
	user := org.Objects["User"]
	user.Definition.Fields["SynthFlag__c"] = storage.Field{APIName: "SynthFlag__c", Type: storage.FieldBoolean, Permissionable: storage.BoolFlag(true)}
	user.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "ProfileId": storage.IDValue(profileID)}}
	org.Objects["User"] = user
	objects := org.Objects["ObjectPermissions"]
	objects.Records["110000000000990"] = storage.Record{ID: "110000000000990", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(setID), "SobjectType": storage.StringValue("User"), "PermissionsRead": storage.BooleanValue(true)}}
	org.Objects["ObjectPermissions"] = objects
	fields := org.Objects["FieldPermissions"]
	fields.Records["0FP000000000990"] = storage.Record{ID: "0FP000000000990", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(setID), "SobjectType": storage.StringValue("User"), "Field": storage.StringValue("User.SynthFlag__c"), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(false)}}
	org.Objects["FieldPermissions"] = fields
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org.APIVersion = api
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.testContext.CurrentUser = vmValueFromRecord(user.Records[userID])
			machine.testContext.RunAsDepth = 1
			machine.currentMethod.APIVersion = api
			want := map[string]bool{"Name": true, "Email": true, "Phone": true, "Alias": true, "IsActive": true, "LastName": true, "ManagerId": true, "UserRoleId": false, "ProfileId": false, "SynthFlag__c": true}
			for name, expected := range want {
				if got := machine.currentUserFieldPermission("User", name, "isAccessible"); got != expected {
					t.Errorf("User.%s isAccessible=%t, native row says %t", name, got, expected)
				}
			}
		})
	}
}

// standard_profile_object_crud/org.tsv:3 and :5 (isTest, API 62.0 and 67.0):
// the Standard User, whose profile-owned set lists every permissionable
// Account and Contact field, still describes the non-permissionable
// Account.Name as accessible, createable and updateable (TTT), and
// native_formula_permission_regex/org.tsv:8 (67.0 isTest:rowOnly) inserts a
// Contact with LastName as that user. The profile's field listing must not
// hide fields that have no field permission.
func TestStandardUserNonPermissionableFieldsWithSeededProfileRowsNative(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureDeterministicPlatformData(&org)
	const userID storage.ID = "005000000000991"
	user := org.Objects["User"]
	user.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic Standard"), "ProfileId": storage.IDValue("00e000000000005")}}
	org.Objects["User"] = user
	if !New(nil).withOrg(&org).parentHasFieldPermissionsForObject(New(nil).withOrg(&org).profileOwnedPermissionSetID("00e000000000005"), "Account") {
		t.Fatal("seed did not give the Standard User profile set Account field permissions")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org.APIVersion = api
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.testContext.CurrentUser = vmValueFromRecord(user.Records[userID])
			machine.testContext.RunAsDepth = 1
			machine.currentMethod.APIVersion = api
			for _, field := range []string{"Account.Name", "Contact.LastName"} {
				object, name, _ := strings.Cut(field, ".")
				for _, method := range []string{"isAccessible", "isCreateable", "isUpdateable"} {
					if !machine.currentUserFieldPermission(object, name, method) {
						t.Errorf("%s %s=false, native row says true", field, method)
					}
				}
			}
			if !machine.currentUserFieldPermission("Account", "Industry", "isAccessible") {
				t.Error("Account.Industry (listed, read true) is not accessible")
			}
		})
	}
}

func (vm *VM) withOrg(org *storage.OrgState) *VM {
	vm.SetOrg(org)
	return vm
}
