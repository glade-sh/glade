package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// case174DirectGaps pins the captured `direct` rows (SELECT ... FROM
// UserRecordAccess) whose Glade value still differs from native, keyed by
// api/route/object/grant/actor/ownership with the exact fields that differ.
// Any other difference fails, and so does a listed field that now matches:
// remove it from the table when that happens. Every unlisted row matches on
// all six fields.
// Each native method ran the relationship query (no clause, WITH USER_MODE in
// Case174ModeUSER, WITH SYSTEM_MODE in Case174ModeSYSTEM) on the same record
// just before the direct query, so the test does the same: a system-mode
// relationship query warms the direct Contract and Order rows
// (ura_direct_carry).
var case174DirectGaps = map[string][]string{}

func TestCase174DirectUserRecordAccessNative(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/case174_access_matrix/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	fields := []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"}
	for _, r := range rows[1:] {
		if (r[1] != "istest" && r[1] != "istest-user" && r[1] != "istest-system") || r[6] != "direct" || (r[4] != "restricted" && r[4] != "admin") {
			continue
		}
		row := r
		t.Run(strings.Join(row[:6], "/"), func(t *testing.T) {
			var want []any
			if err := json.Unmarshal([]byte(row[7]), &want); err != nil || len(want) != 6 {
				t.Fatalf("bad observation %v %v", want, err)
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, row[2])
			org.APIVersion = row[0]
			var profile storage.ID
			for id, p := range org.Objects["Profile"].Records {
				if p.Fields["Name"].String == "Minimum Access - Salesforce" {
					profile = id
					break
				}
			}
			const actor storage.ID = "005000000000099"
			current := actor
			if row[4] == "admin" {
				current = storage.ID(storage.DefaultUserID)
			}
			const perm storage.ID = "0PS000000000099"
			fixture := storage.NewFixture()
			fixture.Objects = []storage.FixtureObject{
				{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("matrix@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Matrix")}}}},
				{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Matrix"), "Label": storage.StringValue("Matrix")}}}},
				{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue(row[2]), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(row[3] != "read"), "PermissionsDelete": storage.BooleanValue(row[3] == "delete")}}}},
				{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
			}
			if err := storage.ApplyFixture(&org, fixture); err != nil {
				t.Fatal(err)
			}
			object := org.Objects[row[2]]
			object.Definition.SharingModel = "ReadWrite"
			id := storage.ID(object.Definition.KeyPrefix + "000000000001")
			other := storage.ID(storage.DefaultUserID)
			if row[4] == "admin" {
				other = actor
			}
			owner := other
			if row[5] == "own" || row[3] == "-" {
				// The OrderBoundary rows use an administrator-owned record.
				owner = current
			}
			if row[2] == "Pricebook2" {
				owner = ""
			}
			object.Records = map[storage.ID]storage.Record{id: {ID: id, Object: row[2], Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: owner}}}
			org.Objects[row[2]] = object
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records[current])
			machine.EnableTestContext()
			if row[5] == "runas" {
				machine.testContext.RunAsDepth = 1 // System.runAs of the administrator
			}
			if err := machine.RegisterClass(Class{Name: "Case174Direct", APIVersion: row[0], Modifiers: []string{"isTest"}}); err != nil {
				t.Fatal(err)
			}
			var src strings.Builder
			mode := map[string]string{"istest-user": " WITH USER_MODE", "istest-system": " WITH SYSTEM_MODE"}[row[1]]
			if row[3] != "-" {
				// The OrderBoundary methods (grant "-") ran the direct query alone.
				fmt.Fprintf(&src, "List<SObject> rel=Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM %s WHERE Id=:rid%s');", row[2], mode)
			}
			fmt.Fprintf(&src, "List<UserRecordAccess> rows=[SELECT %s FROM UserRecordAccess WHERE UserId=:UserInfo.getUserId() AND RecordId=:rid];System.assertEquals(1,rows.size());UserRecordAccess a=rows[0];", strings.Join(fields, ","))
			var mismatches []string
			for i, field := range fields {
				literal := fmt.Sprint(want[i])
				if s, ok := want[i].(string); ok {
					literal = "'" + s + "'"
				}
				prog, err := CompileAnonymousWithOptions(fmt.Sprintf("Id rid='%s';%sSystem.assertEquals(%s,a.get('%s'));", id, src.String(), literal, field), CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(prog, "Case174Direct"); err != nil {
					mismatches = append(mismatches, field)
				}
			}
			key := strings.Join(row[:6], "/")
			allowed := map[string]bool{}
			for _, field := range case174DirectGaps[key] {
				allowed[field] = true
			}
			got := map[string]bool{}
			for _, field := range mismatches {
				got[field] = true
				if !allowed[field] {
					t.Errorf("native %s: %s differs and is not a pinned gap", row[7], field)
				}
			}
			for field := range allowed {
				if !got[field] {
					t.Errorf("pinned gap %s now matches native %s: remove it from case174DirectGaps", field, row[7])
				}
			}
		})
	}
}

// Every captured row where the direct query shows Delete for a Contract or
// Order also shows Edit. A record the user cannot see, or can only read, never
// shows Delete, even when the user holds the object Delete permission.
func TestCase174DirectDeleteRequiresEdit(t *testing.T) {
	for _, object := range []string{"Contract", "Order"} {
		for _, tc := range []struct {
			name, sharing string
			read, edit    bool
		}{
			{"unshared-private", "Private", false, false},
			{"public-read-only", "ReadOnly", true, false},
		} {
			object, tc := object, tc
			t.Run(object+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, object)
				org.APIVersion = "62.0"
				var profile storage.ID
				for id, p := range org.Objects["Profile"].Records {
					if p.Fields["Name"].String == "Minimum Access - Salesforce" {
						profile = id
						break
					}
				}
				const actor storage.ID = "005000000000099"
				const perm storage.ID = "0PS000000000099"
				fixture := storage.NewFixture()
				fixture.Objects = []storage.FixtureObject{
					{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("matrix@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Matrix")}}}},
					{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Matrix"), "Label": storage.StringValue("Matrix")}}}},
					{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue(object), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true)}}}},
					{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
				}
				if err := storage.ApplyFixture(&org, fixture); err != nil {
					t.Fatal(err)
				}
				state := org.Objects[object]
				state.Definition.SharingModel = tc.sharing
				id := storage.ID(state.Definition.KeyPrefix + "000000000001")
				state.Records = map[storage.ID]storage.Record{id: {ID: id, Object: object, Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.ID(storage.DefaultUserID)}}}
				org.Objects[object] = state
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(org.Objects["User"].Records[actor])
				machine.EnableTestContext()
				if err := machine.RegisterClass(Class{Name: "Case174DeleteEdit", APIVersion: "62.0", Modifiers: []string{"isTest"}}); err != nil {
					t.Fatal(err)
				}
				src := fmt.Sprintf("UserRecordAccess a=[SELECT HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess FROM UserRecordAccess WHERE UserId=:UserInfo.getUserId() AND RecordId='%s'];", id)
				if !tc.read {
					src = fmt.Sprintf("List<UserRecordAccess> rows=[SELECT HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess FROM UserRecordAccess WHERE UserId=:UserInfo.getUserId() AND RecordId='%s'];UserRecordAccess a=rows.isEmpty()?new UserRecordAccess():rows[0];System.assertNotEquals(true,a.HasDeleteAccess);System.assertNotEquals(true,a.HasEditAccess);", id)
				} else {
					src += fmt.Sprintf("System.assertEquals(%v,a.HasReadAccess);System.assertEquals(%v,a.HasEditAccess);System.assertEquals(false,a.HasDeleteAccess);System.assertEquals(false,a.HasTransferAccess);", tc.read, tc.edit)
				}
				prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: "62.0"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(prog, "Case174DeleteEdit"); err != nil {
					t.Fatalf("%v\n%s", err, src)
				}
			})
		}
	}
}
