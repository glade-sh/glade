package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These controls preserve prior required-first behavior outside the native
// explicit-null SYSTEM_MODE/no-default case. They are restoration boundaries,
// not additional native observations about those record-type shapes.
func TestRecordTypeRequiredPrecedenceScopeControls(t *testing.T) {
	for _, scenario := range []struct {
		name, mode, shape                         string
		assignedDefault, idPresent, idPlaceholder bool
	}{
		{name: "non-Apex", shape: "null"},
		{name: "USER_MODE", mode: "USER_MODE", shape: "null"},
		{name: "assigned default", mode: "SYSTEM_MODE", shape: "null", assignedDefault: true},
		{name: "explicit Master", mode: "SYSTEM_MODE", shape: "Master"},
		{name: "omitted", mode: "SYSTEM_MODE", shape: "omitted"},
		{name: "real", mode: "SYSTEM_MODE", shape: "real"},
		{name: "existing Id", mode: "SYSTEM_MODE", shape: "null", idPresent: true},
		{name: "generated placeholder Id", mode: "SYSTEM_MODE", shape: "null", idPlaceholder: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			org := testOrg()
			realID := storage.ID("012000000000101")
			org.Objects["RequiredChoice__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RequiredChoice__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Type: storage.FieldString, Required: true, AutoNumber: true, DisplayFormat: "ROW-{0000}"},
				"RequiredNote__c": {APIName: "RequiredNote__c", Type: storage.FieldString, Required: true, Length: 20},
				"RecordTypeId":    {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
			}, RecordTypes: []storage.RecordTypeInfo{{ID: realID, DeveloperName: "Alpha", Name: "Alpha", Active: true, Available: scenario.assignedDefault, Default: scenario.assignedDefault}}}, Records: map[storage.ID]storage.Record{}}
			org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012"}, Records: map[storage.ID]storage.Record{realID: {Object: "RecordType", ID: realID}}}
			row := storage.Record{Object: "RequiredChoice__c", Fields: map[string]storage.Value{}}
			switch scenario.shape {
			case "null":
				row.ExplicitNulls = map[string]bool{"RecordTypeId": true}
			case "Master":
				row.Fields["RecordTypeId"] = storage.IDValue("012000000000000AAA")
			case "real":
				row.Fields["RecordTypeId"] = storage.IDValue(realID)
			}
			if scenario.idPresent {
				row.ID = "a01000000000101"
			}
			if scenario.idPlaceholder {
				row.ID = "pending#owned"
			}
			engine := NewEngine(&org)
			engine.Options.ApexRecordTypeInsertMode = scenario.mode
			results := engine.Insert([]storage.Record{row})
			if len(results) != 1 || results[0].Success || results[0].StatusCode != "REQUIRED_FIELD_MISSING" || results[0].Error != "Required fields are missing: [RequiredNote__c]" {
				t.Fatalf("required-first scope = %#v", results)
			}
			if len(org.Objects["RequiredChoice__c"].Records) != 0 {
				t.Fatal("failed insert persisted a record")
			}
		})
	}
}
