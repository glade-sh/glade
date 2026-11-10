package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Uncaptured operand shapes stay unsupported rather than inferring Salesforce
// comparison semantics. The observed non-filterable field is described at
// testdata/native-capture/flow_type_matching/org.tsv:37.
func TestFlowCollectionUncapturedTypeBoundaries(t *testing.T) {
	no := false
	for _, tc := range []struct {
		name    string
		field   storage.Field
		element storage.Value
	}{
		{"nonfilterable_text", storage.Field{Type: storage.FieldString, Filterable: &no}, storage.StringValue("x")},
		{"id_operand", storage.Field{Type: storage.FieldID}, storage.IDValue(storage.ID("001000000000001"))},
		{"reference_id_operand", storage.Field{Type: storage.FieldReference}, storage.IDValue(storage.ID("001000000000001"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, supported := flowLookupCollectionValuesEqual(tc.field, storage.StringValue("x"), tc.element)
			if supported {
				t.Fatal("uncaptured comparison was modeled")
			}
		})
	}
}
