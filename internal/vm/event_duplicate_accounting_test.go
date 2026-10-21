package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func duplicateEventAccountingFixture(api string) (*VM, Value) {
	org := storage.NewOrgState()
	org.APIVersion = api
	org.Objects["SignalPulse__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "SignalPulse__e", Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	event := Object("SignalPulse__e")
	list := List(event, event)
	list.Type = "List<SignalPulse__e>"
	return machine, list
}

func TestDuplicateImmediateEventPairChargesBeforeRejection(t *testing.T) {
	// Native C04-10: the same empty event in a direct concrete pair rejects
	// publication after charging one immediate invocation, without ordinary DML.
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine, list := duplicateEventAccountingFixture(api)
				if route == "isTest" {
					machine.EnableTestContext()
				}
				_, err := machine.eventBusPublish([]Value{list}, &Result{})
				var thrown *apexThrowError
				if !errors.As(err, &thrown) || exceptionTypeName(thrown.value.Type) != "ListException" ||
					stringField(thrown.value, "message") != "Before Insert or Upsert list must not have two identically equal elements" {
					t.Fatalf("duplicate error = %v", err)
				}
				if machine.limits.PublishImmediateDML != 1 || machine.limits.DMLStatements != 0 || machine.limits.DMLRows != 0 {
					t.Fatalf("duplicate accounting = %#v", machine.limits)
				}
				if machine.eventBusReplaySequence != 0 || machine.testContext != nil && len(machine.testContext.PlatformEvents) != 0 {
					t.Fatal("rejected duplicate pair entered publication")
				}
			})
		}
	}
}

func TestDuplicateImmediateEventPairPreservesUncapturedBoundaries(t *testing.T) {
	// These are guard controls, not native answers for the excluded shapes.
	for _, name := range []string{"generic", "populated", "triple", "namespace", "api66", "api69", "id", "afterCommit", "class"} {
		t.Run(name, func(t *testing.T) {
			machine, list := duplicateEventAccountingFixture("67.0")
			switch name {
			case "generic":
				list.Type, list.Runtime = "List<SObject>", "List<SignalPulse__e>"
			case "populated":
				list.List[0].Fields["PulseCode__c"] = String("a")
			case "triple":
				list.List = append(list.List, list.List[0])
			case "namespace":
				machine.Org.Namespace = "pkg"
			case "api66":
				machine.currentMethod.APIVersion = "66.0"
			case "api69":
				machine.currentMethod.APIVersion = "69.0"
			case "id":
				list.List[0].Fields["Id"] = String("e01000000000001")
			case "afterCommit":
				machine.Org.Objects["SignalPulse__e"].Definition.Metadata["publishBehavior"] = "PublishAfterCommit"
			case "class":
				list.List[0].classInstance = true
			}
			if machine.capturedDuplicateImmediateEventPair(list) {
				t.Fatal("uncaptured duplicate shape acquired counter projection")
			}
		})
	}
}
