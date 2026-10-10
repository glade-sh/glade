package dml

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFormulaDateAdditionPreservesBlankDate(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "DateProbe__c", Fields: map[string]storage.Field{
		"Start__c": {APIName: "Start__c", Type: storage.FieldDate, DisplayType: "DATE"},
		"End__c":   {APIName: "End__c", Type: storage.FieldDate, DisplayType: "DATE"},
		"Days__c":  {APIName: "Days__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"},
	}}
	for _, tc := range []struct {
		name, formula, display, want string
		fields                       map[string]storage.Value
		null                         bool
	}{
		{name: "missing date and days", formula: "Start__c + Days__c", display: "DATE", null: true},
		{name: "blank date with days", formula: "Start__c + Days__c", display: "DATE", fields: map[string]storage.Value{"Start__c": storage.NullValue(), "Days__c": storage.DecimalValue("5")}, null: true},
		{name: "populated date and days", formula: "Start__c + Days__c", display: "DATE", fields: map[string]storage.Value{"Start__c": storage.DateValue("2026-01-01"), "Days__c": storage.DecimalValue("30")}, want: "2026-01-31"},
		// These controls preserve existing arithmetic outside the admitted blank-Date boundary.
		{name: "populated date with blank days", formula: "Start__c + Days__c", display: "DATE", fields: map[string]storage.Value{"Start__c": storage.DateValue("2026-01-01")}, want: "2026-01-01"},
		{name: "blank number", formula: "Days__c + 5", display: "DOUBLE", want: "5"},
		{name: "literal null", formula: "NULL + 5", display: "DOUBLE", want: "5"},
		{name: "date subtraction", formula: "End__c - Start__c", display: "DOUBLE", fields: map[string]storage.Value{"Start__c": storage.DateValue("2026-01-01"), "End__c": storage.DateValue("2026-01-31")}, want: "30"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := storage.Field{Type: storage.FieldCalculated, DisplayType: tc.display, Formula: tc.formula, FormulaTreatBlanksAs: "BlankAsZero"}
			record := storage.Record{Object: definition.APIName, Fields: tc.fields}
			value, isNull, ok := EvaluateRecordFormulaValueInOrg(tc.formula, field, &org, definition, record)
			got := value.String
			if tc.display == "DOUBLE" {
				got = value.Decimal
			}
			if !ok || isNull != tc.null || (!isNull && got != tc.want) {
				t.Fatalf("value=%#v null=%v ok=%v", value, isNull, ok)
			}
		})
	}
}

func TestFormulaNowUsesOrgClockAndMaterializesDatetime(t *testing.T) {
	org := storage.NewOrgState()
	fixed := time.Date(2026, time.January, 15, 12, 34, 56, 789000000, time.UTC)
	org.Now = func() time.Time { return fixed }
	definition := storage.ObjectDefinition{APIName: "Probe__c"}
	field := storage.Field{Type: storage.FieldDateTime, DisplayType: "DATETIME"}
	record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{}}
	value, isNull, ok := EvaluateRecordFormulaValueInOrg("NOW()", field, &org, definition, record)
	if !ok || isNull {
		t.Fatalf("value=%#v null=%v ok=%v", value, isNull, ok)
	}
	if got, want := value.String, "2026-01-15T12:34:56.789Z"; got != want {
		t.Fatalf("NOW()=%q, want %q", got, want)
	}
}

func TestValidateFormulaForDefinitionRejectsUnknownDirectField(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{
		"Name": {APIName: "Name", Type: storage.FieldString},
	}}
	if ValidateFormulaForDefinition("NonExistentField + 123", definition) {
		t.Fatal("unknown direct field should fail schema-aware validation")
	}
	if !ValidateFormulaForDefinition("Name & \"!\"", definition) {
		t.Fatal("known direct field should pass schema-aware validation")
	}
	if !ValidateFormula("NonExistentField + 123") {
		t.Fatal("syntax-only validation contract should remain permissive without a context")
	}
}
