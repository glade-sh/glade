package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestJunctionControlledParentNativeInsert(t *testing.T) {
	f, err := os.Open("testdata/junction_parent_sharing/insert.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[2]+"/"+row[7], func(t *testing.T) {
			var native struct {
				Success bool `json:"isSuccess"`
				Errors  []struct {
					Status string `json:"getStatusCode"`
				} `json:"getErrors"`
			}
			if err := json.Unmarshal([]byte(row[8]), &native); err != nil {
				t.Fatal(err)
			}
			machine := junctionParentInsertFixture(t, row[2], row[7])
			record := Object("Widget__c")
			record.Fields["Name"] = String("inserted")
			record.Fields["First__c"] = String("a11000000000001")
			record.Fields["Second__c"] = String("a12000000000001")
			got, err := machine.applyDMLWithRecordAccess("insert", record, false, "", dml.Options{}, &Result{}, "USER_MODE")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 {
				t.Fatalf("results=%v", got)
			}
			if got[0].Success != native.Success {
				t.Fatalf("success=%v, native=%v, result=%+v", got[0].Success, native.Success, got[0])
			}
			if !native.Success && got[0].StatusCode != native.Errors[0].Status {
				t.Fatalf("status=%s native=%s", got[0].StatusCode, native.Errors[0].Status)
			}
			expected := "false"
			if native.Success {
				expected = "true"
			}
			source := `Database.SaveResult saved=Database.insert(new Widget__c(Name='route', First__c='a11000000000001', Second__c='a12000000000001'),false,AccessLevel.USER_MODE); System.assertEquals(` + expected + `,saved.isSuccess());`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[2]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatalf("Apex Database.insert route: %v", err)
			}
		})
	}
}

func junctionParentInsertFixture(t *testing.T, api, stage string) *VM {
	t.Helper()
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod = Method{APIVersion: api}
	state := machine.Org.Objects["Widget__c"]
	state.Definition.SharingModel = "ControlledByParent"
	for _, parent := range []struct {
		name, field string
		id          storage.ID
	}{{"FirstParent__c", "First__c", "a11000000000001"}, {"SecondParent__c", "Second__c", "a12000000000001"}} {
		state.Definition.Fields[parent.field] = storage.Field{APIName: parent.field, Type: storage.FieldReference, MasterDetail: true, WriteRequiresMasterRead: true, ReferenceTo: []string{parent.name}}
		machine.Org.Objects[parent.name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: parent.name, SharingModel: "Private", KeyPrefix: string(parent.id[:3])}, Records: map[storage.ID]storage.Record{parent.id: {ID: parent.id, Object: parent.name, System: storage.SystemFields{OwnerID: "005000000000001"}}}}
		if stage == "u3_read_both" {
			share := parent.name[:len(parent.name)-3] + "__Share"
			machine.Org.Objects[share] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: share}, Records: map[storage.ID]storage.Record{"a13000000000001": {ID: "a13000000000001", Object: share, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.id), "UserOrGroupId": storage.IDValue("005000000000002"), "AccessLevel": storage.StringValue("Read")}}}}
		}
	}
	machine.Org.Objects["Widget__c"] = state
	permissions := machine.Org.Objects["FieldPermissions"]
	for i, field := range []string{"First__c", "Second__c"} {
		id := storage.ID([]string{"0FP000000000011", "0FP000000000012"}[i])
		permissions.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Widget__c"), "Field": storage.StringValue("Widget__c." + field), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	machine.Org.Objects["FieldPermissions"] = permissions
	return machine
}
