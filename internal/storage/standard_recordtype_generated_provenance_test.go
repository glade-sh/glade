package storage

import "testing"

// standard_recordtype_field_policy/org.tsv:2-4,9-11 records baseline MISSING;
// lines 16-25 record the non-Master field/reference contract at API62/67.
// ORG-RUN.txt documents both profiles. A helper-generated field follows the
// record-type shape; an explicitly configured field has independent provenance.
func TestStandardRecordTypeGeneratedProvenanceStorageRoutes(t *testing.T) {
	for _, route := range []string{"direct-field-helper", "standard-overlay", "fixture-record-seeder"} {
		for _, configured := range []bool{false, true} {
			shape := "generated"
			if configured {
				shape = "configured"
			}
			for _, repair := range []string{"ensure", "refresh"} {
				t.Run(route+"/"+shape+"/"+repair, func(t *testing.T) {
					definition := ObjectDefinition{
						APIName:     "Account",
						Fields:      map[string]Field{"OwnedUnrelated__c": {APIName: "OwnedUnrelated__c", Type: FieldString}},
						RecordTypes: []RecordTypeInfo{{DeveloperName: "OwnedPolicy", Name: "Owned policy", Active: true}},
					}
					if configured {
						definition.Fields["RecordTypeId"] = Field{APIName: "RecordTypeId", Type: FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}
					}
					switch route {
					case "direct-field-helper":
						EnsureRecordTypeIDField(&definition)
					case "standard-overlay":
						ensureStandardObjectFieldsForFeatures(&definition, nil)
					case "fixture-record-seeder":
						org := NewOrgState()
						ensureRecordTypeObject(&org)
						org.Objects["Account"] = ObjectState{Definition: definition}
						context := newRecordTypeSeedContext(&org)
						ensureRecordTypeRecordsForObjectWithContext(&org, "Account", &context)
						definition = org.Objects["Account"].Definition
					}
					if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); !exists {
						t.Fatal("non-Master record type did not provide RecordTypeId")
					}
					EnsureStandardObjectFieldsForFeatures(&definition, nil)
					definition.RecordTypes = nil
					if repair == "refresh" {
						delete(definition.Fields, "Id")
						RefreshStandardObjectFieldOverlay(&definition)
					} else {
						EnsureStandardObjectFieldsForFeatures(&definition, nil)
					}
					if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); exists != configured {
						t.Fatalf("after record-type removal: configured=%t; RecordTypeId=%t", configured, exists)
					}
					if _, exists := ResolveFieldName(definition, "", "OwnedUnrelated__c"); !exists {
						t.Fatal("unrelated configured field was removed")
					}
					if StandardObjectFieldsNeedWrite(definition) {
						t.Fatal("completed repair requests another write")
					}
				})
			}
		}
	}
}

// The native baseline has the Master mapping but no Account.RecordTypeId.
// The shared helper must obey that boundary before any later overlay repairs it.
func TestStandardRecordTypeGeneratedProvenanceDirectMasterBoundary(t *testing.T) {
	definition := ObjectDefinition{APIName: "Account", RecordTypes: []RecordTypeInfo{{DeveloperName: "Master", Name: "Master", Active: true}}}
	EnsureRecordTypeIDField(&definition)
	if _, exists := ResolveFieldName(definition, "", "RecordTypeId"); exists {
		t.Fatal("Master-only Account generated RecordTypeId")
	}
}
