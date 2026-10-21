package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These text-field transaction cases preserve existing local behavior. The
// captured before-save Picklist comparison is covered by its separate native
// matrix; it does not establish these text or after-save shapes.
func TestExecFlowChangedTransactionProvenance(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"existing_changed", `
Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Existing'];
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"existing_unchanged", `
Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Existing'];
update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"created_then_updated", `
Account subject = new Account(Name = 'Fresh', Phase__c = 'Draft'); insert subject;
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"start_test_keeps_transaction", `
Account subject = new Account(Name = 'Fresh', Phase__c = 'Draft'); insert subject;
Test.startTest();
subject.Phase__c = 'Ready'; update subject;
Test.stopTest();
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"savepoint_keeps_earlier_creation", `
Account subject = new Account(Name = 'Fresh', Phase__c = 'Draft'); insert subject;
Savepoint saved = Database.setSavepoint();
insert new Account(Name = 'Temporary', Phase__c = 'Draft');
Database.rollback(saved);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'Temporary']);
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"statement_rollback_keeps_earlier_creation", `
Account subject = new Account(Name = 'Fresh', Phase__c = 'Draft'); insert subject;
Boolean rejected = false;
try { insert new List<Account>{new Account(Name = 'Temporary'), new Account(Id = '001000000000999', Name = 'Reject')}; }
catch (DmlException failure) { rejected = true; }
System.assertEquals(true, rejected);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'Temporary']);
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"partial_row_failure_keeps_successful_creation", `
List<Account> subjects = new List<Account>{new Account(Name = 'Fresh', Phase__c = 'Draft'), new Account(Id = '001000000000999', Name = 'Reject')};
List<Database.SaveResult> saves = Database.insert(subjects, false);
System.assertEquals(true, saves[0].isSuccess());
System.assertEquals(false, saves[1].isSuccess());
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'Reject']);
Account subject = subjects[0];
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
		{"savepoint_restores_existing_value", `
Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Existing'];
Savepoint saved = Database.setSavepoint();
insert new Account(Name = 'Temporary');
Database.rollback(saved);
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := newFlowChangedTransactionMachine(t)
			runFlowChangedTransactionSource(t, machine, tc.source)
		})
	}
}

func TestExecFlowChangedLaterExecutionHasCommittedPrior(t *testing.T) {
	machine := newFlowChangedTransactionMachine(t)
	runFlowChangedTransactionSource(t, machine, `insert new Account(Name = 'Fresh', Phase__c = 'Draft');`)
	runFlowChangedTransactionSource(t, machine, `
Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Fresh'];
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`)
}

func TestExecFlowChangedAsyncHasCommittedPrior(t *testing.T) {
	machine := newFlowChangedTransactionMachine(t)
	body, err := CompileAnonymous(`
Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Fresh'];
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	class := Class{Name: "PhaseWorker", Interfaces: []string{"Queueable"}, Methods: map[string]Method{
		"execute": {Name: "PhaseWorker.execute", ClassName: "PhaseWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: body},
	}}
	if err := machine.RegisterClass(class); err != nil {
		t.Fatal(err)
	}
	runFlowChangedTransactionSource(t, machine, `
insert new Account(Name = 'Fresh', Phase__c = 'Draft');
Test.startTest();
System.enqueueJob(new PhaseWorker());
Test.stopTest();
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Name = 'Fresh'].Outcome__c);
`)
}

func TestVMInsertBookkeepingTracksCreationsWithoutJournal(t *testing.T) {
	for _, journal := range []bool{false, true} {
		name := "without_journal"
		if journal {
			name = "with_journal"
		}
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if journal {
				machine.SetIsolationJournal(storage.NewIsolationJournal(&org))
			}
			id := storage.ID("001000000000001")
			machine.recordIsolationJournalMutation("Account", id, storage.Record{}, false)
			if !machine.recordCreationState().Contains(id) {
				t.Fatal("insert bookkeeping omitted transaction provenance")
			}
			machine.recordIsolationJournalMutation("Account", "001000000000002", storage.Record{}, true)
			if machine.recordCreationState().Contains("001000000000002") {
				t.Fatal("update bookkeeping introduced creation provenance")
			}
		})
	}
}

func newFlowChangedTransactionMachine(t *testing.T) *VM {
	t.Helper()
	org := testDataOrg()
	object := org.Objects["Account"]
	object.Definition.Fields["Phase__c"] = storage.Field{APIName: "Phase__c", Type: storage.FieldString}
	object.Definition.Fields["Outcome__c"] = storage.Field{APIName: "Outcome__c", Type: storage.FieldString}
	object.Definition.FlowRules = []storage.FlowRule{
		{Name: "RoutePhaseChange", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate", Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
			{Name: "Changed", Criteria: []storage.WorkflowCriteriaItem{{Field: "Phase__c", Operation: "ischanged", Value: "true"}}, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Changed"}}}}},
			{Name: "Same", Default: true, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Same"}}}}},
		}}}},
		{Name: "RejectNamedRecord", Active: true, TriggerType: "RecordAfterSave", RecordTriggerType: "CreateAndUpdate", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Reject"}}, Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Reject", Messages: []storage.FlowCustomErrorMessage{{Message: "Synthetic rejection"}}}}}},
	}
	id := storage.ID("001000000000001")
	object.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Existing"), "Phase__c": storage.StringValue("Draft")}}
	org.Objects["Account"] = object
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	return machine
}

func runFlowChangedTransactionSource(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
