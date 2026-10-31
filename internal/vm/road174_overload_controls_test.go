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

// Captured at road174_overload_controls/org.tsv:2-73, including cold and
// repeated overload calls in the same Apex execution.
func TestRoad174OverloadControls(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/road174_overload_controls/org.tsv")
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
		if row[3] != "cold" && row[3] != "sequence" {
			continue
		}
		row := row
		t.Run(strings.Join(row[:6], "/"), func(t *testing.T) {
			machine := road174ControlVM(t, row[0], row[1], row[2])
			fields := "UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel"
			q := "SELECT " + fields + " FROM Order WHERE OwnerId = :uid"
			var source strings.Builder
			source.WriteString("Id uid=UserInfo.getUserId();String q='" + q + "';")
			if row[2] == "runas-named" {
				source.WriteString("System.runAs(new User(Id=uid)){")
			}
			call := "Database.query(q,AccessLevel.USER_MODE)"
			if row[4] == "binds" {
				call = "Database.queryWithBinds(q,new Map<String,Object>{'uid'=>uid},AccessLevel.USER_MODE)"
			}
			if row[3] == "cold" {
				source.WriteString("List<SObject> z=" + call + ";SObject a=z[0].getSObject('UserRecordAccess');System.assertEquals(false,a.get('HasAllAccess'));System.assertEquals('All',a.get('MaxAccessLevel'));")
			} else {
				var sequence [][]any
				if err := json.Unmarshal([]byte(row[6]), &sequence); err != nil {
					t.Fatal(err)
				}
				for i, item := range sequence {
					route := item[0].(string)
					c := "Database.query(q)"
					if route == "user" {
						c = "Database.query(q,AccessLevel.USER_MODE)"
					}
					if route == "system" {
						c = "Database.query(q,AccessLevel.SYSTEM_MODE)"
					}
					if len(item) == 2 {
						fmt.Fprintf(&source, "Boolean caught%d=false;try{%s;}catch(System.SecurityException e){caught%d=true;}System.assert(caught%d);", i, c, i, i)
						continue
					}
					fmt.Fprintf(&source, "List<SObject> z%d=%s;SObject a%d=z%d[0].getSObject('UserRecordAccess');System.assertEquals(%v,a%d.get('HasAllAccess'));System.assertEquals('All',a%d.get('MaxAccessLevel'));", i, c, i, i, item[1], i, i)
				}
			}
			if row[2] == "runas-named" {
				source.WriteString("}")
			}
			prog, err := CompileAnonymousWithOptions(source.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if row[2] == "anonymous" {
				_, err = machine.Execute(prog)
			} else {
				_, err = machine.ExecuteInClass(prog, "Road174Context")
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func road174ControlVM(t *testing.T, version, group, context string) *VM {
	t.Helper()
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Order")
	org.APIVersion = version
	profileName := "Standard User"
	if strings.HasPrefix(group, "min-") {
		profileName = "Minimum Access - Salesforce"
	} else if strings.HasPrefix(group, "marketing-") {
		profileName = "Marketing User"
	}
	var profile storage.ID
	for id, p := range org.Objects["Profile"].Records {
		if p.Fields["Name"].String == profileName {
			profile = id
			break
		}
	}
	if profile == "" {
		t.Fatal("profile missing")
	}
	const actor storage.ID = "005000000000099"
	const perm storage.ID = "0PS000000000099"
	const order storage.ID = "801000000000001"
	fs := storage.NewFixture()
	fs.Objects = []storage.FixtureObject{
		{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("proof@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}}},
		{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Proof"), "PermissionsModifyAllData": storage.BooleanValue(strings.HasSuffix(group, "global"))}}}},
		{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Order"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(true), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}}},
		{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
	}
	if err := storage.ApplyFixture(&org, fs); err != nil {
		t.Fatal(err)
	}
	actorID := actor
	if group == "admin" {
		actorID = storage.ID(storage.DefaultUserID)
	}
	obj := org.Objects["Order"]
	obj.Definition.SharingModel = "ReadWrite"
	obj.Records = map[storage.ID]storage.Record{order: {ID: order, Object: "Order", Fields: map[string]storage.Value{"Description": storage.StringValue("proof")}, System: storage.SystemFields{OwnerID: actorID}}}
	org.Objects["Order"] = obj
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(org.Objects["User"].Records[actorID])
	if context == "runas-named" {
		machine.EnableTestContext()
	}
	if context != "anonymous" {
		if err := machine.RegisterClass(Class{Name: "Road174Context", APIVersion: version}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

// The anonymous SYSTEM_MODE prohibition is independent of projection order,
// LIMIT and permission set: org.tsv:74 onward includes both user and admin.
func TestRoad174AnonymousSystemControls(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/road174_overload_controls/org.tsv")
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
		if row[2] != "anonymous" || row[5] != "system" || row[3] == "cold" || row[3] == "sequence" {
			continue
		}
		row := row
		t.Run(strings.Join(row[:6], "/"), func(t *testing.T) {
			machine := road174ControlVM(t, row[0], row[1], "anonymous")
			fields := "UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel"
			q := "SELECT " + fields + " FROM Order WHERE OwnerId = :uid"
			switch row[3] {
			case "reorder":
				q = "SELECT UserRecordAccess.MaxAccessLevel,UserRecordAccess.HasAllAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasReadAccess FROM Order WHERE OwnerId = :uid"
			case "limit":
				q += " LIMIT 1"
			case "predicate":
				q = "SELECT " + fields + " FROM Order WHERE Description != null LIMIT 1"
			case "unfiltered":
				q = "SELECT " + fields + " FROM Order LIMIT 1"
			case "single":
				q = "SELECT UserRecordAccess.HasAllAccess FROM Order LIMIT 1"
			case "plain":
				q = "SELECT Id FROM Order LIMIT 1"
			case "invalid":
				q = "SELECT UserRecordAccess.UserId FROM Order LIMIT 1"
			}
			call := "Database.query(q,AccessLevel.SYSTEM_MODE)"
			if row[4] == "binds" {
				call = "Database.queryWithBinds(q,new Map<String,Object>{'uid'=>uid},AccessLevel.SYSTEM_MODE)"
			}
			if row[4] == "clause" {
				q = strings.ReplaceAll(q, " LIMIT 1", "") + " WITH SYSTEM_MODE"
				if row[3] == "limit" || row[3] == "predicate" || row[3] == "unfiltered" || row[3] == "single" || row[3] == "plain" || row[3] == "invalid" {
					q += " LIMIT 1"
				}
				call = "Database.query(q)"
			}
			expected := strings.TrimPrefix(row[6], "ERROR:")
			src := fmt.Sprintf("Id uid=UserInfo.getUserId();String q='%s';String caught='';try{%s;}catch(Exception e){caught=e.getTypeName();}System.assertEquals('%s',caught);", q, call, expected)
			prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(prog); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Replays the exact native loop, including query order and all USER_MODE shape
// observations: road174_overload_controls/org.tsv:74-115 and adjacent twins.
func TestRoad174CapturedUserShapeLoop(t *testing.T) {
	root := "../apextest/testdata/conformance/road174_overload_controls/"
	native, err := os.ReadFile(root + "anonymous.apex")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(root + "org.tsv")
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
	for _, api := range []string{"62.0", "67.0"} {
		for _, group := range []string{"std-object", "std-global", "min-object", "min-global", "admin"} {
			api, group := api, group
			t.Run(api+"/"+group, func(t *testing.T) {
				machine := road174ControlVM(t, api, group, "anonymous")
				// The native setup inserted the other-owner record first.
				object := machine.Org.Objects["Order"]
				ownID, otherID := storage.ID("801000000000002"), storage.ID("801000000000001")
				owner, otherOwner := storage.ID("005000000000099"), storage.ID(storage.DefaultUserID)
				object.Records = map[storage.ID]storage.Record{
					otherID: {ID: otherID, Object: "Order", Fields: map[string]storage.Value{"Description": storage.StringValue("other")}, System: storage.SystemFields{OwnerID: otherOwner}},
					ownID:   {ID: ownID, Object: "Order", Fields: map[string]storage.Value{"Description": storage.StringValue("own")}, System: storage.SystemFields{OwnerID: owner}},
				}
				machine.Org.Objects["Order"] = object
				var source strings.Builder
				source.Write(native)
				count := 0
				for _, row := range rows[1:] {
					if row[0] != api || row[1] != group || row[2] != "anonymous" || row[5] != "user" || row[3] == "cold" || row[3] == "sequence" {
						continue
					}
					key := row[3] + "|USER|" + row[4]
					want := strings.ReplaceAll(row[6], "'", "\\'")
					fmt.Fprintf(&source, "System.assertEquals('%s',String.valueOf(out.get('%s')),'%s');", want, key, key)
					count++
				}
				if count == 0 {
					t.Fatal("missing native rows")
				}
				prog, err := CompileAnonymousWithOptions(source.String(), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(prog); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
