package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Update Records uses a record variable's ID and stored field values, just as
// it does for a record collection. This is documented behavior, not a native
// API/context capture:
// https://help.salesforce.com/s/articleView?id=platform.flow_ref_elements_data_update.htm&language=en_US
func TestFlowRecordUpdateUsesSingleRecordVariable(t *testing.T) {
	for _, shape := range []string{"field-id", "record-id", "collection-control"} {
		t.Run(shape, func(t *testing.T) {
			org := testOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Type: storage.FieldString}
			account.Records["001000000000001"] = storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Keep"), "Description": storage.StringValue("Before"),
			}}
			account.Records["001000000000002"] = storage.Record{Object: "Account", ID: "001000000000002", Fields: map[string]storage.Value{"Name": storage.StringValue("Other")}}
			other := account.Records["001000000000002"].Clone()
			org.Objects["Account"] = account
			frame := newFlowFrame()
			input := storage.Record{Fields: map[string]storage.Value{"Description": storage.NullValue()}}
			if shape == "record-id" {
				input.ID = "001000000000001"
			} else {
				input.Fields["Id"] = storage.IDValue("001000000000001")
			}
			if shape == "collection-control" {
				frame.collections[flowFrameKey("pending")] = []storage.Record{input}
			} else {
				frame.records[flowFrameKey("pending")] = input
			}
			before := input.Clone()
			engine := NewEngine(&org)
			count, err := engine.executeFlowRecordUpdate(storage.FlowRecordUpdate{
				Name: "PersistPending", ObjectName: "Account", InputReference: "pending",
			}, storage.Record{}, storage.ObjectDefinition{}, frame)
			if err != nil || count != 1 {
				t.Fatalf("updated=%d, err=%v; want one record", count, err)
			}
			stored := org.Objects["Account"].Records["001000000000001"]
			if stored.Fields["Name"].String != "Keep" {
				t.Fatalf("stored fields=%#v; want unchanged Name and cleared Description", stored.Fields)
			}
			if value, present := stored.Fields["Description"]; present && value.Kind != storage.ValueNull {
				t.Fatalf("Description was not cleared: %#v", value)
			}
			if _, ok := stored.Fields["Id"]; ok {
				t.Fatal("Id persisted as an ordinary field")
			}
			if got := org.Objects["Account"].Records["001000000000002"]; !reflect.DeepEqual(got, other) {
				t.Fatalf("unselected record changed: %#v", got)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("record update mutated the interview variable")
			}
		})
	}
}

func TestFlowRecordVariableUpdateRejectsMissingIdentity(t *testing.T) {
	for _, resource := range []string{"unknown-control", "single", "collection-control"} {
		t.Run(resource, func(t *testing.T) {
			org := testOrg()
			frame := newFlowFrame()
			input := storage.Record{Fields: map[string]storage.Value{"Name": storage.StringValue("MustNotInsert")}}
			if resource == "single" {
				frame.records[flowFrameKey("pending")] = input
			} else if resource == "collection-control" {
				frame.collections[flowFrameKey("pending")] = []storage.Record{input}
			}
			engine := NewEngine(&org)
			count, err := engine.executeFlowRecordUpdate(storage.FlowRecordUpdate{
				Name: "PersistPending", ObjectName: "Account", InputReference: "pending",
			}, storage.Record{}, storage.ObjectDefinition{}, frame)
			if err == nil || count != 0 || len(org.Objects["Account"].Records) != 0 {
				t.Fatalf("updated=%d, err=%v, records=%v; want error and no insert", count, err, org.Objects["Account"].Records)
			}
		})
	}
}
