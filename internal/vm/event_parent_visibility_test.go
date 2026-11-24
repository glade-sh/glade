package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// A runAs user's query is a neighbor of the subscriber query, not a subscriber.
func TestEventParentVisibilityRunAsNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	raw := "SELECT COUNT() FROM Widget__c WHERE Id = 'a00000000000001' WITH USER_MODE"
	query, err := machine.parseSOQLAt(raw)
	if err != nil {
		t.Fatal(err)
	}
	machine.staticSOQLQuery = true
	if got := machine.soqlSharingUserID(query); got != machine.currentUserID() {
		t.Fatalf("runAs sharing user = %q, want %q", got, machine.currentUserID())
	}
	result, err := machine.executeSOQL(raw, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	count, ok := aggregateCount(result)
	if !ok || count.Int != 0 {
		t.Fatalf("runAs count = %#v, want zero", result)
	}
}

// An ordinary runAs user retains the parent-based share rule.
func TestEventShareVisibilityRunAsNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	if !machine.shareTableFollowsParent() {
		t.Fatal("ordinary runAs user lost parent-based share visibility")
	}
}

// Versions without a captured row keep the prior sharing path.
func TestEventParentVisibilityVersionScope(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.eventBusTriggerContext = &eventBusTriggerContext{}
	machine.testContext.StopTimeEventDelivery = true
	machine.staticSOQLQuery = true
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}}
	users := machine.Org.Objects["User"]
	publisher := users.Records["005000000000002"]
	publisher.Fields["ProfileId"] = storage.IDValue("00e000000000002")
	users.Records[publisher.ID] = publisher
	profiles := machine.Org.Objects["Profile"]
	profile := profiles.Records["00e000000000002"]
	profile.Fields["Name"] = storage.StringValue("System Administrator")
	profiles.Records[profile.ID] = profile
	parents := machine.Org.Objects["Widget__c"]
	parent := parents.Records["a00000000000002"]
	parent.System.CreatedByID = "005000000000002"
	parents.Records[parent.ID] = parent
	shares := machine.Org.Objects["Widget__Share"]
	shares.Records = map[storage.ID]storage.Record{
		"a10000000000001": {ID: "a10000000000001", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "RowCause": storage.StringValue("Owner")}},
		"a10000000000002": {ID: "a10000000000002", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "RowCause": storage.StringValue("Reason__c")}},
	}
	machine.Org.Objects["Widget__Share"] = shares
	event := Object("Signal__e")
	event.Fields["ParentRef__c"] = String(string(parent.ID))
	event.Fields["CreatedById"] = String(string(publisher.ID))
	machine.triggerGlobals = map[string]Value{"Trigger.new": List(event)}
	query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue(parent.ID)}}
	shareQuery := query
	shareQuery.Object = "Widget__Share"
	shareQuery.Where = nil
	for version := 50; version <= 68; version++ {
		machine.currentMethod.APIVersion = fmt.Sprintf("%d.0", version)
		if got, want := machine.capturedEventSubscriberCount(query), version == 62 || version == 67; got != want {
			t.Errorf("API %d: captured parent shape=%t, want %t", version, got, want)
		}
		// Share visibility follows the released path, not the parent-count rule.
		if got := machine.capturedEventSubscriberCount(shareQuery); got {
			t.Errorf("API %d: share query entered the parent-count rule", version)
		}
	}
}
