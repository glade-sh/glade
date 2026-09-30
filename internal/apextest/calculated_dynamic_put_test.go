package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF227 exact API62/project40 calculated dynamic-put assertions.
func TestCalculatedDynamicPutSF227(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCalculatedWrite62Assertions.cls"), `@IsTest private class GladeCalculatedWrite62Assertions {
 private static void verify(SObject row,String field,Object attempted,Object expected) {
  System.assertEquals(expected,row.get(field),'before '+field);
  Boolean rejected=false;
  try {row.put(field,attempted);} catch(Exception error) {
   rejected=true;
   System.assertEquals('System.SObjectException',error.getTypeName());
   System.assertEquals('Field '+field+' is not editable',error.getMessage());
  }
  System.assertEquals(true,rejected,'put must reject '+field);
  System.assertEquals(expected,row.get(field),'after '+field);
 }
 @IsTest static void assertCalculatedDynamicWriteBoundary() {
  Contact contact=new Contact(FirstName='Owned',LastName='Person');insert contact;
  GladeCalcParent62__c parent=new GladeCalcParent62__c(Name='Parent');insert parent;
  GladeCalcChild62__c child=new GladeCalcChild62__c(Name='Child',Parent__c=parent.Id,Contact__c=contact.Id,Amount__c=3);insert child;
  GladeCalcChild62__c fresh=new GladeCalcChild62__c(Name='Fresh',Parent__c=parent.Id,Amount__c=1);
  System.assertEquals(null,fresh.Full_Name__c);
  verify(fresh,'Full_Name__c','Injected',null);
  GladeCalcChild62__c populated=new GladeCalcChild62__c(Name='Populated',Parent__c=parent.Id,Contact__c=contact.Id,Amount__c=2);
  System.assertEquals(null,populated.Full_Name__c);
  verify(populated,'Full_Name__c','Injected',null);
  System.assertEquals(null,child.Full_Name__c);
  verify(child,'Full_Name__c','Injected',null);
  GladeCalcChild62__c queried=[SELECT Id,Name,Parent__c,Contact__c,Amount__c,Full_Name__c FROM GladeCalcChild62__c WHERE Id=:child.Id];
  System.assertEquals('Person, Owned',queried.Full_Name__c);
  verify(queried,'Full_Name__c','Injected','Person, Owned');
  GladeCalcParent62__c freshParent=new GladeCalcParent62__c(Name='Fresh parent');
  System.assertEquals(null,freshParent.Total__c);
  verify(freshParent,'Total__c',99,null);
  System.assertEquals(null,parent.Total__c);
  verify(parent,'Total__c',99,null);
  GladeCalcParent62__c queriedParent=[SELECT Id,Name,Total__c FROM GladeCalcParent62__c WHERE Id=:parent.Id];
  System.assertEquals(3,queriedParent.Total__c);
  verify(queriedParent,'Total__c',99,3);
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeCalculatedWrite62Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcParent62__c/GladeCalcParent62__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned GladeCalcParent62__c</label><pluralLabel>Owned records</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcChild62__c/GladeCalcChild62__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned GladeCalcChild62__c</label><pluralLabel>Owned records</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><sharingModel>ControlledByParent</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcChild62__c/fields/Parent__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent__c</label><referenceTo>GladeCalcParent62__c</referenceTo><relationshipLabel>Children</relationshipLabel><relationshipName>Children</relationshipName><reparentableMasterDetail>false</reparentableMasterDetail><type>MasterDetail</type><writeRequiresMasterRead>false</writeRequiresMasterRead></CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcChild62__c/fields/Contact__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Contact__c</fullName><label>Contact__c</label><deleteConstraint>SetNull</deleteConstraint><referenceTo>Contact</referenceTo><relationshipLabel>Owned Calc Children</relationshipLabel><relationshipName>GladeCalcChildren62</relationshipName><required>false</required><type>Lookup</type></CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcChild62__c/fields/Amount__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Amount__c</fullName><label>Amount__c</label><precision>10</precision><scale>0</scale><type>Number</type></CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcChild62__c/fields/Full_Name__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Full_Name__c</fullName><label>Full_Name__c</label><formula>Contact__r.LastName &amp; &quot;, &quot; &amp; Contact__r.FirstName</formula><type>Text</type></CustomField>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeCalcParent62__c/fields/Total__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Total__c</fullName><label>Total__c</label><summarizedField>GladeCalcChild62__c.Amount__c</summarizedField><summaryForeignKey>GladeCalcChild62__c.Parent__c</summaryForeignKey><summaryOperation>sum</summaryOperation><type>Summary</type></CustomField>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("calculated put: %s", b)
	}
}
