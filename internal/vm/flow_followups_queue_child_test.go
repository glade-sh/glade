package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native child subquery uses USER_MODE and filters Private child records
// for a Standard direct queue member at source APIs 62 and 67.
func TestPrivateQueueMemberChildSubquery(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/flow_followups_custom_queue_child_repeat/org.tsv")
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
	if len(rows) != 3 {
		t.Fatalf("native rows=%d, want 2", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1], func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			machine := recordSharingControlMachine(t, "Private")
			queueDerivedRouteMachine(t, machine)
			storage.EnsureStandardObject(machine.Org, "Case")
			caseID := storage.ID("500000000000001")
			cases := machine.Org.Objects["Case"]
			cases.Definition.SharingModel = "ReadWriteTransfer"
			cases.Records = map[storage.ID]storage.Record{caseID: {ID: caseID, Object: "Case", System: storage.SystemFields{OwnerID: "005000000000001"}}}
			machine.Org.Objects["Case"] = cases
			objects := machine.Org.Objects["Widget__c"]
			objects.Definition.Fields["CaseLink__c"] = storage.Field{APIName: "CaseLink__c", Type: storage.FieldReference, ReferenceTo: []string{"Case"}, RelationshipName: "CaseLink__r", ChildRelationshipName: "SyntheticWorks__r"}
			objects.Definition.Relations = []storage.Relationship{{Field: "CaseLink__c", ParentObjects: []string{"Case"}, ParentRelationship: "CaseLink__r", ChildRelationship: "SyntheticWorks__r"}}
			ids := make([]string, 0, len(objects.Records))
			for id := range objects.Records {
				ids = append(ids, string(id))
			}
			sort.Strings(ids)
			if len(ids) != 2 {
				t.Fatalf("fixture records=%d, want 2", len(ids))
			}
			for i, id := range ids {
				record := objects.Records[storage.ID(id)]
				record.System.OwnerID = "005000000000001"
				if i == 0 {
					record.System.OwnerID = "00G000000000001"
				}
				record.Fields["CaseLink__c"] = storage.IDValue(caseID)
				objects.Records[storage.ID(id)] = record
			}
			machine.Org.Objects["Widget__c"] = objects
			permissions := machine.Org.Objects["ObjectPermissions"]
			for id, permission := range permissions.Records {
				casePermission := permission.Clone()
				casePermission.ID = "0OP000000000099"
				casePermission.Fields["SObjectType"] = storage.StringValue("Case")
				permissions.Records[casePermission.ID] = casePermission
				permissions.Records[id] = permission
				break
			}
			machine.Org.Objects["ObjectPermissions"] = permissions
			fields := machine.Org.Objects["FieldPermissions"]
			fields.Records["0FP000000000004"] = storage.Record{ID: "0FP000000000004", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Widget__c"), "Field": storage.StringValue("Widget__c.CaseLink__c"), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
			machine.Org.Objects["FieldPermissions"] = fields
			machine.SetOrg(machine.Org)
			source := "List<Case> rows=[SELECT (SELECT Id FROM SyntheticWorks__r) FROM Case WHERE Id='500000000000001' WITH USER_MODE]; return rows[0].SyntheticWorks__r.size();"
			body, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			for i, sharing := range []string{"with sharing", "without sharing"} {
				key := []string{"childRowsUser", "childRowsUserWithout"}[i]
				expected, ok := want[key].(float64)
				if !ok {
					t.Fatalf("native child observation %s missing: %v", key, want)
				}
				name := []string{"SyntheticChildReaderWith", "SyntheticChildReaderWithout"}[i]
				class := Class{Name: name, APIVersion: row[1], Modifiers: []string{sharing}}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				method := Method{Name: class.Name + ".read", ClassName: class.Name, IsStatic: true, Access: "public", APIVersion: row[1], SourceContextBound: true, SharingMode: sharing, Program: body}
				if err := machine.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
				value, err := machine.callMethod(machine.Methods[method.Name], nil, &Result{})
				if err != nil {
					t.Fatalf("%s: %v", sharing, err)
				}
				if value.Kind != ValueInt || float64(value.Int) != expected {
					t.Fatalf("%s visible child count=%v, native=%v", sharing, value, want[key])
				}
			}
		})
	}
}
