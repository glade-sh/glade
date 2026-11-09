package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows (internal/apextest/testdata/conformance/permission_scope_mute, API 62.0 and 67.0 identical): a scoped permission
// set adds to the running user's grants; it does not replace them. A muting
// permission set removes a grant for its own group only.

func permissionAugmentOrg(t *testing.T) (storage.OrgState, Value) {
	t.Helper()
	org := testDataOrg()
	org.Features = []string{"ApexUserModeWithPermset"}
	storage.EnsureDeterministicPlatformData(&org)
	for _, name := range []string{"PermissionSetAssignment", "PermissionSetGroupComponent", "MutingPermissionSet"} {
		storage.EnsureStandardObject(&org, name)
	}
	account := org.Objects["Account"]
	account.Definition.Fields["ShippingStreet"] = storage.Field{APIName: "ShippingStreet", Type: storage.FieldString}
	account.Records["001000000000901AAA"] = storage.Record{ID: "001000000000901AAA", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}
	org.Objects["Account"] = account
	user := Object("User")
	user.Fields["Id"] = String("005000000000998")
	user.Fields["profileId"] = String("00e000000000002")
	return org, user
}

func grantAccountRows(org *storage.OrgState, parent string, edit bool) {
	org.Objects["ObjectPermissions"].Records[storage.ID("110"+parent)] = storage.Record{
		ID: storage.ID("110" + parent), Object: "ObjectPermissions",
		Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(parent)), "SObjectType": storage.StringValue("Account"),
			"PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(edit),
		},
	}
	org.Objects["FieldPermissions"].Records[storage.ID("0FP"+parent)] = storage.Record{
		ID: storage.ID("0FP" + parent), Object: "FieldPermissions",
		Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(parent)), "SObjectType": storage.StringValue("Account"),
			"Field":           storage.StringValue("Account.ShippingStreet"),
			"PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(edit),
		},
	}
}

func assignSet(org *storage.OrgState, id, set string) {
	org.Objects["PermissionSetAssignment"].Records[storage.ID(id)] = storage.Record{
		ID: storage.ID(id), Object: "PermissionSetAssignment",
		Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000998"), "PermissionSetId": storage.IDValue(storage.ID(set))},
	}
}

func runScopedUpdate(t *testing.T, org storage.OrgState, user Value, scopeID string) error {
	t.Helper()
	program, err := CompileAnonymous(`
Database.update(new Account(Id = '001000000000901AAA', ShippingStreet = 'Baker'), AccessLevel.USER_MODE.withPermissionSetId('` + scopeID + `'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = user
	_, err = machine.Execute(program)
	return err
}

func TestScopedPermissionSetAugmentsRunningUserGrants(t *testing.T) {
	const (
		userSet   = "0PS000000000A01"
		otherSet  = "0PS000000000A02"
		scopedSet = "0PS000000000A03"
	)
	for _, tc := range []struct {
		name    string
		userHas bool
		scope   string
		denied  bool
	}{
		{"user grant, scoped set lacks the rows", true, otherSet, false},
		{"scoped-only grant", false, scopedSet, false},
		{"user and scoped both grant", true, scopedSet, false},
		{"neither grants", false, otherSet, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org, user := permissionAugmentOrg(t)
			grantAccountRows(&org, userSet, true)
			grantAccountRows(&org, scopedSet, true)
			org.Objects["ObjectPermissions"].Records["110"+otherSet] = storage.Record{
				ID: storage.ID("110" + otherSet), Object: "ObjectPermissions",
				Fields: map[string]storage.Value{"ParentId": storage.IDValue(otherSet), "SObjectType": storage.StringValue("Contact"), "PermissionsRead": storage.BooleanValue(true)},
			}
			if tc.userHas {
				assignSet(&org, "0Pa000000000A01", userSet)
			}
			err := runScopedUpdate(t, org, user, tc.scope)
			if tc.denied {
				if err == nil || !strings.Contains(err.Error(), "denied") {
					t.Fatalf("err = %v, want access denial", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("scoped user-mode update: %v", err)
			}
		})
	}
}

func TestScopedPermissionSetSplitsObjectAndFieldGrants(t *testing.T) {
	// Native: the user holds object access; only the scoped set holds the
	// field row. The union lets the field through.
	const (
		userSet   = "0PS000000000B01"
		scopedSet = "0PS000000000B03"
	)
	org, user := permissionAugmentOrg(t)
	org.Objects["ObjectPermissions"].Records["110"+userSet] = storage.Record{
		ID: storage.ID("110" + userSet), Object: "ObjectPermissions",
		Fields: map[string]storage.Value{"ParentId": storage.IDValue(userSet), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)},
	}
	assignSet(&org, "0Pa000000000B01", userSet)
	grantAccountRows(&org, scopedSet, true)
	if err := runScopedUpdate(t, org, user, scopedSet); err != nil {
		t.Fatalf("user object access plus scoped field access: %v", err)
	}
	if err := runScopedUpdate(t, org, user, "0PS000000000B09"); err == nil || !strings.Contains(err.Error(), "ShippingStreet") {
		t.Fatalf("err = %v, want field denial without the scoped field row", err)
	}
}

func TestPermissionSetGroupMutingAffectsOnlyItsGroup(t *testing.T) {
	const (
		grantSet = "0PS000000000C01"
		muteSet  = "0PS000000000C02"
		group    = "0PG000000000C01"
	)
	setup := func(direct bool) *VM {
		org, user := permissionAugmentOrg(t)
		grantAccountRows(&org, grantSet, true)
		// The muting set removes the field; the object rows stay.
		org.Objects["MutingPermissionSet"].Records[muteSet] = storage.Record{ID: muteSet, Object: "MutingPermissionSet", Fields: map[string]storage.Value{}}
		org.Objects["FieldPermissions"].Records["0FP"+muteSet] = storage.Record{
			ID: storage.ID("0FP" + muteSet), Object: "FieldPermissions",
			Fields: map[string]storage.Value{
				"ParentId": storage.IDValue(muteSet), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue("Account.ShippingStreet"),
				"PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true),
			},
		}
		for i, set := range []string{grantSet, muteSet} {
			id := storage.ID("0Ps00000000C0" + string(rune('1'+i)))
			org.Objects["PermissionSetGroupComponent"].Records[id] = storage.Record{ID: id, Object: "PermissionSetGroupComponent",
				Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(storage.ID(set))}}
		}
		org.Objects["PermissionSetAssignment"].Records["0Pa000000000C01"] = storage.Record{ID: "0Pa000000000C01", Object: "PermissionSetAssignment",
			Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000998"), "PermissionSetGroupId": storage.IDValue(group)}}
		if direct {
			assignSet(&org, "0Pa000000000C02", grantSet)
		}
		machine := New(nil)
		machine.SetOrg(&org)
		machine.executionUser = user
		return machine
	}
	// Native (API 62 and 67): the group's grant on the muted field is gone,
	// the object grant remains.
	muted := setup(false)
	if !muted.currentUserObjectPermission("Account", "isCreateable") && !muted.currentUserObjectPermission("Account", "isAccessible") {
		t.Fatal("object access must remain under a field-only mute")
	}
	for _, method := range []string{"isAccessible", "isUpdateable", "isCreateable"} {
		if muted.currentUserFieldPermission("Account", "ShippingStreet", method) {
			t.Fatalf("muted group still grants ShippingStreet %s", method)
		}
	}
	// An independent direct assignment of the same granting set stays effective.
	direct := setup(true)
	for _, method := range []string{"isAccessible", "isUpdateable"} {
		if !direct.currentUserFieldPermission("Account", "ShippingStreet", method) {
			t.Fatalf("direct grant must survive the group's mute for %s", method)
		}
	}
}

// Native (API 62 and 67): the administrator keeps its own object and field
// access under a scoped set that holds no rows for them.
func TestScopedPermissionSetKeepsAdministratorAccess(t *testing.T) {
	org, _ := permissionAugmentOrg(t)
	machine := New(nil)
	machine.SetOrg(&org)
	const emptyScope = "0PS000000000D01"
	for _, object := range []string{"Account", "Contact"} {
		if !machine.currentUserObjectPermissionWithScope(object, "isAccessible", emptyScope) {
			t.Fatalf("scoped set removed administrator access to %s", object)
		}
	}
	if !machine.currentUserFieldPermissionWithScope("Account", "Name", "isAccessible", emptyScope) {
		t.Fatal("scoped set removed administrator access to Account.Name")
	}
}
