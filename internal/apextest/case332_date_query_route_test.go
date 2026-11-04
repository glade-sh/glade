package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

// This is a local preservation check for the named test route, not a native capture.
func TestCase332DateLiteralIsTestRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default/classes/Case332DateTest")
			writeFile(t, base+".cls", `@IsTest private class Case332DateTest {
 @IsTest static void queryDates() {
  insert new Task(Subject='date-route', Status='Not Started', Priority='Normal');
  for (Integer i=0; i<2; i++) {
   List<Task> today = [SELECT Id FROM Task WHERE Subject = 'date-route' AND CreatedDate = TODAY];
   List<Task> old = [SELECT Id FROM Task WHERE Subject = 'date-route' AND CreatedDate = LAST_YEAR];
   System.assertEquals(1, today.size());
   System.assertEquals(0, old.size());
  }
 }
}`)
			writeFile(t, base+".cls-meta.xml", fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			run := Run(loadTestIndex(t, root), Options{})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				data, _ := json.Marshal(run)
				t.Fatalf("date route: %s", data)
			}
		})
	}
}
