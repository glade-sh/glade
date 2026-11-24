package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// The native event carries the publisher-owned parent's ID. Unrelated records
// and share rows have no captured outcome and retain the ordinary user filter.
func TestEventSubscriberUnrelatedRecordSharingNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.eventBusTriggerContext = &eventBusTriggerContext{}
	machine.testContext.StopTimeEventDelivery = true
	machine.staticSOQLQuery = true
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
	event := Object("Signal__e")
	event.Fields["ParentRef__c"] = String("a00000000000001")
	event.Fields["CreatedById"] = String("005000000000001")
	machine.triggerGlobals = map[string]Value{"Trigger.new": List(event)}
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
	query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue("a00000000000002")}}
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("unrelated parent sharing user = %q, want %q", got, want)
	}
	query.Where.Value = storage.IDValue(parent.ID)
	if got := machine.soqlSharingUserID(query); got != "" {
		t.Fatalf("captured parent sharing user = %q, want bypass", got)
	}
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}, Records: map[storage.ID]storage.Record{
		"a10000000000001": {ID: "a10000000000001", Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000002")}},
	}}
	query.Object = "Widget__Share"
	query.Where = nil
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("unrelated share sharing user = %q, want %q", got, want)
	}
	query.Where = &soql.Condition{And: []soql.Condition{
		{Field: "RowCause", Op: "IN", Values: []storage.Value{storage.StringValue("Reason__c")}},
		{Field: "ParentId", Op: "IN", Values: []storage.Value{storage.IDValue("a00000000000002")}},
	}}
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("unrelated filtered share sharing user = %q, want %q", got, want)
	}
	shares := machine.Org.Objects["Widget__Share"]
	shares.Records = map[storage.ID]storage.Record{
		"a10000000000001": {ID: "a10000000000001", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "RowCause": storage.StringValue("Owner")}},
		"a10000000000002": {ID: "a10000000000002", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "RowCause": storage.StringValue("Reason__c")}},
		"a10000000000003": {ID: "a10000000000003", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "RowCause": storage.StringValue("Other__c")}},
	}
	machine.Org.Objects["Widget__Share"] = shares
	query.Where = nil
	if got, want := machine.soqlSharingUserID(query), machine.currentUserID(); got != want {
		t.Fatalf("extra shares sharing user = %q, want %q", got, want)
	}
}

// Verify the observable share row, not only the chosen sharing identity.
func TestEventSubscriberUnrelatedShareRowNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.eventBusTriggerContext = &eventBusTriggerContext{}
	machine.testContext.StopTimeEventDelivery = true
	machine.staticSOQLQuery = true
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
	row := storage.Record{ID: "a10000000000001", Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000001")}}
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}, Records: map[storage.ID]storage.Record{row.ID: row}}
	if machine.userModeRecordVisible("Widget__Share", row, machine.currentUserID()) {
		t.Fatal("unrelated parent share is visible")
	}
}
