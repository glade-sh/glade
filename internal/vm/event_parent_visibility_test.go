package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
)

// A runAs user's query is a neighbor of the subscriber query, not a subscriber.
func TestEventParentVisibilityRunAsNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Where: &soql.Condition{Field: "Id", Op: "="}}
	if got := machine.soqlSharingUserID(query); got != machine.currentUserID() {
		t.Fatalf("runAs sharing user = %q, want %q", got, machine.currentUserID())
	}
	record := machine.Org.Objects["Widget__c"].Records["a00000000000001"]
	if machine.userModeRecordVisible("Widget__c", record, machine.currentUserID()) {
		t.Fatal("runAs user can see another user's unshared private record")
	}
}

// A runAs user inside an event trigger retains the parent-based share rule.
func TestEventShareVisibilityRunAsNeighbor(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentMethod.APIVersion = "62.0"
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	if !machine.shareTableFollowsParent() {
		t.Fatal("runAs user inside subscriber lost parent-based share visibility")
	}
}

// Versions without a captured row keep the prior sharing path.
func TestEventParentVisibilityVersionScope(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	machine.currentTrigger = true
	machine.eventSubscriberTriggerDepth = 1
	machine.testContext.CurrentUser.Fields["UserType"] = String("AutomatedProcess")
	query := soql.Query{Object: "Widget__c", Count: true, SecurityMode: "USER_MODE", Aggregates: []soql.Aggregate{{Func: "COUNT"}}, Where: &soql.Condition{Field: "Id", Op: "="}}
	for version := 50; version <= 68; version++ {
		machine.currentMethod.APIVersion = fmt.Sprintf("%d.0", version)
		if got, want := machine.capturedEventSubscriberParentCount(query), version == 62 || version == 67; got != want {
			t.Errorf("API %d: captured shape=%t, want %t", version, got, want)
		}
	}
}
