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

func TestRoad174OrderModifyAllRecordGrant(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/road174_profile_grants/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[5] != "relationship" {
			continue
		}
		row := row
		t.Run(strings.Join(row[:6], "/"), func(t *testing.T) {
			var want []any
			if err := json.Unmarshal([]byte(row[6]), &want); err != nil || len(want) != 6 {
				t.Fatalf("bad row: %v %v", want, err)
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Order")
			org.APIVersion = row[0]
			var profile storage.ID
			for id, p := range org.Objects["Profile"].Records {
				if p.Fields["Name"].String == "Minimum Access - Salesforce" {
					profile = id
					break
				}
			}
			if profile == "" {
				t.Fatal("missing profile")
			}
			const actor storage.ID = "005000000000099"
			const perm storage.ID = "0PS000000000099"
			fixture := storage.NewFixture()
			fixture.Objects = []storage.FixtureObject{
				{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("proof@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}}},
				{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Proof"), "Label": storage.StringValue("Proof")}}}},
				{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Order"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(true), "PermissionsModifyAllRecords": storage.BooleanValue(row[3] == "modify-all-records")}}}},
				{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
			}
			if err := storage.ApplyFixture(&org, fixture); err != nil {
				t.Fatal(err)
			}
			owner := storage.ID(storage.DefaultUserID)
			if row[4] == "own" {
				owner = actor
			}
			id := storage.ID("801000000000001")
			object := org.Objects["Order"]
			object.Definition.SharingModel = "ReadWrite" // Model captured other-record edit visibility.
			object.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Order", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: owner}}}
			org.Objects["Order"] = object
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records[actor])
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "Road174Grant", APIVersion: row[0], Modifiers: []string{"isTest"}}); err != nil {
				t.Fatal(err)
			}
			fields := []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"}
			var src strings.Builder
			src.WriteString("System.runAs(new User(Id=UserInfo.getUserId())){")
			mode := ""
			if row[1] != "default" {
				mode = " WITH " + strings.ToUpper(row[1]) + "_MODE"
			}
			fmt.Fprintf(&src, "List<SObject> rows=Database.query('SELECT %s FROM Order WHERE Id=\\'%s\\'%s');System.assertEquals(1,rows.size());SObject access=rows[0].getSObject('UserRecordAccess');", "UserRecordAccess."+strings.Join(fields, ",UserRecordAccess."), id, mode)
			for i, field := range fields {
				v := ""
				if b, ok := want[i].(bool); ok {
					v = fmt.Sprint(b)
				} else {
					v = fmt.Sprintf("'%s'", want[i])
				}
				fmt.Fprintf(&src, "System.assertEquals(%s,access.get('%s'));", v, field)
			}
			src.WriteString("}")
			prog, err := CompileAnonymousWithOptions(src.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				if _, err = machine.ExecuteInClass(prog, "Road174Grant"); err != nil {
					t.Fatalf("pass %d, native %s: %v", pass, row[6], err)
				}
			}

		})
	}
}
