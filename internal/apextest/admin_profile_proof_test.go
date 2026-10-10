package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF195 exact API65 Profile query and User insertion assertions.
func TestAdminProfilePredicateAPI65Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeActionPlansProfile65Proof.cls"), `@IsTest private class GladeActionPlansProfile65Proof {
 private static Profile originalProfile() {
  for (Profile p : [SELECT Id, PermissionsModifyAllData, Name FROM Profile WHERE PermissionsModifyAllData = TRUE AND UserType = 'Standard' LIMIT 1]) {
   if (p.PermissionsModifyAllData) { return p; }
  }
  return null;
 }
 @IsTest static void originalModifyAllStandardPredicateFindsProfile() {
  Profile p=originalProfile();
  System.assertNotEquals(null,p);
  System.assertNotEquals(null,p.Id);
  System.assertEquals(true,p.PermissionsModifyAllData);
  Profile selected=[SELECT UserType FROM Profile WHERE Id=:p.Id];
  System.assertEquals('Standard',selected.UserType);
 }
 @IsTest static void originalSelectedProfileCreatesUser() {
  Profile p=originalProfile();
  System.assertNotEquals(null,p);
  User testUser=new User();
  testUser.Email='probe@example.invalid';
  testUser.Username='gladeprofile65.'+UserInfo.getOrganizationId()+'.'+DateTime.now().getTime()+'@example.invalid';
  testUser.LastName='test';
  testUser.Alias='test';
  testUser.ProfileId=p.Id;
  testUser.LanguageLocaleKey='en_US';
  testUser.LocaleSidKey='en_US';
  testUser.TimeZoneSidKey='America/Chicago';
  testUser.EmailEncodingKey='UTF-8';
  insert testUser;
  System.assertNotEquals(null,testUser.Id);
  User stored=[SELECT ProfileId FROM User WHERE Id=:testUser.Id];
  System.assertEquals(p.Id,stored.ProfileId);
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeActionPlansProfile65Proof.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("profile proof: %s", data)
	}
}
