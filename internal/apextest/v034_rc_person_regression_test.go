package apextest

import (
	"path/filepath"
	"testing"
)

const v034RCClassMeta = `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>`

func runV034RCProject(t *testing.T, root, class string) {
	t.Helper()
	index := loadTestIndex(t, root)
	if index.HasErrors() {
		t.Fatal(index.Diagnostics)
	}
	result := Run(index, Options{SelectedClasses: []string{class}, Parallelism: 1})
	if summary := result.Summary(); summary.Total == 0 || summary.Passed != summary.Total {
		t.Fatalf("%+v %s", summary, firstRunProblem(result))
	}
}

// An Id-only merge master supplies no Name. Trigger hydration of the stored
// person account must not turn its stored Name into a supplied write.
func TestV034RCPersonAccountMergeIdOnlyMaster(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
	writeFile(t, filepath.Join(base, "classes/PersonMergeProbeTest.cls"), `@IsTest private class PersonMergeProbeTest {
static Account person(String last) { Id rt=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true LIMIT 1].Id; Account a=new Account(RecordTypeId=rt,FirstName='Synthetic',LastName=last); insert a; return a; }
@IsTest static void mergeIdOnly() { Account a=person('One'); Account b=person('Two'); merge new Account(Id=a.Id) new Account(Id=b.Id); System.assertEquals(1,[SELECT COUNT() FROM Account]); System.assertEquals('Synthetic One',[SELECT Name FROM Account WHERE Id=:a.Id].Name); }
@IsTest static void databaseMergeIdOnly() { Account a=person('One'); Account b=person('Two'); Database.MergeResult r=Database.merge(new Account(Id=a.Id),b.Id,false); System.assertEquals(true,r.isSuccess()); }
}`)
	writeFile(t, filepath.Join(base, "classes/PersonMergeProbeTest.cls-meta.xml"), v034RCClassMeta)
	runV034RCProject(t, root, "PersonMergeProbeTest")
}

// A PersonAccount record type's label is metadata. Apex that selects the type
// by that label inserts a person account with FirstName and LastName.
func TestV034RCPersonRecordTypeSelectedByProjectLabel(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	writeFile(t, filepath.Join(base, "objects/Account/Account.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/Account/recordTypes/Company.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Company</fullName><active>true</active><label>Company</label></RecordType>`)
	writeFile(t, filepath.Join(base, "objects/PersonAccount/PersonAccount.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><sharingModel>Read</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/PersonAccount/recordTypes/PersonAccount.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName><active>true</active><label>Customer Person</label></RecordType>`)
	writeFile(t, filepath.Join(base, "classes/PersonLabelProbeTest.cls"), `@IsTest private class PersonLabelProbeTest {
@IsTest static void insertByLabel() { Account a=new Account(RecordTypeId=Schema.SObjectType.Account.getRecordTypeInfosByName().get('Customer Person').getRecordTypeId(),FirstName='Synthetic',LastName='Person'); insert a; System.assertEquals(true,[SELECT IsPersonAccount FROM Account WHERE Id=:a.Id].IsPersonAccount); }
}`)
	writeFile(t, filepath.Join(base, "classes/PersonLabelProbeTest.cls-meta.xml"), v034RCClassMeta)
	runV034RCProject(t, root, "PersonLabelProbeTest")
}

// A required lookup filter that names the PersonAccount record type label
// keeps that label on the folded type, so a person account selected by the
// label satisfies the filter.
func TestV034RCPersonRecordTypeLabelInRequiredLookupFilter(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	writeFile(t, filepath.Join(base, "objects/Account/Account.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/Account/recordTypes/Company.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Company</fullName><active>true</active><label>Company</label></RecordType>`)
	writeFile(t, filepath.Join(base, "objects/PersonAccount/PersonAccount.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><sharingModel>Read</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/PersonAccount/recordTypes/PersonAccount.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName><active>true</active><label>Customer Person</label></RecordType>`)
	writeFile(t, filepath.Join(base, "objects/Probe__c/Probe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Probe</label><pluralLabel>Probes</pluralLabel><nameField><label>Probe Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/Probe__c/fields/Account__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Account__c</fullName><label>Account</label><type>Lookup</type><referenceTo>Account</referenceTo><relationshipName>Probes</relationshipName><deleteConstraint>SetNull</deleteConstraint><lookupFilter><active>true</active><filterItems><field>Account.RecordTypeId</field><operation>equals</operation><value>Customer Person</value></filterItems><isOptional>false</isOptional></lookupFilter></CustomField>`)
	writeFile(t, filepath.Join(base, "classes/PersonFilterProbeTest.cls"), `@IsTest private class PersonFilterProbeTest {
@IsTest static void filteredLookup() { Account a=new Account(RecordTypeId=Schema.SObjectType.Account.getRecordTypeInfosByName().get('Customer Person').getRecordTypeId(),FirstName='Synthetic',LastName='Person'); insert a; Probe__c p=new Probe__c(Name='Probe',Account__c=a.Id); insert p; System.assertEquals(true,[SELECT IsPersonAccount FROM Account WHERE Id=:a.Id].IsPersonAccount); }
}`)
	writeFile(t, filepath.Join(base, "classes/PersonFilterProbeTest.cls-meta.xml"), v034RCClassMeta)
	runV034RCProject(t, root, "PersonFilterProbeTest")
}
