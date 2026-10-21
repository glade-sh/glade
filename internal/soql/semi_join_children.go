package soql

import (
	"fmt"
	"strings"
)

func semiJoinCountRejection(joins []Condition) error {
	if len(joins) == 3 {
		positive := true
		for _, join := range joins {
			positive = positive && strings.EqualFold(join.Op, "IN")
		}
		if positive {
			return semiJoinError("Maximum 2 semi join sub-selects are allowed")
		}
	}
	return fmt.Errorf("soql: a WHERE clause can contain at most two semi-join or anti-join subqueries")
}

// Captures admit a single child ID projection with a direct positive semi-join
// selecting parent IDs. Other child semi-join shapes keep the previous boundary.
func semiJoinChildQuerySupported(parent Query, child ChildQuery) bool {
	query := child.Query
	join := query.Where
	if len(parent.ChildQueries) != 1 || len(query.Fields) != 1 || !strings.EqualFold(query.Fields[0], "Id") || len(query.ChildQueries) != 0 ||
		query.Count || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 || query.HasLimit || query.HasOffset ||
		join == nil || join.Subquery == nil || join.Not || join.Parenthesized || len(join.And) != 0 || len(join.Or) != 0 || !strings.EqualFold(join.Op, "IN") || strings.Contains(join.Field, ".") {
		return false
	}
	inner := join.Subquery
	return strings.EqualFold(inner.Object, parent.Object) && len(inner.Fields) == 1 && strings.EqualFold(inner.Fields[0], "Id")
}

func semiJoinNestingRejection(query Query) error {
	joins, _, err := semiAntiConditions(query.Where)
	if err == nil && len(joins) == 1 && len(query.ChildQueries) == 0 && query.Having == nil {
		join := joins[0]
		if strings.EqualFold(join.Op, "IN") && len(join.Subquery.Fields) == 1 && strings.EqualFold(join.Subquery.Fields[0], "Id") {
			rejection := semiJoinError("Nesting of semi join sub-selects is not supported")
			rejection.CompileObject = join.Subquery.Object
			return rejection
		}
	}
	return fmt.Errorf("soql: semi-join and anti-join subqueries cannot be nested")
}
