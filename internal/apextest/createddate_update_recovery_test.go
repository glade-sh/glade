package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestCreatedDateJSONWritableAssignmentRecovery(t *testing.T) {
	// Owned native sequence observations cover source/transport 53/65 as a
	// public diagnostic, and 62/67 plus 67/67 as supported controls. Neither
	// observation of the diagnostic establishes namespaced project parity.
	const className = "CreatedDateJSONAssignmentTest"
	const source = `@IsTest private class CreatedDateJSONAssignmentTest {
 private static Account jsonRow(String name) {
  Map<String,Object> values = new Map<String,Object>{
   'Name'=>name,
   'CreatedDate'=>Datetime.newInstanceGmt(2001,2,3,4,5,6)
  };
  return (Account)JSON.deserialize(JSON.serialize(values),Account.class);
 }
 @IsTest static void sameCallerInsertLookupUpdate() {
  Account parent = new Account(Name='Owned assignment parent');
  insert parent;
  Account row = jsonRow('Owned assignment update');
  Datetime supplied = row.CreatedDate;
  insert row;
  System.assertNotEquals(null,row.Id);
  System.assertEquals(supplied,row.CreatedDate);
  Account first = [SELECT CreatedDate FROM Account WHERE Id=:row.Id];
  System.assertNotEquals(null,first.CreatedDate);
  System.assertNotEquals(supplied,first.CreatedDate);
  CreatedDateJSONAssignmentState.targetName = row.Name;
  row.ParentId = parent.Id;
  update row;
  Account stored = [SELECT CreatedDate,ParentId FROM Account WHERE Id=:row.Id];
  System.assertEquals(supplied,row.CreatedDate);
  System.assertEquals(parent.Id,row.ParentId);
  System.assertEquals(parent.Id,stored.ParentId);
  System.assertEquals(first.CreatedDate,stored.CreatedDate);
  System.assertEquals(1,CreatedDateJSONAssignmentState.beforeCount);
  System.assertEquals(1,CreatedDateJSONAssignmentState.afterCount);
  System.assertEquals(first.CreatedDate,CreatedDateJSONAssignmentState.beforeTimestamp);
  System.assertEquals(first.CreatedDate,CreatedDateJSONAssignmentState.afterTimestamp);
 }
 @IsTest static void jsonPicklistAssignmentInsert() {
  Account row = jsonRow('Owned assignment insert');
  Datetime supplied = row.CreatedDate;
  row.Industry = 'Technology';
  insert row;
  Account stored = [SELECT CreatedDate,Industry FROM Account WHERE Id=:row.Id];
  System.assertNotEquals(null,row.Id);
  System.assertEquals(supplied,row.CreatedDate);
  System.assertEquals('Technology',row.Industry);
  System.assertEquals('Technology',stored.Industry);
  System.assertNotEquals(null,stored.CreatedDate);
  System.assertNotEquals(supplied,stored.CreatedDate);
 }
}`
	const observer = `public class CreatedDateJSONAssignmentState {
 public static String targetName;
 public static Integer beforeCount = 0;
 public static Integer afterCount = 0;
 public static Datetime beforeTimestamp;
 public static Datetime afterTimestamp;
}`
	const trigger = `trigger CreatedDateJSONAssignmentObserver on Account (before update,after update) {
 for (Account row : Trigger.new) {
  if (row.Name == CreatedDateJSONAssignmentState.targetName) {
   if (Trigger.isBefore) {
    CreatedDateJSONAssignmentState.beforeCount++;
    CreatedDateJSONAssignmentState.beforeTimestamp = row.CreatedDate;
   } else {
    CreatedDateJSONAssignmentState.afterCount++;
    CreatedDateJSONAssignmentState.afterTimestamp = row.CreatedDate;
   }
  }
 }
}`
	for _, context := range []struct {
		name, sourceAPI, transportAPI string
	}{
		{"diagnostic_source53_transport65", "53.0", "65.0"},
		{"floor_source62_transport67", "62.0", "67.0"},
		{"ceiling_source67_transport67", "67.0", "67.0"},
	} {
		t.Run(context.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.sourceAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default")
			for name, body := range map[string]string{className: source, "CreatedDateJSONAssignmentState": observer} {
				path := filepath.Join(base, "classes", name+".cls")
				writeFile(t, path, body)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+context.sourceAPI+"</apiVersion></ApexClass>")
			}
			writeFile(t, filepath.Join(base, "triggers/CreatedDateJSONAssignmentObserver.trigger"), trigger)
			writeFile(t, filepath.Join(base, "triggers/CreatedDateJSONAssignmentObserver.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+context.sourceAPI+"</apiVersion><status>Active</status></ApexTrigger>")
			run := Run(loadTestIndex(t, root), Options{
				RuntimeRESTAPIVersion: context.transportAPI,
				SelectedClasses:       []string{className},
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
				data, _ := json.Marshal(run)
				t.Fatalf("CreatedDate JSON writable assignment controls: %s", data)
			}
		})
	}
}
