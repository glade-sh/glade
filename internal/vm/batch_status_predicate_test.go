package vm

import (
	"fmt"
	"testing"
)

// Owned async lifecycle proofs admit Queued -> Completed for direct batches.
// SOQL IN compares the actual field value, not a synthetic pending alias:
// https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-comparisonoperators.html
func TestExecCompletedBatchStatusPredicates(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, rows := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/%d", api, rows), func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				machine.EnableTestContext()
				start, err := CompileAnonymous("return Database.getQueryLocator([SELECT Id FROM Account]);")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "PredicateBatch", Interfaces: []string{"Database.Batchable<SObject>"}, Methods: map[string]Method{
					"start":   {Name: "PredicateBatch.start", ClassName: "PredicateBatch", ReturnType: "Database.QueryLocator", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, Program: start},
					"execute": {Name: "PredicateBatch.execute", ClassName: "PredicateBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<SObject>"}}},
					"finish":  {Name: "PredicateBatch.finish", ClassName: "PredicateBatch", ReturnType: "void", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}},
				}}); err != nil {
					t.Fatal(err)
				}
				seed := ""
				if rows == 1 {
					seed = "insert new Account(Name='predicate input');"
				}
				program, err := CompileAnonymousWithOptions(seed+`
Set<String> pending=new Set<String>{'Queued','Processing','Preparing'};
Set<String> names=new Set<String>{'PredicateBatch'};
String jobType='BatchApex';
Test.startTest();
Id jobId=Database.executeBatch(new PredicateBatch(),100);
System.assertEquals('Queued',[SELECT Status FROM AsyncApexJob WHERE Id=:jobId].Status);
System.assertEquals(1,[SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId AND Status IN :pending]);
Test.stopTest();
System.assertEquals('Completed',[SELECT Status FROM AsyncApexJob WHERE Id=:jobId].Status);
System.assertEquals(1,[SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId AND Status='Completed']);
System.assertEquals(1,[SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId AND Status IN ('Completed')]);
System.assertEquals(0,[SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId AND Status='Queued']);
System.assertEquals(0,[SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId AND Status IN :pending]);
System.assertEquals(1,[SELECT COUNT() FROM AsyncApexJob WHERE Id=:jobId AND Status NOT IN :pending]);
System.assertEquals(0,Database.query('SELECT Id FROM AsyncApexJob WHERE JobType=:jobType AND ApexClass.Name IN :names AND Status IN :pending').size());
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
