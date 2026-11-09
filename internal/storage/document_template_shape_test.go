package storage

import "testing"

func TestDocumentTemplateContentDocumentFields(t *testing.T) {
	definition, ok := StandardObjectDefinition("documenttemplatecontentdoc")
	if !ok || definition.APIName != "DocumentTemplateContentDoc" {
		t.Fatalf("document template content document missing: %#v, %v", definition, ok)
	}
	for name, want := range map[string]FieldType{"Id": FieldID, "Name": FieldString} {
		if field, ok := definition.Fields[name]; !ok || field.Type != want {
			t.Errorf("%s = %#v, %v; want %s", name, field, ok, want)
		}
	}
	// b5b9_feature_objects/org.tsv:858-863 captures these reference facets
	// independently at API 62 and 67 in an isolated DocGen org.
	for name, want := range map[string]struct {
		parent, relationship   string
		createable, updateable bool
	}{
		"DocumentTemplateId":     {"DocumentTemplate", "DocumentTemplate", true, false},
		"ContentDocumentId":      {"ContentDocument", "ContentDocument", true, true},
		"LatestContentVersionId": {"ContentVersion", "LatestContentVersion", false, false},
	} {
		field, ok := definition.Fields[name]
		if !ok || field.Type != FieldReference || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != want.parent || field.RelationshipName != want.relationship || field.Createable == nil || *field.Createable != want.createable || field.Updateable == nil || *field.Updateable != want.updateable {
			t.Errorf("captured reference %s = %#v, %v", name, field, ok)
		}
		found := false
		for _, relation := range definition.Relations {
			if relation.Field == name && relation.ParentRelationship == want.relationship && len(relation.ParentObjects) == 1 && relation.ParentObjects[0] == want.parent {
				found = true
			}
		}
		if !found {
			t.Errorf("captured relationship missing for %s", name)
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
