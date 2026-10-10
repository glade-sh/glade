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

// SF187 admitted the eleven API63 JSON field-order observations for custom metadata.
func TestCustomMetadataJSONFieldOrderAPI63Project(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"63.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/classes/GladeCmtOrder187Proof.cls": `@IsTest private class GladeCmtOrder187Proof {
 @IsTest static void preservesFieldAssignmentOrder() {
  Map<String,String> rows=new Map<String,String>();
  GladeCmtOrder187__mdt standardFirst=new GladeCmtOrder187__mdt();
  standardFirst.DeveloperName='Owned'; standardFirst.Label='Owned Label'; standardFirst.Flag_Value__c='Owned Value'; standardFirst.isEnabled__c=true;
  rows.put('standardFirst',JSON.serialize(standardFirst));
  GladeCmtOrder187__mdt customFirst=new GladeCmtOrder187__mdt();
  customFirst.isEnabled__c=true; customFirst.Flag_Value__c='Owned Value'; customFirst.Label='Owned Label'; customFirst.DeveloperName='Owned';
  rows.put('customFirstReverse',JSON.serialize(customFirst));
  GladeCmtOrder187__mdt interleaved=new GladeCmtOrder187__mdt();
  interleaved.Flag_Value__c='Owned Value'; interleaved.DeveloperName='Owned'; interleaved.isEnabled__c=true; interleaved.Label='Owned Label';
  rows.put('interleaved',JSON.serialize(interleaved));
  GladeCmtOrder187__mdt constructed=new GladeCmtOrder187__mdt(isEnabled__c=true,Flag_Value__c='Owned Value',Label='Owned Label',DeveloperName='Owned');
  rows.put('constructorReverse',JSON.serialize(constructed));
  GladeCmtOrder187__mdt constructorStandard=new GladeCmtOrder187__mdt(DeveloperName='Owned',Label='Owned Label',Flag_Value__c='Owned Value',isEnabled__c=true);
  rows.put('constructorStandardFirst',JSON.serialize(constructorStandard));
  GladeCmtOrder187__mdt dynamicRecord=new GladeCmtOrder187__mdt();
  dynamicRecord.put('isEnabled__c',true); dynamicRecord.put('Flag_Value__c','Owned Value'); dynamicRecord.put('Label','Owned Label'); dynamicRecord.put('DeveloperName','Owned');
  rows.put('putReverse',JSON.serialize(dynamicRecord));
  customFirst.DeveloperName='Updated'; customFirst.Label='Updated Label'; customFirst.Flag_Value__c='Updated Value'; customFirst.isEnabled__c=false;
  rows.put('updateExisting',JSON.serialize(customFirst));
  dynamicRecord.put('DeveloperName','Updated'); dynamicRecord.put('Label','Updated Label'); dynamicRecord.put('Flag_Value__c','Updated Value'); dynamicRecord.put('isEnabled__c',false);
  rows.put('putUpdateExisting',JSON.serialize(dynamicRecord));
  standardFirst.isEnabled_After__c=Date.newInstance(2026,1,2); rows.put('appendDate',JSON.serialize(standardFirst));
  standardFirst.Flag_Value__c=null; rows.put('existingToNull',JSON.serialize(standardFirst));
  standardFirst.Flag_Value__c='Restored Value'; rows.put('restoreExisting',JSON.serialize(standardFirst));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"DeveloperName":"Owned","Label":"Owned Label","Flag_Value__c":"Owned Value","isEnabled__c":true}',rows.get('standardFirst'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"isEnabled__c":true,"Flag_Value__c":"Owned Value","Label":"Owned Label","DeveloperName":"Owned"}',rows.get('customFirstReverse'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"Flag_Value__c":"Owned Value","DeveloperName":"Owned","isEnabled__c":true,"Label":"Owned Label"}',rows.get('interleaved'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"isEnabled__c":true,"Flag_Value__c":"Owned Value","Label":"Owned Label","DeveloperName":"Owned"}',rows.get('constructorReverse'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"DeveloperName":"Owned","Label":"Owned Label","Flag_Value__c":"Owned Value","isEnabled__c":true}',rows.get('constructorStandardFirst'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"isEnabled__c":true,"Flag_Value__c":"Owned Value","Label":"Owned Label","DeveloperName":"Owned"}',rows.get('putReverse'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"isEnabled__c":false,"Flag_Value__c":"Updated Value","Label":"Updated Label","DeveloperName":"Updated"}',rows.get('updateExisting'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"isEnabled__c":false,"Flag_Value__c":"Updated Value","Label":"Updated Label","DeveloperName":"Updated"}',rows.get('putUpdateExisting'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"DeveloperName":"Owned","Label":"Owned Label","Flag_Value__c":"Owned Value","isEnabled__c":true,"isEnabled_After__c":"2026-01-02"}',rows.get('appendDate'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"DeveloperName":"Owned","Label":"Owned Label","Flag_Value__c":null,"isEnabled__c":true,"isEnabled_After__c":"2026-01-02"}',rows.get('existingToNull'));
  System.assertEquals('{"attributes":{"type":"GladeCmtOrder187__mdt"},"DeveloperName":"Owned","Label":"Owned Label","Flag_Value__c":"Restored Value","isEnabled__c":true,"isEnabled_After__c":"2026-01-02"}',rows.get('restoreExisting'));
 }
}`,
		"force-app/main/default/classes/GladeCmtOrder187Proof.cls-meta.xml":                             `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>`,
		"force-app/main/default/objects/GladeCmtOrder187__mdt/GladeCmtOrder187__mdt.object-meta.xml":    `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Feature Flag</label><pluralLabel>Feature Flags</pluralLabel><visibility>Public</visibility></CustomObject>`,
		"force-app/main/default/objects/GladeCmtOrder187__mdt/fields/Flag_Value__c.field-meta.xml":      `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag_Value__c</fullName><label>Flag Value</label><length>255</length><type>Text</type></CustomField>`,
		"force-app/main/default/objects/GladeCmtOrder187__mdt/fields/isEnabled__c.field-meta.xml":       `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>isEnabled__c</fullName><label>isEnabled</label><type>Checkbox</type></CustomField>`,
		"force-app/main/default/objects/GladeCmtOrder187__mdt/fields/isEnabled_After__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>isEnabled_After__c</fullName><label>isEnabled On or After</label><type>Date</type></CustomField>`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
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
		t.Fatalf("custom metadata JSON field order: %+v; %+v", got, r)
	}
}
