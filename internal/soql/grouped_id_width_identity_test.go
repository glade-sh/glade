package soql

import (
	"fmt"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestGroupedReferenceIDWidthIdentity(t *testing.T) {
	const short = "001000000000001"
	const long = "001000000000001AAA"
	id := func(text string) storage.Value { return storage.IDValue(storage.ID(text)) }
	text := storage.StringValue
	for _, tc := range []struct {
		name       string
		fieldType  storage.FieldType
		values     []storage.Value
		wantCounts []int64
		wantNull   bool
	}{
		{"reference-long-first", storage.FieldReference, []storage.Value{id(long), id(long), id(short)}, []int64{3}, false},
		{"reference-short-first", storage.FieldReference, []storage.Value{id(short), id(long), id(long)}, []int64{3}, false},
		{"legacy-reference-long-first", storage.FieldReference, []storage.Value{text(long), text(long), text(short)}, []int64{3}, false},
		{"legacy-reference-short-first", storage.FieldReference, []storage.Value{text(short), text(long), text(long)}, []int64{3}, false},
		{"text-width-control", storage.FieldString, []storage.Value{text(long), text(long), text(short)}, []int64{1, 2}, false},
		{"case-distinct-reference-control", storage.FieldReference, []storage.Value{id("00100000000000A"), id("00100000000000aAAA")}, []int64{1, 1}, false},
		{"null-reference-control", storage.FieldReference, []storage.Value{storage.NullValue(), id(short), storage.NullValue()}, []int64{1, 2}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			groupField := storage.Field{APIName: "Parent__c", Type: tc.fieldType}
			if tc.fieldType == storage.FieldReference {
				groupField.ReferenceTo = []string{"Account"}
				groupField.RelationshipName = "Parent__r"
			}
			state := storage.ObjectState{
				Definition: storage.ObjectDefinition{
					APIName: "GroupedEntry__c",
					Fields: map[string]storage.Field{
						"Id":        {APIName: "Id", Type: storage.FieldID},
						"Parent__c": groupField,
					},
				},
				Records: make(map[storage.ID]storage.Record),
			}
			// Execute visits records in ID order, so these two fixtures also cover
			// which width supplies the first value for the same logical group.
			for i, value := range tc.values {
				recordID := storage.ID(fmt.Sprintf("a01%012d", i+1))
				state.Records[recordID] = storage.Record{
					ID: recordID, Object: "GroupedEntry__c",
					Fields: map[string]storage.Value{"Parent__c": value},
				}
			}
			org.Objects["GroupedEntry__c"] = state
			if len(tc.wantCounts) == 1 {
				matched, err := ParseAndExecute(org, "SELECT Id FROM GroupedEntry__c WHERE Parent__c = '"+long+"'")
				if err != nil {
					t.Fatal(err)
				}
				if matched.Rows != 3 {
					t.Fatalf("Id predicate matched %d rows, want 3", matched.Rows)
				}
			}
			query, err := Parse("SELECT Parent__c, COUNT(Id) total FROM GroupedEntry__c GROUP BY Parent__c")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Execute(org, query)
			if err != nil {
				t.Fatal(err)
			}
			if result.Rows != len(tc.wantCounts) || len(result.Records) != len(tc.wantCounts) {
				t.Fatalf("groups=%d records=%d, want %d: %#v", result.Rows, len(result.Records), len(tc.wantCounts), result.Records)
			}
			counts := make([]int64, 0, len(result.Records))
			hasNull := false
			for _, record := range result.Records {
				total := record.Fields["total"]
				if total.Kind != storage.ValueInteger {
					t.Fatalf("total=%#v, want Integer", total)
				}
				counts = append(counts, total.Integer)
				value := record.Fields["Parent__c"]
				if value.Kind == storage.ValueNull {
					hasNull = true
				} else if tc.fieldType == storage.FieldReference && value.Kind != storage.ValueID {
					t.Fatalf("reference group value=%#v, want Id", value)
				} else if tc.fieldType == storage.FieldString && value.Kind != storage.ValueString {
					t.Fatalf("text group value=%#v, want String", value)
				}
			}
			sort.Slice(counts, func(i, j int) bool { return counts[i] < counts[j] })
			for i, want := range tc.wantCounts {
				if counts[i] != want {
					t.Fatalf("group counts=%v, want %v", counts, tc.wantCounts)
				}
			}
			if hasNull != tc.wantNull {
				t.Fatalf("null group=%t, want %t", hasNull, tc.wantNull)
			}
		})
	}
}
