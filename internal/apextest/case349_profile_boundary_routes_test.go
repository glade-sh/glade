package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Person-profile named tests and warm project execution for the native
// pre-trigger upsert rows at case349_profile_boundary/org.tsv:26-31,62-67.
func TestCase349ProfileBoundaryNamedProjectRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/case349_profile_boundary/force-app/main/default"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			for _, path := range []string{"objects/Account/fields/ProbeKey__c.field-meta.xml", "objects/Account/recordTypes/Business_Default.recordType-meta.xml", "objects/PersonAccount/recordTypes/Provider.recordType-meta.xml", "profiles/Person Probe.profile-meta.xml"} {
				writeFile(t, filepath.Join(base, path), readFixtureFile(t, filepath.Join(fixture, path)))
			}
			// Decimal external-ID equality: case349_review/org.tsv:2-4,10-12.
			writeFile(t, filepath.Join(base, "objects/Account/fields/Decimal_Key__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Decimal_Key__c</fullName><label>Decimal Key</label><type>Number</type><precision>12</precision><scale>3</scale><externalId>true</externalId></CustomField>`)
			meta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			writeFile(t, filepath.Join(base, "classes/BoundaryTrace.cls"), `public class BoundaryTrace { public static Integer hits=0; }`)
			writeFile(t, filepath.Join(base, "classes/BoundaryTrace.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(base, "triggers/BoundaryObserver.trigger"), `trigger BoundaryObserver on Account (before insert) { BoundaryTrace.hits++; }`)
			writeFile(t, filepath.Join(base, "triggers/BoundaryObserver.trigger-meta.xml"), fmt.Sprintf(`<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			source := `@IsTest private class BoundaryProfileTest { @IsTest static void run() {
Profile p=[SELECT Id FROM Profile WHERE Name='Person Probe']; User u=new User(ProfileId=p.Id,Username='probe@example.invalid',Email='probe@example.invalid',Alias='probe',LastName='Synthetic',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u;
Account pw=new Account(Name='Parent wrong',Decimal_Key__c=1.25); Account pright=new Account(Name='Parent right',Decimal_Key__c=2.500); Contact pc=new Contact(LastName='Stored'); pc.putSObject('Account',new Account(Decimal_Key__c=2.5)); insert new List<Account>{pw,pright}; upsert pc; System.assertEquals(pright.Id,[SELECT AccountId FROM Contact WHERE Id=:pc.Id].AccountId);
Account qw=new Account(Name='Pending wrong',Decimal_Key__c=3.75); Account qright=new Account(Name='Pending right',Decimal_Key__c=4.500); Contact qc=new Contact(LastName='Pending'); qc.putSObject('Account',new Account(Decimal_Key__c=4.5)); insert new List<SObject>{qw,qright,qc}; System.assertEquals(qright.Id,[SELECT AccountId FROM Contact WHERE Id=:qc.Id].AccountId);
System.runAs(u) { Id personType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Provider').getRecordTypeId(); Id businessType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Business_Default').getRecordTypeId();
Account a=new Account(ProbeKey__c='first',Name='Synthetic',FirstName='Synthetic'); BoundaryTrace.hits=0; Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,r.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals(true,r.isCreated()); System.assertEquals(0,BoundaryTrace.hits);
Account b=new Account(ProbeKey__c='second',Name='Synthetic',FirstName='Synthetic',RecordTypeId=personType); BoundaryTrace.hits=0; Database.UpsertResult s=Database.upsert(b,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,s.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(s.getErrors()[0].getStatusCode())); System.assertEquals(0,BoundaryTrace.hits); System.assertEquals('Name',s.getErrors()[0].getFields()[0]); System.assertEquals('Unable to create/update fields: Name. Please check the security settings of this field and verify that it is read/write for your profile or permission set.',s.getErrors()[0].getMessage());
Account c=new Account(ProbeKey__c='third',Name='Synthetic',FirstName='Synthetic',RecordTypeId=businessType); BoundaryTrace.hits=0; Database.UpsertResult t=Database.upsert(c,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,t.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(t.getErrors()[0].getStatusCode())); System.assertEquals(0,BoundaryTrace.hits); System.assertEquals('FirstName',t.getErrors()[0].getFields()[0]); System.assertEquals('Account: bad field names on insert/update call: FirstName',t.getErrors()[0].getMessage());
Account d=new Account(ProbeKey__c='fourth',Name='Synthetic',FirstName='Synthetic',RecordTypeId='012000000000099'); BoundaryTrace.hits=0; Database.UpsertResult v=Database.upsert(d,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,v.isSuccess()); System.assertEquals('INVALID_CROSS_REFERENCE_KEY',String.valueOf(v.getErrors()[0].getStatusCode())); System.assertEquals(0,BoundaryTrace.hits); System.assertEquals('RecordTypeId',v.getErrors()[0].getFields()[0]);
for(Integer i=0;i<3;i++){ Account invalidExtra=new Account(ProbeKey__c='invalid-'+i,Name='Synthetic',FirstName='Synthetic',RecordTypeId='012000000000099'); if(i==0)invalidExtra.BillingCity='Synthetic'; if(i==1)invalidExtra.put('BillingCity',null); if(i==2){invalidExtra.LastName='Person';invalidExtra.PersonTitle='Synthetic';} BoundaryTrace.hits=0; Database.UpsertResult ir=Database.upsert(invalidExtra,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,ir.isSuccess()); System.assertEquals('INVALID_CROSS_REFERENCE_KEY',String.valueOf(ir.getErrors()[0].getStatusCode())); System.assertEquals('RecordTypeId',ir.getErrors()[0].getFields()[0]); System.assertEquals(true,ir.isCreated()); System.assertEquals(0,BoundaryTrace.hits); }
for(String f:new List<String>{'PersonMobilePhone','PersonEmail'}) { Account mixed=new Account(Name='Synthetic',FirstName='Synthetic',LastName='Person',RecordTypeId=businessType); mixed.put(f,f=='PersonEmail'?'synthetic@example.invalid':'4155550123'); BoundaryTrace.hits=0; Database.SaveResult mr=Database.insert(mixed,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,mr.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(mr.getErrors()[0].getStatusCode())); System.assertEquals(0,BoundaryTrace.hits); }
Account e=new Account(ProbeKey__c='fifth',Name='Synthetic',FirstName='Synthetic',LastName='Person'); BoundaryTrace.hits=0; Database.UpsertResult w=Database.upsert(e,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,w.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(w.getErrors()[0].getStatusCode())); System.assertEquals(0,BoundaryTrace.hits);
Account f=new Account(ProbeKey__c='sixth',Name='Synthetic',FirstName='Synthetic',PersonTitle='Synthetic'); BoundaryTrace.hits=0; Database.UpsertResult x=Database.upsert(f,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); System.assertEquals(false,x.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(x.getErrors()[0].getStatusCode())); System.assertEquals(0,BoundaryTrace.hits);
} } }`
			writeFile(t, filepath.Join(base, "classes/BoundaryProfileTest.cls"), source)
			writeFile(t, filepath.Join(base, "classes/BoundaryProfileTest.cls-meta.xml"), meta)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for _, route := range []string{"project", "warm"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"BoundaryProfileTest"}, Parallelism: 1, ParallelMethods: true})
					if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("%s: %+v %s", route, summary, firstRunProblem(result))
					}
				})
			}
		})
	}
}
