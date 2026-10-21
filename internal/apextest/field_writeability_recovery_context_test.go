package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestFieldWriteabilityRecoveryCreationTimestampContext(t *testing.T) {
	// Native source-62/67, transport-67 observations cover default trigger
	// timing and the separately planned USER_MODE ordinary/JSON insert pair.
	const className = "CreationTimestampContextTest"
	const source = `@IsTest public class CreationTimestampContextTest {
 public static Datetime beforeTimestamp;
 public static Datetime afterTimestamp;
 public static Integer beforeCount = 0;
 public static Integer afterCount = 0;
 @IsTest static void ordinaryTriggerTiming() {
  Account record = new Account(Name='Ordinary timing control');
  insert record;
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertEquals(1, beforeCount);
  System.assertEquals(1, afterCount);
  System.assertEquals(null, beforeTimestamp);
  System.assertNotEquals(null, afterTimestamp);
  System.assertEquals(stored.CreatedDate, afterTimestamp);
 }
 @IsTest static void jsonTriggerTiming() {
  Account record = jsonRecord();
  Datetime supplied = record.CreatedDate;
  Database.SaveResult result = Database.insert(record, false);
  System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertEquals(1, beforeCount);
  System.assertEquals(1, afterCount);
  System.assertEquals(null, beforeTimestamp);
  System.assertNotEquals(null, afterTimestamp);
  System.assertEquals(stored.CreatedDate, afterTimestamp);
  System.assertNotEquals(supplied, afterTimestamp);
  System.assertEquals(supplied, record.CreatedDate);
 }
 @IsTest static void ordinaryUserMode() {
  Account record = new Account(Name='Ordinary user mode control');
  Database.SaveResult result = Database.insert(record, false, AccessLevel.USER_MODE);
  System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
 }
 @IsTest static void jsonUserMode() {
  Account record = jsonRecord();
  Datetime supplied = record.CreatedDate;
  Database.SaveResult result = Database.insert(record, false, AccessLevel.USER_MODE);
  System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertNotEquals(supplied, stored.CreatedDate);
  System.assertEquals(supplied, record.CreatedDate);
 }
 private static Account jsonRecord() {
  return (Account)JSON.deserialize('{"Name":"JSON timing control","CreatedDate":"2001-02-03T04:05:06.000Z"}', Account.class);
 }
}`
	const trigger = `trigger CreationTimestampContext on Account (before insert, after insert) {
 for (Account record : Trigger.new) {
  if (Trigger.isBefore) {
   CreationTimestampContextTest.beforeCount++;
   CreationTimestampContextTest.beforeTimestamp = record.CreatedDate;
  } else {
   CreationTimestampContextTest.afterCount++;
   CreationTimestampContextTest.afterTimestamp = record.CreatedDate;
  }
 }
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "classes", className+".cls"), source)
			writeFile(t, filepath.Join(base, "classes", className+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			writeFile(t, filepath.Join(base, "triggers/CreationTimestampContext.trigger"), trigger)
			writeFile(t, filepath.Join(base, "triggers/CreationTimestampContext.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
			run := Run(loadTestIndex(t, root), Options{
				RuntimeRESTAPIVersion: "67.0",
				SelectedClasses:       []string{className},
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 {
				data, _ := json.Marshal(run)
				t.Fatalf("creation timestamp context controls: %s", data)
			}
		})
	}
}
