package vm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCase349HostRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/case349"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			// Native decimal equality rows: case349_review/org.tsv:2-4,10-12.
			parentBody := `Account pw=new Account(Name='Parent wrong',Decimal_Key__c=1.25); Account pright=new Account(Name='Parent right',Decimal_Key__c=2.500); Contact pc=new Contact(LastName='Stored'); pc.putSObject('Account',new Account(Decimal_Key__c=2.5)); insert new List<Account>{pw,pright}; upsert pc; System.assertEquals(pright.Id,[SELECT AccountId FROM Contact WHERE Id=:pc.Id].AccountId);
Account qw=new Account(Name='Pending wrong',Decimal_Key__c=3.75); Account qright=new Account(Name='Pending right',Decimal_Key__c=4.500); Contact qc=new Contact(LastName='Pending'); qc.putSObject('Account',new Account(Decimal_Key__c=4.5)); insert new List<SObject>{qw,qright,qc}; System.assertEquals(qright.Id,[SELECT AccountId FROM Contact WHERE Id=:qc.Id].AccountId); `
			body := `EqualityProbe__c a=new EqualityProbe__c(Key__c=1.25); insert a;
EqualityProbe__c b=new EqualityProbe__c(Key__c=2.50); Database.UpsertResult r=Database.upsert(b,EqualityProbe__c.Key__c,false);
EqualityProbe__c c=new EqualityProbe__c(Key__c=1.250); Database.UpsertResult equalResult=Database.upsert(c,EqualityProbe__c.Key__c,false);
Account business=new Account(Name='Business probe',PersonEmail='   '); Database.SaveResult s=Database.insert(business,false);
Id rt=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Person_Commerce').getRecordTypeId();
Account phone=new Account(Name='Phone probe',RecordTypeId=rt,PersonHomePhone='2125550100'); Database.SaveResult pr=Database.insert(phone,false);
Account named=new Account(Name='Name probe',RecordTypeId=rt); named.put('FirstName',null); Database.SaveResult nr=Database.insert(named,false);
Account defaultPhone=new Account(Name='Default phone',PersonMobilePhone='2125550100'); Database.SaveResult dr=Database.insert(defaultPhone,false);
Account malformed=new Account(Name='Malformed email',PersonEmail='x'); Database.SaveResult er=Database.insert(malformed,false);
Account mixedOnly=new Account(Name='Mixed only',PersonMemo__c='synthetic'); insert mixedOnly; System.assertEquals(false,[SELECT IsPersonAccount FROM Account WHERE Id=:mixedOnly.Id].IsPersonAccount);
Account mixedPhone=new Account(Name='Mixed phone',PersonMemo__c='synthetic',PersonHomePhone='4155550123'); Database.SaveResult mixResult=Database.insert(mixedPhone,false); System.assertEquals(false,mixResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(mixResult.getErrors()[0].getStatusCode()));
Account mixedTitle=new Account(Name='Mixed title',RecordTypeId=rt,PersonTitle='Synthetic',PersonHomePhone='4155550123'); Database.SaveResult titleResult=Database.insert(mixedTitle,false); System.assertEquals(false,titleResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(titleResult.getErrors()[0].getStatusCode()));
Account extBusiness=new Account(Name='External business',ProbeKey__c='business'); insert extBusiness; Case349TriggerCounter.hits=0;
Account extFirst=new Account(ProbeKey__c='business',FirstName='Synthetic'); Database.UpsertResult extFirstResult=Database.upsert(extFirst,Account.Fields.ProbeKey__c,false);
System.assertEquals(false,extFirstResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(extFirstResult.getErrors()[0].getStatusCode())); System.assertEquals(0,Case349TriggerCounter.hits);
Account extEmail=new Account(ProbeKey__c='business',PersonEmail='invalid'); Database.UpsertResult extEmailResult=Database.upsert(extEmail,Account.Fields.ProbeKey__c,false);
System.assertEquals(false,extEmailResult.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(extEmailResult.getErrors()[0].getStatusCode())); System.assertEquals(0,Case349TriggerCounter.hits);
Account extNew=new Account(Name='Unmatched',ProbeKey__c='unmatched',FirstName='Synthetic'); Database.UpsertResult extNewResult=Database.upsert(extNew,Account.Fields.ProbeKey__c,false);
System.assertEquals(false,extNewResult.isSuccess()); System.assertEquals(true,extNewResult.isCreated()); System.assertEquals(0,Case349TriggerCounter.hits);
Account nativeBatchSecond=new Account(Name='Batch second',ProbeKey__c='batch-second'); insert nativeBatchSecond;
Id nativeBatchType=[SELECT RecordTypeId FROM Account WHERE Id=:extBusiness.Id].RecordTypeId;
for(String batchOp:new List<String>{'insert','update','upsert'}) {
 List<Account> mixedBatch;
 if(batchOp=='insert') mixedBatch=new List<Account>{new Account(Name='First',PersonEmail='invalid'),new Account(Name='Second',RecordTypeId=nativeBatchType,FirstName='Synthetic')};
 else if(batchOp=='update') mixedBatch=new List<Account>{new Account(Id=extBusiness.Id,PersonEmail='invalid'),new Account(Id=nativeBatchSecond.Id,FirstName='Synthetic')};
 else mixedBatch=new List<Account>{new Account(ProbeKey__c='business',PersonEmail='invalid'),new Account(ProbeKey__c='batch-second',FirstName='Synthetic')};
 Case349TriggerCounter.hits=0; Boolean mixedCaught=false;
 try { if(batchOp=='insert') Database.insert(mixedBatch,true); else if(batchOp=='update') Database.update(mixedBatch,true); else Database.upsert(mixedBatch,Account.Fields.ProbeKey__c,true); }
 catch(DmlException e) { mixedCaught=true; System.assertEquals(2,e.getNumDml()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(e.getDmlStatusCode(0))); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(e.getDmlStatusCode(1))); }
 System.assertEquals(true,mixedCaught); System.assertEquals(0,Case349TriggerCounter.hits);
}
Account personSeed=new Account(LastName='Synthetic Person'); insert personSeed;
Account personUpdate=new Account(Id=personSeed.Id,PersonHomePhone='4155550123');
System.assertEquals(true,Database.update(personUpdate,false).isSuccess());
Account queriedPerson=[SELECT Id,Name FROM Account WHERE Id=:personSeed.Id]; queriedPerson.PersonHomePhone='4155550123'; Case349TriggerCounter.hits=0; Database.SaveResult queriedResult=Database.update(queriedPerson,false); System.assertEquals(false,queriedResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(queriedResult.getErrors()[0].getStatusCode())); System.assertEquals(0,Case349TriggerCounter.hits);
Account componentFirst=[SELECT Id,Name FROM Account WHERE Id=:personSeed.Id]; componentFirst.FirstName='Changed'; Case349TriggerCounter.hits=0; System.assertEquals(false,Database.update(componentFirst,false).isSuccess()); System.assertEquals(0,Case349TriggerCounter.hits);
Account componentLast=[SELECT Id,Name FROM Account WHERE Id=:personSeed.Id]; componentLast.LastName='Changed'; System.assertEquals(false,Database.upsert(componentLast,false).isSuccess());
Account componentControl=[SELECT Id FROM Account WHERE Id=:personSeed.Id]; componentControl.FirstName='Changed'; System.assertEquals(true,Database.update(componentControl,false).isSuccess());
Account idSeed=new Account(Name='Id seed'); insert idSeed;
Account idFirst=new Account(Id=idSeed.Id,FirstName='Synthetic'); Database.UpsertResult idFirstResult=Database.upsert(idFirst,Account.Fields.Id,false);
System.assertEquals(false,idFirstResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(idFirstResult.getErrors()[0].getStatusCode()));
Account idBad=new Account(Id=idSeed.Id,PersonEmail='bad'); Database.UpsertResult idBadResult=Database.upsert(idBad,Account.Fields.Id,false);
System.assertEquals(false,idBadResult.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(idBadResult.getErrors()[0].getStatusCode()));
Account idBlank=new Account(Id=idSeed.Id,PersonEmail='   '); System.assertEquals(true,Database.upsert(idBlank,Account.Fields.Id,false).isSuccess());
Account reading=new Account(Name='Read probe',Balance__c=1.250); insert reading; Account partial=[SELECT Id FROM Account WHERE Id=:reading.Id]; Boolean hidden=false;
try { Decimal ignored=partial.Balance__c; } catch(SObjectException ex) { hidden=true; }
` + parentBody + `return r.isSuccess()+','+r.isCreated()+','+(a.Id==b.Id)+','+equalResult.isCreated()+','+s.isSuccess()+','+[SELECT IsPersonAccount FROM Account WHERE Id=:business.Id].IsPersonAccount+','+pr.isSuccess()+','+String.valueOf(pr.getErrors()[0].getStatusCode())+','+nr.isSuccess()+','+String.valueOf(nr.getErrors()[0].getStatusCode())+','+dr.isSuccess()+','+String.valueOf(dr.getErrors()[0].getStatusCode())+','+er.isSuccess()+','+String.valueOf(er.getErrors()[0].getStatusCode())+','+hidden;`
			files := map[string]string{
				"sfdx-project.json":                                                           fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api),
				"config/project-scratch-def.json":                                             `{"edition":"Developer","features":["PersonAccounts"]}`,
				"force-app/main/default/classes/Case349TriggerCounter.cls":                    "public class Case349TriggerCounter { public static Integer hits=0; }",
				"force-app/main/default/classes/Case349TriggerCounter.cls-meta.xml":           fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api),
				"force-app/main/default/triggers/Case349Observer.trigger":                     "trigger Case349Observer on Account (before insert, before update) { Case349TriggerCounter.hits++; }",
				"force-app/main/default/triggers/Case349Observer.trigger-meta.xml":            fmt.Sprintf(`<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api),
				"force-app/main/default/objects/Account/fields/Decimal_Key__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Decimal_Key__c</fullName><label>Decimal Key</label><type>Number</type><precision>12</precision><scale>3</scale><externalId>true</externalId></CustomField>`,
				"force-app/main/default/classes/Case349Host.cls":                              "global class Case349Host { @AuraEnabled global static String lwc() { " + body + " } global String vf() { " + body + " } }",
				"force-app/main/default/classes/Case349Host.cls-meta.xml":                     fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api),
				"force-app/main/default/objects/Account/fields/ProbeKey__c.field-meta.xml":    `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>ProbeKey__c</fullName><label>Probe Key</label><length>80</length><type>Text</type><externalId>true</externalId><unique>true</unique></CustomField>`,
				"force-app/main/default/objects/Account/fields/PersonMemo__c.field-meta.xml":  `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonMemo__c</fullName><label>Person Memo</label><length>100</length><type>Text</type></CustomField>`,
				"force-app/main/default/objects/Account/fields/Balance__c.field-meta.xml":     `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Balance__c</fullName><label>Balance</label><type>Number</type><precision>12</precision><scale>3</scale><defaultValue>1.25</defaultValue></CustomField>`,
			}
			for _, suffix := range []string{"EqualityProbe__c.object-meta.xml", "fields/Key__c.field-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(fixture, "objects/EqualityProbe__c", suffix))
				if err != nil {
					t.Fatal(err)
				}
				files["force-app/main/default/objects/EqualityProbe__c/"+suffix] = string(data)
			}
			// Native capture: testdata/conformance/case349/ORG-RUN.txt assigns this
			// permission set to the administrator; deployment alone grants nothing.
			permissionSet, err := os.ReadFile("../../testdata/conformance/case349/permissionsets/Case349Access.permissionset-meta.xml")
			if err != nil {
				t.Fatal(err)
			}
			// The Account.ProbeKey__c upserts below run in default mode; the
			// standard_ext_upsert rows deny an ungranted custom key to the
			// administrator, so the assigned set also grants the key.
			grantedSet := strings.Replace(string(permissionSet), "</PermissionSet>", "<fieldPermissions><editable>true</editable><field>Account.ProbeKey__c</field><readable>true</readable></fieldPermissions></PermissionSet>", 1)
			files["force-app/main/default/permissionsets/Case349Access.permissionset-meta.xml"] = string(permissionSet)
			files["force-app/main/default/permissionsets/Case349Access.permissionset-meta.xml"] = grantedSet
			files["glade.yml"] = "test:\n  permissionSets: [Case349Access]\n"
			recordType, err := os.ReadFile("../../testdata/conformance/person_account_followups/force-app/main/default/objects/Account/recordTypes/Business_Default.recordType-meta.xml")
			if err != nil {
				t.Fatal(err)
			}
			files["force-app/main/default/objects/Account/recordTypes/Person_Commerce.recordType-meta.xml"] = strings.ReplaceAll(string(recordType), "Business_Default", "Person_Commerce")
			for name, source := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
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
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			for _, route := range []string{"lwc", "vf"} {
				t.Run(route, func(t *testing.T) {
					machine := vm.New(nil)
					org, err := apextest.OrgFromIndexWithConfiguredPermissions(index)
					if err != nil {
						t.Fatal(err)
					}
					org.APIVersion = api
					machine.SetOrg(&org)
					if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
						t.Fatal(err)
					}
					var result vm.UIInvocationResult
					if route == "lwc" {
						result, err = machine.InvokeLWCMethod("Case349Host", "lwc", nil)
					} else {
						result, err = machine.InvokeVisualforceAction("Case349Host", "vf", "/apex/Case349Host", nil)
					}
					if err != nil || !result.Success {
						t.Fatalf("host: %v %#v", err, result.Error)
					}
					if result.ReturnValue != "true,true,false,false,true,false,false,INVALID_FIELD_FOR_INSERT_UPDATE,false,INVALID_FIELD_FOR_INSERT_UPDATE,false,INVALID_FIELD_FOR_INSERT_UPDATE,false,INVALID_EMAIL_ADDRESS,true" {
						t.Fatalf("got %#v", result.ReturnValue)
					}
				})
			}
		})
	}
}
