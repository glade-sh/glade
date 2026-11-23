package dml

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// These neighboring shapes were not captured natively. They pin the general
// evaluator's arithmetic; it is not a Salesforce claim.
func TestDefaultFormulaDatetimeNeighbors(t *testing.T) {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 5, 15, 10, 7, 0, 0, time.UTC) }
	def := storage.ObjectDefinition{APIName: "DefaultProbe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
		"Name":            {APIName: "Name", Type: storage.FieldString},
		"TimeOnly__c":     {APIName: "TimeOnly__c", Type: storage.FieldTime, DefaultValue: "TIMEVALUE(NOW())"},
		"TextTime__c":     {APIName: "TextTime__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: `MINUTE(TIMEVALUE("12:34:56"))`},
		"Extra__c":        {APIName: "Extra__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "MINUTE(TIMEVALUE(NOW()))+1"},
		"DirectOffset__c": {APIName: "DirectOffset__c", Type: storage.FieldDateTime, DefaultValue: "NOW()+MINUTE(TIMEVALUE(NOW()))/1440"},
		"Shifted__c":      {APIName: "Shifted__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "MINUTE(TIMEVALUE(NOW()+1))"},
		"Currency__c":     {APIName: "Currency__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY", DefaultValue: "MINUTE(TIMEVALUE(NOW()))"},
		"Percent__c":      {APIName: "Percent__c", Type: storage.FieldDecimal, DisplayType: "PERCENT", DefaultValue: "MINUTE(TIMEVALUE(NOW()))"},
		"Unknown__c":      {APIName: "Unknown__c", Type: storage.FieldDecimal, DefaultValue: "MINUTE(TIMEVALUE(NOW()))"},
	}}
	org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{}}
	engine := NewEngine(&org)
	result := engine.Insert([]storage.Record{{Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("probe")}}})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("insert: %+v", result)
	}
	record := org.Objects[def.APIName].Records[result[0].ID]
	for name, want := range map[string]string{"TimeOnly__c": "10:07:00.000Z", "TextTime__c": "34", "Shifted__c": "7", "Extra__c": "8", "DirectOffset__c": "2026-05-15T10:14:00Z", "Currency__c": "7", "Percent__c": "7", "Unknown__c": "7"} {
		value := record.Fields[name]
		got := value.String
		if value.Kind == storage.ValueDecimal {
			got = value.Decimal
		}
		if got != want {
			t.Errorf("%s: got %+v; want %s", name, value, want)
		}
	}
	_, _, ok := EvaluateRecordFormulaValueInOrg("MINUTE(TIMEVALUE(NOW()))", storage.Field{Type: storage.FieldDecimal}, &org, def, storage.Record{})
	if ok {
		t.Error("non-default formula changed from baseline")
	}
}
