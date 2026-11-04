package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Compare each profile and API against the captured unmatched-upsert rows.
func TestCase349ProfileBoundaryNativeRows(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/case349_profile_boundary/org.tsv")
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
	count := 0
	for _, row := range rows[1:] {
		if len(row) != 6 || (row[4] != "omitted" && row[4] != "person" && row[4] != "business" && row[4] != "invalid" && row[4] != "last" && row[4] != "title") {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1]+"/"+row[2]+"/"+row[4], func(t *testing.T) {
			var native struct {
				Status  string `json:"status"`
				Created bool   `json:"created"`
				Success bool   `json:"success"`
			}
			if err := json.Unmarshal([]byte(row[5]), &native); err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.APIVersion = row[0]
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000002", DeveloperName: "Individual", Active: true, Available: true, Default: true, PersonAccount: true}}
			org.Objects["Account"] = account
			if row[2] == "person" {
				org.Metadata.AccountRecordTypeDefaults = []storage.AccountRecordTypeDefault{{ProfileID: storage.DefaultProfileID, DeveloperName: "Individual", PersonAccount: true}}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("new"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic")}}
			switch row[4] {
			case "person":
				record.Fields["RecordTypeId"] = storage.IDValue("012000000000002")
			case "business":
				record.Fields["RecordTypeId"] = storage.IDValue("012000000000001")
			case "invalid":
				record.Fields["RecordTypeId"] = storage.IDValue("012000000000099")
			case "last":
				record.Fields["LastName"] = storage.StringValue("Person")
			case "title":
				record.Fields["PersonTitle"] = storage.StringValue("Synthetic")
			}
			got := machine.preflightBusinessAccountNameFailures([]storage.Record{record}, "upsert", "ProbeKey__c")
			if len(got) != 1 || got[0].StatusCode != native.Status || got[0].Success != native.Success || got[0].UpsertCreated != native.Created {
				t.Fatalf("native=%+v local=%+v", native, got)
			}
		})
	}
	if count != 36 {
		t.Fatalf("native rows=%d, want 36", count)
	}
}
