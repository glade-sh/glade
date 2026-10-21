package vm

import "testing"

// Departed-if Id binding is captured independently. The Flow text-field fixture
// is a local transaction control, not an additional native Flow parity claim.
func TestClosedIDCountPreservesCreationTransactionBoundary(t *testing.T) {
	machine := newFlowChangedTransactionMachine(t)
	for _, source := range []string{
		`Account subject = new Account(Name = 'Fresh', Phase__c = 'Draft');
insert subject;
if (true) { Set<Id> selected = new Set<Id>{subject.Id}; }
System.assertEquals(1, Database.countQuery('SELECT COUNT() FROM Account WHERE Id IN :selected'));
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);`,
		`Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Fresh'];
subject.Phase__c = 'Final'; update subject;
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);`,
	} {
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			t.Fatal(err)
		}
	}
}
