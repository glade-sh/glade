package storage

import "testing"

// A record type id held in 15 characters must match the 18-character id on the
// record: standard objects keep the short form in their definition. The
// native defaults are in apextest/testdata/conformance/standard_recordtype_picklist_default.
func TestDefaultPicklistValueForRecordTypeMatchesShortAndLongIDs(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", RecordTypes: []RecordTypeInfo{
		{ID: "012000000000001", DeveloperName: "RT", PicklistDefaults: map[string]string{"Industry": "Banking"}},
	}}
	field := Field{APIName: "Industry", Type: FieldPicklist}
	for _, id := range []ID{"012000000000001", "012000000000001AAA"} {
		record := Record{Object: "Account", Fields: map[string]Value{"RecordTypeId": IDValue(id)}}
		got, ok := DefaultValueForRecordField(definition, record, field)
		if !ok || got.String != "Banking" {
			t.Fatalf("record type id %s: default = %#v, ok=%v; want Banking", id, got, ok)
		}
	}
	other := Record{Object: "Account", Fields: map[string]Value{"RecordTypeId": IDValue("012000000000002AAA")}}
	if got, ok := DefaultValueForRecordField(definition, other, field); ok {
		t.Fatalf("another record type defaulted: %#v", got)
	}
}
