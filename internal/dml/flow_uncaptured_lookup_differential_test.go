package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These neighbors intentionally omit the captured metadata provenance. The
// same test is also run against the origin/main source snapshot.
func TestFlowUncapturedLookupNeighborsDifferential(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	storage.EnsureStandardObject(&org, "Pricebook2")
	books := org.Objects["Pricebook2"]
	ids := storage.NewIDGenerator(map[string]string{"Pricebook2": books.Definition.KeyPrefix})
	id, err := ids.Next("Pricebook2")
	if err != nil {
		t.Fatal(err)
	}
	book := storage.Record{ID: id, Object: "Pricebook2", Fields: map[string]storage.Value{"Name": storage.StringValue("Ordinary")}}
	books.SetRecords(map[storage.ID]storage.Record{id: book})
	org.Objects["Pricebook2"] = books
	frame := &flowFrame{capturedAfterCreate: true}
	t.Run("reference_without_captured_metadata", func(t *testing.T) {
		engine := NewEngine(&org)
		if field := reflect.ValueOf(&engine).Elem().FieldByName("FlowLookupRecordVisible"); field.IsValid() {
			field.Set(reflect.ValueOf(func(storage.ID) bool { return false }))
		}
		source := storage.Record{Object: "Opportunity", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(id)}}
		lookup := storage.FlowRecordLookup{ObjectName: "Pricebook2", GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "Pricebook2Id"}}}
		got, _, err := engine.flowRecordLookupRecords(lookup, source, org.Objects["Opportunity"].Definition, frame)
		if err != nil || len(got) != 1 || got[0].ID != id {
			t.Fatalf("unexpected result: %#v, %v", got, err)
		}
	})
	t.Run("two_direct_filters", func(t *testing.T) {
		engine := NewEngine(&org)
		lookup := storage.FlowRecordLookup{ObjectName: "Pricebook2", GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Ordinary"}, {Field: "Name", Operation: "equals", Value: "Ordinary"}}}
		got, _, err := engine.flowRecordLookupRecords(lookup, book, books.Definition, frame)
		if err != nil || len(got) != 1 || got[0].ID != id {
			t.Fatalf("unexpected result: %#v, %v", got, err)
		}
	})
}
