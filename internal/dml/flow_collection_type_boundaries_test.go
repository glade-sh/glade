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
