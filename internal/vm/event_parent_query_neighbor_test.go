package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Start every neighbor from the captured private-parent and share shape.
func eventParentQueryControl(t *testing.T) (*VM, soql.Query) {
	t.Helper()
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.eventBusTriggerContext = &eventBusTriggerContext{}
	machine.testContext.StopTimeEventDelivery = true
	machine.staticSOQLQuery = true
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")

	users := machine.Org.Objects["User"]
	publisher := users.Records["005000000000001"]
	publisher.Fields["ProfileId"] = storage.IDValue("00e000000000001")
	users.Records[publisher.ID] = publisher
	profiles := machine.Org.Objects["Profile"]
	profiles.Records["00e000000000001"] = storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}}
	parents := machine.Org.Objects["Widget__c"]
	parent := parents.Records["a00000000000001"]
	parent.System.CreatedByID = publisher.ID
	parents.Records[parent.ID] = parent
	delete(parents.Records, "a00000000000002")
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}},
		Records: map[storage.ID]storage.Record{
			"a10000000000001": {ID: "a10000000000001", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue("Owner"), "AccessLevel": storage.StringValue("All")}},
			"a10000000000002": {ID: "a10000000000002", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue("Reason__c"), "AccessLevel": storage.StringValue("Read")}},
		},
	}
	event := Object("Signal__e")
	event.Fields["ParentRef__c"] = String(string(parent.ID))
	event.Fields["CreatedById"] = String(string(publisher.ID))
	qualifyEventParentUnit(machine, event, "62.0")
	machine.triggerGlobals = map[string]Value{"Trigger.new": List(event)}
	query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue(parent.ID)}}
	if got := machine.soqlSharingUserID(query); got != "" {
		t.Fatalf("captured parent sharing user = %q, want bypass", got)
	}
	return machine, query
}

func TestEventParentModifiedQuerySharingNeighbor(t *testing.T) {
	machine, query := eventParentQueryControl(t)
	query.ForView = true
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("modified query sharing user = %q, want %q", got, want)
	}
}

// A multi-value parent predicate reaches the same captured-context gate, but
// is not the captured Id-equality query.
func TestEventParentMultiValueQuerySharingNeighbor(t *testing.T) {
	machine, query := eventParentQueryControl(t)
	query.Where = &soql.Condition{Field: "Id", Op: "IN", Values: []storage.Value{storage.IDValue("a00000000000001"), storage.IDValue("a00000000000002")}}
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("multi-value parent query sharing user = %q, want %q", got, want)
	}
}

// The parent bypass must not extend to share queries, including multi-value IN.
func TestEventShareMultiValueQuerySharingNeighbor(t *testing.T) {
	machine, query := eventParentQueryControl(t)
	query.Object = "Widget__Share"
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("share object sharing user = %q, want %q", got, want)
	}
	query.Where = &soql.Condition{And: []soql.Condition{
		{Field: "RowCause", Op: "IN", Values: []storage.Value{storage.StringValue("Reason__c"), storage.StringValue("Other__c")}},
		{Field: "ParentId", Op: "IN", Values: []storage.Value{storage.IDValue("a00000000000001")}},
	}}
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("multi-value share query sharing user = %q, want %q", got, want)
	}
}

func TestEventParentCountContextNeighbors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*VM)
	}{
		{"record trigger", func(vm *VM) { vm.eventBusTriggerContext = nil }},
		{"explicit delivery", func(vm *VM) { vm.testContext.StopTimeEventDelivery = false }},
		{"dynamic query", func(vm *VM) { vm.staticSOQLQuery = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine, query := eventParentQueryControl(t)
			tc.change(machine)
			if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
				t.Fatalf("sharing user = %q, want %q", got, want)
			}
		})
	}
}
