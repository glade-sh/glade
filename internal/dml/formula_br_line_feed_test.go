package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// BR() is the 12-character text _BR_ENCODED_ while a formula evaluates and
// finishes as <br> on the stored surface (SOQL), but as a line feed in a flow
// formula, a workflow field update formula and a field default value formula
// (org capture flow_br_surfaces, API 62 and 67).
func TestFormulaBRFinishesAsLineFeedOutsideStoredFields(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Br__c", Fields: map[string]storage.Field{
		"T__c":    {APIName: "T__c", Type: storage.FieldString, DisplayType: "STRING"},
		"Calc__c": {APIName: "Calc__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "'a' & BR() & 'b'"},
	}}
	field := definition.Fields["T__c"]
	record := storage.Record{Object: definition.APIName}
	const expression = "'a' & BR() & 'b'"

	stored, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(expression, field, &org, definition, record, FormulaEvaluationOptions{})
	if !ok || stored.String != "a<br>b" {
		t.Fatalf("stored surface: %q ok=%v", stored.String, ok)
	}
	for name, options := range map[string]FormulaEvaluationOptions{"line feed": {LineFeedBR: true}, "flow": {FlowContext: true}} {
		value, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(expression, field, &org, definition, record, options)
		if !ok || value.String != "a\nb" {
			t.Fatalf("%s: %q ok=%v", name, value.String, ok)
		}
		length, _, ok := EvaluateRecordFormulaValueInOrgWithOptions("LEN('a' & BR() & 'b')", storage.Field{APIName: "N", Type: storage.FieldInteger}, &org, definition, record, options)
		if !ok || length.Integer != 14 {
			t.Fatalf("%s: LEN inside the formula is %v ok=%v, want 14", name, length, ok)
		}
	}

	// Workflow field update: a line feed, and LEN still sees the placeholder.
	value, _, ok := workflowExpressionValue(field, record, `"w" & BR() & "z"`, definition, &org)
	if !ok || value.String != "w\nz" {
		t.Fatalf("workflow update: %q ok=%v", value.String, ok)
	}
	value, _, ok = workflowExpressionValue(field, record, `"w" & BR() & "z"`, definition, nil)
	if !ok || value.String != "w\nz" {
		t.Fatalf("workflow update without an org: %q ok=%v", value.String, ok)
	}

	// Default value.
	defaulted := storage.Field{APIName: "D__c", Type: storage.FieldString, DisplayType: "STRING", DefaultValue: `"x" & BR() & "y"`}
	value, ok = defaultValueForRecordFieldWithOptions(&org, definition, record, defaulted, FormulaEvaluationOptions{})
	if !ok || value.String != "x\ny" {
		t.Fatalf("default value: %q ok=%v", value.String, ok)
	}

	// Flow evaluation, direct parser callers.
	flowValue, _, ok := evaluateFlowFormulaValue(expression, record)
	if !ok || flowValue.String != "a\nb" {
		t.Fatalf("flow formula: %q ok=%v", flowValue.String, ok)
	}
	flowLength, _, ok := evaluateFlowFormulaValue("LEN("+expression+")", record)
	if !ok || flowLength.Integer != 14 {
		t.Fatalf("flow LEN: %v ok=%v", flowLength, ok)
	}
	// A stored formula field a flow formula references reads as the finished value.
	flowOptions := FormulaEvaluationOptions{FlowContext: true}
	length, _, ok := EvaluateRecordFormulaValueInOrgWithOptions("LEN(Calc__c)", storage.Field{APIName: "N", Type: storage.FieldInteger}, &org, definition, record, flowOptions)
	if !ok || length.Integer != 3 {
		t.Fatalf("flow LEN of a formula field: %v ok=%v, want 3", length, ok)
	}
	length, _, ok = EvaluateRecordFormulaValueInOrgWithOptions("LEN(Calc__c)", storage.Field{APIName: "N", Type: storage.FieldInteger}, &org, definition, record, FormulaEvaluationOptions{})
	if !ok || length.Integer != 14 {
		t.Fatalf("LEN of a formula field outside a flow: %v ok=%v, want 14", length, ok)
	}
	// A flow field update copying a stored formula field reads the finished value.
	update := storage.WorkflowFieldUpdate{Field: "T__c", SourceField: "Calc__c"}
	value, _, ok = workflowUpdateValue(field, record, update, definition, &org, flowOptions)
	if !ok || value.String != "a\nb" {
		t.Fatalf("flow copy of a formula field: %q ok=%v", value.String, ok)
	}
}
