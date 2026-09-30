package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// SF183 admitted the original API53 static Map field-token contract.
func TestQualifiedMetadataFieldTokenAPI53Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"force-app/main/default/classes/GladePkgxFieldToken53Proof.cls":                                "@IsTest private class GladePkgxFieldToken53Proof {\n private static final Map<SObjectField, Set<String>> UNSUPPORTED_OPERATIONS_BY_ROLLUP_FIELD = new Map<SObjectField, Set<String>> {\n  Schema.GladePkgxRollup53__mdt.Date_Field__c => new Set<String> {} // All Operations Supported\n };\n @IsTest static void qualifiedMetadataFieldTokenInitializesMap() {\n  Schema.SObjectField direct=Schema.GladePkgxRollup53__mdt.Date_Field__c;\n  Schema.SObjectField described=GladePkgxRollup53__mdt.SObjectType.getDescribe().fields.getMap().get('Date_Field__c');\n  System.assertNotEquals(null,described);\n  System.assertEquals(described,direct);\n  System.assertEquals('Date_Field__c',direct.getDescribe().getName());\n  System.assertEquals(1,UNSUPPORTED_OPERATIONS_BY_ROLLUP_FIELD.size());\n  System.assert(UNSUPPORTED_OPERATIONS_BY_ROLLUP_FIELD.containsKey(described));\n  System.assertEquals(0,UNSUPPORTED_OPERATIONS_BY_ROLLUP_FIELD.get(described).size());\n }\n}\n",
		"force-app/main/default/classes/GladePkgxFieldToken53Proof.cls-meta.xml":                       "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n",
		"force-app/main/default/objects/GladePkgxRollup53__mdt/GladePkgxRollup53__mdt.object-meta.xml": "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><label>Glade PKGX Rollup</label><pluralLabel>Glade PKGX Rollups</pluralLabel><visibility>Public</visibility></CustomObject>\n",
		"force-app/main/default/objects/GladePkgxRollup53__mdt/fields/Date_Object__c.field-meta.xml":   "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Date_Object__c</fullName>\n    <externalId>false</externalId>\n    <fieldManageability>DeveloperControlled</fieldManageability>\n    <label>Date Object</label>\n    <referenceTo>EntityDefinition</referenceTo>\n    <relationshipLabel>RollupsDate</relationshipLabel>\n    <relationshipName>RollupsDate</relationshipName>\n    <required>false</required>\n    <type>MetadataRelationship</type>\n    <unique>false</unique>\n</CustomField>\n",
		"force-app/main/default/objects/GladePkgxRollup53__mdt/fields/Date_Field__c.field-meta.xml":    "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Date_Field__c</fullName>\n    <externalId>false</externalId>\n    <fieldManageability>DeveloperControlled</fieldManageability>\n    <label>Date Field</label>\n    <metadataRelationshipControllingField>GladePkgxRollup53__mdt.Date_Object__c</metadataRelationshipControllingField>\n    <referenceTo>FieldDefinition</referenceTo>\n    <relationshipLabel>RollupsDate</relationshipLabel>\n    <relationshipName>RollupsDate</relationshipName>\n    <required>false</required>\n    <type>MetadataRelationship</type>\n    <unique>false</unique>\n</CustomField>\n",
		"sfdx-project.json": "{\"namespace\": \"\", \"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}",
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true})
	if got := r.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		t.Fatalf("qualified field token: %+v; %+v", got, r)
	}
}
