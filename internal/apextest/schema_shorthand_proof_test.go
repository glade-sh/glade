package apextest

import (
	"path/filepath"
	"testing"
)

func TestSchemaFieldShorthandAPI52(t *testing.T) {
	for _, ns := range []string{"", "hed"} {
		t.Run(ns, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSchemaShorthand52Proof.cls"), "@IsTest private class GladeSchemaShorthand52Proof {\n @IsTest static void standardFieldName() {\n  System.assertEquals('LastName',Schema.Contact.fields.LastName.getDescribe().getName());\n  System.assertEquals(Contact.LastName,Schema.Contact.fields.LastName);\n }\n @IsTest static void customFieldLabelAndStorage() {\n  System.assertEquals('Glade Schema Shorthand',Schema.Contact.fields.GladeSchemaShorthand__c.getDescribe().getLabel());\n  System.assertEquals(Contact.GladeSchemaShorthand__c,Schema.Contact.fields.GladeSchemaShorthand__c);\n  Contact row=new Contact(LastName='Owned',GladeSchemaShorthand__c='stored');insert row;\n  Contact saved=[SELECT GladeSchemaShorthand__c FROM Contact WHERE Id=:row.Id];\n  System.assertEquals('stored',saved.GladeSchemaShorthand__c);\n }\n}\n")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSchemaShorthand52Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>52.0</apiVersion><status>Active</status></ApexClass>")
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/GladeSchemaShorthand__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>GladeSchemaShorthand__c</fullName><label>Glade Schema Shorthand</label><length>40</length><type>Text</type></CustomField>")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"52.0","namespace":"`+ns+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
			if s := run.Summary(); s.Total != 2 || s.Passed != 2 || s.Errors != 0 {
				t.Fatalf("shorthand proof: %#v", run)
			}
		})
	}
}
