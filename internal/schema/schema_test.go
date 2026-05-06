package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
)

func TestLoadProject(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml")
	fieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/Parent__c.field-meta.xml")
	formulaFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/Score__c.field-meta.xml")
	encryptedFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/Secret__c.field-meta.xml")
	rootFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/Legacy__c.field-meta.xml")
	recordTypePath := filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Business.recordType-meta.xml")
	lowercaseRecordTypePath := filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Consumer.recordtype-meta.xml")
	validationRulePath := filepath.Join(root, "force-app/main/objects/Thing__c/validationRules/Block.validationRule-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><type>Picklist</type><referenceTo>Thing__c</referenceTo><referenceTo>Account</referenceTo><relationshipName>Parent__r</relationshipName><childRelationshipName>Children__r</childRelationshipName><deleteConstraint>Cascade</deleteConstraint><valueSet><valueSetDefinition><value><fullName>Hot</fullName><default>true</default><label>Hot Label</label></value><value><fullName>Cold</fullName><isActive>false</isActive></value></valueSetDefinition></valueSet></CustomField>`)
	writeFile(t, formulaFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Score__c</fullName><label>Score</label><type>Number</type><formula>1 + 1</formula></CustomField>`)
	writeFile(t, encryptedFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Secret__c</fullName><label>Secret</label><type>EncryptedText</type></CustomField>`)
	writeFile(t, rootFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Legacy__c</fullName><label>Legacy</label><type>Text</type></CustomField>`)
	writeFile(t, recordTypePath, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Business</fullName><label>Business Thing</label><active>true</active><default>true</default><description>Business records</description></RecordType>`)
	writeFile(t, lowercaseRecordTypePath, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><label>Consumer Thing</label><active>false</active></RecordType>`)
	writeFile(t, validationRulePath, `<ValidationRule xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Block</fullName><active>true</active><errorConditionFormula>Parent__c = "Blocked"</errorConditionFormula><errorMessage>blocked by rule</errorMessage><errorDisplayField>Parent__c</errorDisplayField></ValidationRule>`)

	s, err := LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{fieldPath, formulaFieldPath, encryptedFieldPath, rootFieldPath}, RecordTypeFiles: []string{recordTypePath, lowercaseRecordTypePath}, ValidationRuleFiles: []string{validationRulePath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 || s.Objects[0].Name != "Thing__c" {
		t.Fatalf("objects = %#v", s.Objects)
	}
	if len(s.Objects[0].Fields) != 4 {
		t.Fatalf("fields = %#v", s.Objects[0].Fields)
	}
	fields := fieldsByName(s.Objects[0].Fields)
	parent := fields["Parent__c"]
	if len(parent.ReferenceTo) != 2 || parent.ReferenceTo[0] != "Thing__c" || parent.ReferenceTo[1] != "Account" {
		t.Fatalf("parent field = %#v", parent)
	}
	if _, ok := fields["Legacy__c"]; !ok {
		t.Fatalf("missing root object field: %#v", s.Objects[0].Fields)
	}
	if got := fields["Score__c"].Formula; got != "1 + 1" {
		t.Fatalf("formula = %q", got)
	}
	if !fields["Secret__c"].Encrypted {
		t.Fatalf("encrypted field = %#v", fields["Secret__c"])
	}
	if got := parent.DeleteConstraint; got != "Cascade" {
		t.Fatalf("delete constraint = %q", got)
	}
	if got := parent.ChildRelationshipName; got != "Children__r" {
		t.Fatalf("child relationship name = %q", got)
	}
	values := parent.PicklistValues
	if len(values) != 2 || values[0].FullName != "Hot" || !values[0].Default || !values[0].Active || values[0].Label != "Hot Label" {
		t.Fatalf("picklist values = %#v", values)
	}
	if values[1].Active {
		t.Fatalf("inactive picklist value marked active: %#v", values[1])
	}
	recordTypes := s.Objects[0].RecordTypes
	if len(recordTypes) != 2 || recordTypes[0].DeveloperName != "Business" || recordTypes[0].Label != "Business Thing" || !recordTypes[0].Active || !recordTypes[0].Default {
		t.Fatalf("record types = %#v", recordTypes)
	}
	if recordTypes[1].DeveloperName != "Consumer" || recordTypes[1].Label != "Consumer Thing" || recordTypes[1].Active {
		t.Fatalf("record types = %#v", recordTypes)
	}
	rules := s.Objects[0].ValidationRules
	if len(rules) != 1 || rules[0].Name != "Block" || !rules[0].Active || rules[0].ErrorMessage != "blocked by rule" || rules[0].ErrorDisplayField != "Parent__c" {
		t.Fatalf("validation rules = %#v", rules)
	}
}

func fieldsByName(fields []Field) map[string]Field {
	out := make(map[string]Field, len(fields))
	for _, field := range fields {
		out[field.Name] = field
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
