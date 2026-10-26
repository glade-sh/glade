package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowChainLookupNullAndSubflowOutputFieldRoutes(t *testing.T) {
	for _, route := range []string{"before_save_lookup_missing", "after_save_lookup_missing", "before_save_subflow_output", "after_save_subflow_output"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			trigger := "RecordBeforeSave"
			if strings.HasPrefix(route, "after_save") {
				trigger = "RecordAfterSave"
			}
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Parent", TriggerType: trigger, RecordTriggerType: "Create"}}
			org.Objects["Account"] = object
			engine := NewEngine(&org)
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}, ExplicitNulls: map[string]bool{}}
			frame := newFlowFrame()
			frame.saveDecisions, frame.priorDecisions, frame.beforeSaveDecisions = flowSaveDecisionMode("Parent", object.Definition)
			flowCoreSeedScalarDefaults([]storage.FlowVariable{{Name: "Message", DataType: "String"}}, frame)
			seedFlowFrame(&engine, &record, frame)
			want := storage.NullValue()
			if strings.HasSuffix(route, "lookup_missing") {
				err := engine.applyFlowRecordLookupStep(storage.FlowRecordLookup{Name: "Find", ObjectName: "Account", GetFirstRecordOnly: true, OutputAssignments: []storage.FlowAssignment{{Target: "Message", SourceField: "Name"}}}, record, object.Definition, frame)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				child := storage.FlowRule{Name: "Child", Variables: []storage.FlowVariable{{Name: "Result", DataType: "String"}}, Steps: []storage.FlowStep{{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "Set", Target: "Result", Operator: "Assign", LiteralValue: "constant & text"}}}}
				err := engine.runSubflow(child, object.Definition, storage.FlowSubflow{Name: "Call", OutputAssignments: []storage.FlowAssignment{{Target: "Message", SourceField: "Result"}}}, record, object.Definition, frame)
				if err != nil {
					t.Fatal(err)
				}
				want = storage.StringValue("constant & text")
			}
			_, err := engine.applyFlowFieldUpdate("Parent", "Account", &record, object.Definition, storage.WorkflowFieldUpdate{Name: "Write", Field: "Description", SourceField: "Message"}, frame)
			if err != nil {
				t.Fatal(err)
			}
			if want.Kind == storage.ValueNull {
				if !record.HasExplicitNull("Description") {
					t.Fatalf("native null not written: %+v", record)
				}
			} else if got := record.Fields["Description"]; !storage.ValuesEqual(got, want) {
				t.Fatalf("got=%#v native=%#v", got, want)
			}
		})
	}
}

func TestFlowChainOwnerIDStringCollectionDirectAndSaveRoutes(t *testing.T) {
	for _, route := range []string{"direct_interview", "trigger_subflow", "before_save", "after_save"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			engine.InvocableFlow = route == "direct_interview"
			frame := newFlowFrame()
			frame.saveDecisions = route == "before_save" || route == "after_save"
			frame.beforeSaveDecisions = route == "before_save"
			flowCoreSeedScalarDefaults([]storage.FlowVariable{{Name: "owners", DataType: "String", IsCollection: true}}, frame)
			source := storage.Record{Object: "Account", System: storage.SystemFields{OwnerID: "005000000000001"}}
			seedFlowFrame(&engine, &source, frame)
			err := engine.applyFlowAssignmentStep(storage.FlowAssignment{Name: "CollectOwner", Target: "owners", Operator: "Add", SourceField: "$Record.OwnerId"}, frame)
			if err != nil {
				t.Fatal(err)
			}
			if got := frame.scalars["owners"]; got.Kind != storage.ValueList || len(got.List) != 1 {
				t.Fatalf("owner not added: %+v", got)
			}
		})
	}
}
