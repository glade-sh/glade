package dml

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// These neighboring shapes were not captured. The same check runs on the
// origin/main baseline and on the candidate; it is not a Salesforce claim.
func TestDefaultFormulaDatetimeNeighborUnchanged(t *testing.T) {
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
	for _, name := range []string{"TimeOnly__c", "TextTime__c", "Shifted__c", "Extra__c", "DirectOffset__c", "Currency__c", "Percent__c", "Unknown__c"} {
		if value, ok := record.Fields[name]; ok && value.Kind != storage.ValueNull {
			t.Errorf("%s changed from baseline: %+v", name, value)
		}
	}
	_, _, ok := EvaluateRecordFormulaValueInOrg("MINUTE(TIMEVALUE(NOW()))", storage.Field{Type: storage.FieldDecimal}, &org, def, storage.Record{})
	if ok {
		t.Error("non-default formula changed from baseline")
	}
}
