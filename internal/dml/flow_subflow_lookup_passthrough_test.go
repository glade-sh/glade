package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Selected-field provenance must preserve the existing record input/output
// copy boundary even when the child never assigns a field on the input.
func TestFlowSubflowLookupInputPassThroughOutput(t *testing.T) {
	org := testOrg()
	definition := org.Objects["Account"].Definition
	engine := NewEngine(&org)
	parent := newFlowFrame()
	record := storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("Boundary seed")}}
	parent.lookupOutputs["pending"] = flowLookupOutput{record: record.Clone(), definition: definition}
	child := storage.FlowRule{Name: "BoundaryLeaf", Active: true,
		Variables: []storage.FlowVariable{{Name: "candidate", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true}},
	}
	err := engine.runSubflow(child, definition, storage.FlowSubflow{
		Name: "CallChild", FlowName: child.Name,
		InputAssignments:  []storage.WorkflowFieldUpdate{{Field: "candidate", SourceField: "pending"}},
		OutputAssignments: []storage.FlowAssignment{{Target: "returned", SourceField: "candidate"}},
	}, storage.Record{}, definition, parent)
	if err != nil {
		t.Fatal(err)
	}
	if returned, found := parent.records["returned"]; !found || !reflect.DeepEqual(returned, record) {
		t.Fatalf("unmodified record output: found=%t record=%+v", found, returned)
	}
}
