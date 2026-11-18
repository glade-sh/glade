package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Only a Number field of scale 0 was captured reading 5.0 in a trigger.
func TestSpellProcessNumbersOnlyCapturedFieldShape(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{"Cedar__c": {Definition: storage.ObjectDefinition{Fields: map[string]storage.Field{
		"Plain__c": {APIName: "Plain__c", Type: storage.FieldDecimal, Scale: 0},
		"Two__c":   {APIName: "Two__c", Type: storage.FieldDecimal, Scale: 2},
		"Cash__c":  {APIName: "Cash__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY"},
	}}}}}
	engine := &Engine{Org: &org}
	record := storage.Record{Object: "Cedar__c", Fields: map[string]storage.Value{
		"Plain__c": storage.DecimalValue("5"),
		"Two__c":   storage.DecimalValue("5"),
		"Cash__c":  storage.DecimalValue("5"),
	}}
	engine.spellProcessNumbers(&record)
	for field, want := range map[string]string{"Plain__c": "5.0", "Two__c": "5", "Cash__c": "5"} {
		if got := record.Fields[field].Decimal; got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
}
