package soql

import (
	"math/big"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Local guards keep the queried-field policy off ordinary Number storage and
// the separate FormulaEval surface; these are not additional native captures.
func TestCalculatedNumberQueryScopeGuards(t *testing.T) {
	field := storage.Field{APIName: "Computed__c", Type: storage.FieldCalculated, DisplayType: "DOUBLE", Formula: "Num__c + 1", FormulaTreatBlanksAs: "BlankAsBlank", Scale: 2, ScaleSpecified: true}
	def := storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Num__c": {APIName: "Num__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Scale: 2, ScaleSpecified: true}, "Computed__c": field}}
	id := storage.ID("a01000000000001")
	row := storage.Record{ID: id, Object: def.APIName, Fields: map[string]storage.Value{"Num__c": storage.DecimalValue("1.25")}}
	org := storage.NewOrgState()
	org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{id: row}}
	result, err := ParseAndExecute(org, "SELECT Num__c FROM Probe__c")
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Records[0].Fields["Num__c"].Decimal; got != "1.25" {
		t.Fatalf("ordinary Number=%q", got)
	}
	row.Fields = map[string]storage.Value{}
	value, isNull, ok := dml.EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, def, row, dml.FormulaEvaluationOptions{FormulaSurface: true})
	if !ok || isNull {
		t.Fatalf("FormulaEval value=%v null=%t ok=%t", value, isNull, ok)
	}
	number, ok := new(big.Rat).SetString(value.Decimal)
	if !ok || number.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("FormulaEval value=%v", value)
	}
}
