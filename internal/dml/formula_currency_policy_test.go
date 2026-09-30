package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// BlankAsBlank/absent/literal NULL are noninterference controls, not new
// Salesforce parity claims for those metadata policies.
func TestCurrencyFormulaExplicitBlankOperandPolicy(t *testing.T) {
	org := testOrg()
	def := storage.ObjectDefinition{APIName: "Policy__c", Fields: map[string]storage.Field{
		"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY"},
		"Text__c":   {APIName: "Text__c", Type: storage.FieldString},
	}}
	for _, tc := range []struct {
		name, policy, formula string
		preserve, null        bool
		want                  string
	}{
		{"explicit zero", "BlankAsZero", "IF(true, Amount__c, 2)", false, false, "0.00"},
		{"blank unchanged", "BlankAsBlank", "Amount__c", false, true, ""},
		{"absent unchanged", "", "Amount__c", false, true, ""},
		{"unknown unchanged", "future-policy", "Amount__c", false, true, ""},
		{"literal null unchanged", "BlankAsZero", "NULL", false, true, ""},
		{"text null unchanged", "BlankAsZero", "Text__c", false, true, ""},
		{"preserve option", "BlankAsZero", "Amount__c", true, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := storage.Field{APIName: "Total__c", Type: storage.FieldCalculated, DisplayType: "CURRENCY", Scale: 2, ScaleSpecified: true, Formula: tc.formula, FormulaTreatBlanksAs: tc.policy}
			for _, explicit := range []bool{false, true} {
				record := storage.Record{Object: def.APIName, Fields: map[string]storage.Value{}, ExplicitNulls: map[string]bool{"Amount__c": explicit}}
				value, isNull, ok := EvaluateRecordFormulaValueInOrgWithOptions(tc.formula, field, &org, def, record, FormulaEvaluationOptions{PreserveNumericNull: tc.preserve})
				if !ok || isNull != tc.null || (!isNull && value.Decimal != tc.want) {
					t.Fatalf("explicit=%v value=%#v null=%v ok=%v", explicit, value, isNull, ok)
				}
			}
		})
	}
}

func TestCurrencyFormulaDeclaredScalePresence(t *testing.T) {
	org := testOrg()
	def := storage.ObjectDefinition{APIName: "Policy__c"}
	for _, tc := range []struct {
		name      string
		scale     int
		specified bool
		want      string
	}{
		{"omitted", 0, false, "133.335"}, {"explicit zero", 0, true, "133"},
		{"declared two", 2, true, "133.34"}, {"legacy positive scale", 2, false, "133.34"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := storage.Field{Type: storage.FieldCalculated, DisplayType: "CURRENCY", Formula: "100 + 33.335", Scale: tc.scale, ScaleSpecified: tc.specified}
			value, isNull, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, def, storage.Record{Object: def.APIName})
			if !ok || isNull || value.Decimal != tc.want {
				t.Fatalf("value=%#v null=%v ok=%v", value, isNull, ok)
			}
		})
	}
}
