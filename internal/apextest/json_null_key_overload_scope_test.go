package apextest

import (
	"path/filepath"
	"testing"
)

// This preserves prior local behavior; it does not assert a native result for
// the two-argument overload, for which no capture is available.
func TestJSONNullKeyOverloadProjectScope(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			path := filepath.Join(root, "force-app/main/default/classes/OverloadScopeTest.cls")
			writeFile(t, path, `@IsTest private class OverloadScopeTest {
  @IsTest static void scope() {
    Map<String,Integer> m = new Map<String,Integer>(); m.put(null, 1);
    System.assertEquals('{"null":1}', JSON.serialize(m, true));
    System.assertEquals('{"null":1}', JSON.serialize(m, false));
  }
}`)
			writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"OverloadScopeTest"}, NoDiskCache: true, Parallelism: 1})
			summary := run.Summary()
			if summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("local overload scope: %+v; suites=%+v", summary, run.Suites)
			}
		})
	}
}
