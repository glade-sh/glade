package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

// IN must select the same rows as testing the selected field values. The
// finish-reschedule lifecycle itself is intentionally not asserted:
// ordinary direct-batch native rows do not establish its callback boundary.
// https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-comparisonoperators.html
func TestRunBatchFinishPendingPredicateConsistency(t *testing.T) {
	for _, api := range []string{"58.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			classes := filepath.Join(root, "force-app/main/default/classes")
			for name, source := range map[string]string{
				"RescheduleBatch":        rescheduleBatchSource,
				"BatchPredicateContract": batchPredicateContractSource,
			} {
				writeFile(t, filepath.Join(classes, name+".cls"), source)
				writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			run := Run(loadTestIndex(t, root), Options{})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("batch predicate consistency: %s", data)
			}
		})
	}
}

const rescheduleBatchSource = `public class RescheduleBatch implements Database.Batchable<SObject>, Database.Stateful, Database.AllowsCallouts, Schedulable {
    private Boolean child;
    public RescheduleBatch(){child=false;}
    public RescheduleBatch(Boolean value){child=value;}
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator([SELECT Id FROM Account WHERE Name='predicate input']);
    }
    public void execute(Database.BatchableContext context, List<SObject> scope) {
        insert new Account(Name='predicate execute');
    }
    public void finish(Database.BatchableContext context) {
        Set<String> pending=new Set<String>{'Preparing','Processing','Queued','Holding'};
        Set<String> kinds=new Set<String>{'BatchApex','ScheduledApex'};
        if (!child && [SELECT COUNT() FROM AsyncApexJob WHERE ApexClass.Name='RescheduleBatch' AND JobType IN :kinds AND Status IN :pending]==0) {
            System.schedule('predicate continuation','0 0 0 1 1 ? 2099',new RescheduleBatch(true));
        }
    }
    public void execute(SchedulableContext context) {
        insert new Account(Name='predicate callback');
        System.abortJob(context.getTriggerId());
        Database.executeBatch(new RescheduleBatch(true),100);
    }
}
`

const batchPredicateContractSource = `@IsTest private class BatchPredicateContract {
    private static void run(Boolean withInput) {
        if(withInput) insert new Account(Name='predicate input');
        Test.startTest();
        Id parent=Database.executeBatch(new RescheduleBatch(),100);
        Test.stopTest();
        AsyncApexJob parentJob=[SELECT Id,ApexClassId,JobType,Status FROM AsyncApexJob WHERE Id=:parent];
        Id classId=parentJob.ApexClassId;
        String kind='BatchApex';
        Set<String> pending=new Set<String>{'Preparing','Processing','Queued','Holding'};
        List<AsyncApexJob> allJobs=[SELECT Id,Status FROM AsyncApexJob WHERE ApexClassId=:classId AND JobType=:kind];
        Set<Id> expected=new Set<Id>();
        for(AsyncApexJob job:allJobs) if(pending.contains(job.Status)) expected.add(job.Id);
        List<AsyncApexJob> selected=Database.query('SELECT Id,Status FROM AsyncApexJob WHERE ApexClassId=:classId AND Status IN :pending AND JobType=:kind');
        Set<Id> actual=new Set<Id>();
        for(AsyncApexJob job:selected) actual.add(job.Id);
        System.assertEquals(expected,actual,'IN selects the actual status');
        for(String status:pending) {
            Integer equalCount=[SELECT COUNT() FROM AsyncApexJob WHERE ApexClassId=:classId AND JobType=:kind AND Status=:status];
            Set<String> singleton=new Set<String>{status};
            Integer inCount=[SELECT COUNT() FROM AsyncApexJob WHERE ApexClassId=:classId AND JobType=:kind AND Status IN :singleton];
            System.assertEquals(equalCount,inCount,'Equality and singleton IN agree');
        }
    }
    @IsTest static void emptyScope(){run(false);}
    @IsTest static void oneRecordScope(){run(true);}
}
`
