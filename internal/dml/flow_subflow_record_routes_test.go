package dml

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowSubflowRecordInputDMLAfterSaveRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operation := range []string{"insert", "update"} {
			t.Run(api+"/after-save/"+operation, func(t *testing.T) {
				// Share the native oracle with the project-level route twins.
				path := fmt.Sprintf("../apextest/testdata/conformance/flow_subflow_record_routes/api%s/istest/org.tsv", strings.TrimSuffix(api, ".0"))
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var want struct{ Description string }
				for _, line := range strings.Split(string(body), "\n") {
					if value, ok := strings.CutPrefix(line, "after-flow\t"+operation+"\t"); ok {
						if err := json.Unmarshal([]byte(value), &want); err != nil {
							t.Fatal(err)
						}
					}
				}
				if want.Description == "" {
					t.Fatal("missing native after-save DML row")
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Account")
				storage.EnsureStandardObject(&org, "Contact")
				engine := NewEngine(&org)
				insert := func(record storage.Record) storage.ID {
					t.Helper()
					result := engine.Insert([]storage.Record{record})
					if len(result) != 1 || !result[0].Success {
						t.Fatalf("insert: %+v", result)
					}
					return result[0].ID
				}
				first := insert(storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Cedar first")}})
				second := insert(storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Cedar second")}})
				branch := insert(storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Cedar branch"), "ParentId": storage.IDValue(first)}})
				lookup := func(name, source string) storage.FlowStep {
					return storage.FlowStep{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{
						Name: name, ObjectName: "Account", GetFirstRecordOnly: true, StoreOutputAutomatically: true,
						Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: source}},
					}}
				}
				org.Metadata.Flows = []storage.FlowRule{{
					Name: "CedarChild", Active: true, Variables: []storage.FlowVariable{{Name: "sourceAccount", DataType: "SObject", ObjectType: "Account", IsInput: true}, {Name: "result", DataType: "String", IsOutput: true}},
					Steps: []storage.FlowStep{lookup("GetRoot", "sourceAccount.ParentId"), {Kind: "assignment", Assignment: storage.FlowAssignment{Target: "result", SourceField: "GetRoot.Name"}}},
				}}
				contact := org.Objects["Contact"]
				contact.Definition.FlowRules = []storage.FlowRule{{
					Name: "CedarAfter", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "CreateAndUpdate",
					Criteria: []storage.WorkflowCriteriaItem{
						{Field: "Department", Operation: "equals", Value: "after-flow"},
						{Field: "Description", Operation: "isnull", Value: "true"},
					},
					Variables: []storage.FlowVariable{{Name: "result", DataType: "String"}},
					Steps: []storage.FlowStep{
						lookup("GetAccount", "AccountId"),
						{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "CallChild", FlowName: "CedarChild",
							InputAssignments:  []storage.WorkflowFieldUpdate{{Field: "sourceAccount", SourceField: "GetAccount"}},
							OutputAssignments: []storage.FlowAssignment{{Target: "result", SourceField: "result"}}}},
						{Kind: "recordUpdate", RecordUpdate: storage.FlowRecordUpdate{Name: "SaveResult", ObjectName: "Contact",
							Criteria:         []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "Id"}},
							InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Description", SourceField: "result"}}}},
					},
				}}
				org.Objects["Contact"] = contact
				row := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Cedar contact"), "AccountId": storage.IDValue(branch)}}
				if operation == "insert" {
					row.Fields["Department"] = storage.StringValue("after-flow")
				}
				id := insert(row)
				if operation == "update" {
					result := engine.Update([]storage.Record{{Object: "Account", ID: branch, Fields: map[string]storage.Value{"ParentId": storage.IDValue(second)}}})
					if !result[0].Success {
						t.Fatalf("update parent: %+v", result)
					}
					result = engine.Update([]storage.Record{{Object: "Contact", ID: id, Fields: map[string]storage.Value{"Department": storage.StringValue("after-flow")}}})
					if !result[0].Success {
						t.Fatalf("update through subflow: %+v", result)
					}
				}
				if got := org.Objects["Contact"].Records[id].Fields["Description"].String; got != want.Description {
					t.Fatalf("native description=%q; DML description=%q", want.Description, got)
				}
			})
		}
	}
}

func TestFlowSubflowBeforeSaveRouteRejectsSubflow(t *testing.T) {
	for _, operation := range []string{"insert", "update"} {
		t.Run(operation, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Contact")
			contact := org.Objects["Contact"]
			contact.Definition.FlowRules = []storage.FlowRule{{Name: "CedarBeforeSubflow", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate", Steps: []storage.FlowStep{{Kind: "subflow", Subflow: storage.FlowSubflow{Name: "CallChild", FlowName: "CedarChild"}}}}}
			org.Objects["Contact"] = contact
			row := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Cedar")}}
			if operation == "update" {
				row.ID = "003000000000001"
			}
			engine := NewEngine(&org)
			if err := engine.ApplyBeforeSaveFlows([]storage.Record{row}); err == nil || !strings.Contains(err.Error(), "unsupported side-effect steps") {
				t.Fatalf("native rejects before-save Subflow; local error=%v", err)
			}
		})
	}
}
