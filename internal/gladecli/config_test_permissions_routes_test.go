package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguredPermissionsCLIRouteTwins(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	var fixture struct{ Files map[string]string }
	raw, err := os.ReadFile("../apextest/testdata/conformance/configured_permissions/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, data := range fixture.Files {
				if strings.HasSuffix(name, ".cls-meta.xml") {
					data = strings.ReplaceAll(data, "62.0", api)
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default", name), data)
			}
			writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Synthetic_Access, Synthetic_Access]\n")
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/SyntheticPermissionObservation.cls"), `@IsTest private class SyntheticPermissionObservation {
 static Boolean initialized = FeatureManagement.checkPermission('Synthetic_View');
 @TestSetup static void setup() {
  System.assertEquals(true, initialized);
  System.assertEquals(true, FeatureManagement.checkPermission('Synthetic_View'));
  insert new Account(Name='Grant ready');
 }
 @IsTest static void configuredRunningUser() {
  System.assertEquals(true, initialized);
  System.assertEquals(true, FeatureManagement.checkPermission('Synthetic_View'));
  System.assertEquals(false, FeatureManagement.checkPermission('Synthetic_Missing'));
  System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name='Grant ready']);
  System.assertEquals(1, [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='Synthetic_Access']);
  System.assertEquals(true, Synthetic_Item__c.SObjectType.getDescribe().isAccessible());
  System.assertEquals(true, Synthetic_Item__c.Secret__c.getDescribe().isAccessible());
  List<Synthetic_Item__c> visible = [SELECT Secret__c FROM Synthetic_Item__c WITH USER_MODE];
  System.assertEquals(0, visible.size());
 }
 @IsTest static void runAsDoesNotInheritGrant() {
  Profile profile = [SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
  User other = new User(Alias='synuser', Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Synthetic', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=profile.Id, Username='synthetic.'+UserInfo.getOrganizationId()+'@example.invalid');
  insert other;
  System.runAs(other) {
   System.assertEquals(false, FeatureManagement.checkPermission('Synthetic_View'));
   System.assertEquals(false, Synthetic_Item__c.SObjectType.getDescribe().isAccessible());
   System.assertEquals(false, Synthetic_Item__c.Secret__c.getDescribe().isAccessible());
  }
 }
}`)
			run := func(t *testing.T, args ...string) (int, string) {
				t.Helper()
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), args, &stdout, &stderr)
				return code, stdout.String() + stderr.String()
			}
			t.Run("named_isTest_runner_before_setup", func(t *testing.T) {
				code, output := run(t, "test", "--project", root, "--class", "SyntheticPermissionObservation", "--json", "--no-progress", "--no-cache", "--no-serve")
				if code != 0 {
					t.Fatalf("configured test runner exit=%d: %s", code, output)
				}
				var result struct {
					Data struct {
						Summary struct{ Total, Passed int }
					}
				}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Fatal(err)
				}
				if summary := result.Data.Summary; summary.Total != 2 || summary.Passed != 2 {
					t.Fatalf("expected both configured tests to run: %s", output)
				}
			})
			t.Run("anonymous_does_not_apply_test_grants", func(t *testing.T) {
				code, output := run(t, "exec", "--project", root, "--json", "System.assertEquals(false,FeatureManagement.checkPermission('Synthetic_View')); System.assertEquals(0,[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='Synthetic_Access']);")
				if code != 0 {
					t.Fatalf("anonymous test-config boundary exit=%d: %s", code, output)
				}
			})
			t.Run("unknown_name_config_error", func(t *testing.T) {
				writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Synthetic_NotPresent]\n")
				for _, args := range [][]string{
					{"config", "validate", "--project", root},
					{"test", "--project", root, "--no-serve"},
					{"exec", "--project", root, "System.assert(true);"},
				} {
					code, output := run(t, args...)
					if code == 0 || !strings.Contains(output, "test.permissionSets") || !strings.Contains(output, "Synthetic_NotPresent") {
						t.Fatalf("%s missing named config error (exit=%d): %s", args[0], code, output)
					}
				}
			})
		})
	}
}
