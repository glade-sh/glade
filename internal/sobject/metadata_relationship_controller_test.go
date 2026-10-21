package sobject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// Metadata API CustomField.metadataRelationshipControllingField is the explicit
// EntityDefinition controller for a FieldDefinition metadata relationship.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/api_meta.pdf
func TestMetadataRelationshipControllerImportRoundTrip(t *testing.T) {
	for _, controller := range []string{"Object__c", "Binding__mdt.Object__c", "other__Binding__mdt.other__Object__c", ""} {
		t.Run(controller, func(t *testing.T) {
			root := t.TempDir()
			objectPath := filepath.Join(root, "objects", "Binding__mdt", "Binding__mdt.object-meta.xml")
			fieldPath := filepath.Join(root, "objects", "Binding__mdt", "fields", "Field__c.field-meta.xml")
			if err := os.MkdirAll(filepath.Dir(fieldPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(objectPath, []byte(`<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Binding</label><pluralLabel>Bindings</pluralLabel><visibility>Public</visibility></CustomObject>`), 0644); err != nil {
				t.Fatal(err)
			}
			metadata := `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Field__c</fullName><type>MetadataRelationship</type><referenceTo>FieldDefinition</referenceTo><metadataRelationshipControllingField>` + controller + `</metadataRelationshipControllingField></CustomField>`
			if err := os.WriteFile(fieldPath, []byte(metadata), 0644); err != nil {
				t.Fatal(err)
			}
			loaded, err := schema.LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{fieldPath}})
			if err != nil {
				t.Fatal(err)
			}
			var imported schema.Field
			for _, object := range loaded.Objects {
				if object.Name == "Binding__mdt" {
					imported = object.Fields[0]
				}
			}
			described := BuildDescribeRegistry(loaded).Objects["Binding__mdt"]
			stored := ToObjectDefinition(described)
			restored := FromObjectDefinition(stored)
			// Inspect the serialized public carriers so this regression compiles on the
			// base where the metadata attribute has no Go struct field at all.
			for stage, value := range map[string]any{"schema": imported, "describe": described.Fields["Field__c"], "storage": stored.Fields["Field__c"], "restored": restored.Fields["Field__c"]} {
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var properties map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &properties); err != nil {
					t.Fatal(err)
				}
				got := ""
				if raw := properties["metadataRelationshipControllingField"]; raw != nil {
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatal(err)
					}
				}
				if got != controller {
					t.Errorf("%s controller=%q, want explicit metadata %q", stage, got, controller)
				}
			}
		})
	}
}
