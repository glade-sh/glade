package vm

import (
	"github.com/glade-sh/glade/internal/soql"
	"testing"
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
