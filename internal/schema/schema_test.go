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
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><type>Lookup</type><referenceTo>Thing__c</referenceTo><relationshipName>Parent__r</relationshipName><childRelationshipName>Children__r</childRelationshipName><deleteConstraint>Cascade</deleteConstraint></CustomField>`)

	s, err := LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{fieldPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 || s.Objects[0].Name != "Thing__c" {
		t.Fatalf("objects = %#v", s.Objects)
	}
	if len(s.Objects[0].Fields) != 1 || s.Objects[0].Fields[0].ReferenceTo != "Thing__c" {
		t.Fatalf("fields = %#v", s.Objects[0].Fields)
	}
	if got := s.Objects[0].Fields[0].DeleteConstraint; got != "Cascade" {
		t.Fatalf("delete constraint = %q", got)
	}
	if got := s.Objects[0].Fields[0].ChildRelationshipName; got != "Children__r" {
		t.Fatalf("child relationship name = %q", got)
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
