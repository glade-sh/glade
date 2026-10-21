package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLastActivityDateInsertContext(t *testing.T) {
	// Native source-62/67, transport-67 observations cover these seven insert
	// contexts per source. The passive observer samples its own entry/return.
	const className = "ActivityDateInsertContextTest"
	const source = `@IsTest public class ActivityDateInsertContextTest {
 public static List<Map<String,Object>> events = new List<Map<String,Object>>();
 public static void observe(String phase, Account record) {
  if (record.Name == 'Owned activity date context') {
   events.add(new Map<String,Object>{'phase'=>phase, 'date'=>record.LastActivityDate,
    'populated'=>record.getPopulatedFieldsAsMap().containsKey('LastActivityDate'), 'name'=>record.Name});
  }
 }
 @IsTest static void jsonDefault() { checkInsert(true, 'DEFAULT'); }
 @IsTest static void jsonUserMode() { checkInsert(true, 'USER'); }
 @IsTest static void ordinaryUserMode() { checkInsert(false, 'USER'); }
 @IsTest static void jsonSystemMode() { checkInsert(true, 'SYSTEM'); }
 @IsTest static void ordinarySystemMode() { checkInsert(false, 'SYSTEM'); }
 @IsTest static void jsonKeywordDefault() { checkInsert(true, 'KEYWORD'); }
 @IsTest static void jsonDMLOptionsDefault() { checkInsert(true, 'OPTIONS'); }
 private static void checkInsert(Boolean fromJSON, String route) {
  events = new List<Map<String,Object>>();
  Account record;
  Date expectedCaller;
  if (fromJSON) {
   record = (Account)JSON.deserialize('{"Name":"Owned activity date context","LastActivityDate":"2007-02-03"}', Account.class);
   expectedCaller = Date.newInstance(2007, 2, 3);
  } else {
   record = new Account(Name='Owned activity date context');
  }
  System.assertEquals(expectedCaller, record.LastActivityDate);
  Database.SaveResult result;
  if (route == 'USER') {
   result = Database.insert(record, false, AccessLevel.USER_MODE);
  } else if (route == 'SYSTEM') {
   result = Database.insert(record, false, AccessLevel.SYSTEM_MODE);
  } else if (route == 'KEYWORD') {
   insert record;
  } else if (route == 'OPTIONS') {
   Database.DMLOptions options = new Database.DMLOptions();
   options.optAllOrNone = false;
   result = Database.insert(record, options);
  } else {
   result = Database.insert(record, false);
  }
  if (route != 'KEYWORD') {
   System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
   System.assertEquals(0, result.getErrors().size());
  }
  System.assertEquals(expectedCaller, record.LastActivityDate);
  System.assertEquals(fromJSON, record.getPopulatedFieldsAsMap().containsKey('LastActivityDate'));
  System.assertEquals('Owned activity date context', record.Name);
  System.assertEquals(3, events.size());
  List<String> phases = new List<String>{'before-entry','before-return','after'};
  for (Integer i = 0; i < events.size(); i++) {
   System.assertEquals(phases[i], events[i].get('phase'));
   System.assertEquals(null, events[i].get('date'));
   System.assertEquals(false, events[i].get('populated'));
   System.assertEquals(record.Name, events[i].get('name'));
  }
  Account stored = [SELECT LastActivityDate, Name FROM Account WHERE Id=:record.Id];
  System.assertEquals(null, stored.LastActivityDate);
  System.assertEquals(record.Name, stored.Name);
 }
}`
	const trigger = `trigger ActivityDateInsertContext on Account (before insert, after insert) {
 for (Account record : Trigger.new) {
  if (Trigger.isBefore) {
   ActivityDateInsertContextTest.observe('before-entry', record);
   ActivityDateInsertContextTest.observe('before-return', record);
  } else {
   ActivityDateInsertContextTest.observe('after', record);
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
			writeFile(t, filepath.Join(base, "triggers/ActivityDateInsertContext.trigger"), trigger)
			writeFile(t, filepath.Join(base, "triggers/ActivityDateInsertContext.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
			run := Run(loadTestIndex(t, root), Options{
				RuntimeRESTAPIVersion: "67.0",
				SelectedClasses:       []string{className},
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if summary := run.Summary(); summary.Total != 7 || summary.Passed != 7 {
				data, _ := json.Marshal(run)
				t.Fatalf("native activity-date insert contexts: %s", data)
			}
		})
	}
}
