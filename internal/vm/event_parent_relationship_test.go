package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func qualifyEventParentUnit(machine *VM, event Value, version string) {
	machine.Classes["EventParentUnit"] = Class{Name: "EventParentUnit", IsTest: true, APIVersion: version}
	machine.testExecutionEntry = &testExecutionEntry{className: "EventParentUnit"}
	event.Fields["Id"] = String("e00000000000001")
	machine.testContext.eventParentPublishedIDs = []storage.ID{"e00000000000001"}
}

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
	qualifyEventParentUnit(machine, event, "62.0")
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
	delete(parents.Records, "a00000000000002")
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}, Records: map[storage.ID]storage.Record{
		"a10000000000001": {ID: "a10000000000001", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue("Owner"), "AccessLevel": storage.StringValue("All")}},
		"a10000000000002": {ID: "a10000000000002", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent.ID), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue("Reason__c"), "AccessLevel": storage.StringValue("Read")}},
	}}
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

// Native event_subscriber_unrelated_visibility rows 2-3 and released main
// expose a share row for a second private parent.
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
	if !machine.userModeRecordVisible("Widget__Share", row, machine.currentUserID()) {
		t.Fatal("unrelated parent share is hidden")
	}
}

// Native event_subscriber_unrelated_visibility rows 2-3 count the second
// publisher-owned parent when both parents have the captured share set.
func TestEventSecondParentCapturedScope(t *testing.T) {
	for _, change := range []string{"captured", "extra parent", "different owner", "extra share"} {
		t.Run(change, func(t *testing.T) {
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
			for id, parent := range parents.Records {
				parent.System.OwnerID = publisher.ID
				parent.System.CreatedByID = publisher.ID
				parents.Records[id] = parent
			}
			event := Object("Signal__e")
			event.Fields["ParentRef__c"] = String("a00000000000001")
			event.Fields["CreatedById"] = String(string(publisher.ID))
			qualifyEventParentUnit(machine, event, "62.0")
			machine.triggerGlobals = map[string]Value{"Trigger.new": List(event)}
			shares := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}, Records: map[storage.ID]storage.Record{}}
			for i, parent := range []storage.ID{"a00000000000001", "a00000000000002"} {
				for j, cause := range []string{"Owner", "Reason__c"} {
					id := storage.ID(fmt.Sprintf("a1000000000000%d", 1+2*i+j))
					access := "Read"
					if cause == "Owner" {
						access = "All"
					}
					shares.Records[id] = storage.Record{ID: id, Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue(cause), "AccessLevel": storage.StringValue(access)}}
				}
			}
			switch change {
			case "extra parent":
				parents.Records["a00000000000003"] = storage.Record{ID: "a00000000000003", Object: "Widget__c", System: storage.SystemFields{OwnerID: publisher.ID, CreatedByID: publisher.ID}}
			case "different owner":
				parent := parents.Records["a00000000000002"]
				parent.System.OwnerID = "005000000000002"
				parents.Records[parent.ID] = parent
			case "extra share":
				shares.Records["a10000000000005"] = storage.Record{ID: "a10000000000005", Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000002"), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue("Extra__c"), "AccessLevel": storage.StringValue("Read")}}
			}
			machine.Org.Objects["Widget__Share"] = shares
			query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue("a00000000000002")}}
			for version := 50; version <= 68; version++ {
				machine.currentMethod.APIVersion = fmt.Sprintf("%d.0", version)
				qualifyEventParentUnit(machine, event, machine.currentMethod.APIVersion)
				want := change == "captured" && (version == 62 || version == 67)
				if got := machine.capturedEventSubscriberCount(query); got != want {
					t.Errorf("API %d: second parent scope = %t, want %t", version, got, want)
				}
			}
		})
	}
}

// Uncaptured referenced-parent record and share sets retain main sharing.
func TestEventReferencedParentShapeNeighbors(t *testing.T) {
	for _, change := range []string{"captured", "extra parent", "extra share"} {
		t.Run(change, func(t *testing.T) {
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
			for id, parent := range parents.Records {
				parent.System.OwnerID = publisher.ID
				parent.System.CreatedByID = publisher.ID
				parents.Records[id] = parent
			}
			event := Object("Signal__e")
			event.Fields["ParentRef__c"] = String("a00000000000001")
			event.Fields["CreatedById"] = String(string(publisher.ID))
			qualifyEventParentUnit(machine, event, "62.0")
			machine.triggerGlobals = map[string]Value{"Trigger.new": List(event)}
			shares := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share", Metadata: map[string]string{"kind": "generatedShare", "parentObject": "Widget__c"}}, Records: map[storage.ID]storage.Record{}}
			for i, parent := range []storage.ID{"a00000000000001", "a00000000000002"} {
				for j, cause := range []string{"Owner", "Reason__c"} {
					id := storage.ID(fmt.Sprintf("a1000000000000%d", 1+2*i+j))
					access := "Read"
					if cause == "Owner" {
						access = "All"
					}
					shares.Records[id] = storage.Record{ID: id, Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue(cause), "AccessLevel": storage.StringValue(access)}}
				}
			}
			switch change {
			case "extra parent":
				parents.Records["a00000000000003"] = storage.Record{ID: "a00000000000003", Object: "Widget__c", System: storage.SystemFields{OwnerID: publisher.ID, CreatedByID: publisher.ID}}
			case "different owner":
				parent := parents.Records["a00000000000002"]
				parent.System.OwnerID = "005000000000002"
				parents.Records[parent.ID] = parent
			case "extra share":
				shares.Records["a10000000000005"] = storage.Record{ID: "a10000000000005", Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000002"), "UserOrGroupId": storage.IDValue(publisher.ID), "RowCause": storage.StringValue("Extra__c"), "AccessLevel": storage.StringValue("Read")}}
			}
			machine.Org.Objects["Widget__Share"] = shares
			query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue("a00000000000001")}}
			for version := 50; version <= 68; version++ {
				machine.currentMethod.APIVersion = fmt.Sprintf("%d.0", version)
				qualifyEventParentUnit(machine, event, machine.currentMethod.APIVersion)
				want := change == "captured" && (version == 62 || version == 67)
				got := machine.soqlSharingUserID(query)
				if want && got != "" || !want && got != machine.currentUserID() {
					t.Errorf("API %d: sharing user = %q, bypass = %t", version, got, want)
				}
			}
		})
	}
}
