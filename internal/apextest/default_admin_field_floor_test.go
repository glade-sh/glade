package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native unassigned isTest field describe: replay_describe/assignments/
// ordinary_controls/org.tsv:19,39,59,79 (APIs 59, 62, 67 and 68 respectively).
func TestDefaultAdminFieldAPIFloor(t *testing.T) {
	for _, tc := range []struct {
		api  string
		want string
	}{{"59.0", "true"}, {"62.0", "false"}, {"67.0", "false"}, {"68.0", "false"}} {
		t.Run(tc.api, func(t *testing.T) {
			root := t.TempDir()
			src := "testdata/conformance/default_permissions_v035/project"
			if err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				if strings.Contains(name, "classes/") {
					return nil
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", tc.api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "force-app/main/default/classes/AdminFieldFloor.cls")
			writeFile(t, path, `@IsTest private class AdminFieldFloor { @IsTest static void observe(){ System.assertEquals(`+tc.want+`, AccessNote__c.Detail__c.getDescribe().isAccessible()); } }`)
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+tc.api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"AdminFieldFloor"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("API %s: %+v %+v", tc.api, summary, run.Suites)
			}
		})
	}
}
