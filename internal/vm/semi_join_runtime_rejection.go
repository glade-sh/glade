package vm

import (
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
)

func (vm *VM) validateSemiJoinRelationshipOperand(query soql.Query, queryText string) error {
	if vm.Org == nil || !apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 67) {
		// Keep the existing API <=62 path. APIs 63-66 are unbisected and
		// retain that existing validation order as well.
		return nil
	}
	if rejection := soql.SemiJoinMissingRelationshipLeafRejection(*vm.Org, query); rejection != nil {
		return soqlQueryException(rejection, queryText)
	}
	return nil
}
