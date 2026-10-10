package storage

import (
	"strings"
	"testing"
)

func TestStandardSchemaReferenceTargetsExcludeProjectObjects(t *testing.T) {
	for name, entry := range standardObjectCatalogData {
		for fieldName, field := range entry.Definition.Fields {
			for _, target := range field.ReferenceTo {
				if strings.HasSuffix(strings.ToLower(target), "__c") || strings.HasSuffix(strings.ToLower(target), "__mdt") {
					t.Errorf("%s.%s contains project reference target %s", name, fieldName, target)
				}
			}
		}
	}
}

func TestStandardSchemaMergePreservesDeclaredCustomReferences(t *testing.T) {
	def := ObjectDefinition{APIName: "ContentVersion", Fields: map[string]Field{
		"LocalParent__c": {APIName: "LocalParent__c", Type: FieldReference, ReferenceTo: []string{"pkg__LocalRecord__c"}},
	}}
	EnsureStandardObjectFields(&def)
	if got := def.Fields["LocalParent__c"].ReferenceTo; len(got) != 1 || got[0] != "pkg__LocalRecord__c" {
		t.Fatalf("project reference changed: %v", got)
	}
	if !stringSliceContains(def.Fields["FirstPublishLocationId"].ReferenceTo, "Account") {
		t.Fatal("standard Account reference lost")
	}
	for _, name := range []string{"Knowledge__ka", "Knowledge__kav", "Knowledge__Feed"} {
		if !IsKnownStandardObject(name) {
			t.Errorf("Knowledge shape lost: %s", name)
		}
	}
}
