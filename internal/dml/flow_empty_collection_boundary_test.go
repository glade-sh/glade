package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Empty collection outcomes were captured only for the matrix field types in
// testdata/native-capture/flow_type_matrix/org.tsv (empty rows); other field
// types stay unsupported whatever the list holds.
func TestFlowEmptyCollectionFieldTypeBoundary(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	for _, tc := range []struct {
		name      string
		fieldType storage.FieldType
		supported bool
	}{
		{"string", storage.FieldString, true},
		{"picklist", storage.FieldPicklist, true},
		{"reference", storage.FieldReference, true},
		{"integer", storage.FieldInteger, true},
		{"decimal", storage.FieldDecimal, true},
		{"boolean", storage.FieldBoolean, true},
		{"date", storage.FieldDate, true},
		{"datetime", storage.FieldDateTime, true},
		{"multipicklist", storage.FieldMultiPicklist, false},
		{"time", storage.FieldTime, false},
		{"address", storage.FieldAddress, false},
		{"location", storage.FieldLocation, false},
	} {
		for _, source := range []struct {
			name  string
			value storage.Value
		}{{"empty", storage.ListValue()}, {"null", storage.NullValue()}} {
			t.Run(tc.name+"_"+source.name, func(t *testing.T) {
				definition := storage.ObjectDefinition{APIName: "Sample__c", Fields: map[string]storage.Field{"Value__c": {APIName: "Value__c", Type: tc.fieldType}}}
				target := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{}}
				frame := newFlowFrame()
				frame.scalars[flowFrameKey("Source")] = source.value
				frame.variables[flowFrameKey("Source")] = storage.FlowVariable{Name: "Source", DataType: "String", IsCollection: true}
				for _, operation := range []string{"In", "NotIn"} {
					_, supported := engine.evaluateFlowLookupCriteria(storage.WorkflowCriteriaItem{Field: "Value__c", Operation: operation, SourceField: "Source"}, target, definition, storage.Record{}, storage.ObjectDefinition{}, frame)
					if supported != tc.supported {
						t.Fatalf("%s supported=%t, want %t", operation, supported, tc.supported)
					}
				}
			})
		}
	}
}
