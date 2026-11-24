package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE: custom_setting_org_defaults/org.tsv rows 2, 22, 42 and 62 show
// that a missing org default is non-null but has no Id. Rows 3, 23, 43
// and 63 show that an upsert creates it.
func TestCustomSettingOrgDefaultGuard(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "custom_setting_org_defaults", "project")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, name := range []string{
				"force-app/main/default/objects/Prefs__c/Prefs__c.object-meta.xml",
				"force-app/main/default/objects/Prefs__c/fields/Flag__c.field-meta.xml",
			} {
				body, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, name), string(body))
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DefaultGuardTest.cls"), `@IsTest private class DefaultGuardTest {
    @IsTest static void nullGuard() {
        Prefs__c p = Prefs__c.getOrgDefaults();
        if (p == null) { p = new Prefs__c(); insert p; }
        p.Flag__c = true;
        update p;
    }
    @IsTest static void upsertGuard() {
        Prefs__c p = Prefs__c.getOrgDefaults();
        p.Flag__c = true;
        upsert p;
        System.assertNotEquals(null, p.Id);
    }
}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DefaultGuardTest.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			for _, tc := range []struct {
				method string
				want   string
			}{{"nullGuard", "MISSING_ARGUMENT"}, {"upsertGuard", ""}} {
				t.Run(tc.method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"DefaultGuardTest"}, SelectedMethod: tc.method, NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("unexpected test selection: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if tc.want == "" {
						if problem != nil {
							t.Fatalf("expected success: %+v", problem)
						}
					} else if problem == nil || !strings.Contains(problem.Message, tc.want) {
						t.Fatalf("expected %s: %+v", tc.want, problem)
					}
				})
			}
		})
	}
}
