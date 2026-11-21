package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDescribeChildPruningExecutionContexts(t *testing.T) {
	for _, features := range [][]string{nil, {"MultiCurrency"}, {"PersonAccounts"}} {
		org := shapedLazyOrg(features)
		template := NewRestoredRuntimeTemplate(org, New(nil))
		machine := describePruningClone(template)
		for _, api := range []string{"62.0", "67.0", "68.0"} {
			machine.currentMethod = Method{APIVersion: api}
			compareChildDescribeWithBaseline(t, machine, "Contact")
		}
	}
}

func TestDescribeChildPruningPrewarmedDefinition(t *testing.T) {
	template := describePruningTemplate()
	machine := describePruningClone(template)
	storage.MaterializeStandardObjectSpread(machine.Org)
	machine.ensureMetadataCacheGeneration()
	state := machine.Org.Objects["Branch000__c"]
	_ = machine.describePreparedDefinition("Branch000__c", state.Definition)
	compareChildDescribeWithBaseline(t, machine, "Root001__c")
}
