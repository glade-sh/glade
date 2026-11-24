package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// An uncaptured query modifier must retain the ordinary sharing identity.
func TestEventParentModifiedQuerySharingNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
	query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "="}}
	query.ForView = true
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("modified query sharing user = %q, want %q", got, want)
	}
}

// The capture used one value in each IN clause, not a multi-value list.
func TestEventShareMultiValueQuerySharingNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}}
	query := soql.Query{Object: "Widget__Share", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{And: []soql.Condition{
		{Field: "RowCause", Op: "IN", Values: []storage.Value{storage.StringValue("First"), storage.StringValue("Second")}},
		{Field: "ParentId", Op: "IN", Values: []storage.Value{storage.StringValue("a00000000000001")}},
	}}}
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("multi-value query sharing user = %q, want %q", got, want)
	}
}

func TestEventParentCountContextNeighbors(t *testing.T) {
	for _, name := range []string{"record trigger", "explicit delivery", "dynamic query"} {
		t.Run(name, func(t *testing.T) {
			machine := recordSharingControlMachine(t, "Private")
			machine.currentMethod.APIVersion = "62.0"
			machine.currentTrigger = true
			machine.eventSubscriberTriggerDepth = 1
			machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
			machine.eventBusTriggerContext = &eventBusTriggerContext{}
			machine.testContext.StopTimeEventDelivery = true
			machine.staticSOQLQuery = true
			switch name {
			case "record trigger":
				machine.eventBusTriggerContext = nil
			case "explicit delivery":
				machine.testContext.StopTimeEventDelivery = false
			case "dynamic query":
				machine.staticSOQLQuery = false
			}
			query := soql.Query{
				Object:       "Widget__c",
				Count:        true,
				SecurityMode: "USER_MODE",
				Aggregates:   []soql.Aggregate{{Func: "COUNT"}},
				Where:        &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue("a00000000000001")},
			}
			if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
				t.Fatalf("sharing user = %q, want %q", got, want)
			}
		})
	}
}
