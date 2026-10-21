package storage

import "testing"

func TestDocumentTemplateContentDocumentFields(t *testing.T) {
	definition, ok := StandardObjectDefinition("documenttemplatecontentdoc")
	if !ok || definition.APIName != "DocumentTemplateContentDoc" {
		t.Fatalf("document template content document missing: %#v, %v", definition, ok)
	}
	for name, want := range map[string]FieldType{
		"Id": FieldID, "Name": FieldString, "DocumentTemplateId": FieldID,
		"ContentDocumentId": FieldID, "LatestContentVersionId": FieldID,
	} {
		if field, ok := definition.Fields[name]; !ok || field.Type != want {
			t.Errorf("%s = %#v, %v; want %s", name, field, ok, want)
		}
	}
	// Query examples establish ID values, not describe facets or forward
	// relationship names. Do not manufacture those from the ID field names.
	for _, name := range []string{"DocumentTemplateId", "ContentDocumentId", "LatestContentVersionId"} {
		field := definition.Fields[name]
		if len(field.ReferenceTo) != 0 || field.RelationshipName != "" || field.Createable != nil || field.Updateable != nil {
			t.Errorf("query shape invented field metadata: %#v", field)
		}
		for _, relation := range definition.Relations {
			if relation.Field == name {
				t.Errorf("query shape invented relationship: %#v", relation)
			}
		}
	}
	if _, ok := ResolveFieldName(definition, "", "InventedField"); ok {
		t.Fatal("query shape admitted an undocumented field")
	}
}

func TestDocumentTemplateShapePreservesProjectFields(t *testing.T) {
	definition := ObjectDefinition{APIName: "DocumentTemplateContentDoc", Fields: map[string]Field{
		"Name":      {APIName: "Name", Type: FieldString, Label: "Configured label", Length: 37},
		"Source__c": {APIName: "Source__c", Type: FieldString},
	}}
	EnsureStandardObjectFields(&definition)
	EnsureStandardObjectFields(&definition)
	if name := definition.Fields["Name"]; name.Label != "Configured label" || name.Length != 37 {
		t.Fatalf("project field overridden: %#v", name)
	}
	if _, ok := definition.Fields["Source__c"]; !ok {
		t.Fatal("project field lost")
	}
	if _, ok := StandardObjectDefinition("MissingTemplateShape"); ok {
		t.Fatal("unknown type admitted")
	}
}
