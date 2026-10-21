package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native query parity is asserted by the 28-cell apextest family. These scope
// controls preserve the existing behavior outside its explicit configuration;
// they do not claim Salesforce answers for the carried configuration twins.
func TestConfiguredUserReadPermissionScope(t *testing.T) {
	for _, model := range []string{"Read", "", "Private", "ReadWrite", "PublicReadOnly"} {
		t.Run("model/"+model, func(t *testing.T) {
			machine := configuredUserReadMachine(t)
			users := machine.Org.Objects["User"]
			users.Definition.SharingModel = model
			machine.Org.Objects["User"] = users
			if got := machine.currentUserObjectPermission("User", "isAccessible"); got != (model == "Read") {
				t.Fatalf("configured %q read=%v", model, got)
			}
		})
	}
	for _, facet := range []string{"isCreateable", "isUpdateable", "isDeletable"} {
		t.Run(facet, func(t *testing.T) {
			if configuredUserReadMachine(t).currentUserObjectPermission("User", facet) {
				t.Fatalf("User read configuration granted %s", facet)
			}
		})
	}
	t.Run("other-object", func(t *testing.T) {
		machine := configuredUserReadMachine(t)
		if machine.currentUserObjectPermission("Case", "isAccessible") {
			t.Fatal("User read configuration granted Case access")
		}
	})
	for _, shape := range []string{"guest", "external", "missing-user-type", "other-license", "missing-license"} {
		t.Run(shape, func(t *testing.T) {
			machine := configuredUserReadMachine(t)
			profiles := machine.Org.Objects["Profile"]
			profile := profiles.Records["00e000000000002"]
			switch shape {
			case "guest":
				profile.Fields["UserType"] = storage.StringValue("Guest")
			case "external":
				profile.Fields["UserType"] = storage.StringValue("CspLitePortal")
			case "missing-user-type":
				delete(profile.Fields, "UserType")
			case "missing-license":
				delete(profile.Fields, "UserLicenseId")
			case "other-license":
				licenses := machine.Org.Objects["UserLicense"]
				id := profile.Fields["UserLicenseId"].ID
				license := licenses.Records[id]
				license.Fields["LicenseDefinitionKey"] = storage.StringValue("Other")
				licenses.Records[id] = license
				machine.Org.Objects["UserLicense"] = licenses
			}
			profiles.Records[profile.ID] = profile
			machine.Org.Objects["Profile"] = profiles
			if machine.currentUserObjectPermission("User", "isAccessible") {
				t.Fatal("unobserved actor configuration gained User access")
			}
		})
	}
}

func TestConfiguredUserReadHonorsExplicitPermissions(t *testing.T) {
	for _, parent := range []string{"00e000000000002", "0PS000000000004"} {
		t.Run(parent, func(t *testing.T) {
			machine := configuredUserReadMachine(t)
			if !machine.currentUserObjectPermission("User", "isAccessible") || !machine.currentUserFieldPermission("User", "LastName", "isAccessible") {
				t.Fatal("missing observed User/LastName read")
			}
			seed := storage.NewFixture()
			seed.Objects = []storage.FixtureObject{{Name: "FieldPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{
				"ParentId": storage.IDValue(storage.ID(parent)), "SObjectType": storage.StringValue("User"),
				"Field": storage.StringValue("User.LastName"), "PermissionsRead": storage.BooleanValue(false),
			}}}}}
			if err := storage.ApplyFixture(machine.Org, seed); err != nil {
				t.Fatal(err)
			}
			if machine.currentUserFieldPermission("User", "LastName", "isAccessible") {
				t.Fatal("explicit FLS denial was overridden")
			}
			seed.Objects = []storage.FixtureObject{{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{
				"ParentId": storage.IDValue(storage.ID(parent)), "SObjectType": storage.StringValue("User"), "PermissionsRead": storage.BooleanValue(false),
			}}}}}
			if err := storage.ApplyFixture(machine.Org, seed); err != nil {
				t.Fatal(err)
			}
			if machine.currentUserObjectPermission("User", "isAccessible") {
				t.Fatal("explicit CRUD denial was overridden")
			}
		})
	}
	t.Run("permission-set-scope", func(t *testing.T) {
		machine := configuredUserReadMachine(t)
		if machine.currentUserObjectPermissionWithScope("User", "isAccessible", "0PS000000000001") {
			t.Fatal("User read configuration escaped explicit permission-set scope")
		}
	})
}

func configuredUserReadMachine(t *testing.T) *VM {
	t.Helper()
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureDeterministicPlatformData(&org)
	users := org.Objects["User"]
	users.Definition.SharingModel = "Read"
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String("005000000000099")
	machine.executionUser.Fields["ProfileId"] = String("00e000000000002")
	return machine
}
