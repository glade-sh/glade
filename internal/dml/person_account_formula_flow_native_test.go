package dml

import (
	"encoding/json"
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"strings"
	"testing"
)

func TestPersonAccountFormulaNativeFlowLookupRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_formula_inputs/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"single", "collection"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var want string
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					row := strings.SplitN(line, "\t", 4)
					if row[0] == api && row[1] == "anonymous" && row[2] == "flow_lookup_collection" {
						if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
							t.Fatal(err)
						}
					}
				}
				if want == "" {
					t.Fatal("missing native Flow lookup")
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				account := org.Objects["Account"]
				account.Definition = account.Definition.Clone()
				account.Definition.Fields["Note__pc"] = storage.Field{APIName: "Note__pc", Type: storage.FieldString}
				account.Definition.Fields["Derived__pc"] = storage.Field{APIName: "Derived__pc", Type: storage.FieldCalculated, Formula: `Note__pc & "-derived"`, DisplayType: "STRING"}
				id := storage.ID("001000000000001")
				account.Records = map[storage.ID]storage.Record{id: {Object: "Account", ID: id, Fields: map[string]storage.Value{"Note__pc": storage.StringValue("seed")}}}
				org.Objects["Account"] = account
				engine := NewEngine(&org)
				frame := newFlowFrame()
				lookup := storage.FlowRecordLookup{Name: "Fetched", ObjectName: "Account", StoreOutputAutomatically: true, GetFirstRecordOnly: route == "single"}
				if err := engine.applyFlowRecordLookupStep(lookup, storage.Record{}, storage.ObjectDefinition{}, frame); err != nil {
					t.Fatal(err)
				}
				record := frame.lookupOutputs[flowFrameKey("Fetched")].record
				if route == "collection" {
					records := frame.lookupCollections[flowFrameKey("Fetched")].records
					if len(records) != 1 {
						t.Fatal(records)
					}
					record = records[0]
				}
				if got := record.Fields["Derived__pc"].String; got != want {
					t.Fatalf("native=%q; Glade=%q", want, got)
				}
				if record.Fields["Note__pc"].String != "seed" {
					t.Fatal("Get Records changed formula input")
				}
				if _, exists := org.Objects["Account"].Records[id].Fields["Derived__pc"]; exists {
					t.Fatal("lookup mutated stored formula value")
				}
			})
		}
	}
}
