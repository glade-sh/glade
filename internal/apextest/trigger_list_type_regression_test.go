package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Keep the pre-existing trigger-list contract distinct from an explicitly
// allocated List<SObject>. A handler's generic parameter is not an allocation.
func TestTriggerListTypeThroughHandler(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default")
			for _, name := range []string{"DispatchParcel", "UnassignedParcel"} {
				writeFile(t, filepath.Join(base, "objects", name+"__c", name+"__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>`+name+`</label><pluralLabel>Parcels</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
			}
			writeFile(t, filepath.Join(base, "classes/DispatchRelay.cls"), `public class DispatchRelay {
 public static Integer calls=0;
 public static void handle(List<SObject> rows) { nested(rows); }
 private static void nested(List<SObject> rows) {
  System.assertEquals('DispatchParcel__c',rows.getSObjectType().getDescribe().getName());
  System.assertEquals(DispatchParcel__c.SObjectType,rows.clone().getSObjectType());
  System.assertEquals(DispatchParcel__c.SObjectType,rows.deepClone().getSObjectType());
  calls++;
 }
}`)
			writeFile(t, filepath.Join(base, "classes/DispatchProbe.cls"), `@IsTest private class DispatchProbe {
 @IsTest static void afterInsertHandler() {
  DispatchParcel__c row=new DispatchParcel__c(Name='parcel');
  Database.insert(row);
  System.assertNotEquals(null,row.Id);
  System.assertEquals(1,DispatchRelay.calls);
  row.Name='changed'; update row; delete row;
  System.assertEquals(4,DispatchRelay.calls);
 }
 @IsTest static void noPermissionsControl() {
  UnassignedParcel__c row=new UnassignedParcel__c(Name='plain');
  Database.insert(row);
  System.assertNotEquals(null,row.Id);
 }
 @IsTest static void standardObjectControl() {
  Account row=new Account(Name='plain'); Database.insert(row);
  System.assertNotEquals(null,row.Id);
 }
}`)
			for _, name := range []string{"DispatchRelay", "DispatchProbe"} {
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(base, "triggers/DispatchWitness.trigger"), `trigger DispatchWitness on DispatchParcel__c (after insert, after update, after delete) {
 if(Trigger.new!=null) {
  System.assertEquals(DispatchParcel__c.SObjectType,Trigger.new.getSObjectType());
  DispatchRelay.handle(Trigger.new);
 }
 if(Trigger.old!=null) {
  System.assertEquals(DispatchParcel__c.SObjectType,Trigger.old.getSObjectType());
  DispatchRelay.handle(Trigger.old);
 }
}`)
			writeFile(t, filepath.Join(base, "triggers/DispatchWitness.trigger-meta.xml"), `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
				data, _ := json.Marshal(run)
				t.Fatalf("trigger-list handler regression: %s", data)
			}
		})
	}
}
