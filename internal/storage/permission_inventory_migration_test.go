package storage

import "testing"

// Previously persisted orgs can contain the former counter-style setup rows.
// Migrate those rows while leaving independently fabricated IDs untouched.
func TestPermissionInventoryLegacyMigration(t *testing.T) {
	org := NewOrgState()
	org.Objects["PermissionSet"] = ObjectState{Records: map[ID]Record{
		"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("GladeBaseline"), "Label": StringValue("GLADE Baseline"), "IsOwnedByProfile": BooleanValue(false)}},
		"0PS000000000002": {ID: "0PS000000000002", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("Fabricated"), "Label": StringValue("Fabricated"), "IsOwnedByProfile": BooleanValue(false)}},
		"0PS000000000003": {ID: "0PS000000000003", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("X" + DefaultProfileID), "ProfileId": IDValue(DefaultProfileID), "IsOwnedByProfile": BooleanValue(true)}},
	}}
	org.Objects["PermissionSetAssignment"] = ObjectState{Records: map[ID]Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]Value{"AssigneeId": IDValue(DefaultUserID), "PermissionSetId": IDValue("0PS000000000003")}}}}
	org.Objects["ObjectPermissions"] = ObjectState{Records: map[ID]Record{"110000000000003": {ID: "110000000000003", Object: "ObjectPermissions", Fields: map[string]Value{"ParentId": IDValue("0PS000000000003"), "SobjectType": StringValue("Account"), "PermissionsRead": BooleanValue(true)}}}}
	EnsureDeterministicPlatformData(&org)
	if got := org.Objects["PermissionSet"].Records["0PS000000000002"].Fields["Name"].String; got != "Fabricated" {
		t.Fatalf("fabricated permission set changed: %q", got)
	}
	if _, ok := org.Objects["PermissionSet"].Records["0PS000000000001"]; ok {
		t.Fatal("legacy synthetic baseline survived migration")
	}
	if _, ok := org.Objects["PermissionSet"].Records["0PS000000000003"]; ok {
		t.Fatal("legacy administrator set survived migration")
	}
	var admin ID
	for id, record := range org.Objects["PermissionSet"].Records {
		if record.Fields["IsOwnedByProfile"].Boolean && IDsEqual(record.Fields["ProfileId"].ID, DefaultProfileID) {
			if admin != "" {
				t.Fatal("duplicate admin profile sets")
			}
			admin = id
		}
	}
	if admin == "" {
		t.Fatal("missing administrator set")
	}
	if got := org.Objects["ObjectPermissions"].Records["110000000000003"].Fields["ParentId"].ID; !IDsEqual(got, admin) {
		t.Errorf("legacy object permission parent=%s, want %s", got, admin)
	}
	count := 0
	for _, record := range org.Objects["PermissionSetAssignment"].Records {
		if IDsEqual(record.Fields["AssigneeId"].ID, DefaultUserID) && IDsEqual(record.Fields["PermissionSetId"].ID, admin) {
			count++
		}
	}
	if count != 1 {
		t.Errorf("administrator assignment count=%d, want 1", count)
	}
	EnsureDeterministicPlatformData(&org)
	if len(org.Objects["PermissionSet"].Records) != 89 {
		t.Errorf("permission sets after repeated seeding=%d, want 89", len(org.Objects["PermissionSet"].Records))
	}
}

func TestPermissionInventoryPreservesImportedSet(t *testing.T) {
	org := NewOrgState()
	const imported ID = "0PS5k0000G00001"
	org.Objects["PermissionSet"] = ObjectState{Records: map[ID]Record{
		imported: {ID: imported, Object: "PermissionSet", Fields: map[string]Value{
			"Name": StringValue("CommerceUser"), "NamespacePrefix": StringValue("force"), "Label": StringValue("Shopper"),
			"IsOwnedByProfile": BooleanValue(false), "ImportedMarker": StringValue("preserve"),
		}},
	}}
	EnsureDeterministicPlatformData(&org)
	count := 0
	for _, set := range org.Objects["PermissionSet"].Records {
		if set.Fields["Name"].String == "CommerceUser" && set.Fields["NamespacePrefix"].String == "force" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("imported CommerceUser rows=%d, want 1", count)
	}
	if got := org.Objects["PermissionSet"].Records[imported].Fields["ImportedMarker"].String; got != "preserve" {
		t.Errorf("imported marker=%q", got)
	}
}

// Every field that references PermissionSet is repointed, not only the four
// the first migration listed. Synthetic sets with user-created dependents are
// kept; unreferenced ones go. Migrated ObjectPermissions rows are not seeded
// a second time, and the former Marketing User Account row gains the captured
// delete flag.
func TestPermissionInventoryLegacyMigrationReferences(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "PermissionSetGroupComponent")
	EnsureStandardObject(&org, "PermissionSetTabSetting")
	for object, field := range map[string]string{"PermissionSetGroupComponent": "PermissionSetId", "PermissionSetTabSetting": "ParentId"} {
		definition := org.Objects[object].Definition.Fields[field]
		if definition.Type != FieldReference || !referencesObject(definition.ReferenceTo, "PermissionSet") {
			t.Fatalf("%s.%s = %#v, want a PermissionSet reference", object, field, definition)
		}
	}
	const marketingProfile ID = "00e000000000006"
	org.Objects["PermissionSet"] = ObjectState{Records: map[ID]Record{
		"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("GladeBaseline"), "Label": StringValue("GLADE Baseline"), "IsOwnedByProfile": BooleanValue(false)}},
		"0PS000000000002": {ID: "0PS000000000002", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("Proving_With_User_Mode_Works"), "Label": StringValue("Proving"), "IsOwnedByProfile": BooleanValue(false)}},
		"0PS000000000003": {ID: "0PS000000000003", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("X" + DefaultProfileID), "ProfileId": IDValue(DefaultProfileID), "IsOwnedByProfile": BooleanValue(true)}},
		"0PS000000000008": {ID: "0PS000000000008", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("X" + string(marketingProfile)), "ProfileId": IDValue(marketingProfile), "IsOwnedByProfile": BooleanValue(true)}},
		"0PS000000000011": {ID: "0PS000000000011", Object: "PermissionSet", Fields: map[string]Value{"Name": StringValue("CommerceUser"), "Label": StringValue("Shopper"), "IsOwnedByProfile": BooleanValue(false)}},
	}}
	const otherUser ID = "005000000000777"
	put := func(object string, id ID, fields map[string]Value) {
		state := org.Objects[object]
		if state.Records == nil {
			state.Records = map[ID]Record{}
		}
		state.Records[id] = Record{ID: id, Object: object, Fields: fields}
		org.Objects[object] = state
	}
	put("PermissionSetAssignment", "0Pa000000000001", map[string]Value{"AssigneeId": IDValue(DefaultUserID), "PermissionSetId": IDValue("0PS000000000003")})
	put("PermissionSetAssignment", "0Pa000000000777", map[string]Value{"AssigneeId": IDValue(otherUser), "PermissionSetId": IDValue("0PS000000000001")})
	put("SetupEntityAccess", "0J0000000000777", map[string]Value{"ParentId": IDValue("0PS000000000001"), "SetupEntityId": IDValue("01p000000000777")})
	put("SetupEntityAccess", "0J0000000000778", map[string]Value{"ParentId": IDValue("0PS000000000003"), "SetupEntityId": IDValue("01p000000000778")})
	put("ObjectPermissions", "110000000000003", map[string]Value{"ParentId": IDValue("0PS000000000003"), "SobjectType": StringValue("Account"), "PermissionsRead": BooleanValue(true), "PermissionsCreate": BooleanValue(true), "PermissionsEdit": BooleanValue(true), "PermissionsDelete": BooleanValue(true)})
	put("ObjectPermissions", "110000000000006", map[string]Value{"ParentId": IDValue("0PS000000000008"), "SobjectType": StringValue("Account"), "PermissionsRead": BooleanValue(true), "PermissionsCreate": BooleanValue(true), "PermissionsEdit": BooleanValue(true), "PermissionsDelete": BooleanValue(false)})
	put("FieldPermissions", "0FP000000000777", map[string]Value{"ParentId": IDValue(ID(ID18("0PS000000000003"))), "SobjectType": StringValue("Account"), "Field": StringValue("Account.Site"), "PermissionsRead": BooleanValue(true), "PermissionsEdit": BooleanValue(false)})
	put("PermissionSetGroupComponent", "0PM000000000777", map[string]Value{"PermissionSetGroupId": IDValue("0PG000000000777"), "PermissionSetId": IDValue("0PS000000000011")})
	put("PermissionSetTabSetting", "01P000000000777", map[string]Value{"ParentId": IDValue("0PS000000000003"), "Name": StringValue("standard-Account"), "Visibility": StringValue("DefaultOn")})
	EnsureDeterministicPlatformData(&org)

	sets := org.Objects["PermissionSet"].Records
	if _, ok := sets["0PS000000000001"]; !ok {
		t.Fatal("GladeBaseline with a user assignment was dropped")
	}
	if _, ok := sets["0PS000000000002"]; ok {
		t.Fatal("unreferenced synthetic set survived")
	}
	for _, legacy := range []ID{"0PS000000000003", "0PS000000000008", "0PS000000000011"} {
		if _, ok := sets[legacy]; ok {
			t.Fatalf("legacy set %s survived", legacy)
		}
	}
	if len(sets) != 89 {
		t.Fatalf("permission sets=%d, want 88 seeded plus the retained GladeBaseline", len(sets))
	}
	profileSet := func(profileID ID) ID {
		for id, set := range sets {
			if set.Fields["IsOwnedByProfile"].Boolean && IDsEqual(set.Fields["ProfileId"].ID, profileID) {
				return id
			}
		}
		t.Fatalf("no profile set for %s", profileID)
		return ""
	}
	admin, marketing := profileSet(DefaultProfileID), profileSet(marketingProfile)
	commerce, ok := findNamedPermissionSet(&org, "CommerceUser", "force")
	if !ok {
		t.Fatal("seeded CommerceUser missing")
	}
	want := map[ID]struct {
		object, field string
		target        ID
	}{
		"0Pa000000000777": {"PermissionSetAssignment", "PermissionSetId", "0PS000000000001"},
		"0J0000000000777": {"SetupEntityAccess", "ParentId", "0PS000000000001"},
		"0J0000000000778": {"SetupEntityAccess", "ParentId", admin},
		"110000000000003": {"ObjectPermissions", "ParentId", admin},
		"110000000000006": {"ObjectPermissions", "ParentId", marketing},
		"0FP000000000777": {"FieldPermissions", "ParentId", admin},
		"0PM000000000777": {"PermissionSetGroupComponent", "PermissionSetId", commerce},
		"01P000000000777": {"PermissionSetTabSetting", "ParentId", admin},
	}
	for id, expect := range want {
		record, ok := org.Objects[expect.object].Records[id]
		if !ok {
			t.Errorf("%s %s was deleted", expect.object, id)
			continue
		}
		if got := record.Fields[expect.field].ID; !IDsEqual(got, expect.target) {
			t.Errorf("%s %s.%s=%s, want %s", expect.object, id, expect.field, got, expect.target)
		}
	}
	for _, reference := range permissionSetReferenceFields(&org) {
		for id, record := range org.Objects[reference.object].Records {
			target := record.Fields[reference.field].ID
			if target == "" {
				continue
			}
			if _, _, found := LookupRecordByID(sets, target); !found {
				if _, _, profile := LookupRecordByID(org.Objects["Profile"].Records, target); !profile {
					t.Errorf("%s %s.%s points at missing %s", reference.object, id, reference.field, target)
				}
			}
		}
	}
	if got := org.Objects["ObjectPermissions"].Records["110000000000006"].Fields["PermissionsDelete"]; !got.Boolean {
		t.Error("migrated Marketing User Account row kept PermissionsDelete=false; the capture says true")
	}
	seen := map[string]ID{}
	for id, record := range org.Objects["ObjectPermissions"].Records {
		key := string(record.Fields["ParentId"].ID) + "/" + record.Fields["SobjectType"].String
		if first, dup := seen[key]; dup {
			t.Errorf("duplicate ObjectPermissions rows for %s: %s and %s", key, first, id)
		}
		seen[key] = id
	}
	fresh := NewOrgState()
	EnsureDeterministicPlatformData(&fresh)
	if len(org.Objects["ObjectPermissions"].Records) != len(fresh.Objects["ObjectPermissions"].Records) {
		t.Errorf("migrated org has %d ObjectPermissions rows, fresh seed %d", len(org.Objects["ObjectPermissions"].Records), len(fresh.Objects["ObjectPermissions"].Records))
	}
}
