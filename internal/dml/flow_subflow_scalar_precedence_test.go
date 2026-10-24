package dml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowSubflowScalarPrecedenceRecordAndLookupRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"Record", "Lookup"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				body, err := os.ReadFile("../apextest/testdata/conformance/flow_subflow_scalar_precedence/native/api" + api + "/anonymous/org.tsv")
				if err != nil {
					t.Fatal(err)
				}
				var want string
				for _, row := range strings.Split(string(body), "\n") {
					if value, found := strings.CutPrefix(row, "direct/dynamic/"+route+"\t"); found {
						if err := json.Unmarshal([]byte(value), &want); err != nil {
							t.Fatal(err)
						}
					}
				}
				if want != "null" {
					t.Fatalf("missing native null input observation: %q", want)
				}
				org := testOrg()
				org.APIVersion = api + ".0"
				definition := org.Objects["Account"].Definition
				engine := NewEngine(&org)
				parent := newFlowFrame()
				stale := storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("Binding seed")}}
				if route == "Record" {
					parent.records["pending"] = stale
				} else {
					parent.lookupOutputs["pending"] = flowLookupOutput{record: stale, definition: definition}
				}
				// A later assignment of null supersedes the fetched record snapshot.
				parent.scalars["pending"] = storage.NullValue()
				child := storage.FlowRule{Name: "BindingLeaf", Active: true, Variables: []storage.FlowVariable{{Name: "candidate", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true}}}
				err = engine.runSubflow(child, definition, storage.FlowSubflow{
					Name: "CallChild", FlowName: child.Name,
					InputAssignments:  []storage.WorkflowFieldUpdate{{Field: "candidate", SourceField: "Pending"}},
					OutputAssignments: []storage.FlowAssignment{{Target: "returned", SourceField: "candidate"}},
				}, storage.Record{}, definition, parent)
				if err != nil {
					t.Fatal(err)
				}
				if record, exists := parent.records["returned"]; exists {
					t.Fatalf("null input rebound as stale record: %s", record.ID)
				}
				value, exists := parent.scalars["returned"]
				if !exists || value.Kind != storage.ValueNull {
					t.Fatalf("child output=%+v present=%t; native input=%s", value, exists, want)
				}
			})
		}
	}
}
