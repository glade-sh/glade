package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestImmediateEventListKeepsAllocatedGenericBoundary(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Objects["SignalPulse__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "SignalPulse__e", Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	event := Object("SignalPulse__e")
	widened := List(event)
	widened.Type, widened.Runtime = "List<SObject>", "List<SignalPulse__e>"
	// Owned event_list_accounting.json rows 002-004 distinguish a forwarded
	// concrete singleton from an allocated generic singleton at 62/67/68.
	if !machine.capturedImmediateEventList(widened) {
		t.Fatal("lost captured concrete singleton provenance")
	}
	generic := widened
	generic.Runtime = "List<SObject>"
	if machine.capturedImmediateEventList(generic) {
		t.Fatal("inferred concrete list provenance from a generic list's element")
	}
	// Populating/adding generic elements cannot change allocation provenance.
	event.Fields["PulseCode__c"] = String("a")
	generic.List = append(generic.List, Object("SignalPulse__e"))
	if machine.capturedImmediateEventList(generic) {
		t.Fatal("populated generic list acquired concrete allocation provenance")
	}
}
