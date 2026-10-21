package vm

import (
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
)

// Owned picklist-order observations distinguish source 62/64/65 from 66/67,
// including mixed source/transport calls. The source policy follows the method.
func (vm *VM) applyPicklistSourcePolicy(query soql.Query) soql.Query {
	major, ok := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !ok || major < 62 || major >= 66 {
		return query
	}
	return withLegacyPicklistNullOrder(query)
}

func withLegacyPicklistNullOrder(query soql.Query) soql.Query {
	query.LegacyPicklistNullOrder = true
	if len(query.ChildQueries) != 0 {
		query.ChildQueries = append([]soql.ChildQuery(nil), query.ChildQueries...)
		for i := range query.ChildQueries {
			query.ChildQueries[i].Query = withLegacyPicklistNullOrder(query.ChildQueries[i].Query)
		}
	}
	return query
}
