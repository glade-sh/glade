package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF228 exact TaskPriority API65 subset; Contract observer excluded.
func TestTaskPrioritySF228(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTaskPriority65Assertions.cls"), `@IsTest private class GladeTaskPriority65Assertions {
 @IsTest static void assertSetupPriorityRows() {
  List<TaskPriority> rows=[SELECT Id,ApiName,MasterLabel,IsDefault,IsHighPriority,SortOrder FROM TaskPriority ORDER BY SortOrder];
  List<String> names=new List<String>{'High','Normal','Low'};
  System.assertEquals(3,rows.size()); Set<Id> ids=new Set<Id>();
  for(Integer i=0;i<3;i++) { TaskPriority r=rows[i]; System.assertNotEquals(null,r.Id); ids.add(r.Id); System.assertEquals(names[i],r.ApiName); System.assertEquals(names[i],r.MasterLabel); System.assertEquals(i==1,r.IsDefault); System.assertEquals(i==0,r.IsHighPriority); System.assertEquals(i+1,r.SortOrder); }
  System.assertEquals(3,ids.size());
  System.assertEquals('High',[SELECT MasterLabel FROM TaskPriority WHERE IsHighPriority=TRUE LIMIT 1].MasterLabel);
  List<String> values=new List<String>(); for(Schema.PicklistEntry p:Task.Priority.getDescribe().getPicklistValues()) values.add(p.getValue()); System.assertEquals(names,values);
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTaskPriority65Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("TaskPriority: %s", b)
	}
}
