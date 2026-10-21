package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce's FlowOrchestrationEvent reference requires OrchestrationInstanceId.
// The Summer '22 Metadata API release notes support publishing this event from
// Apex. Publish Event Messages with Apex specifies failed SaveResults for
// unsuccessful publication, rather than an exception for the failed operation.
// https://developer.salesforce.com/docs/platform/platform-events/guide/sforce-api-objects-floworchestrationevent.html
// https://developer.salesforce.com/docs/platform/platform-events/guide/platform-events-publish-apex.html
// https://help.salesforce.com/s/articleView?id=release-notes.rn_api_meta.htm&language=en_US&release=238&type=5
func TestExecStandardOrchestrationEventMissingRequiredFieldReturnsFailure(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, source string }{
			{"scalar", `Database.SaveResult result = EventBus.publish(new FlowOrchestrationEvent());`},
			{"erased_list", `List<Database.SaveResult> results = EventBus.publish(new List<SObject>{new FlowOrchestrationEvent()});
System.assertEquals(1, results.size());
Database.SaveResult result = results[0];`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(tc.source+`
System.assertEquals(false, result.isSuccess());
System.assert(result.getErrors().size() > 0);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				definition, ok := storage.StandardObjectDefinition("FlowOrchestrationEvent")
				if !ok || !definition.Fields["OrchestrationInstanceId"].Required {
					t.Fatal("retained standard describe must require OrchestrationInstanceId")
				}
				org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// This is a local service boundary, not a claim that the supplied instance exists
// or that Salesforce returns this exception for the same input.
func TestExecStandardOrchestrationEventRequiredFieldPresentKeepsHostedBoundary(t *testing.T) {
	program, err := CompileAnonymous(`
EventBus.publish(new FlowOrchestrationEvent(OrchestrationInstanceId = 'owned-instance'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	definition, ok := storage.StandardObjectDefinition("FlowOrchestrationEvent")
	if !ok {
		t.Fatal("standard orchestration event definition unavailable")
	}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "hosted orchestration service") {
		t.Fatalf("got %v, want explicit hosted orchestration boundary", err)
	}
}

func TestExecOrdinaryCalendarEventRemainsUnpublishable(t *testing.T) {
	program, err := CompileAnonymous(`EventBus.publish(new Event());`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil || !strings.Contains(err.Error(), "platform event") {
		t.Fatalf("got %v, want ordinary Event rejection", err)
	}
}

// Callback eligibility for this standard event is unobserved. Preserve the
// existing local rejection instead of extending the plain-publication contract.
func TestExecStandardOrchestrationEventCallbackKeepsExistingRejection(t *testing.T) {
	program, err := CompileAnonymous(`EventBus.publish(new FlowOrchestrationEvent(), null);`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	definition, ok := storage.StandardObjectDefinition("FlowOrchestrationEvent")
	if !ok {
		t.Fatal("standard orchestration event definition unavailable")
	}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "platform event") {
		t.Fatalf("got %v, want existing local callback rejection", err)
	}
}
