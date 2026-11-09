package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestJunctionControlledParentNativeUpdate(t *testing.T) {
	f, err := os.Open("testdata/junction_parent_sharing/update.tsv")
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
		var native struct {
			Operation string `json:"operation"`
			Success   bool   `json:"isSuccess"`
			Errors    []struct {
				Status string `json:"getStatusCode"`
			} `json:"getErrors"`
		}
		if err := json.Unmarshal([]byte(row[8]), &native); err != nil {
			t.Fatal(err)
		}
		t.Run(row[2]+"/"+row[7]+"/"+native.Operation, func(t *testing.T) {
			machine := junctionParentInsertFixture(t, row[2], row[7])
			state := machine.Org.Objects["Widget__c"]
			stored := state.Records["a00000000000001"]
			stored.Fields["First__c"] = storage.IDValue("a11000000000001")
			stored.Fields["Second__c"] = storage.IDValue("a12000000000001")
			state.Records[stored.ID] = stored
			for _, field := range []string{"First__c", "Second__c"} {
				f := state.Definition.Fields[field]
				f.ReparentableMasterDetail = true
				state.Definition.Fields[field] = f
			}
			machine.Org.Objects["Widget__c"] = state
			for _, parent := range []struct {
				name                string
				original, alternate storage.ID
			}{{"FirstParent__c", "a11000000000001", "a11000000000002"}, {"SecondParent__c", "a12000000000001", "a12000000000002"}} {
				st := machine.Org.Objects[parent.name]
				p := st.Records[parent.original]
				p.ID = parent.alternate
				st.Records[p.ID] = p
				machine.Org.Objects[parent.name] = st
				if row[7] == "u3_read_both" {
					name := parent.name[:len(parent.name)-3] + "__Share"
					shares := machine.Org.Objects[name]
					share := shares.Records["a13000000000001"]
					share.ID = "a13000000000002"
					share.Fields = map[string]storage.Value{"ParentId": storage.IDValue(parent.alternate), "UserOrGroupId": storage.IDValue("005000000000002"), "AccessLevel": storage.StringValue("Read")}
					shares.Records[share.ID] = share
					machine.Org.Objects[name] = shares
				}
			}
			record := Object("Widget__c")
			record.Fields["Id"] = String(string(stored.ID))
			switch native.Operation {
			case "scalar":
				record.Fields["Amount__c"] = Int(20)
			case "reparent_p1":
				record.Fields["First__c"] = String("a11000000000002")
			case "reparent_p2":
				record.Fields["Second__c"] = String("a12000000000002")
			case "reparent_both":
				record.Fields["First__c"] = String("a11000000000002")
				record.Fields["Second__c"] = String("a12000000000002")
			}
			got, err := machine.applyDMLWithRecordAccess("update", record, false, "", dml.Options{}, &Result{}, "USER_MODE")
			if err != nil {
				t.Fatalf("native SaveResult, got exception: %v", err)
			}
			if len(got) != 1 || got[0].Success != native.Success {
				t.Fatalf("results=%+v native success=%v", got, native.Success)
			}
			if !native.Success && got[0].StatusCode != native.Errors[0].Status {
				t.Fatalf("status=%s native=%s", got[0].StatusCode, native.Errors[0].Status)
			}
		})
	}
}
