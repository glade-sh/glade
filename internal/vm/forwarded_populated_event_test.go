package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Candidate populated-pair extension; empty/generic native controls stay separate.
func TestForwardedPopulatedEventPairUsesImmediateCounter(t *testing.T) {
	m := New(nil)
	o := testDataOrg()
	o.APIVersion = "67.0"
	o.Objects["SignalPulse__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "SignalPulse__e", KeyPrefix: "e01", Fields: map[string]storage.Field{"PulseCode__c": {APIName: "PulseCode__c", Type: storage.FieldString}}, Metadata: map[string]string{"publishBehavior": "PublishImmediately"}}, Records: map[storage.ID]storage.Record{}}
	m.SetOrg(&o)
	a := Object("SignalPulse__e")
	a.Fields["PulseCode__c"] = String("a")
	b := Object("SignalPulse__e")
	b.Fields["PulseCode__c"] = String("b")
	xs := List(a, b)
	xs.Type = "List<SObject>"
	xs.Runtime = "List<SignalPulse__e>"
	if _, err := m.eventBusPublish([]Value{xs}, &Result{}); err != nil {
		t.Fatal(err)
	}
	if m.limits.PublishImmediateDML != 1 || m.limits.DMLStatements != 0 {
		t.Fatalf("limits: %+v", m.limits)
	}
}
