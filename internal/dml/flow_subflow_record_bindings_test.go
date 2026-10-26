package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The project-level native matrix covers the same copy boundary through Apex
// interviews, invocable actions, after-save flows and before-delete flows.
func TestFlowSubflowRecordBindings(t *testing.T) {
	for _, route := range []string{"record-variable", "lookup-output", "trigger-record"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			definition := org.Objects["Account"].Definition
			record := storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Boundary seed"), "Phone": storage.StringValue("111"), "BillingCity": storage.StringValue("Origin"),
			}}
			frame := newFlowFrame()
			engine := NewEngine(&org)
			source := "pending"
			switch route {
			case "record-variable":
				frame.records[source] = record.Clone()
			case "lookup-output":
				frame.lookupOutputs[source] = flowLookupOutput{record: record.Clone(), definition: definition}
			case "trigger-record":
				source = "$Record"
				seedFlowFrame(&engine, &record, frame)
			}
			before := record.Clone()
			rule := storage.FlowRule{Name: "BoundaryLeaf", Active: true,
				Variables: []storage.FlowVariable{{Name: "candidate", DataType: "SObject", ObjectType: "Account", IsInput: true, IsOutput: true}},
				Steps:     []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "candidate.Phone", Operator: "Assign", LiteralValue: "222"}}},
			}
			err := engine.runSubflow(rule, definition, storage.FlowSubflow{
				Name: "CallChild", FlowName: rule.Name,
				InputAssignments:  []storage.WorkflowFieldUpdate{{Field: "candidate", SourceField: source}},
				OutputAssignments: []storage.FlowAssignment{{Target: "returned", SourceField: "candidate"}},
			}, record, definition, frame)
			if err != nil {
				t.Fatal(err)
			}
			returned, ok := frame.records["returned"]
			want := before.Clone()
			want.Fields["Phone"] = storage.StringValue("222")
			if !ok || !reflect.DeepEqual(returned, want) {
				t.Fatalf("record boundary: present=%t id=%q phone=%q; want id=%q phone=222", ok, returned.ID, returned.Fields["Phone"].String, want.ID)
			}
			if !reflect.DeepEqual(record, before) {
				t.Fatal("child mutated the triggering record")
			}
			if original, ok := frame.records[flowFrameKey(source)]; ok && !reflect.DeepEqual(original, before) {
				t.Fatal("child mutated parent input")
			}
			if output, ok := frame.lookupOutputs[flowFrameKey(source)]; ok && !reflect.DeepEqual(output.record, before) {
				t.Fatal("child mutated lookup output")
			}
			returned.Fields["Phone"] = storage.StringValue("later")
			if !reflect.DeepEqual(record, before) {
				t.Fatal("returned record aliases parent input")
			}
		})
	}
}

func TestFlowSubflowRecordCollectionInputRoute(t *testing.T) {
	org := testOrg()
	definition := org.Objects["Account"].Definition
	engine := NewEngine(&org)
	parent := newFlowFrame()
	parent.collections["pending"] = []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Binding seed")}}}
	before := cloneFlowRecords(parent.collections["pending"])
	child := storage.FlowRule{Name: "BindingCollectionLeaf", Active: true,
		Variables: []storage.FlowVariable{{Name: "candidates", DataType: "SObject", ObjectType: "Account", IsCollection: true, IsInput: true}},
		Steps: []storage.FlowStep{{Kind: "loop", Loop: storage.FlowLoop{CollectionReference: "candidates", CurrentItemReference: "item", Steps: []storage.FlowStep{
			{Kind: "assignment", Assignment: storage.FlowAssignment{Target: "item.Name", LiteralValue: "Binding copy"}},
			{Kind: "recordCreate", RecordCreate: storage.FlowRecordCreate{Name: "CreateCopy", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "item.Name"}}}},
		}}}},
	}
	if err := engine.runSubflow(child, definition, storage.FlowSubflow{Name: "CallChild", FlowName: child.Name, InputAssignments: []storage.WorkflowFieldUpdate{{Field: "candidates", SourceField: "Pending"}}}, storage.Record{}, definition, parent); err != nil {
		t.Fatal(err)
	}
	copies := 0
	for _, record := range org.Objects["Account"].Records {
		if record.Fields["Name"].String == "Binding copy" {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("child persisted %d collection records, want 1", copies)
	}
	if !reflect.DeepEqual(parent.collections["pending"], before) {
		t.Fatal("child mutated parent collection")
	}
}
