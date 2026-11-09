package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A text formula reads differently on a Trigger.new or Trigger.old record than
// from SOQL (org capture blank_text_formula, API 62 and 67): the trigger keeps
// whitespace, so a single space stays one space and a trailing space is kept,
// only an empty result is null, and BR() reads as _BR_ENCODED_; SOQL trims the
// result, reads a single space as null and renders BR() as <br>.
func TestFormulaTriggerRecordSurface(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Fm__c", Fields: map[string]storage.Field{
		"A__c":  {APIName: "A__c", Type: storage.FieldString, DisplayType: "STRING"},
		"B__c":  {APIName: "B__c", Type: storage.FieldString, DisplayType: "STRING"},
		"Fs__c": {APIName: "Fs__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "If(IsBlank(A__c), '', A__c & BR()) & B__c & ' '"},
		"Fb__c": {APIName: "Fb__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "IF(ISBLANK(A__c), '', A__c)"},
	}}
	for _, tc := range []struct {
		name, field                   string
		fields                        map[string]storage.Value
		stored, trigger               string
		storedNull, triggerNull, same bool
	}{
		{name: "blank inputs read one space in the trigger", field: "Fs__c", storedNull: true, trigger: " "},
		{name: "BR and the trailing space read as text in the trigger", field: "Fs__c", fields: map[string]storage.Value{"A__c": storage.StringValue("a")}, stored: "a<br>", trigger: "a_BR_ENCODED_ "},
		{name: "an empty result is null on both surfaces", field: "Fb__c", storedNull: true, triggerNull: true, same: true},
		{name: "a plain result is the same on both surfaces", field: "Fb__c", fields: map[string]storage.Value{"A__c": storage.StringValue("a")}, stored: "a", trigger: "a", same: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := definition.Fields[tc.field]
			record := storage.Record{Object: definition.APIName, Fields: tc.fields}
			surfaces, ok := EvaluateRecordFormulaSurfacesInOrg(field.Formula, field, &org, definition, record, FormulaEvaluationOptions{})
			if !ok {
				t.Fatal("formula did not evaluate")
			}
			if surfaces.StoredNull != tc.storedNull || (!tc.storedNull && surfaces.Stored.String != tc.stored) {
				t.Fatalf("stored surface: null=%v value=%q", surfaces.StoredNull, surfaces.Stored.String)
			}
			if surfaces.TriggerNull != tc.triggerNull || (!tc.triggerNull && surfaces.Trigger.String != tc.trigger) {
				t.Fatalf("trigger surface: null=%v value=%q", surfaces.TriggerNull, surfaces.Trigger.String)
			}
			if surfaces.Differs == tc.same {
				t.Fatalf("differs=%v", surfaces.Differs)
			}
			value, isNull, ok := EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, definition, record, FormulaEvaluationOptions{TriggerRecord: true})
			if !ok || isNull != tc.triggerNull || (!tc.triggerNull && value.String != tc.trigger) {
				t.Fatalf("trigger read: null=%v value=%q ok=%v", isNull, value.String, ok)
			}
			value, isNull, ok = EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, definition, record)
			if !ok || isNull != tc.storedNull || (!tc.storedNull && value.String != tc.stored) {
				t.Fatalf("stored read: null=%v value=%q ok=%v", isNull, value.String, ok)
			}
		})
	}
}
