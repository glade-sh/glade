package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF243 exact API62/project40 MasterDetail reparentability assertion.
func TestMasterDetailReparentabilitySF243(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"40.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeReparentableMasterAssertions241.cls"), `@IsTest private class GladeReparentableMasterAssertions241 {
 @IsTest static void assertMasterDetailReparentability() {
  GladeReparentParent241__c first=new GladeReparentParent241__c(Name='first');insert first;GladeReparentParent241__c second=new GladeReparentParent241__c(Name='second');insert second;
  GladeReparentTrue241__c enabled=new GladeReparentTrue241__c(Name='enabled',Parent__c=first.Id);insert enabled;
  System.assertEquals(true,GladeReparentTrue241__c.Parent__c.getDescribe().isUpdateable(),'reparentable describe updateable');
  enabled.Parent__c=second.Id;update enabled;GladeReparentTrue241__c enabledStored=[SELECT Id,Parent__c FROM GladeReparentTrue241__c WHERE Id=:enabled.Id];System.assertEquals(second.Id,enabledStored.Parent__c,'reparented parent persisted');
  GladeReparentFalse241__c disabled=new GladeReparentFalse241__c(Name='disabled',Parent__c=first.Id);insert disabled;
  System.assertEquals(false,GladeReparentFalse241__c.Parent__c.getDescribe().isUpdateable(),'non-reparentable describe updateable');Boolean rejected=false;
  try{disabled.Parent__c=first.Id;}catch(SObjectException error){rejected=true;System.assertEquals('System.SObjectException',error.getTypeName(),'same parent assignment type');System.assertEquals('Field is not writeable: GladeReparentFalse241__c.Parent__c',error.getMessage(),'same parent assignment message');}
  System.assertEquals(true,rejected,'same parent assignment rejected');disabled.Name='disabled-updated';update disabled;GladeReparentFalse241__c disabledStored=[SELECT Id,Name,Parent__c FROM GladeReparentFalse241__c WHERE Id=:disabled.Id];System.assertEquals(first.Id,disabledStored.Parent__c,'Name-only update retains parent');System.assertEquals('disabled-updated',disabledStored.Name,'Name-only update persists');
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeReparentableMasterAssertions241.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	for path, content := range map[string]string{
		"force-app/main/default/objects/GladeReparentParent241__c/GladeReparentParent241__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Glade Reparent Parent 241</label><nameField><label>Glade Reparent Parent 241 Name</label><type>Text</type></nameField><pluralLabel>Glade Reparent Parents 241</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>
`,
		"force-app/main/default/objects/GladeReparentTrue241__c/GladeReparentTrue241__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Glade Reparent True 241</label><nameField><label>Glade Reparent True 241 Name</label><type>Text</type></nameField><pluralLabel>Glade Reparent Trues 241</pluralLabel><sharingModel>ControlledByParent</sharingModel></CustomObject>
`,
		"force-app/main/default/objects/GladeReparentFalse241__c/GladeReparentFalse241__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Glade Reparent False 241</label><nameField><label>Glade Reparent False 241 Name</label><type>Text</type></nameField><pluralLabel>Glade Reparent Falses 241</pluralLabel><sharingModel>ControlledByParent</sharingModel></CustomObject>
`,
		"force-app/main/default/objects/GladeReparentTrue241__c/fields/Parent__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><externalId>false</externalId><label>Parent</label><referenceTo>GladeReparentParent241__c</referenceTo><relationshipLabel>GladeReparentTrue241 Children</relationshipLabel><relationshipName>GladeReparentTrue241</relationshipName><relationshipOrder>0</relationshipOrder><reparentableMasterDetail>true</reparentableMasterDetail><trackFeedHistory>false</trackFeedHistory><trackTrending>false</trackTrending><type>MasterDetail</type><writeRequiresMasterRead>true</writeRequiresMasterRead></CustomField>
`,
		"force-app/main/default/objects/GladeReparentFalse241__c/fields/Parent__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><externalId>false</externalId><label>Parent</label><referenceTo>GladeReparentParent241__c</referenceTo><relationshipLabel>GladeReparentFalse241 Children</relationshipLabel><relationshipName>GladeReparentFalse241</relationshipName><relationshipOrder>0</relationshipOrder><reparentableMasterDetail>false</reparentableMasterDetail><trackFeedHistory>false</trackFeedHistory><trackTrending>false</trackTrending><type>MasterDetail</type><writeRequiresMasterRead>true</writeRequiresMasterRead></CustomField>
`,
	} {
		writeFile(t, filepath.Join(root, path), content)
	}
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		body, _ := json.Marshal(run)
		t.Fatalf("SF243 MasterDetail reparentability: %s", body)
	}
}
