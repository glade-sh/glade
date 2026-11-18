package storage

import (
	"reflect"
	"testing"
)

// A runtime clone shares the rows of metadata and setup objects with its
// template and siblings. Every seed or migration write to one of those objects
// must copy the rows first (EnsureMutableObjectRecords).

func sharedRowTemplate(objects map[string][]Record) OrgState {
	org := NewOrgState()
	for name, records := range objects {
		state := ObjectState{Definition: ObjectDefinition{APIName: name}}
		for _, record := range records {
			state.PutRecord(record.ID, record)
		}
		org.Objects[name] = state
	}
	return org
}

func requireRowsShared(t *testing.T, clone OrgState, names ...string) {
	t.Helper()
	for _, name := range names {
		if !clone.Objects[name].RecordsShared {
			t.Fatalf("%s rows are not shared with the clone; the test no longer exercises the guard", name)
		}
	}
}

func requireTemplateUntouched(t *testing.T, before map[string]string, orgs map[string]OrgState) {
	t.Helper()
	for name, org := range orgs {
		if after := rowsDigest(org); !reflect.DeepEqual(before, after) {
			t.Fatalf("clone write reached %s rows", name)
		}
	}
}

func TestCloneRuntimeFrozenSharedLegacyPermissionInventoryMigrationIsolation(t *testing.T) {
	const marketingProfile ID = "00e000000000006"
	const adminSet, marketingSet ID = "0PS000000000090", "0PS000000000091"
	template := sharedRowTemplate(map[string][]Record{
		"Profile": {
			{ID: ID(DefaultProfileID), Object: "Profile", Fields: map[string]Value{"Name": StringValue("System Administrator")}},
			{ID: marketingProfile, Object: "Profile", Fields: map[string]Value{"Name": StringValue("Marketing User")}},
		},
		"PermissionSet": {
			{ID: "0PS000000000002", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("Proving_With_User_Mode_Works"), "IsOwnedByProfile": BooleanValue(false)}},
			{ID: "0PS000000000003", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("X" + DefaultProfileID), "ProfileId": IDValue(DefaultProfileID), "IsOwnedByProfile": BooleanValue(true)}},
			{ID: "0PS000000000008", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("X" + string(marketingProfile)), "ProfileId": IDValue(marketingProfile), "IsOwnedByProfile": BooleanValue(true)}},
		},
		"ObjectPermissions": {
			{ID: "110000000000003", Object: "ObjectPermissions", Fields: map[string]Value{"ParentId": IDValue("0PS000000000003"), "SobjectType": StringValue("Account"), "PermissionsRead": BooleanValue(true)}},
			{ID: "110000000000006", Object: "ObjectPermissions", Fields: map[string]Value{"ParentId": IDValue(marketingSet), "SobjectType": StringValue("Account"), "PermissionsDelete": BooleanValue(false)}},
		},
	})
	EnsureStandardObject(&template, "ObjectPermissions")
	for _, name := range []string{"ObjectPermissions", "PermissionSet", "Profile"} {
		state := template.Objects[name]
		state.Definition.APIName = name
		template.Objects[name] = state
	}
	before := rowsDigest(template)
	clone := template.CloneRuntimeFrozenShared()
	sibling := template.CloneRuntimeFrozenShared()
	requireRowsShared(t, clone, "Profile", "PermissionSet", "ObjectPermissions")

	migrateLegacyPermissionInventory(&clone, map[string]ID{"System Administrator": adminSet, "Marketing User": marketingSet})

	sets := clone.Objects["PermissionSet"].Records
	if _, ok := sets["0PS000000000002"]; ok {
		t.Fatal("clone kept the unreferenced synthetic set")
	}
	if _, ok := sets["0PS000000000003"]; ok {
		t.Fatal("clone kept the legacy administrator set")
	}
	permissions := clone.Objects["ObjectPermissions"].Records
	if got := permissions["110000000000003"].Fields["ParentId"].ID; got != adminSet {
		t.Fatalf("clone ObjectPermissions parent = %s, want %s", got, adminSet)
	}
	if !permissions["110000000000006"].Fields["PermissionsDelete"].Boolean {
		t.Fatal("clone did not apply the captured Marketing User delete flag")
	}
	requireTemplateUntouched(t, before, map[string]OrgState{"template": template, "sibling": sibling})
}

func TestCloneRuntimeFrozenSharedLegacyLicenseProfileMigrationIsolation(t *testing.T) {
	const community, chatter ID = "100000000000002AAA", "100000000000003AAA"
	const guest ID = "00e000000000007"
	template := sharedRowTemplate(map[string][]Record{
		"Profile": {
			{ID: "00e000000000003", Object: "Profile", Fields: map[string]Value{"Name": StringValue("Chatter External User"), "UserLicenseId": IDValue(community)}},
			{ID: guest, Object: "Profile", Fields: map[string]Value{"Name": StringValue("Customer Community Guest User"), "UserType": StringValue("Guest"), "UserLicenseId": IDValue(community)}},
		},
		"PermissionSet": {
			{ID: "0PS000000000050", Object: "PermissionSet", Fields: map[string]Value{"ProfileId": IDValue(guest), "IsOwnedByProfile": BooleanValue(true)}},
		},
	})
	before := rowsDigest(template)
	clone := template.CloneRuntimeFrozenShared()
	sibling := template.CloneRuntimeFrozenShared()
	requireRowsShared(t, clone, "Profile", "PermissionSet")

	migrateLegacyLicenseSeeds(&clone, "", community, chatter, "")

	if got := clone.Objects["Profile"].Records["00e000000000003"].Fields["UserLicenseId"].ID; got != chatter {
		t.Fatalf("clone profile license = %s, want %s", got, chatter)
	}
	if _, ok := clone.Objects["Profile"].Records[guest]; ok {
		t.Fatal("clone kept the legacy guest profile")
	}
	if _, ok := clone.Objects["PermissionSet"].Records["0PS000000000050"]; ok {
		t.Fatal("clone kept the legacy guest permission set")
	}
	requireTemplateUntouched(t, before, map[string]OrgState{"template": template, "sibling": sibling})
}

func TestCloneRuntimeFrozenSharedApplyFixtureIsolation(t *testing.T) {
	for _, object := range []string{"Profile", "UserLicense"} {
		t.Run(object, func(t *testing.T) {
			const existing ID = "0xx000000000001"
			template := sharedRowTemplate(map[string][]Record{
				object: {{ID: existing, Object: object, Fields: map[string]Value{"Name": StringValue("Seed")}}},
			})
			before := rowsDigest(template)
			clone := template.CloneRuntimeFrozenShared()
			sibling := template.CloneRuntimeFrozenShared()
			requireRowsShared(t, clone, object)

			fixture := Fixture{Version: FixtureVersion, Objects: []FixtureObject{{Name: object, Records: []FixtureRecord{
				{ID: "0xx000000000002", Fields: map[string]Value{"Name": StringValue("Added")}},
				{ID: existing, Fields: map[string]Value{"Name": StringValue("Replaced")}},
			}}}}
			if err := ApplyFixture(&clone, fixture); err != nil {
				t.Fatal(err)
			}
			records := clone.Objects[object].Records
			if got := records["0xx000000000002"].Fields["Name"].String; got != "Added" {
				t.Fatalf("clone added row = %q", got)
			}
			if got := records[existing].Fields["Name"].String; got != "Replaced" {
				t.Fatalf("clone replaced row = %q", got)
			}
			requireTemplateUntouched(t, before, map[string]OrgState{"template": template, "sibling": sibling})
		})
	}
}
