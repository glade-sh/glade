package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// shapedLazyOrg is the lazy org of a test run once the project shape applied.
func shapedLazyOrg(features []string) storage.OrgState {
	org := storage.NewOrgState()
	org.OrgID = storage.DefaultOrgID
	org.EnableStandardSpreadSource()
	org.APIVersion = storage.DefaultRESTAPIVersion
	storage.ApplyProjectOrgShape(&org, features)
	return org
}

// A name that is not a child relationship must not complete the lazy spread.
// Reading an unset field reaches this lookup through missingSObjectFieldValue.
func TestChildRelationshipMissKeepsLazySpread(t *testing.T) {
	org := shapedLazyOrg([]string{"MultiCurrency"})
	held := len(org.Objects)
	machine := New(nil)
	machine.SetOrg(&org)

	for _, name := range []string{"Nonsense__r", "Status__c", "NoSuchField", "nonsense__R"} {
		if got, ok := machine.jsonSObjectChildRelationshipType("Account", name); ok || got != "" {
			t.Fatalf("Account.%s = %q, %v; want no child relationship", name, got, ok)
		}
	}
	receiver := Object("Account")
	machine.missingSObjectFieldValue(receiver, "Nonsense__r")
	if !org.LazyStandardSpreadEnabled() {
		t.Fatal("a miss on a name that no catalog object declares completed the lazy spread")
	}
	if installed := len(org.Objects); installed > held+50 {
		t.Fatalf("org holds %d objects after the misses, %d before", installed, held)
	}
}
