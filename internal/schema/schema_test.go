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
	recordTypePath := filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Business.recordType-meta.xml")
	lowercaseRecordTypePath := filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Consumer.recordtype-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><type>Picklist</type><referenceTo>Thing__c</referenceTo><referenceTo>Account</referenceTo><relationshipName>Parent__r</relationshipName><childRelationshipName>Children__r</childRelationshipName><deleteConstraint>Cascade</deleteConstraint><valueSet><valueSetDefinition><value><fullName>Hot</fullName><default>true</default><label>Hot Label</label></value><value><fullName>Cold</fullName><isActive>false</isActive></value></valueSetDefinition></valueSet></CustomField>`)
	writeFile(t, recordTypePath, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Business</fullName><label>Business Thing</label><active>true</active><default>true</default><description>Business records</description></RecordType>`)
	writeFile(t, lowercaseRecordTypePath, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><label>Consumer Thing</label><active>false</active></RecordType>`)

	s, err := LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{fieldPath}, RecordTypeFiles: []string{recordTypePath, lowercaseRecordTypePath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 || s.Objects[0].Name != "Thing__c" {
		t.Fatalf("objects = %#v", s.Objects)
	}
	if len(s.Objects[0].Fields) != 1 || len(s.Objects[0].Fields[0].ReferenceTo) != 2 || s.Objects[0].Fields[0].ReferenceTo[0] != "Thing__c" || s.Objects[0].Fields[0].ReferenceTo[1] != "Account" {
		t.Fatalf("fields = %#v", s.Objects[0].Fields)
	}
	if got := s.Objects[0].Fields[0].DeleteConstraint; got != "Cascade" {
		t.Fatalf("delete constraint = %q", got)
	}
	if got := s.Objects[0].Fields[0].ChildRelationshipName; got != "Children__r" {
		t.Fatalf("child relationship name = %q", got)
	}
	values := s.Objects[0].Fields[0].PicklistValues
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
