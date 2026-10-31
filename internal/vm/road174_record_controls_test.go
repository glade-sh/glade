package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func road174RecordRows(t *testing.T) [][]string {
	t.Helper()
	f, err := os.Open("../apextest/testdata/conformance/road174_record_controls/org.tsv")
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
	return rows[1:]
}
func road174TwoRecords(machine *VM) {
	obj := machine.Org.Objects["Order"]
	ownID, otherID := storage.ID("801000000000002"), storage.ID("801000000000001")
	obj.Records = map[storage.ID]storage.Record{
		ownID:   {ID: ownID, Object: "Order", Fields: map[string]storage.Value{"Description": storage.StringValue("own")}, System: storage.SystemFields{OwnerID: storage.ID("005000000000099")}},
		otherID: {ID: otherID, Object: "Order", Fields: map[string]storage.Value{"Description": storage.StringValue("other")}, System: storage.SystemFields{OwnerID: storage.ID(storage.DefaultUserID)}},
	}
	machine.Org.Objects["Order"] = obj
}

// The native scripts and results are preserved together. Each script runs in
// one execution, preserving the order of queries and intervening changes.
func TestRoad174RecordStateControls(t *testing.T) {
	for _, row := range road174RecordRows(t) {
		row := row
		if row[2] != "records" && row[2] != "failure" && row[2] != "empty" && row[2] != "mutation" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			machine := road174ControlVM(t, row[0], row[1]+"-object", "anonymous")
			road174TwoRecords(machine)
			src, err := os.ReadFile("../apextest/testdata/conformance/road174_record_controls/" + row[2] + ".apex")
			if err != nil {
				t.Fatal(err)
			}
			// Assert the value already emitted by the unmodified native probe.
			code := string(src) + fmt.Sprintf("System.assertEquals('%s',JSON.serialize(out));", row[3])
			prog, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(prog); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRoad174ThirdProfileSequence(t *testing.T) {
	for _, row := range road174RecordRows(t) {
		row := row
		if row[1] != "marketing" || row[2] != "seq-anonymous" && row[2] != "seq-named" {
			continue
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			context := "anonymous"
			if row[2] == "seq-named" {
				context = "named"
			}
			machine := road174ControlVM(t, row[0], "marketing-object", context)
			road174TwoRecords(machine)
			src, err := os.ReadFile("../apextest/testdata/conformance/road174_overload_controls/seq-anonymous.apex")
			if err != nil {
				t.Fatal(err)
			}
			code := string(src) + fmt.Sprintf("System.assertEquals('%s',JSON.serialize(out));", row[3])
			prog, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if context == "named" {
				_, err = machine.ExecuteInClass(prog, "Road174Context")
			} else {
				_, err = machine.Execute(prog)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRoad174InvalidProjectionPrecedence(t *testing.T) {
	for _, row := range road174RecordRows(t) {
		row := row
		if !strings.HasPrefix(row[2], "reject-") {
			continue
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			machine := road174ControlVM(t, row[0], "std-object", "anonymous")
			parts := strings.Split(strings.TrimPrefix(row[2], "reject-"), "|")
			field, route := parts[0], parts[1]
			q := "SELECT UserRecordAccess." + field + " FROM Order LIMIT 1"
			call := "Database.query(q,AccessLevel.SYSTEM_MODE)"
			if route == "binds" {
				call = "Database.queryWithBinds(q,new Map<String,Object>(),AccessLevel.SYSTEM_MODE)"
			}
			if route == "clause" {
				q = "SELECT UserRecordAccess." + field + " FROM Order WITH SYSTEM_MODE LIMIT 1"
				call = "Database.query(q)"
			}
			src := fmt.Sprintf("String q='%s';String got='';try{%s;}catch(Exception e){got=e.getTypeName();}System.assertEquals('%s',got);", q, call, row[3])
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

// Scoped access has no native enabled-feature rows. Retain the old security
// contract rather than applying the unscoped projection exemption.
func TestRoad174ScopedProjectionPreservesPriorSecurity(t *testing.T) {
	machine := road174ControlVM(t, "67.0", "std-object", "named")
	q, err := machine.parseSOQLAt("SELECT UserRecordAccess.HasAllAccess FROM Order WITH USER_MODE")
	if err != nil {
		t.Fatal(err)
	}
	if err = machine.enforceSOQLSecurity(q, "0PS000000000099"); err == nil {
		t.Fatal("scoped projection unexpectedly bypassed prior field security")
	}
	record := machine.Org.Objects["Order"].Records[storage.ID("801000000000001")]
	want := machine.userRecordAccessForRecord("Order", record)
	got := machine.userRecordAccessForRelationship("Order", record, "USER_MODE", "USER_MODE", 0, true)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scoped projection changed shipping behavior: got %#v want %#v", got, want)
	}
}

func TestRoad174PermissionChangeSequence(t *testing.T) {
	for _, row := range road174RecordRows(t) {
		row := row
		if row[2] != "perm" {
			continue
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			machine := road174ControlVM(t, row[0], row[1]+"-object", "runas-named")
			road174TwoRecords(machine)
			machine.SetCurrentUser(machine.Org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
			var want []string
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			q := "SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE OwnerId = :uid"
			var src strings.Builder
			src.WriteString("Id who='005000000000099';Id pid='0PS000000000099';PermissionSetAssignment pa=[SELECT Id FROM PermissionSetAssignment WHERE AssigneeId=:who AND PermissionSetId=:pid LIMIT 1];")
			for i := 0; i < 3; i++ {
				if i == 1 {
					src.WriteString("delete pa;")
				}
				if i == 2 {
					src.WriteString("insert new PermissionSetAssignment(AssigneeId=who,PermissionSetId=pid);")
				}
				fmt.Fprintf(&src, "System.runAs(new User(Id=who)){Id uid=UserInfo.getUserId();List<SObject> rows=Database.query('%s',AccessLevel.USER_MODE);SObject a=rows[0].getSObject('UserRecordAccess');System.assertEquals('%s',JSON.serialize(new List<Object>{a.get('HasAllAccess'),a.get('MaxAccessLevel')}));}", q, want[i])
			}
			prog, err := CompileAnonymousWithOptions(src.String(), CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.ExecuteInClass(prog, "Road174Context"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Local route control only: enabled custom-scope Salesforce behavior is not
// captured; the expected query denial is the shipping field-security path.
func TestRoad174ScopedProjectionAnonymousRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := road174ControlVM(t, api, "std-object", "anonymous")
			machine.Org.Features = append(machine.Org.Features, "ApexUserModeWithPermset")
			src := "AccessLevel level=AccessLevel.USER_MODE.withPermissionSetId('0PS000000000099');String got='';try{Database.query('SELECT UserRecordAccess.HasAllAccess FROM Order',level);}catch(Exception e){got=e.getTypeName();}System.assertEquals('System.QueryException',got);"
			prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(prog); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Group-assigned record grants have no native capture in this family. Preserve
// the shipping HasAllAccess projection rather than extending direct-set rows.
func TestRoad174GroupGrantPreservesPriorProjection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := road174ControlVM(t, api, "std-object", "named")
			storage.EnsureStandardObject(machine.Org, "PermissionSetGroupComponent")
			assignments := machine.Org.Objects["PermissionSetAssignment"]
			for id, r := range assignments.Records {
				v, _ := r.GetField("AssigneeId")
				if storage.IDsEqual(storage.ID(storageValueIDText(v)), storage.ID("005000000000099")) {
					r.Fields["PermissionSetId"] = storage.NullValue()
					r.Fields["PermissionSetGroupId"] = storage.IDValue("0PG000000000099")
					assignments.Records[id] = r
				}
			}
			machine.Org.Objects["PermissionSetAssignment"] = assignments
			components := machine.Org.Objects["PermissionSetGroupComponent"]
			components.Records[storage.ID("0PC000000000099")] = storage.Record{ID: "0PC000000000099", Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue("0PG000000000099"), "PermissionSetId": storage.IDValue("0PS000000000099")}}
			machine.Org.Objects["PermissionSetGroupComponent"] = components
			record := machine.Org.Objects["Order"].Records[storage.ID("801000000000001")]
			got := machine.userRecordAccessForRelationship("Order", record, "USER_MODE", "USER_MODE", 2, false)
			if !got.Fields["HasAllAccess"].Boolean {
				t.Fatal("unobserved group grant changed prior HasAllAccess")
			}
			for _, route := range []string{"anonymous", "named", "istest", "runas"} {
				t.Run(route, func(t *testing.T) {
					src := "Id uid=UserInfo.getUserId();"
					if route == "istest" {
						machine.EnableTestContext()
						if err := machine.RegisterClass(Class{Name: "Road174GroupTest", APIVersion: api, IsTest: true}); err != nil {
							t.Fatal(err)
						}
					}
					if route == "runas" {
						machine.EnableTestContext()
						src += "System.runAs(new User(Id=uid)){"
					}
					src += "String q='SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE OwnerId = :uid';List<SObject> a=Database.query(q,AccessLevel.USER_MODE);System.assertEquals(true,a[0].getSObject('UserRecordAccess').get('HasAllAccess'));List<SObject> b=Database.query(q,AccessLevel.USER_MODE);System.assertEquals(true,b[0].getSObject('UserRecordAccess').get('HasAllAccess'));"
					if route == "runas" {
						src += "}"
					}
					prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if route == "anonymous" {
						_, err = machine.Execute(prog)
					} else if route == "istest" {
						_, err = machine.ExecuteInClass(prog, "Road174GroupTest")
					} else {
						_, err = machine.ExecuteInClass(prog, "Road174Context")
					}
					if err != nil {
						t.Fatal(err)
					}
				})
			}

		})
	}
}
