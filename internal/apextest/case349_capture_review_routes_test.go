package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Named and warm project routes for trigger and external shapes: case349_trigger_capture/org.tsv:3,19-21,24,40-42.
func TestCase349CapturedReviewNamedProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["PersonAccounts"]}`)
			writeFile(t, filepath.Join(base, "objects/Account/fields/ProbeKey__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>ProbeKey__c</fullName><label>Probe Key</label><length>80</length><type>Text</type><externalId>true</externalId><unique>true</unique></CustomField>`)
			// The upserts run in default mode; an ungranted custom key is not visible to the
			// administrator (standard_ext_upsert rows), so the profile grants it.
			writeFile(t, filepath.Join(base, "profiles/Admin.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom><fieldPermissions><editable>true</editable><field>Account.ProbeKey__c</field><readable>true</readable></fieldPermissions></Profile>`)
			meta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			writeFile(t, filepath.Join(base, "triggers/CaptureObserver.trigger"), `trigger CaptureObserver on Account (before update) { for(Account a:Trigger.new) a.Name=a.Name; }`)
			writeFile(t, filepath.Join(base, "triggers/CaptureObserver.trigger-meta.xml"), fmt.Sprintf(`<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			source := `@IsTest private class CaptureRouteTest { @IsTest static void run() {
Account seed=new Account(LastName='Synthetic'); insert seed; Account a=new Account(Id=seed.Id,PersonHomePhone='4155550123'); Database.SaveResult s=Database.update(a,false); System.assertEquals(true,s.isSuccess());
Account actual=[SELECT Name FROM Account WHERE Id=:seed.Id]; System.assertEquals('Synthetic',actual.Name);
Account queried=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; queried.PersonHomePhone='4155550123'; Database.SaveResult qr=Database.update(queried,false); System.assertEquals(false,qr.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(qr.getErrors()[0].getStatusCode()));
Account idOnly=[SELECT Id FROM Account WHERE Id=:seed.Id]; idOnly.PersonHomePhone='4155550123'; Database.SaveResult ir=Database.update(idOnly,false); System.assertEquals(true,ir.isSuccess());
Account componentFirst=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; componentFirst.FirstName='Changed'; System.assertEquals(false,Database.update(componentFirst,false).isSuccess());
Account componentLast=[SELECT Id,Name FROM Account WHERE Id=:seed.Id]; componentLast.LastName='Changed'; System.assertEquals(false,Database.upsert(componentLast,false).isSuccess());
Account componentControl=[SELECT Id FROM Account WHERE Id=:seed.Id]; componentControl.FirstName='Changed'; System.assertEquals(true,Database.update(componentControl,false).isSuccess());
Account seedBusiness=new Account(Name='Business Seed'); insert seedBusiness; seedBusiness=[SELECT RecordTypeId FROM Account WHERE Id=:seedBusiness.Id]; Account b=new Account(Name='Synthetic Business',RecordTypeId=seedBusiness.RecordTypeId,FirstName='Synthetic',LastName='Person',PersonTitle='Synthetic',ProbeKey__c='new'); Database.UpsertResult r=Database.upsert(b,Account.Fields.ProbeKey__c,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals(true,r.isCreated());
} }`
			writeFile(t, filepath.Join(base, "classes/CaptureRouteTest.cls"), source)
			writeFile(t, filepath.Join(base, "classes/CaptureRouteTest.cls-meta.xml"), meta)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for _, route := range []string{"project", "warm"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"CaptureRouteTest"}, Parallelism: 1, ParallelMethods: true})
					if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("%s: %+v %s", route, summary, firstRunProblem(result))
					}
				})
			}
		})
	}
}
