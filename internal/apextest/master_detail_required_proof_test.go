package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF214 exact API62/project40 implicit MasterDetail requiredness assertions.
func TestMasterDetailRequiredAPI62Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRequiredMaster62Proof.cls"), `@IsTest private class GladeRequiredMaster62Proof {
 private static List<SObject> records() {
  return new List<SObject>{new Campaign(Name='Owned invalid batch'),new Contact(LastName='Owned invalid batch'),new GladeRequiredJob62__c(Name='Owned invalid batch',Campaign__c=null)};
 }
 private static void validMasterControl() {
  Campaign parent=new Campaign(Name='Owned valid parent');insert parent;
  GladeRequiredJob62__c child=new GladeRequiredJob62__c(Name='Owned valid child',Campaign__c=parent.Id);insert child;
  System.assertEquals(parent.Id,[SELECT Campaign__c FROM GladeRequiredJob62__c WHERE Id=:child.Id].Campaign__c);
 }
 @IsTest static void missingMasterRollsBackAllRecords() {
  List<SObject> rows=records();
  Database.DMLOptions options=new Database.DMLOptions();options.OptAllOrNone=true;
  Boolean caught=false;
  try {Database.insert(rows,options);} catch(DmlException error) {
   caught=true;System.assert(error.getMessage().contains('REQUIRED_FIELD_MISSING'));
  }
  System.assert(caught,'missing MasterDetail parent must reject');
  for(SObject row:rows){System.assertEquals(null,row.get('Id'));}
  System.assertEquals(0,[SELECT COUNT() FROM Campaign WHERE Name='Owned invalid batch']);
  System.assertEquals(0,[SELECT COUNT() FROM Contact WHERE LastName='Owned invalid batch']);
  System.assertEquals(0,[SELECT COUNT() FROM GladeRequiredJob62__c WHERE Name='Owned invalid batch']);
  validMasterControl();
 }
 @IsTest static void missingMasterFailsOnlyInvalidRecord() {
  List<SObject> rows=records();
  Database.DMLOptions options=new Database.DMLOptions();options.OptAllOrNone=false;
  Database.SaveResult[] results=Database.insert(rows,options);
  System.assertEquals(3,results.size());
  System.assert(results[0].isSuccess());System.assert(results[1].isSuccess());System.assert(!results[2].isSuccess());
  System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING,results[2].getErrors()[0].getStatusCode());
  System.assertNotEquals(null,rows[0].get('Id'));System.assertNotEquals(null,rows[1].get('Id'));System.assertEquals(null,rows[2].get('Id'));
  System.assertEquals(1,[SELECT COUNT() FROM Campaign WHERE Name='Owned invalid batch']);
  System.assertEquals(1,[SELECT COUNT() FROM Contact WHERE LastName='Owned invalid batch']);
  System.assertEquals(0,[SELECT COUNT() FROM GladeRequiredJob62__c WHERE Name='Owned invalid batch']);
  validMasterControl();
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRequiredMaster62Proof.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRequiredJob62__c/GladeRequiredJob62__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Required Job</label><nameField><label>Job Name</label><type>Text</type></nameField><pluralLabel>Owned Required Jobs</pluralLabel><sharingModel>ControlledByParent</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeRequiredJob62__c/fields/Campaign__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Campaign__c</fullName><externalId>false</externalId><label>Campaign</label><referenceTo>Campaign</referenceTo><relationshipLabel>Owned Required Jobs</relationshipLabel><relationshipName>GladeRequiredJobs62</relationshipName><relationshipOrder>0</relationshipOrder><reparentableMasterDetail>true</reparentableMasterDetail><trackTrending>false</trackTrending><type>MasterDetail</type><writeRequiresMasterRead>true</writeRequiresMasterRead></CustomField>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		b, _ := json.Marshal(run)
		t.Fatalf("MasterDetail proof: %s", b)
	}
}

func TestOptionalLookupRemainsNullable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/OwnedOptional62__c/OwnedOptional62__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Optional</label><pluralLabel>Owned Optional</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/OwnedOptional62__c/fields/Parent__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><type>Lookup</type><referenceTo>Account</referenceTo><relationshipName>OwnedOptional62</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OptionalLookup62.cls"), `@IsTest private class OptionalLookup62 {
 @IsTest static void missingLookupIsAllowed(){
  OwnedOptional62__c row=new OwnedOptional62__c(Name='nullable',Parent__c=null);insert row;
  System.assertNotEquals(null,row.Id);
  System.assertEquals(null,[SELECT Parent__c FROM OwnedOptional62__c WHERE Id=:row.Id].Parent__c);
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OptionalLookup62.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("nullable lookup: %s", b)
	}
}
