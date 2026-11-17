package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// NATIVE the end-of-setup drain, test_runner_coverage_setup_async/org.tsv
// R3EventTest, R3SchedTest and R3BatchChainTest rows, API 62.0 and 67.0: a
// platform event published in setup is not delivered before the tests,
// scheduled Apex queued in setup runs, and a Queueable a batch finish()
// enqueues runs.
func TestSetupAsyncDrainMatchesNativeKinds(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/objects/DrainMark__e/DrainMark__e.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><eventType>HighVolume</eventType><label>Drain Mark</label><pluralLabel>Drain Marks</pluralLabel><publishBehavior>PublishAfterCommit</publishBehavior></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/objects/DrainMark__e/fields/Tag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Tag__c</fullName><label>Tag</label><length>40</length><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/triggers/DrainMarkTrigger.trigger"), "trigger DrainMarkTrigger on DrainMark__e (after insert) {\n    insert new Account(Name = 'DM-EVENT');\n}")
	classes := map[string]string{
		"DrainSched":     "public class DrainSched implements Schedulable {\n    public void execute(SchedulableContext ctx) { insert new Account(Name = 'DM-SCHED'); }\n}",
		"DrainLater":     "public class DrainLater implements Queueable {\n    public void execute(QueueableContext ctx) { insert new Account(Name = 'DM-LATER'); }\n}",
		"DrainBatch":     "public class DrainBatch implements Database.Batchable<SObject> {\n    public Database.QueryLocator start(Database.BatchableContext ctx) { return Database.getQueryLocator([SELECT Id FROM Account WHERE Name = 'DM-SEED']); }\n    public void execute(Database.BatchableContext ctx, List<SObject> scope) {}\n    public void finish(Database.BatchableContext ctx) { insert new Account(Name = 'DM-FINISH'); System.enqueueJob(new DrainLater()); }\n}",
		"DrainEventTest": "@IsTest private class DrainEventTest {\n    @TestSetup static void seed() { EventBus.publish(new DrainMark__e(Tag__c = 'x')); }\n    @IsTest static void check() { System.assertEquals(0, [SELECT count() FROM Account WHERE Name = 'DM-EVENT']); }\n}",
		"DrainSchedTest": "@IsTest private class DrainSchedTest {\n    @TestSetup static void seed() { System.schedule('drain-sched', '0 0 0 1 1 ? 2099', new DrainSched()); }\n    @IsTest static void check() { System.assertEquals(1, [SELECT count() FROM Account WHERE Name = 'DM-SCHED']); }\n}",
		"DrainBatchTest": "@IsTest private class DrainBatchTest {\n    @TestSetup static void seed() { insert new Account(Name = 'DM-SEED'); Database.executeBatch(new DrainBatch()); }\n    @IsTest static void check() {\n        System.assertEquals(1, [SELECT count() FROM Account WHERE Name = 'DM-FINISH']);\n        System.assertEquals(1, [SELECT count() FROM Account WHERE Name = 'DM-LATER']);\n    }\n}",
	}
	for name, body := range classes {
		writeFile(t, filepath.Join(root, "force-app/classes", name+".cls"), body)
	}
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
	seen := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			seen++
			if c.Status != testreport.StatusPass {
				t.Errorf("%s.%s status=%s problem=%+v", c.ClassName, c.MethodName, c.Status, c.Problem)
			}
		}
	}
	if seen != 3 {
		t.Fatalf("ran %d cases, want 3", seen)
	}
}
