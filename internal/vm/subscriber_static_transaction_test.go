package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Preserve existing local Flow creation provenance independently of the native
// subscriber-static facets. This does not extend their native observation scope.
func TestStopTimeSubscriberKeepsRecordCreationScope(t *testing.T) {
	machine := newFlowChangedTransactionMachine(t)
	machine.Org.Objects["PhaseSignal__e"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:  "PhaseSignal__e",
			Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
			Fields:   map[string]storage.Field{},
		},
		Records: map[storage.ID]storage.Record{},
	}
	body, err := CompileAnonymous(`
Account subject = [SELECT Id, Phase__c FROM Account WHERE Name = 'Fresh'];
subject.Phase__c = 'Ready'; update subject;
System.assertEquals('Changed', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{
		Name: "PhaseSignalSubscriber", Object: "PhaseSignal__e",
		Timing: triggerTimingAfter, Operation: "insert", Program: body,
	}); err != nil {
		t.Fatal(err)
	}
	runFlowChangedTransactionSource(t, machine, `
insert new Account(Name = 'Fresh', Phase__c = 'Draft');
Test.startTest();
EventBus.publish(new PhaseSignal__e());
Test.stopTest();
Account subject = [SELECT Id, Phase__c, Outcome__c FROM Account WHERE Name = 'Fresh'];
System.assertEquals('Changed', subject.Outcome__c);
subject.Phase__c = 'Final'; update subject;
System.assertEquals('Same', [SELECT Outcome__c FROM Account WHERE Id = :subject.Id].Outcome__c);
`)
}
