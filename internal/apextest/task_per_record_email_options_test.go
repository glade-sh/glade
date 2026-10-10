package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF210 exact API65 statement insert/upsert options and rollback assertions.
func TestTaskPerRecordEmailOptionsAPI65Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTaskHeader65ContextAssertions.cls"), `@IsTest private class GladeTaskHeader65ContextAssertions {
 private static void exercise(Boolean isUpsert) {
  Task first=new Task(Subject='Owned before',OwnerId=UserInfo.getUserId(),Status='Not Started',Priority='Normal');
  Task second=new Task(Subject='Owned before',OwnerId=UserInfo.getUserId(),Status='Not Started',Priority='Normal');
  if(isUpsert) insert first;
  Database.DMLOptions dmlEmail=new Database.DMLOptions();
  dmlEmail.EmailHeader.TriggerUserEmail=true;
  Database.DMLOptions dmlNoEmail=new Database.DMLOptions();
  dmlEmail.EmailHeader.TriggerUserEmail=false;
  first.setOptions(dmlEmail);
  second.setOptions(dmlNoEmail);
  Map<String,Object> observations=new Map<String,Object>{'isUpsert'=>isUpsert,'configuredEmailFlag'=>dmlEmail.EmailHeader.TriggerUserEmail,'defaultEmailFlag'=>dmlNoEmail.EmailHeader.TriggerUserEmail,'firstOptionsBefore'=>first.getOptions().EmailHeader.TriggerUserEmail,'secondOptionsBefore'=>second.getOptions().EmailHeader.TriggerUserEmail};
  first.Subject='Owned after';second.Subject='Owned after';
  List<Task> rows=new List<Task>{first,second};
  Integer before=Limits.getEmailInvocations();
  observations.put('emailInvocationsBeforeStart',before);
  Test.startTest();
  Savepoint point=Database.setSavepoint();
  observations.put('emailInvocationsBeforeDml',Limits.getEmailInvocations());
  if(isUpsert) {upsert rows;} else {insert rows;}
  observations.put('emailInvocationsAfterDml',Limits.getEmailInvocations());
  observations.put('firstOptionsAfter',first.getOptions().EmailHeader.TriggerUserEmail);
  observations.put('secondOptionsAfter',second.getOptions().EmailHeader.TriggerUserEmail);
  System.assertNotEquals(null,first.Id);System.assertNotEquals(null,second.Id);
  System.assertNotEquals(first.Id,second.Id);
  Set<Id> ids=new Set<Id>{first.Id,second.Id};
  List<Task> saved=[SELECT Subject,OwnerId FROM Task WHERE Id IN :ids];
  System.assertEquals(2,saved.size());
  for(Task row:saved){System.assertEquals('Owned after',row.Subject);System.assertEquals(UserInfo.getUserId(),row.OwnerId);}
  Database.rollback(point);
  List<Task> restored=[SELECT Id,Subject FROM Task WHERE Id IN :ids];
  System.assertEquals(isUpsert ? 1 : 0,restored.size());
  if(isUpsert){System.assertEquals(first.Id,restored[0].Id);System.assertEquals('Owned before',restored[0].Subject);}
  observations.put('persistedRowsVerified',true);observations.put('rollbackVerified',true);
  Test.stopTest();
  observations.put('emailInvocationsAfterStop',Limits.getEmailInvocations());
  Map<String,Object> expected=new Map<String,Object>{
   'emailInvocationsAfterStop'=>0,
   'rollbackVerified'=>true,
   'persistedRowsVerified'=>true,
   'secondOptionsAfter'=>null,
   'firstOptionsAfter'=>null,
   'emailInvocationsAfterDml'=>0,
   'emailInvocationsBeforeDml'=>0,
   'emailInvocationsBeforeStart'=>0,
   'secondOptionsBefore'=>null,
   'firstOptionsBefore'=>null,
   'defaultEmailFlag'=>null,
   'configuredEmailFlag'=>false,
   'isUpsert'=>isUpsert
  };
  System.assertEquals(expected.keySet(),observations.keySet());
  for(String key:expected.keySet()){System.assertEquals(expected.get(key),observations.get(key),key);}
 }
 @IsTest static void assertOriginalPerRecordOptionsInsert(){exercise(false);}
 @IsTest static void assertOriginalPerRecordOptionsUpsert(){exercise(true);}
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTaskHeader65ContextAssertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("email options proof: %s", data)
	}
}

// Preservation control: projecting getOptions must not mutate either the
// caller's options or the options later consumed by real DML.
func TestTaskPerRecordEmailOptionsPreserveAppliedOptions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"65.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/TaskOptionsIsolation.cls"), `@IsTest private class TaskOptionsIsolation {
 @IsTest static void returnedOptionsAreIndependent() {
  Database.DMLOptions applied=new Database.DMLOptions();
  applied.AllowFieldTruncation=true;
  applied.EmailHeader.TriggerUserEmail=false;
  Task row=new Task(Subject='owned option isolation',Status='Not Started',Priority='Normal');
  row.setOptions(applied);
  Database.DMLOptions returned=row.getOptions();
  System.assertEquals(null,returned.EmailHeader.TriggerUserEmail);
  System.assertEquals(true,returned.AllowFieldTruncation);
  System.assertEquals(false,applied.EmailHeader.TriggerUserEmail);
  returned.EmailHeader.TriggerUserEmail=true;
  returned.AllowFieldTruncation=false;
  System.assertEquals(null,row.getOptions().EmailHeader.TriggerUserEmail);
  System.assertEquals(true,row.getOptions().AllowFieldTruncation);
  insert row;
  System.assertEquals(false,applied.EmailHeader.TriggerUserEmail);
  System.assertEquals(true,applied.AllowFieldTruncation);
  System.assertEquals('owned option isolation',[SELECT Subject FROM Task WHERE Id=:row.Id].Subject);
  System.assertEquals(0,Limits.getEmailInvocations());
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/TaskOptionsIsolation.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("options isolation: %s", data)
	}
}
