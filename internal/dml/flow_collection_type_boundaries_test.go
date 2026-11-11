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
		{"invalid_id_string", storage.Field{Type: storage.FieldID}, storage.StringValue("invalid")},
		{"invalid_reference_string", storage.Field{Type: storage.FieldReference}, storage.StringValue("invalid")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, supported := flowLookupCollectionValuesEqual(tc.field, storage.StringValue("x"), tc.element)
			if supported {
				t.Fatal("uncaptured comparison was modeled")
			}
		})
	}
}

// Transform source resources outside the captured direct scalar variable and
// string-literal shapes remain unsupported; this is a local scope boundary.
func TestFlowTransformUncapturedSourceBoundary(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	engine := NewEngine(&org)
	for _, tc := range []struct {
		name, sourceType string
		wantSupported    bool
	}{
		{"captured_string", "String", true},
		{"uncaptured_currency", "Currency", false},
		{"unknown_resource", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := newFlowFrame()
			frame.scalars[flowFrameKey("Source")] = storage.StringValue("x")
			if tc.sourceType != "" {
				frame.variables[flowFrameKey("Source")] = storage.FlowVariable{Name: "Source", DataType: tc.sourceType}
			}
			transform := storage.FlowTransform{Name: "MapRecord", TransformType: "RecordMap", ObjectType: "Account", FieldMappings: []storage.WorkflowFieldUpdate{{Field: "Name", SourceField: "Source"}}}
			err := engine.applyFlowTransformStep(transform, frame)
			if (err == nil) != tc.wantSupported {
				t.Fatalf("supported=%t, want %t: %v", err == nil, tc.wantSupported, err)
			}
		})
	}
}

// Collection values with an unobserved or unknown declared Flow type remain
// outside the native matrix even when their elements have a familiar kind.
func TestFlowLookupUncapturedSourceBoundary(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	definition := storage.ObjectDefinition{APIName: "Sample__c", Fields: map[string]storage.Field{"Label__c": {APIName: "Label__c", Type: storage.FieldString}}}
	target := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Label__c": storage.StringValue("42")}}
	for _, tc := range []struct {
		name, sourceType string
		wantSupported    bool
	}{
		{"captured_string", "String", true},
		{"uncaptured_currency", "Currency", false},
		{"unknown_resource", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := newFlowFrame()
			frame.scalars[flowFrameKey("Source")] = storage.ListValue(storage.StringValue("42"))
			if tc.sourceType != "" {
				frame.variables[flowFrameKey("Source")] = storage.FlowVariable{Name: "Source", DataType: tc.sourceType, IsCollection: true}
			}
			_, supported := engine.evaluateFlowLookupCriteria(storage.WorkflowCriteriaItem{Field: "Label__c", Operation: "In", SourceField: "Source"}, target, definition, storage.Record{}, storage.ObjectDefinition{}, frame)
			if supported != tc.wantSupported {
				t.Fatalf("supported=%t, want %t", supported, tc.wantSupported)
			}
		})
	}
}
