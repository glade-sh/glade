package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Assigned defaults: omission keeps the existing engine path. An explicit null
// RecordTypeId with a profile default is rejected (native rows
// record_type_id_null_insert_default, APIs 62, 67 and 68), and so is the Master
// token in Apex modes (record_type_visibility profile rows).
func TestApexRecordTypeInsertPreservesAssignedDefaults(t *testing.T) {
	for _, mode := range []string{"", "SYSTEM_MODE", "USER_MODE"} {
		t.Run(mode, func(t *testing.T) {
			org := testOrg()
			const realID storage.ID = "012000000000101AAA"
			org.Objects["ScopedRecord__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "ScopedRecord__c", KeyPrefix: "a01",
				Fields: map[string]storage.Field{
					"Name":         {APIName: "Name", Type: storage.FieldString},
					"RecordTypeId": {APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}},
				},
				RecordTypes: []storage.RecordTypeInfo{{ID: realID, DeveloperName: "Assigned", Active: true, Available: true, Default: true}},
			}, Records: map[storage.ID]storage.Record{}}
			org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012"}, Records: map[storage.ID]storage.Record{realID: {Object: "RecordType", ID: realID}}}
			engine := NewEngine(&org)
			engine.Options.ApexRecordTypeInsertMode = mode
			results := engine.Insert([]storage.Record{
				{Object: "ScopedRecord__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Omitted")}},
				{Object: "ScopedRecord__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Null")}, ExplicitNulls: map[string]bool{"RecordTypeId": true}},
				{Object: "ScopedRecord__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Master"), "RecordTypeId": storage.IDValue("012000000000000AAA")}},
			})
			for index, result := range results {
				if index == 2 && mode != "" {
					if result.Success || result.StatusCode != "INVALID_CROSS_REFERENCE_KEY" {
						t.Fatalf("Master token with a default = %#v", result)
					}
					continue
				}
				if index == 1 {
					if result.Success || result.StatusCode != "INVALID_CROSS_REFERENCE_KEY" {
						t.Fatalf("explicit null with a default = %#v", result)
					}
					continue
				}
				if !result.Success {
					t.Fatalf("assigned-default row %d: %#v", index, result)
				}
			}
			omitted := org.Objects["ScopedRecord__c"].Records[results[0].ID]
			value, _ := omitted.GetField("RecordTypeId")
			if !storage.IDsEqual(idFromStorageValue(value), realID) {
				t.Fatalf("omitted assigned default = %#v", value)
			}
		})
	}
}
