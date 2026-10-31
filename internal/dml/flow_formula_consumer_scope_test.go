package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These record-only consumers have no native capture; preserve shipping-base
// behavior even when an unrelated element uses a colliding String variable.
func TestFlowFormulaSeparateConsumerPriorRecordValue(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range []string{"Site", "Name"} {
			t.Run(api+"/"+field, func(t *testing.T) {
				org := testOrg()
				org.APIVersion = api
				account := org.Objects["Account"]
				account.Definition.Fields["Site"] = storage.Field{APIName: "Site", Type: storage.FieldString}
				account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
				account.Definition.FlowRules = []storage.FlowRule{{Name: "Probe", TriggerType: "RecordBeforeSave", RecordTriggerType: "Create"}}
				org.Objects["Account"] = account
				engine := NewEngine(&org)
				record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("record-name"), "Site": storage.StringValue("record-site")}, ExplicitNulls: make(map[string]bool)}
				frame := engine.newFlowStepFrame("Probe", &record, account.Definition, nil, nil, []storage.FlowVariable{{Name: field, DataType: "String"}})
				want := record.Fields[field]
				update := storage.WorkflowFieldUpdate{Name: "Separate", Field: "Description", Formula: field}
				if _, err := engine.applyFlowFieldUpdate("Probe", "Account", &record, account.Definition, update, frame); err != nil {
					t.Fatal(err)
				}
				if got := record.Fields["Description"]; !storage.ValuesEqual(got, want) {
					t.Errorf("assignment = %#v, want %#v", got, want)
				}
				create := storage.FlowRecordCreate{Name: "SeparateCreate", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "created"}, {Field: "Description", Formula: field}}}
				id, err := engine.flowCoreRecordCreateWithFrame(create, record, account.Definition, frame)
				if err != nil {
					t.Fatal(err)
				}
				if got := org.Objects["Account"].Records[id].Fields["Description"]; !storage.ValuesEqual(got, want) {
					t.Errorf("create = %#v, want %#v", got, want)
				}
			})
		}
	}
}
