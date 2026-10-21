package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A subflow input variable fed by a parent Get Records output must reach the
// subflow as the record itself, so a Get Records filtered on {!input.Lookup__c}
// resolves. Before the fix the input arrived as an empty scalar and the lookup
// failed with "unsupported criteria for Account.Id".
func TestFlowSubflowRecordInputFromParentLookupOutput(t *testing.T) {
	org := testOrg()
	org.Objects["Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Thing__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Type: storage.FieldString},
				"Acct__c": {APIName: "Acct__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	accounts := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}}})
	if !accounts[0].Success {
		t.Fatalf("account insert = %#v", accounts)
	}
	thing := storage.Record{Object: "Thing__c", ID: "a01000000000001", Fields: map[string]storage.Value{
		"Name":    storage.StringValue("t"),
		"Acct__c": storage.IDValue(accounts[0].ID),
	}}
	parent := newFlowFrame()
	parent.lookupOutputs["getthing"] = flowLookupOutput{record: thing, definition: org.Objects["Thing__c"].Definition}

	child := storage.FlowRule{
		Name: "Child", Active: true,
		Steps: []storage.FlowStep{
			{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{
				Name: "GetAcct", ObjectName: "Account", GetFirstRecordOnly: true, StoreOutputAutomatically: true,
				Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "thingVar.Acct__c"}},
			}},
			{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{
				Name: "Copy", ObjectName: "Account",
				InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "GetAcct.Name"}},
			}},
		},
	}
	err := engine.runSubflow(child, storage.ObjectDefinition{}, storage.FlowSubflow{
		Name: "CallChild", FlowName: "Child",
		InputAssignments: []storage.WorkflowFieldUpdate{{Name: "thingVar", Field: "thingVar", SourceField: "GetThing"}},
	}, storage.Record{}, storage.ObjectDefinition{}, parent)
	if err != nil {
		t.Fatalf("runSubflow = %v", err)
	}
	copies := 0
	for _, record := range org.Objects["Account"].Records {
		if record.Fields["Name"].String == "Parent" {
			copies++
		}
	}
	if copies != 2 {
		t.Fatalf("accounts named Parent = %d, want 2 (original plus the subflow copy)", copies)
	}
}
