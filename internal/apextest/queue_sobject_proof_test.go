package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF201 exact API65 owned QueueSobject assertions.
func TestQueueSobjectAPI65Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQueueSobject200Proof.cls"), `@IsTest private class GladeQueueSobject200Proof {
 @TestSetup static void originalQueueSetup() {
  Group queueGroup=new Group(Name='glade task queue',Type='Queue');
  insert queueGroup;
  QueueSobject assignment=new QueueSobject(QueueID=queueGroup.Id,SobjectType='Task');
  insert assignment;
 }
 @IsTest static void originalQueueAssignmentQueriesAndDeletes() {
  Group queueGroup=[SELECT Id FROM Group WHERE Name='glade task queue' AND Type='Queue' LIMIT 1];
  QueueSobject assignment=[SELECT Id,QueueId,SobjectType FROM QueueSobject WHERE QueueId=:queueGroup.Id AND SobjectType='Task' LIMIT 1];
  System.assertEquals(queueGroup.Id,assignment.QueueId);
  System.assertEquals('Task',assignment.SobjectType);
  delete assignment;
  System.assertEquals(0,[SELECT COUNT() FROM QueueSobject WHERE Id=:assignment.Id]);
 }
 @IsTest static void originalRegisteredTaskQueueOwnsTask() {
  Group queueGroup=[SELECT Id FROM Group WHERE Name='glade task queue' AND Type='Queue' LIMIT 1];
  Task taskRecord=new Task(Subject='queue use',ActivityDate=Date.today());
  insert taskRecord;
  taskRecord.OwnerId=queueGroup.Id;
  update taskRecord;
  Task selected=[SELECT OwnerId FROM Task WHERE Id=:taskRecord.Id];
  System.assertEquals(queueGroup.Id,selected.OwnerId);
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeQueueSobject200Proof.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("queue proof: %s", data)
	}
}
