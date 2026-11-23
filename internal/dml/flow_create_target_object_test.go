package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Only the objects with native rows (flow_create_unset_variable_reference,
// anonymous route) are materialized for a flow create; every other name keeps
// the origin/main error.
func TestFlowCreateTargetObjectNameOnlyMaterializesCapturedObjects(t *testing.T) {
	for _, name := range []string{"PricebookEntry", "Pricebook2"} {
		org := storage.NewOrgState()
		engine := &Engine{Org: &org}
		if resolved, ok := engine.flowCreateTargetObjectName(name); !ok || resolved != name {
			t.Errorf("%s: resolved %q ok=%v", name, resolved, ok)
		}
	}
	for _, name := range []string{"Entitlement", "Asset", "Nope__c", ""} {
		org := storage.NewOrgState()
		before := len(org.Objects)
		engine := &Engine{Org: &org}
		if resolved, ok := engine.flowCreateTargetObjectName(name); ok && len(org.Objects) != before {
			t.Errorf("%q: materialized %q", name, resolved)
		}
	}
}
