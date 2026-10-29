package sobject

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// Native profile twins: standard_recordtype_field_policy/org.tsv:2-4,9-11
// (baseline MISSING), 16-25 (non-Master reference/constructor), ORG-RUN.txt.
// Conversion must preserve whether RecordTypeId was declared or synthesized.
func TestStandardRecordTypeGeneratedProvenanceConversionRoutes(t *testing.T) {
	for _, route := range []string{"raw-describe-to-storage", "schema-registry-to-storage", "raw-storage-describe-storage"} {
		for _, configured := range []bool{false, true} {
			shape := "generated"
			if configured {
				shape = "configured"
			}
			for _, repair := range []string{"ensure", "refresh"} {
				t.Run(route+"/"+shape+"/"+repair, func(t *testing.T) {
					var definition storage.ObjectDefinition
					switch route {
					case "raw-describe-to-storage":
						describe := DescribeSObjectResult{
							Name:        "Account",
							Fields:      map[string]DescribeFieldResult{"OwnedUnrelated__c": {Name: "OwnedUnrelated__c", Type: storage.FieldString}},
							RecordTypes: []DescribeRecordTypeInfo{{DeveloperName: "OwnedPolicy", Name: "Owned policy", Active: true}},
						}
						if configured {
							describe.Fields["RecordTypeId"] = DescribeFieldResult{Name: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}
						}
						definition = ToObjectDefinition(describe)
					case "schema-registry-to-storage":
						object := schema.Object{
							Name: "Account", Fields: []schema.Field{{Name: "OwnedUnrelated__c", Type: "Text"}},
							RecordTypes: []schema.RecordType{{DeveloperName: "OwnedPolicy", Label: "Owned policy", Active: true}},
						}
						if configured {
							object.Fields = append(object.Fields, schema.Field{Name: "RecordTypeId", Type: "Lookup", ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"})
						}
						registry := BuildDescribeRegistry(schema.Schema{Objects: []schema.Object{object}})
						definition = ToObjectDefinition(registry.Objects["Account"])
					case "raw-storage-describe-storage":
						raw := storage.ObjectDefinition{
							APIName:     "Account",
							Fields:      map[string]storage.Field{"OwnedUnrelated__c": {APIName: "OwnedUnrelated__c", Type: storage.FieldString}},
							RecordTypes: []storage.RecordTypeInfo{{DeveloperName: "OwnedPolicy", Name: "Owned policy", Active: true}},
						}
						if configured {
							raw.Fields["RecordTypeId"] = storage.Field{APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"}, RelationshipName: "RecordType"}
						}
						definition = ToObjectDefinition(FromObjectDefinition(raw))
						if _, exists := raw.Fields["RecordTypeId"]; exists != configured || raw.Metadata != nil {
							t.Fatal("describe conversion mutated its raw input")
						}
					}
					if _, exists := storage.ResolveFieldName(definition, "", "RecordTypeId"); !exists {
						t.Fatal("non-Master record type did not provide RecordTypeId")
					}
					storage.EnsureStandardObjectFieldsForFeatures(&definition, nil)
					definition.RecordTypes = nil
					if repair == "refresh" {
						delete(definition.Fields, "Id")
						storage.RefreshStandardObjectFieldOverlay(&definition)
					} else {
						storage.EnsureStandardObjectFieldsForFeatures(&definition, nil)
					}
					if _, exists := storage.ResolveFieldName(definition, "", "RecordTypeId"); exists != configured {
						t.Fatalf("after record-type removal: configured=%t; RecordTypeId=%t", configured, exists)
					}
					if _, exists := FromObjectDefinition(definition).Fields["RecordTypeId"]; exists != configured {
						t.Fatalf("final describe: configured=%t; RecordTypeId=%t", configured, exists)
					}
					if _, exists := storage.ResolveFieldName(definition, "", "OwnedUnrelated__c"); !exists {
						t.Fatal("unrelated configured field was removed")
					}
				})
			}
		}
	}
}
