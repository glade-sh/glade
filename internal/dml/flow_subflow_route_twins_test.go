package dml

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowSubflowCollectionInputRoutes(t *testing.T) {
	for _, route := range []string{"lookup", "variable"} {
		for _, populated := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/populated=%t", route, populated), func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				definition := org.Objects["Account"].Definition
				parent := newFlowFrame()
				var records []storage.Record
				if populated {
					records = []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Collection member")}}}
				}
				if route == "lookup" {
					parent.lookupCollections["parentrecords"] = flowRecordCollection{records: records, definition: definition}
				} else {
					parent.collections["parentrecords"] = records
				}
				child := storage.FlowRule{Name: "CollectionChild", Variables: []storage.FlowVariable{{Name: "things", DataType: "SObject", ObjectType: "Account", IsInput: true, IsCollection: true}}, Steps: []storage.FlowStep{{Kind: "loop", Loop: storage.FlowLoop{
					Name: "EachThing", CollectionReference: "things", Steps: []storage.FlowStep{{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{
						Name: "Copy", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "EachThing.Name"}},
					}}},
				}}}}
				err := engine.runSubflow(child, definition, storage.FlowSubflow{Name: "CallCollectionChild", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "things", SourceField: "parentRecords"}}}, storage.Record{}, definition, parent)
				if err != nil {
					t.Fatalf("runSubflow = %v", err)
				}
				if got := len(org.Objects["Account"].Records); got != len(records) {
					t.Fatalf("child copies = %d; want %d", got, len(records))
				}
			})
		}
	}
}

func TestFlowSubflowRecordInputKeepsUnsupportedLookupOperator(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	definition := org.Objects["Account"].Definition
	frame := newFlowFrame()
	frame.lookupOutputs["parentrecord"] = flowLookupOutput{
		record:     storage.Record{Object: "Account", ID: "001000000000001AAA", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}},
		definition: definition,
	}
	object := org.Objects["Account"]
	object.Records = map[storage.ID]storage.Record{"001000000000001AAA": frame.lookupOutputs["parentrecord"].record}
	org.Objects["Account"] = object
	rule := storage.FlowRule{Name: "LookupChild", Variables: []storage.FlowVariable{{Name: "inputRecord", DataType: "SObject", ObjectType: "Account", IsInput: true}}, Steps: []storage.FlowStep{{Kind: "recordLookup", RecordLookup: storage.FlowRecordLookup{
		Name: "FindAccount", ObjectName: "Account", GetFirstRecordOnly: true, StoreOutputAutomatically: true,
		Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "unsupported", SourceField: "inputRecord.Name"}},
	}}}}
	err := engine.runSubflow(rule, definition, storage.FlowSubflow{Name: "CallChild", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "inputRecord", SourceField: "parentRecord"}}}, storage.Record{}, definition, frame)
	if err == nil || !strings.Contains(err.Error(), "unsupported criteria for Account.Name") {
		t.Fatalf("unsupported operator must still fail: %v", err)
	}
}
