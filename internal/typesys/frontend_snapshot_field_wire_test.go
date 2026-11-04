package typesys

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
)

func TestFrontendFieldWirePreservesPrivateMetadataAndPicklists(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field schema.Field
	}{
		{"ordinary", schema.Field{Name: "Sample__c"}},
		{"explicit-empty", schema.Field{Name: "Choice__c", PicklistValuesConfigured: true, PicklistValues: []schema.PicklistValue{}}},
		{"populated", schema.Field{Name: "Choice__c", PicklistValuesConfigured: true, PicklistValues: []schema.PicklistValue{{FullName: "Sample"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := frontendField{Field: tc.field, File: "sample.field-meta.xml", ChildRelationshipNameInferred: true}
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var restored frontendField
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.Name != original.Name || restored.File != original.File || !restored.ChildRelationshipNameInferred || restored.PicklistValuesConfigured != original.PicklistValuesConfigured || len(restored.PicklistValues) != len(original.PicklistValues) {
				t.Fatalf("wire lost field metadata: %s; restored=%+v", data, restored)
			}
		})
	}
}
