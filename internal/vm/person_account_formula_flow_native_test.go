package vm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountFormulaNativeFlowInputRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_formula_inputs/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"factory_single", "factory_collection", "constructor_single", "constructor_collection"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var want string
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					row := strings.SplitN(line, "\t", 4)
					if row[0] == api && row[1] == "anonymous" && row[2] == "flow_collection" {
						if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
							t.Fatal(err)
						}
					}
				}
				if want == "" {
					t.Fatal("missing native Flow read")
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				account := org.Objects["Account"]
				account.Definition = account.Definition.Clone()
				account.Definition.Fields["Derived__pc"] = storage.Field{APIName: "Derived__pc", Type: storage.FieldCalculated, Formula: `Note__pc & "-derived"`, DisplayType: "STRING"}
				org.Objects["Account"] = account
				machine := New(nil)
				machine.Org = &org
				record := Object("Account")
				record.Fields["Derived__pc"] = String(want)
				variables := Map()
				value := record
				if strings.HasSuffix(route, "collection") {
					value = List(record)
				}
				key := mapKey(String("Person"))
				variables.Map[key] = value
				variables.MapKeys[key] = String("Person")
				receiver := Object("Flow.Interview")
				inputKey := "variables"
				if strings.HasPrefix(route, "constructor") {
					inputKey = "__arg0"
					receiver.Type = "Flow.Interview.PersonFieldRead"
				}
				receiver.Fields[inputKey] = variables
				input, _, err := machine.flowInterviewInput(receiver)
				if err != nil {
					t.Fatal(err)
				}
				stored := input.Records["Person"]
				if strings.HasSuffix(route, "collection") {
					if len(input.Collections["Person"]) != 1 {
						t.Fatal(input.Collections)
					}
					stored = input.Collections["Person"][0]
				}
				if got := stored.Fields["Derived__pc"].String; got != want {
					t.Fatalf("native=%q; Glade=%q", want, got)
				}
				if record.Fields["Derived__pc"].Text != want {
					t.Fatal("Flow conversion mutated Apex input")
				}
				dmlRecord, err := machine.recordFromValue(&record)
				if err != nil {
					t.Fatal(err)
				}
				if _, exists := dmlRecord.Fields["Derived__pc"]; exists {
					t.Fatal("Flow conversion changed DML projection")
				}
			})
		}
	}
}
