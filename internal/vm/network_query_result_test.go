package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEmptyNetworkQuerySingletonNativeBoundary(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Network")
	machine.SetOrg(&org)
	empty, err := machine.executeSOQLQuery("SELECT Id FROM Network WHERE Name = 'Missing Synthetic Community'", &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.coerceAssignable("SObject", empty); err == nil || !strings.Contains(err.Error(), "List has no rows for assignment to SObject") {
		t.Fatalf("missing configuration must retain native QueryException: %v", err)
	}
	for _, populated := range []bool{false, true} {
		records := List()
		records.Type = "List<Network>"
		if populated {
			records.List = []Value{Object("Network")}
			tagSOQLQueryList(&records, "SELECT Id FROM Network")
			records.List = nil
		}
		if _, err := machine.coerceAssignable("SObject", records); err == nil || !strings.Contains(err.Error(), "List has no rows for assignment to SObject") {
			t.Fatalf("ordinary/cleared list must retain its query exception: %v", err)
		}
	}
}
