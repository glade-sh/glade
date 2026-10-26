package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// cdc_journal_delivery/org.tsv cdcRollback and cdcUpsert pin the queue
// boundary; both storage rollback implementations must preserve that result.
func TestChangeDataCaptureRollbackSnapshotAndJournalAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, journal := range []bool{false, true} {
			route := "snapshot"
			if journal {
				route = "journal"
			}
			t.Run(api+"/istest/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Account")
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if journal {
					machine.SetIsolationJournal(storage.NewIsolationJournal(&org))
				}
				if err := machine.RegisterClass(Class{Name: "JournalCounter", StaticFields: map[string]Field{"hits": {Name: "hits", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)}}}); err != nil {
					t.Fatal(err)
				}
				trigger, err := CompileAnonymousWithOptions(`JournalCounter.hits += Trigger.new.size();`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterTrigger(Trigger{Name: "JournalSubscriber", Object: "AccountChangeEvent", Timing: "after", Operation: "insert", APIVersion: api, Program: trigger}); err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(`
Test.enableChangeDataCapture();
Account record=new Account(Name='cedar-upsert-source');
upsert record;
Test.getEventBus().deliver();
System.assertEquals(1,JournalCounter.hits);
record.Description='cedar-upsert-updated';
upsert record;
Test.getEventBus().deliver();
System.assertEquals(2,JournalCounter.hits);
Savepoint point=Database.setSavepoint();
upsert new Account(Name='cedar-upsert-rolledback');
Database.rollback(point);
Test.getEventBus().deliver();
System.assertEquals(2,JournalCounter.hits);
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
