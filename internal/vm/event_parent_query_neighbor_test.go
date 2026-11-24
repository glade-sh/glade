package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
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
