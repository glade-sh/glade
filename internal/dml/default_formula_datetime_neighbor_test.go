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

// An explicitly supplied record type was not part of the native capture.
func TestDefaultFormulaRecordTypeInsertNeighborUnchanged(t *testing.T) {
	org := storage.NewOrgState()
	org.Now = func() time.Time { return time.Date(2026, 5, 15, 10, 7, 0, 0, time.UTC) }
	generator := storage.NewStandardIDGenerator()
	recordTypeID, err := generator.Next("RecordType")
	if err != nil {
		t.Fatal(err)
	}
	definition := storage.ObjectDefinition{APIName: "DefaultProbe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
		"Name":         {APIName: "Name", Type: storage.FieldString},
		"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
		"Minute__c":    {APIName: "Minute__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "MINUTE(TIMEVALUE(NOW()))"},
	}, RecordTypes: []storage.RecordTypeInfo{{ID: recordTypeID, Active: true, Available: true}}}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: storage.StandardKeyPrefix("RecordType"), Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}}, Records: map[storage.ID]storage.Record{recordTypeID: {ID: recordTypeID, Object: "RecordType"}}}
	definition.RecordTypes[0].Default = true
	state := org.Objects[definition.APIName]
	state.Definition = definition
	org.Objects[definition.APIName] = state
	cases := []struct {
		name   string
		fields map[string]storage.Value
		nulls  map[string]bool
	}{
		{name: "explicit", fields: map[string]storage.Value{"Name": storage.StringValue("probe"), "RecordTypeId": storage.IDValue(recordTypeID)}},
		{name: "explicit-null", fields: map[string]storage.Value{"Name": storage.StringValue("probe")}, nulls: map[string]bool{"RecordTypeId": true}},
		{name: "defaulted", fields: map[string]storage.Value{"Name": storage.StringValue("probe")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{{Object: definition.APIName, Fields: tc.fields, ExplicitNulls: tc.nulls}})
			if tc.name == "explicit-null" {
				if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_CROSS_REFERENCE_KEY" {
					t.Fatalf("explicit-null baseline: %+v", result)
				}
				return
			}
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("insert: %+v", result)
			}
			record := org.Objects[definition.APIName].Records[result[0].ID]
			if tc.name == "defaulted" {
				value, ok := record.GetField("RecordTypeId")
				if !ok || value.ID != recordTypeID {
					t.Fatalf("default record type missing: %+v", value)
				}
			}
			value, ok := record.Fields["Minute__c"]
			if ok && value.Kind != storage.ValueNull {
				t.Errorf("record type changed from baseline: %+v", value)
			}
		})
	}
}
