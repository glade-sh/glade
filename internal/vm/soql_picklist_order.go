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
	// Restricted picklists were captured only at source 62.0 and 67.0.
	return withLegacyPicklistNullOrder(query, major == 62)
}

func withLegacyPicklistNullOrder(query soql.Query, restricted bool) soql.Query {
	query.LegacyPicklistNullOrder = true
	query.LegacyRestrictedPicklistNullOrder = restricted
	if len(query.ChildQueries) != 0 {
		query.ChildQueries = append([]soql.ChildQuery(nil), query.ChildQueries...)
		for i := range query.ChildQueries {
			query.ChildQueries[i].Query = withLegacyPicklistNullOrder(query.ChildQueries[i].Query, restricted)
		}
	}
	return query
}
