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
	choices := make([]storage.ID, 0, 100)
	for i := 0; i < 100; i++ {
		id, err := ids.Next("Pricebook2")
		if err != nil {
			b.Fatal(err)
		}
		records[id] = storage.Record{ID: id, Object: "Pricebook2"}
		choices = append(choices, id)
	}
	books.SetRecords(records)
	org.Objects["Pricebook2"] = books
	definition := org.Objects["Opportunity"].Definition
	source := storage.Record{Object: "Opportunity", Fields: map[string]storage.Value{"Pricebook2Id": storage.IDValue(choices[0])}}
	lookup := storage.FlowRecordLookup{ObjectName: "Pricebook2", GetFirstRecordOnly: true, StoreOutputAutomatically: true, Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "Pricebook2Id"}}}
	if field := reflect.ValueOf(&lookup).Elem().FieldByName("CapturedTestReferenceLookup"); field.IsValid() {
		field.SetBool(true)
	}
	frame := &flowFrame{capturedAfterCreate: true}
	engine := NewEngine(&org)
	// The baseline has no visibility hook; the candidate hook admits this
	// test-created record. Both iterations must return the identical ID.
	engine.SetFlowLookupRecordVisible(func(storage.ID) bool { return true })
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chosen := choices[i%len(choices)]
		source.Fields["Pricebook2Id"] = storage.IDValue(chosen)
		got, _, err := engine.flowRecordLookupRecords(lookup, source, definition, frame)
		if err != nil || len(got) != 1 || got[0].ID != chosen {
			b.Fatalf("lookup outcome changed: count=%d err=%v", len(got), err)
		}
	}
}
