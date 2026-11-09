package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// GLADEFLOWRESOURCE is internal to the flow loader. A user-written call is an
// unknown function, as FOO() is; only the loader's marked call evaluates.
func TestFlowResourceFormulaFunctionIsLoaderOnly(t *testing.T) {
	org := testOrg()
	definition := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}
	if got := ValidateFormulaInstance("GLADEFLOWRESOURCE('a')", &org, definition, "String", false); got != "Unknown function GLADEFLOWRESOURCE. Check spelling." {
		t.Fatalf("validation=%q", got)
	}
	if got, want := ValidateFormulaInstance("FOO('a')", &org, definition, "String", false), "Unknown function FOO. Check spelling."; got != want {
		t.Fatalf("baseline=%q", got)
	}
	field := storage.Field{APIName: "X__c", Type: storage.FieldString}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
	for _, formula := range []string{"GLADEFLOWRESOURCE('a' & BR() & 'b')", "gladeflowresource('a')"} {
		if _, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(formula, field, &org, definition, record, FormulaEvaluationOptions{FlowContext: true}); ok {
			t.Fatalf("user-written %s evaluated", formula)
		}
	}
	value, _, ok := EvaluateRecordFormulaValueInOrgWithOptions("TEXT(LEN("+FlowResourceFormulaCall+"('a' & BR() & 'b')))", field, &org, definition, record, FormulaEvaluationOptions{FlowContext: true})
	if !ok || value.String != "3" {
		t.Fatalf("loader call=%#v ok=%v", value, ok)
	}
}
