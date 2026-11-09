package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestCase349NativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/case349"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{}
	for _, row := range rows[1:] {
		if row[1] != "anonymous" {
			continue
		}
		if want[row[0]] == nil {
			want[row[0]] = map[string]string{}
		}
		want[row[0]][row[2]] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			cases := want[api]
			if cases["K1-distinct"] != "success=true,created=true,sameId=false" || cases["K1-equal"] != "success=true,created=false,sameId=true" || cases["K4-spaces"] != "success=true,isPerson=false" {
				t.Fatalf("native rows missing: %v", cases)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			objectRoot := filepath.Join(root, "force-app/main/default/objects/EqualityProbe__c")
			writeFile(t, filepath.Join(objectRoot, "EqualityProbe__c.object-meta.xml"), readFixtureFile(t, filepath.Join(fixture, "objects/EqualityProbe__c/EqualityProbe__c.object-meta.xml")))
			writeFile(t, filepath.Join(objectRoot, "fields/Key__c.field-meta.xml"), readFixtureFile(t, filepath.Join(fixture, "objects/EqualityProbe__c/fields/Key__c.field-meta.xml")))
			// ORG-RUN.txt: the permission set is assigned to the scratch administrator.
			writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/Case349Access.permissionset-meta.xml"), readFixtureFile(t, filepath.Join(fixture, "permissionsets/Case349Access.permissionset-meta.xml")))
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Case349Access]\n")
			for _, name := range []string{"Person_Commerce", "Individual_Business"} {
				path := "objects/Account/recordTypes/" + name + ".recordType-meta.xml"
				writeFile(t, filepath.Join(root, "force-app/main/default", path), readFixtureFile(t, filepath.Join("../../testdata/conformance/case349_review/force-app/main/default", path)))
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Balance__c.field-meta.xml"), readFixtureFile(t, "../../testdata/conformance/case349_unqueried/force-app/main/default/objects/ReadProbe__c/fields/Balance__c.field-meta.xml"))
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/ProbeKey__c.field-meta.xml"), readFixtureFile(t, "../../testdata/conformance/case349_external/force-app/main/default/objects/Account/fields/ProbeKey__c.field-meta.xml"))
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/PersonMemo__c.field-meta.xml"), readFixtureFile(t, "../../testdata/conformance/case349_mixed/force-app/main/default/objects/Account/fields/PersonMemo__c.field-meta.xml"))
			source := `
EqualityProbe__c a = new EqualityProbe__c(Key__c=1.25); insert a;
EqualityProbe__c b = new EqualityProbe__c(Key__c=2.50);
Database.UpsertResult r=Database.upsert(b, EqualityProbe__c.Key__c, false);
System.assertEquals(true,r.isSuccess(),'distinct success'); System.assertEquals(true,r.isCreated(),'distinct created'); System.assertNotEquals(a.Id,b.Id,'distinct identity');
EqualityProbe__c c = new EqualityProbe__c(Key__c=1.25);
Database.UpsertResult s=Database.upsert(c, EqualityProbe__c.Key__c, false);
System.assertEquals(true,s.isSuccess(),'equal success'); System.assertEquals(false,s.isCreated(),'equal created'); System.assertEquals(a.Id,c.Id,'equal identity');
EqualityProbe__c scaled = new EqualityProbe__c(Key__c=1.250);
Database.UpsertResult scaledResult=Database.upsert(scaled,EqualityProbe__c.Key__c,false);
System.assertEquals(false,scaledResult.isCreated(),'scaled created'); System.assertEquals(a.Id,scaled.Id,'scaled identity');
for (String typeName : new List<String>{'Person_Commerce','Individual_Business'}) {
  Id businessType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get(typeName).getRecordTypeId();
  Account candidate=new Account(Name='Business',RecordTypeId=businessType,PersonHomePhone='4155550123');
  Database.SaveResult candidateResult=Database.insert(candidate,false);
  System.assertEquals(false,candidateResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(candidateResult.getErrors()[0].getStatusCode()));
}
Account mixedOnly=new Account(Name='Mixed only',PersonMemo__c='synthetic'); insert mixedOnly;
System.assertEquals(false,[SELECT IsPersonAccount FROM Account WHERE Id=:mixedOnly.Id].IsPersonAccount);
Account mixedPhone=new Account(Name='Mixed phone',PersonMemo__c='synthetic',PersonHomePhone='4155550123');
Database.SaveResult mixedResult=Database.insert(mixedPhone,false); System.assertEquals(false,mixedResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(mixedResult.getErrors()[0].getStatusCode()));
Id mixedType=Account.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Person_Commerce').getRecordTypeId();
Account mixedTitle=new Account(Name='Mixed title',RecordTypeId=mixedType,PersonTitle='Synthetic',PersonHomePhone='4155550123');
Database.SaveResult titleResult=Database.insert(mixedTitle,false); System.assertEquals(false,titleResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(titleResult.getErrors()[0].getStatusCode()));
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
Account idSeed=new Account(Name='Explicit Id seed'); insert idSeed;
Account idFirst=new Account(Id=idSeed.Id, FirstName='Synthetic');
Database.UpsertResult idFirstResult=Database.upsert(idFirst,Account.Fields.Id,false);
System.assertEquals(false,idFirstResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(idFirstResult.getErrors()[0].getStatusCode()));
Account idBad=new Account(Id=idSeed.Id, PersonEmail='bad');
Database.UpsertResult idBadResult=Database.upsert(idBad,Account.Fields.Id,false);
System.assertEquals(false,idBadResult.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(idBadResult.getErrors()[0].getStatusCode()));
Account idBlank=new Account(Id=idSeed.Id, PersonEmail='   ');
System.assertEquals(true,Database.upsert(idBlank,Account.Fields.Id,false).isSuccess());
Account reading=new Account(Name='Read probe',Balance__c=1.250); insert reading;
Account partial=[SELECT Id FROM Account WHERE Id=:reading.Id]; Boolean hidden=false;
try { Decimal ignored=partial.Balance__c; } catch(SObjectException ex) { hidden=true; }
System.assertEquals(true,hidden,'unqueried numeric default');
Account business = new Account(Name='Business probe',PersonEmail='   ');
Database.SaveResult saved=Database.insert(business,false);
System.assertEquals(true,saved.isSuccess(),'person whitespace success');
System.assertEquals(false,[SELECT IsPersonAccount FROM Account WHERE Id=:business.Id].IsPersonAccount,'person whitespace identity');
`
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "Case349Test.cls"), "@IsTest private class Case349Test { @IsTest static void exercise() { "+source+" } }")
			writeFile(t, filepath.Join(classes, "Case349Test.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			writeFile(t, filepath.Join(classes, "Case349TriggerCounter.cls"), "public class Case349TriggerCounter { public static Integer hits=0; }")
			writeFile(t, filepath.Join(classes, "Case349TriggerCounter.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			triggers := filepath.Join(root, "force-app/main/default/triggers")
			writeFile(t, filepath.Join(triggers, "Case349Observer.trigger"), "trigger Case349Observer on Account (before insert, before update) { Case349TriggerCounter.hits++; }")
			writeFile(t, filepath.Join(triggers, "Case349Observer.trigger-meta.xml"), fmt.Sprintf(`<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			index := loadTestIndex(t, root)
			t.Run("anonymous", func(t *testing.T) {
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = runner.newMachine().Execute(program); err != nil {
					t.Fatal(err)
				}
			})
			for _, route := range []string{"project", "warm"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"Case349Test"}, Parallelism: 1, ParallelMethods: true})
					if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("%s: %+v %s", route, summary, strings.TrimSpace(fmt.Sprint(result.Suites)))
					}
				})
			}
		})
	}
}
