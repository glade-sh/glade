package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Supplemental project, @IsTest and warm-cache route coverage for the owned
// event_publish_entity_access/org.tsv capture_03 and capture_05 observations.
func TestEventPublishEntityAccessProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			event := readFixtureFile(t, "testdata/conformance/event_publish_entity_access/force-app/main/default/objects/ProbeSignal__e/ProbeSignal__e.object-meta.xml")
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/ProbeSignal__e/ProbeSignal__e.object-meta.xml"), event)
			meta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/EventPublisher.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/EventPublisher.cls"), `public class EventPublisher { public static Database.SaveResult publish() { return EventBus.publish(new ProbeSignal__e()); } }`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/EventPermissionTest.cls-meta.xml"), meta)
			expected := `System.assertEquals(true,r.isSuccess());`
			if api == "67.0" {
				expected = `System.assertEquals(false,r.isSuccess()); System.assertEquals('Access to entity \'ProbeSignal__e\' denied',r.getErrors()[0].getMessage());`
			}
			test := `@IsTest private class EventPermissionTest { @IsTest static void check() { Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1]; User u=new User(ProfileId=p.Id,Username='probe@example.invalid',Email='probe@example.invalid',Alias='probe',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u; System.runAs(u) { Database.SaveResult r=EventPublisher.publish(); ` + expected + ` } } }`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/EventPermissionTest.cls"), test)
			index := loadTestIndex(t, root)
			for _, route := range []string{"project", "warm"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"EventPermissionTest"}, Parallelism: 1})
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
						t.Fatalf("got %d cases: %+v", seen, result)
					}
				})
			}
		})
	}
}
