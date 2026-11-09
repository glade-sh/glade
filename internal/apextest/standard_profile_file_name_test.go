package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Retrieve and deploy name the System Administrator profile Admin.profile and
// Standard User Standard.profile (Metadata API Profile file names). A project's
// grants in those files belong to the standard profiles, so the default
// administrator gets the Admin.profile field grants at source API 62 to 67.
func TestStandardProfileFileNamesMapToStandardProfiles(t *testing.T) {
	profileXML := `<Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom>` +
		`<fieldPermissions><editable>true</editable><field>AccessNote__c.Detail__c</field><readable>true</readable></fieldPermissions>` +
		`<objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>AccessNote__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>` +
		`<userLicense>Salesforce</userLicense></Profile>`
	adminBody := `
        System.assertEquals('System Administrator', [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name);
        System.assertEquals(0, [SELECT COUNT() FROM Profile WHERE Name = 'Admin' OR Name = 'Standard']);
        System.assertEquals(%s, AccessNote__c.Detail__c.getDescribe().isCreateable());
        Boolean failed = false;
        try {
            insert new AccessNote__c(Name = 'n', Detail__c = 'd');
        } catch (DmlException e) {
            failed = true;
        }
        System.assertEquals(%s, failed);`
	standardBody := `
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
        User u = new User(Alias = 'stdu', Email = 'std@example.com', EmailEncodingKey = 'UTF-8', LastName = 'Std', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'std.profile.file@example.com');
        System.runAs(u) {
            System.assertEquals(%s, AccessNote__c.Detail__c.getDescribe().isCreateable());
        }`
	for _, tc := range []struct {
		name    string
		file    string
		body    string
		granted string
		failed  string
	}{
		{"admin file name", "Admin", adminBody, "true", "false"},
		{"system administrator file name", "System Administrator", adminBody, "true", "false"},
		{"no profile metadata", "", adminBody, "false", "true"},
		{"standard file name", "Standard", standardBody, "true", ""},
		{"standard user file name", "Standard User", standardBody, "true", ""},
		{"standard without grant", "", standardBody, "false", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			objectDir := filepath.Join(root, "force-app/main/default/objects/AccessNote__c")
			writeFile(t, filepath.Join(objectDir, "AccessNote__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Access Note</label><nameField><label>Access Note Name</label><type>Text</type></nameField><pluralLabel>Access Notes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(objectDir, "fields/Detail__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Detail__c</fullName><label>Detail</label><length>80</length><type>Text</type></CustomField>`)
			if tc.file != "" {
				writeFile(t, filepath.Join(root, "force-app/main/default/profiles", tc.file+".profile-meta.xml"), profileXML)
			}
			body := fmt.Sprintf(tc.body, tc.granted)
			if tc.failed != "" {
				body = fmt.Sprintf(tc.body, tc.granted, tc.failed)
			}
			path := filepath.Join(root, "force-app/main/default/classes/ProfileFileNameTest.cls")
			writeFile(t, path, "@IsTest private class ProfileFileNameTest { @IsTest static void observe() {"+body+"\n    } }")
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ProfileFileNameTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("%+v %+v", summary, run.Suites)
			}
		})
	}
}

// Percent-encoded metadata file names deploy to the decoded profile name.
func TestProfileNameFromMetadataFileName(t *testing.T) {
	for file, want := range map[string]string{
		"Admin.profile-meta.xml":                       "System Administrator",
		"admin.profile":                                "System Administrator",
		"System Administrator.profile-meta.xml":        "System Administrator",
		"Standard.profile-meta.xml":                    "Standard User",
		"ReadOnly.profile-meta.xml":                    "Read Only",
		"MarketingProfile.profile-meta.xml":            "Marketing User",
		"ContractManager.profile-meta.xml":             "Contract Manager",
		"SolutionManager.profile-meta.xml":             "Solution Manager",
		"StandardAul.profile-meta.xml":                 "Standard Platform User",
		"Custom%3A Sales Profile.profile-meta.xml":     "Custom: Sales Profile",
		"Custom%3A Marketing Profile.profile-meta.xml": "Custom: Marketing Profile",
		"Custom%3A Support Profile.profile-meta.xml":   "Custom: Support Profile",
		"Runtime Full.profile-meta.xml":                "Runtime Full",
	} {
		if got := profileNameFromPath(filepath.Join("force-app/main/default/profiles", file)); got != want {
			t.Errorf("%s: got %q want %q", file, got, want)
		}
	}
}
