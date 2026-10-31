package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Local preservation control for the shipping permission path. This does not
// claim a new Salesforce observation for orgs without PersonAccounts.
func TestPersonAccountDisabledProjectPreservesUpsertBaseline(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_profile_disabled"
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
					return nil
				}
				source := readFixtureFile(t, path)
				if strings.HasSuffix(rel, ".cls-meta.xml") {
					source = strings.ReplaceAll(strings.ReplaceAll(source, "62.0", api), "67.0", api)
				}
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DisabledPermissionControl.cls"), `@IsTest private class DisabledPermissionControl { @IsTest static void check() { Profile p=[SELECT Id FROM Profile WHERE Name='Route False']; User u=new User(ProfileId=p.Id,Username='disabled.control@example.invalid',Email='disabled@example.invalid',Alias='ctrl',LastName='Control',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u; System.runAs(u) { Boolean denied=false; try { upsert as user new Account(Name='Control'); } catch(SecurityException e) { denied=true; System.assertEquals('Access to entity \'Account\' denied',e.getMessage()); } System.assert(denied); } } }`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DisabledPermissionControl.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			for _, route := range []string{"project", "warm", "disk_restore"} {
				t.Run(route, func(t *testing.T) {
					if route == "disk_restore" {
						InvalidateRuntimeCaches()
					}
					result := Run(index, Options{SelectedClasses: []string{"DisabledPermissionControl"}, Parallelism: 1, PerfCounters: route == "disk_restore"})
					if route == "disk_restore" && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
						t.Fatalf("disk restore was not used: %+v", SnapshotPerfCounters().Phases)
					}
					seen := 0
					for _, suite := range result.Suites {
						for _, c := range suite.Cases {
							seen++
							if c.Problem != nil {
								t.Fatalf("%s: %+v", c.MethodName, c.Problem)
							}
						}
					}
					if seen != 1 {
						t.Fatalf("got %d control tests", seen)
					}
				})
			}
		})
	}
}
