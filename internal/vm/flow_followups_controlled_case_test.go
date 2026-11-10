package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows cover an explicit public Case master and its controlled child,
// including current-user with-sharing and USER_MODE queries at APIs 62/67.
func TestPublicCaseControlledDetailQueryRoutes(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/flow_followups_case_controlled_detail/org.tsv")
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
			var want map[string]int
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			for _, route := range []struct {
				key, object, actor, sharing, suffix string
			}{
				{"ownerParent", "Case", "005000000000001", "with sharing", ""},
				{"ownerChild", "Widget__c", "005000000000001", "with sharing", ""},
				{"otherParent", "Case", "005000000000002", "with sharing", ""},
				{"otherChild", "Widget__c", "005000000000002", "with sharing", ""},
				{"otherParentUser", "Case", "005000000000002", "without sharing", " WITH USER_MODE"},
				{"otherChildUser", "Widget__c", "005000000000002", "without sharing", " WITH USER_MODE"},
			} {
				t.Run(route.key, func(t *testing.T) {
					machine := recordSharingControlMachine(t, "ControlledByParent")
					machine.testContext.CurrentUser.Fields["Id"] = String(route.actor)
					machine.testContext.CurrentUser.Fields["UserType"] = String("Standard")
					storage.EnsureStandardObject(machine.Org, "Case")
					parentID := storage.ID("500000000000001")
					cases := machine.Org.Objects["Case"]
					cases.Definition.SharingModel = "ReadWriteTransfer"
					cases.Records = map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "Case", System: storage.SystemFields{OwnerID: "005000000000001"}}}
					machine.Org.Objects["Case"] = cases
					details := machine.Org.Objects["Widget__c"]
					details.Definition.Fields["Case__c"] = storage.Field{APIName: "Case__c", Type: storage.FieldReference, MasterDetail: true, ReferenceTo: []string{"Case"}}
					id := storage.ID("a00000000000001")
					details.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Widget__c", Fields: map[string]storage.Value{"Case__c": storage.IDValue(parentID)}, System: storage.SystemFields{OwnerID: "005000000000001"}}}
					machine.Org.Objects["Widget__c"] = details
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
					machine.SetOrg(machine.Org)
					body, err := CompileAnonymousWithOptions("return [SELECT COUNT() FROM "+route.object+route.suffix+"];", CompileOptions{APIVersion: row[1]})
					if err != nil {
						t.Fatal(err)
					}
					class := Class{Name: "SyntheticDetailReader", APIVersion: row[1], Modifiers: []string{route.sharing}}
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
					method := Method{Name: class.Name + ".read", ClassName: class.Name, IsStatic: true, Access: "public", APIVersion: row[1], SourceContextBound: true, SharingMode: route.sharing, Program: body}
					if err := machine.RegisterMethod(method); err != nil {
						t.Fatal(err)
					}
					value, err := machine.callMethod(machine.Methods[method.Name], nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					if value.Kind != ValueInt || value.Int != int64(want[route.key]) {
						t.Fatalf("visible count=%v, native=%d", value, want[route.key])
					}
				})
			}
		})
	}
}
