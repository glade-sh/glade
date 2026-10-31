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

func TestRoad174GrantExecutionContexts(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/road174_grant_context/org.tsv")
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
		row := row
		if row[4] == "boundary" {
			continue
		}
		t.Run(strings.Join(row[:5], "/"), func(t *testing.T) {
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Order")
			org.APIVersion = row[0]
			var profile storage.ID
			for id, p := range org.Objects["Profile"].Records {
				if p.Fields["Name"].String == "Standard User" {
					profile = id
					break
				}
			}
			if profile == "" {
				t.Fatal("missing standard profile")
			}
			const actor storage.ID = "005000000000099"
			const perm storage.ID = "0PS000000000099"
			id := storage.ID("801000000000001")
			fs := storage.NewFixture()
			fs.Objects = []storage.FixtureObject{
				{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("proof@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}}},
				{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Proof"), "PermissionsModifyAllData": storage.BooleanValue(row[1] == "modify-all-data")}}}},
				{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Order"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(true), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}}},
				{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
			}
			if err := storage.ApplyFixture(&org, fs); err != nil {
				t.Fatal(err)
			}
			owner := storage.ID(storage.DefaultUserID)
			if row[4] == "own" {
				owner = actor
			}
			obj := org.Objects["Order"]
			obj.Definition.SharingModel = "ReadWrite"
			obj.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Order", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: owner}}}
			org.Objects["Order"] = obj
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(org.Objects["User"].Records[actor])
			if row[2] == "runas-named" {
				machine.EnableTestContext()
			}
			if row[2] != "anonymous" {
				if err := machine.RegisterClass(Class{Name: "Road174Context", APIVersion: row[0]}); err != nil {
					t.Fatal(err)
				}
			}
			mode := ""
			if row[3] != "default" {
				mode = " WITH " + strings.ToUpper(row[3]) + "_MODE"
			}
			fields := []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"}
			operator := "="
			if row[4] == "other" {
				operator = "!="
			}
			query := fmt.Sprintf("SELECT %s FROM Order WHERE OwnerId %s :uid%s", "UserRecordAccess."+strings.Join(fields, ",UserRecordAccess."), operator, mode)
			var src strings.Builder
			src.WriteString("Id uid=UserInfo.getUserId();")
			if row[2] == "runas-named" {
				src.WriteString("System.runAs(new User(Id=UserInfo.getUserId())){")
			}
			if row[5] == "reject" {
				fmt.Fprintf(&src, "try{Database.query('%s');System.assert(false,'expected rejection');}catch(System.SecurityException e){}", query)
			} else {
				var want []any
				if err := json.Unmarshal([]byte(row[6]), &want); err != nil || len(want) != 6 {
					t.Fatalf("bad row %v %v", want, err)
				}
				// The earlier native USER_MODE rows followed prior queries. The
				// direct clause sequence in road174_record_controls/org.tsv:2
				// records false first and true on the repeated query.
				if row[3] == "user" {
					fmt.Fprintf(&src, "Database.query('%s');", query)
				}
				fmt.Fprintf(&src, "List<SObject> q=Database.query('%s');System.assertEquals(1,q.size());SObject a=q[0].getSObject('UserRecordAccess');", query)
				for i, field := range fields {
					v := fmt.Sprint(want[i])
					if _, ok := want[i].(string); ok {
						v = fmt.Sprintf("'%s'", want[i])
					}
					fmt.Fprintf(&src, "System.assertEquals(%s,a.get('%s'));", v, field)
				}
			}
			if row[2] == "runas-named" {
				src.WriteString("}")
			}
			prog, err := CompileAnonymousWithOptions(src.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			for pass := 0; pass < 2; pass++ {
				if row[2] == "anonymous" {
					_, err = machine.Execute(prog)
				} else {
					_, err = machine.ExecuteInClass(prog, "Road174Context")
				}
				if err != nil {
					t.Fatalf("pass %d native %s: %v", pass, row[6], err)
				}
			}
		})
	}
}
