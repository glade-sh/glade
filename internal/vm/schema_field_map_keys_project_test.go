package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Owned API53 source accepted by SF138; no plugin dependency at test runtime.
func TestSchemaFieldMapKeysAPI53Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion": "53.0", "packageDirectories": [{"path": "force-app", "default": true}]}`,
		"force-app/main/default/classes/GladeSchemaMap138.cls": `@IsTest private class GladeSchemaMap138 {
    @IsTest static void declaredMixedCaseKeys() {
        Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
        System.assertEquals(true,fields.containsKey('GladeLookup138__c'));
        System.assertEquals(true,fields.containsKey('gladelookup138__c'));
        System.assertEquals(Account.GladeLookup138__c,fields.get('GLADELOOKUP138__C'));
        System.assertEquals('GladeLookup138__c',fields.get('GladeLookup138__c').getDescribe().getName());
        Account parent=new Account(Name='Parent');insert parent;
        Account child=new Account(Name='Child',GladeLookup138__c=parent.Id);insert child;
        System.assertEquals(parent.Id,[SELECT GladeLookup138__c FROM Account WHERE Id=:child.Id].GladeLookup138__c);
    }
    @IsTest static void undeclaredPrefixDoesNotAliasField() {
        Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
        System.assertEquals(true,fields.containsKey('GladeLookup138__c'));
        System.assertEquals(false,fields.containsKey('absent138__GladeLookup138__c'),'Undeclared namespace cannot alias owned field');
        System.assertEquals(null,fields.get('absent138__GladeLookup138__c'));
        System.assertEquals(false,fields.keySet().contains('absent138__gladelookup138__c'));
    }
    @IsTest static void doubledPrefixDoesNotAliasField() {
        Map<String,Schema.SObjectField> fields=Account.SObjectType.getDescribe().fields.getMap();
        System.assertEquals(true,fields.containsKey('GladeLookup138__c'));
        System.assertEquals(false,fields.containsKey('outer138__absent138__GladeLookup138__c'),'Double namespace cannot alias owned field');
        System.assertEquals(null,fields.get('outer138__absent138__GladeLookup138__c'));
        System.assertEquals(false,fields.containsKey('absent138__Name'),'Absent prefixed standard field');
        System.assertEquals(null,fields.get('absent138__Name'));
    }
}
`,
		"force-app/main/default/classes/GladeSchemaMap138.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/objects/Account/fields/GladeLookup138__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>GladeLookup138__c</fullName><deleteConstraint>SetNull</deleteConstraint><label>Owned Lookup 138</label><referenceTo>Account</referenceTo><relationshipLabel>Owned Children 138</relationshipLabel><relationshipName>GladeChildren138</relationshipName><required>false</required><type>Lookup</type></CustomField>
`,
	}
	for name, source := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, schema.Schema{})
	run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
	count := 0
	for _, suite := range run.Suites {
		for _, tc := range suite.Cases {
			count++
			t.Run(tc.MethodName, func(t *testing.T) {
				if tc.Status != testreport.StatusPass {
					t.Fatalf("%s: %+v", tc.Status, tc.Problem)
				}
			})
		}
	}
	if count != 3 {
		t.Fatalf("executed %d methods; want all three", count)
	}
}
