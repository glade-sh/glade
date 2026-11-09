package apextest

import (
	"path/filepath"
	"testing"
)

func TestStandardProfileAccountProjectMetadataGrant(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Minimum Access - Salesforce.profile-meta.xml"), `<Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom><objectPermissions><object>Account</object><allowRead>true</allowRead></objectPermissions></Profile>`)
			path := filepath.Join(root, "force-app/main/default/classes/AccountGrantTest.cls")
			writeFile(t, path, `@IsTest private class AccountGrantTest { @IsTest static void observe() { Profile p=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce']; User u=new User(Alias='grant',Email='grant@example.invalid',EmailEncodingKey='UTF-8',LastName='Grant',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='grant@example.invalid'); System.runAs(u) { System.assertEquals(true, Account.SObjectType.getDescribe().isAccessible()); } } }`)
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"AccountGrantTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("%+v %+v", summary, run.Suites)
			}
		})
	}
}
