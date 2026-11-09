package vm

import (
	"encoding/csv"
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"strings"
	"testing"
)

// Compare the FormulaEval and FeatureManagement fields from the native
// supplemental context rows, including both API versions and assignment states.
func TestFormulaInstanceNativePermissionSet(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/native_formula_permission_regex/supplemental/contexts/org.tsv")
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
	if len(rows) != 5 {
		t.Fatalf("native row count %d", len(rows))
	}
	const uid storage.ID = "005000000000001"
	const set storage.ID = "0PS000000000001"
	const cp storage.ID = "0CP000000000001"
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			assigned := strings.HasSuffix(row[1], ":assigned")
			org := storage.NewOrgState()
			org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: map[string]storage.Value{}}}}
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{set: {ID: set, Fields: map[string]storage.Value{}}}}
			org.Objects["CustomPermission"] = storage.ObjectState{Records: map[storage.ID]storage.Record{cp: {ID: cp, Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Probe_Bypass")}}}}
			org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0SE000000000001": {Fields: map[string]storage.Value{"ParentId": storage.IDValue(set), "SetupEntityId": storage.IDValue(cp)}}}}
			assignments := map[storage.ID]storage.Record{}
			if assigned {
				assignments["0Pa000000000001"] = storage.Record{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(set)}}
			}
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: assignments}
			org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{}}
			machine := New(nil)
			machine.Org = &org
			machine.executionUser = Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String(string(uid))}}
			apex := machine.currentUserHasPermission("Probe_Bypass")
			instance := Object("FormulaEval.FormulaInstance")
			instance.Fields["returnType"] = String("BOOLEAN")
			got, ok, err := machine.evaluateFormulaInstanceValue(instance, Object("Probe__c"), "$Permission.Probe_Bypass")
			if err != nil || !ok || got.Kind != ValueBool || got.Bool != apex || apex != assigned {
				t.Fatalf("native=%s apex=%v formula=%+v ok=%v err=%v", row[2], apex, got, ok, err)
			}
		})
	}
}
