package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func BenchmarkFlowTriggeredReferenceLookup(b *testing.B) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	storage.EnsureStandardObject(&org, "Pricebook2")
	books := org.Objects["Pricebook2"]
	ids := storage.NewIDGenerator(map[string]string{"Pricebook2": books.Definition.KeyPrefix})
	records := make(map[storage.ID]storage.Record, 100)
	var chosen storage.ID
	for i := 0; i < 100; i++ {
		id, err := ids.Next("Pricebook2")
		if err != nil {
			b.Fatal(err)
		}
		records[id] = storage.Record{ID: id, Object: "Pricebook2"}
		chosen = id
	}
	books.SetRecords(records)
	org.Objects["Pricebook2"] = books
	definition := org.Objects["Opportunity"].Definition
	source := storage.Record{Object: "Opportunity", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(chosen)}}
	lookup := storage.FlowRecordLookup{ObjectName: "Pricebook2", GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "Pricebook2Id"}}}
	frame := &flowFrame{capturedAfterCreate: true}
	engine := NewEngine(&org)
	// The baseline has no visibility hook; the candidate hook admits this
	// test-created record. Both iterations must return the identical ID.
	if field := reflect.ValueOf(&engine).Elem().FieldByName("FlowLookupRecordVisible"); field.IsValid() {
		field.Set(reflect.ValueOf(func(storage.ID) bool { return true }))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, _, err := engine.flowRecordLookupRecords(lookup, source, definition, frame)
		if err != nil || len(got) != 1 || got[0].ID != chosen {
			b.Fatalf("lookup outcome changed: count=%d err=%v", len(got), err)
		}
	}
}
