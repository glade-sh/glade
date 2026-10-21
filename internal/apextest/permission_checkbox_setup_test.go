package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestRunSparsePermissionSetCheckboxSetup(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classes := map[string]string{
				"CheckboxSetupFixture": `@IsTest @TestVisible private class CheckboxSetupFixture {
 @TestSetup @TestVisible static void seed() {
  PermissionSet sparse = new PermissionSet(Name='SparseGrant', Label='Sparse grant');
  PermissionSet enabled = new PermissionSet(Name='EnabledGrant', Label='Enabled grant', PermissionsEmailSingle=true);
  PermissionSet disabled = new PermissionSet(Name='DisabledGrant', Label='Disabled grant', PermissionsEmailSingle=false);
  insert new List<PermissionSet>{sparse,enabled,disabled};
  insert new ObjectPermissions(ParentId=sparse.Id, SObjectType='Contact', PermissionsRead=true);
  insert new FieldPermissions(ParentId=sparse.Id, SObjectType='Contact', Field='Contact.Birthdate', PermissionsRead=true);
 }
}
`,
				"PermissionCheckboxSetupProbe": `@IsTest private class PermissionCheckboxSetupProbe {
 @TestSetup static void seed() { CheckboxSetupFixture.seed(); }
 @IsTest static void storedCheckboxDefaults() {
  Map<String,Schema.SObjectField> fields = PermissionSet.SObjectType.getDescribe().fields.getMap();
  List<String> checkboxes = new List<String>();
  List<String> selected = new List<String>{'Name'};
  for (String name : fields.keySet()) {
   Schema.DescribeFieldResult field = fields.get(name).getDescribe();
   if (field.getType() == Schema.DisplayType.BOOLEAN && field.isCreateable()) {
    checkboxes.add(name);
    selected.add(name);
   }
  }
  Assert.isTrue(checkboxes.size() > 0);
  List<SObject> grants = Database.query('SELECT '+String.join(selected,',')+' FROM PermissionSet WHERE Name IN (\'SparseGrant\',\'EnabledGrant\',\'DisabledGrant\')');
  Assert.areEqual(3,grants.size());
  for (SObject grant : grants) {
   for (String checkbox : checkboxes) {
    Boolean expected = grant.get('Name') == 'EnabledGrant' && checkbox.equalsIgnoreCase('PermissionsEmailSingle');
    Assert.areEqual(expected,grant.get(checkbox),'Stored checkbox '+checkbox+' on '+grant.get('Name'));
   }
  }
  PermissionSet sparse = [SELECT Id FROM PermissionSet WHERE Name='SparseGrant'];
  ObjectPermissions objectGrant = [SELECT PermissionsRead,PermissionsEdit FROM ObjectPermissions WHERE ParentId=:sparse.Id];
  Assert.isTrue(objectGrant.PermissionsRead);
  Assert.isFalse(objectGrant.PermissionsEdit);
  FieldPermissions fieldGrant = [SELECT PermissionsRead,PermissionsEdit FROM FieldPermissions WHERE ParentId=:sparse.Id];
  Assert.isTrue(fieldGrant.PermissionsRead);
  Assert.isFalse(fieldGrant.PermissionsEdit);
 }
 @IsTest static void nameStillRequired() {
  Database.SaveResult result = Database.insert(new PermissionSet(Label='Missing name'),false);
  Assert.isFalse(result.isSuccess());
 }
}
`,
			}
			for name, source := range classes {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"PermissionCheckboxSetupProbe"}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
				data, _ := json.Marshal(run)
				t.Fatalf("permission checkbox setup: %s", data)
			}
		})
	}
}
