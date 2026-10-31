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

func TestRoad174AccessMatrix(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/road174_access_matrix/org.tsv")
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
	for _, r := range rows[1:] {
		if (r[0] != "62.0" && r[0] != "67.0") || !strings.HasPrefix(r[1], "istest") || (r[4] != "restricted" && r[4] != "admin") || r[3] == "-" || r[6] != "relationship" {
			continue
		}
		row := r
		t.Run(strings.Join(row[:7], "/"), func(t *testing.T) {
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
			if profile == "" {
				t.Fatal("missing minimum profile")
			}
			const actor storage.ID = "005000000000099"
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
			// Model the captured other-record visibility and edit access.
			object := org.Objects[row[2]]
			object.Definition.SharingModel = "ReadWrite"
			id := storage.ID(object.Definition.KeyPrefix + "000000000001")
			current := actor
			other := storage.ID(storage.DefaultUserID)
			if row[4] == "admin" {
				current, other = storage.ID(storage.DefaultUserID), actor
			}
			owner := other
			if row[5] == "own" {
				owner = current
			}
			if row[2] == "Pricebook2" {
				owner = "" // The native price book has no owner field.
			}
			object.Records = map[storage.ID]storage.Record{id: {ID: id, Object: row[2], Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: owner}}}
			org.Objects[row[2]] = object
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records[current])
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "Road174Matrix", APIVersion: row[0], Modifiers: []string{"isTest"}}); err != nil {
				t.Fatal(err)
			}
			fields := []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"}
			var src strings.Builder
			mode := ""
			if row[1] == "istest-user" {
				mode = " WITH USER_MODE"
			}
			if row[1] == "istest-system" {
				mode = " WITH SYSTEM_MODE"
			}
			fmt.Fprintf(&src, "List<SObject> rows=Database.query('SELECT %s FROM %s WHERE Id=\\'%s\\'%s');System.assertEquals(1,rows.size());SObject access=rows[0].getSObject('UserRecordAccess');", "UserRecordAccess."+strings.Join(fields, ",UserRecordAccess."), row[2], id, mode)
			for i, field := range fields {
				var value string
				if b, ok := want[i].(bool); ok {
					value = fmt.Sprint(b)
				} else {
					value = fmt.Sprintf("'%s'", want[i])
				}
				fmt.Fprintf(&src, "System.assertEquals(%s,access.get('%s'));", value, field)
			}
			prog, err := CompileAnonymousWithOptions(src.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				if _, err = machine.ExecuteInClass(prog, "Road174Matrix"); err != nil {
					t.Fatalf("pass %d, native %s: %v", pass, row[7], err)
				}
			}
		})
	}
}
