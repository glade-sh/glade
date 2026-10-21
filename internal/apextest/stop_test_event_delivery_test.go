package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Events that asynchronous work publishes while Test.stopTest runs it are
// delivered by stopTest after that work returns, through the same subscriber
// delivery as events published before stopTest. The subscriber never runs
// inside EventBus.publish. Native APIs 62/67/68 preserve test-scope statics
// in subscribers.
func TestStopTestDeliversEventsPublishedByAsyncWorkAfterThePublisher(t *testing.T) {
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n"
	writeClass := func(name, body string) {
		writeFile(t, filepath.Join(classes, name+".cls"), body)
		writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), meta)
	}
	writeClass("RelaySink", `public class RelaySink {
 public static Integer seed = 0;
 public static void receive(List<RelaySignal__e> events) {
  System.assertEquals(7, seed);
  Integer after = [SELECT COUNT() FROM Account WHERE Name = 'after publish'];
  insert new Account(Name = 'delivered ' + events[0].Note__c, NumberOfEmployees = after);
 }
}
`)
	writeClass("RelayWorker", `public class RelayWorker implements Queueable {
 public void execute(QueueableContext context) {
  try {
   EventBus.publish(new RelaySignal__e(Note__c = 'queueable'));
  } catch (Exception e) {
   insert new Account(Name = 'publisher caught ' + e.getTypeName());
  }
  insert new Account(Name = 'after publish');
 }
}
`)
	writeClass("RelayBatch", `public class RelayBatch implements Database.Batchable<SObject> {
 public Database.QueryLocator start(Database.BatchableContext context) {
  return Database.getQueryLocator([SELECT Id FROM Account WHERE Name = 'seed']);
 }
 public void execute(Database.BatchableContext context, List<SObject> scope) {}
 public void finish(Database.BatchableContext context) {
  EventBus.publish(new RelaySignal__e(Note__c = 'finish'));
  insert new Account(Name = 'after publish');
 }
}
`)
	writeClass("RelayDeliveryTest", `@IsTest private class RelayDeliveryTest {
 @IsTest static void queueablePublisherDoesNotRunSubscriber() {
  RelaySink.seed = 7;
  Test.startTest();
  System.enqueueJob(new RelayWorker());
  Test.stopTest();
  System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name LIKE 'publisher caught%']);
  Account delivered = [SELECT NumberOfEmployees FROM Account WHERE Name = 'delivered queueable'];
  System.assertEquals(1, delivered.NumberOfEmployees);
 }
 @IsTest static void batchFinishPublisherDoesNotRunSubscriber() {
  insert new Account(Name = 'seed');
  RelaySink.seed = 7;
  Test.startTest();
  Database.executeBatch(new RelayBatch());
  Test.stopTest();
  Account delivered = [SELECT NumberOfEmployees FROM Account WHERE Name = 'delivered finish'];
  System.assertEquals(1, delivered.NumberOfEmployees);
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/RelaySignalTrigger.trigger"), "trigger RelaySignalTrigger on RelaySignal__e (after insert) {\n RelaySink.receive(Trigger.new);\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/RelaySignalTrigger.trigger-meta.xml"), "<ApexTrigger xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexTrigger>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/RelaySignal__e/RelaySignal__e.object-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <deploymentStatus>Deployed</deploymentStatus>\n    <eventType>HighVolume</eventType>\n    <label>Relay Signal</label>\n    <pluralLabel>Relay Signals</pluralLabel>\n    <publishBehavior>PublishImmediately</publishBehavior>\n</CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/RelaySignal__e/fields/Note__c.field-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Note__c</fullName>\n    <externalId>false</externalId>\n    <isFilteringDisabled>false</isFilteringDisabled>\n    <isNameField>false</isNameField>\n    <isSortingDisabled>false</isSortingDisabled>\n    <label>Note</label>\n    <length>255</length>\n    <required>false</required>\n    <type>Text</type>\n    <unique>false</unique>\n</CustomField>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"namespace\": \"\", \"sourceApiVersion\": \"66.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, _ := json.Marshal(run)
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("stopTest event delivery: %s", data)
	}
}
