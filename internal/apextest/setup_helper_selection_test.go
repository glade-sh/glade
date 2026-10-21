package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// This preserves the runner's existing full-run method-call behavior when
// selection omits the class that owns a callable setup method.
func TestRunSelectedClassRetainsCallableSetupHelpers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := map[string]string{
		"PermissionSetupFixture": `@IsTest @TestVisible private class PermissionSetupFixture {
 @TestSetup @TestVisible static void seed() {
  Profile profile = [SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
  PermissionSet grant = new PermissionSet(Name='SyntheticReadGrant', Label='Synthetic read grant');
  insert grant;
  insert new ObjectPermissions(ParentId=grant.Id, SObjectType='Contact', PermissionsRead=true);
  insert new FieldPermissions(ParentId=grant.Id, SObjectType='Contact', Field='Contact.Birthdate', PermissionsRead=true);
  User person = new User(ProfileId=profile.Id, Alias='synset', LastName='Synthetic',
    Email='synthetic@example.invalid', Username='synthetic.setup@example.invalid',
    EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles');
  insert person;
  insert new PermissionSetAssignment(AssigneeId=person.Id, PermissionSetId=grant.Id);
 }
 @IsTest static void ownSetup() { Assert.areEqual(1,[SELECT COUNT() FROM User WHERE Alias='synset']); }
}
`,
		"SelectedSetupProbe": `@IsTest private class SelectedSetupProbe {
 @TestSetup static void seed() { PermissionSetupFixture.seed(); }
 @IsTest static void userAndPermissionAssignment() {
  User person = [SELECT Id FROM User WHERE Alias='synset'];
  PermissionSet grant = [SELECT Id FROM PermissionSet WHERE Name='SyntheticReadGrant'];
  Assert.areEqual(1,[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:person.Id AND PermissionSetId=:grant.Id]);
  Assert.areEqual(0,[SELECT COUNT() FROM PermissionSet WHERE Name='UnselectedGrant']);
 }
}
`,
		"UnselectedSetupFixture": `@IsTest private class UnselectedSetupFixture {
 @TestSetup static void seed() { insert new PermissionSet(Name='UnselectedGrant',Label='Unselected grant'); }
 @IsTest static void ownSetup() { Assert.areEqual(1,[SELECT COUNT() FROM PermissionSet WHERE Name='UnselectedGrant']); }
}
`,
	}
	for name, source := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
	}
	index := loadTestIndex(t, root)
	for _, row := range []struct {
		name    string
		classes []string
		total   int
	}{
		{"selected caller", []string{"SelectedSetupProbe"}, 1},
		{"all classes", nil, 3},
		{"selected helper owner", []string{"PermissionSetupFixture"}, 1},
		{"selected caller after cache reuse", []string{"SelectedSetupProbe"}, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: row.classes, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != row.total || summary.Passed != row.total {
				data, _ := json.Marshal(run)
				t.Fatalf("setup helper selection: %s", data)
			}
		})
	}
}
