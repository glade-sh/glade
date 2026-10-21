package storage

import "testing"

func TestKnowledgeLanguageRepairsPersistedCreationPolicy(t *testing.T) {
	definition := ObjectDefinition{APIName: "Knowledge__kav"}
	EnsureStandardObjectFields(&definition)
	field := definition.Fields["Language"]
	field.Createable = BoolFlag(false)
	definition.Fields["Language"] = field
	if !StandardObjectFieldsNeedWrite(definition) {
		t.Fatal("older non-createable Language overlay was retained")
	}
	EnsureStandardObjectFields(&definition)
	field = definition.Fields["Language"]
	if field.Createable == nil || !*field.Createable || field.Updateable == nil || *field.Updateable {
		t.Fatal("native create-only Language facets were not restored")
	}
	if StandardObjectFieldsNeedWrite(definition) {
		t.Fatal("corrected Knowledge overlay remains dirty")
	}
	version := definition.Fields["VersionNumber"]
	if version.Createable == nil || *version.Createable {
		t.Fatal("unrelated system field became createable")
	}
}
