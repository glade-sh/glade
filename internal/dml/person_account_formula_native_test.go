package dml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountFormulaNativeBusinessUpdateRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_formulas/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		var business, related []string
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			row := strings.SplitN(line, "\t", 4)
			if row[0] != api || row[1] != "anonymous" {
				continue
			}
			switch row[2] {
			case "business":
				if err := json.Unmarshal([]byte(row[3]), &business); err != nil {
					t.Fatal(err)
				}
			case "related_update":
				if err := json.Unmarshal([]byte(row[3]), &related); err != nil {
					t.Fatal(err)
				}
			}
		}
		if len(business) != 3 || len(related) != 2 || !strings.Contains(related[0], "New Sponsor") {
			t.Fatal("missing native Business Account update observation")
		}
		for _, route := range []string{"update", "upsert_update"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				contact := org.Objects["Contact"]
				contact.Definition = contact.Definition.Clone()
				contact.Definition.Fields["Note__c"] = storage.Field{APIName: "Note__c", Type: storage.FieldString}
				contact.Definition.Fields["Derived__c"] = storage.Field{APIName: "Derived__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: `Note__c & "-derived"`, Updateable: storage.BoolFlag(false), Createable: storage.BoolFlag(false)}
				org.Objects["Contact"] = contact
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				engine := NewEngine(&org)
				inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Garden Sponsor")}}})
				if len(inserted) != 1 || !inserted[0].Success {
					t.Fatalf("insert: %+v", inserted)
				}
				// Trigger/update preparation can carry the native calculated read value.
				// It must be removed before checking for caller-supplied person fields.
				record := storage.Record{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("New Sponsor"), "Derived__pc": storage.StringValue(business[0])}}
				var results []Result
				if route == "update" {
					results = engine.Update([]storage.Record{record})
				} else {
					results = engine.Upsert([]storage.Record{record})
				}
				if len(results) != 1 || !results[0].Success {
					t.Fatalf("native related_update accepts Business Account rename; Glade=%+v", results)
				}
				if got := org.Objects["Account"].Records[inserted[0].ID].Fields["Name"].String; got != "New Sponsor" {
					t.Fatalf("rename=%q", got)
				}
			})
		}
	}
}
