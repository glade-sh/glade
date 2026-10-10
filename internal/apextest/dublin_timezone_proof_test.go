package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Original API63 summer dates and winter control admitted by Salesforce wave158.
func TestDublinTimezoneAPI63Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeDublin63Proof.cls"), "@IsTest private class GladeDublin63Proof {\n private static User userInDublin() {\n  User u=new User(Username='glade'+UserInfo.getOrganizationId()+Datetime.now().getTime()+'@example.invalid',LastName='Dublin proof',Email='proof@example.invalid',Alias='dublin',TimeZoneSidKey='Europe/Dublin',LocaleSidKey='en_IE_EURO',EmailEncodingKey='ISO-8859-1',LanguageLocaleKey='en_US',ProfileId=UserInfo.getProfileId());\n  insert u; return u;\n }\n @IsTest static void originalSummerLocalParts() {\n  System.runAs(userInDublin()) {\n   for(Integer day:new List<Integer>{1,2,8,9}) {\n    Datetime local=Datetime.newInstance(2020,6,day,1,2,3);\n    System.assertEquals(Datetime.newInstanceGmt(2020,6,day,0,2,3),local);\n    System.assertEquals(3600000,UserInfo.getTimeZone().getOffset(local));\n   }\n  }\n }\n @IsTest static void winterAndSummerOffsetControls() {\n  System.runAs(userInDublin()) {\n   Datetime winter=Datetime.newInstance(2020,1,8,1,2,3);\n   System.assertEquals(Datetime.newInstanceGmt(2020,1,8,1,2,3),winter);\n   System.assertEquals(0,UserInfo.getTimeZone().getOffset(winter));\n   System.assertEquals('Europe/Dublin',UserInfo.getTimeZone().getID());\n  }\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeDublin63Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"63.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("Dublin proof: %s", data)
	}
}
