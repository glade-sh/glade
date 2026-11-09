package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestJunctionControlledParentNativeAccess(t *testing.T) {
	f, err := os.Open("testdata/junction_parent_sharing/org.tsv")
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
				FullRow struct {
					Read bool `json:"HasReadAccess"`
					Edit bool `json:"HasEditAccess"`
				} `json:"fullRow"`
			}
			if err := json.Unmarshal([]byte(row[8]), &native); err != nil {
				t.Fatal(err)
			}
			machine := recordSharingControlMachine(t, "Private")
			machine.currentMethod = Method{APIVersion: row[2]}
			state := machine.Org.Objects["Widget__c"]
			state.Definition.SharingModel = "ControlledByParent"
			var first, second storage.Field
			if err := json.Unmarshal([]byte(`{"apiName":"First__c","type":"REFERENCE","masterDetail":true,"referenceTo":["FirstParent__c"],"writeRequiresMasterRead":true}`), &first); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`{"apiName":"Second__c","type":"REFERENCE","masterDetail":true,"referenceTo":["SecondParent__c"],"writeRequiresMasterRead":true}`), &second); err != nil {
				t.Fatal(err)
			}
			state.Definition.Fields["First__c"] = first
			state.Definition.Fields["Second__c"] = second
			record := state.Records["a00000000000001"]
			record.Fields["First__c"] = storage.IDValue("a11000000000001")
			record.Fields["Second__c"] = storage.IDValue("a12000000000001")
			state.Records[record.ID] = record
			machine.Org.Objects["Widget__c"] = state
			for _, parent := range []struct {
				name string
				id   storage.ID
			}{{"FirstParent__c", "a11000000000001"}, {"SecondParent__c", "a12000000000001"}} {
				machine.Org.Objects[parent.name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: parent.name, SharingModel: "Private"}, Records: map[storage.ID]storage.Record{parent.id: {ID: parent.id, Object: parent.name, System: storage.SystemFields{OwnerID: "005000000000001"}}}}
				if row[7] == "u3_read_both" {
					share := parent.name[:len(parent.name)-3] + "__Share"
					machine.Org.Objects[share] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: share}, Records: map[storage.ID]storage.Record{"a13000000000001": {ID: "a13000000000001", Object: share, Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.id), "UserOrGroupId": storage.IDValue("005000000000002"), "AccessLevel": storage.StringValue("Read")}}}}
				}
			}
			user := machine.currentUserID()
			if got := machine.userModeRecordVisible("Widget__c", record, user); got != native.FullRow.Read {
				t.Errorf("read=%v native=%v", got, native.FullRow.Read)
			}
			if got := machine.currentUserCanWriteRecord("Widget__c", record, user, "update"); got != native.FullRow.Edit {
				t.Errorf("edit=%v native=%v", got, native.FullRow.Edit)
			}
		})
	}
}
