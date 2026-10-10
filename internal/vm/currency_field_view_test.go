package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCurrencyFieldCallerAndMaterializedViews(t *testing.T) {
	field := storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY", Scale: 2, ScaleSpecified: true}
	caller := Object("Amount__c")
	queried := Object("Amount__c")
	queried.Fields[sobjectQueriedFieldsField] = queriedSObjectFieldsValue("Amount__c", map[string]bool{"amount__c": true})
	trigger := Object("Amount__c")
	markTriggerSObject(&trigger)
	inputAfter := Object("Amount__c")
	inputAfter.Fields[sobjectDMLAccessibleField] = Bool(true)
	inputAfter.Fields[sobjectQueriedFieldsField] = queried.Fields[sobjectQueriedFieldsField]
	assigned := coerceSObjectFieldRuntimeValue(Int(100), field)
	if assigned.Kind != ValueDecimal || decimalPlainText(assigned) != "100" {
		t.Fatalf("caller assignment=%#v", assigned)
	}
	for _, tc := range []struct {
		name     string
		receiver Value
		want     string
	}{
		{"caller", caller, "100"}, {"inputAfter", inputAfter, "100"}, {"query", queried, "100.00"}, {"trigger", trigger, "100.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := coerceReadSObjectFieldRuntimeValue(tc.receiver, assigned, field)
			if decimalPlainText(got) != tc.want {
				t.Fatalf("read=%#v want=%s", got, tc.want)
			}
			if decimalPlainText(assigned) != "100" {
				t.Fatal("read mutated input")
			}
			precise, _ := decimalFromText("33.335")
			if got := coerceReadSObjectFieldRuntimeValue(tc.receiver, precise, field); decimalPlainText(got) != "33.335" {
				t.Fatalf("rounded nonformula=%#v", got)
			}
		})
	}
	number := field
	number.DisplayType = "DOUBLE"
	// A25 N006 captures scale-zero Integer assignment to an explicit Number.
	if got := coerceSObjectFieldRuntimeValue(Int(100), number); decimalPlainText(got) != "100" {
		t.Fatalf("number assignment=%#v", got)
	}
	null := coerceReadSObjectFieldRuntimeValue(queried, Null, field)
	if null.Kind != ValueNull {
		t.Fatalf("null became=%#v", null)
	}
	exponent, _ := decimalFromText("1e3")
	if got := coerceReadSObjectFieldRuntimeValue(queried, exponent, field); decimalPlainText(got) != "1000.00" {
		t.Fatalf("exponent=%#v", got)
	}
}
