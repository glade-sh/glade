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

// Captured direct child semi-joins accept both ID-to-reference matching and
// the earlier reference-to-parent-ID form. Other shapes retain their boundary.
func semiJoinChildQuerySupported(parent Query, child ChildQuery) bool {
	if conjunctiveChildSemiJoinSupported(parent, child.Query) {
		return true
	}
	query := child.Query
	join := query.Where
	if len(parent.ChildQueries) != 1 || len(query.ChildQueries) != 0 ||
		query.Count || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 || query.HasLimit || query.HasOffset ||
		join == nil || join.Subquery == nil || join.Not || join.Parenthesized || len(join.And) != 0 || len(join.Or) != 0 || strings.Contains(join.Field, ".") {
		return false
	}
	inner := join.Subquery
	if len(inner.Fields) != 1 {
		return false
	}
	if strings.EqualFold(join.Field, "Id") && (strings.EqualFold(join.Op, "IN") || strings.EqualFold(join.Op, "NOT IN")) {
		return true
	}
	return len(query.Fields) == 1 && strings.EqualFold(query.Fields[0], "Id") && strings.EqualFold(join.Op, "IN") &&
		strings.EqualFold(inner.Object, parent.Object) && strings.EqualFold(inner.Fields[0], "Id")
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

// Captured sibling-child queries use one ID semi-join alongside scalar
// conjuncts and a single-row child window. Other shapes retain their boundary.
func conjunctiveChildSemiJoinSupported(parent, query Query) bool {
	if len(parent.ChildQueries) < 3 || len(parent.ChildQueries) > 4 ||
		parent.HasLimit || parent.HasOffset || len(parent.Order) != 0 || parent.OrderBy != "" || parent.Count || len(parent.Aggregates) != 0 || len(parent.GroupBy) != 0 || parent.Having != nil || len(parent.Typeofs) != 0 || parent.ForUpdate || parent.ForView || parent.ForReference || parent.AllRows || parent.SecurityMode != "" || parent.UsingScope != "" || parent.OptionsBind != "" || parent.DataCategoryGroup != "" || parent.GroupMode != "" || (parent.Where != nil && conditionContainsSubquery(*parent.Where)) ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || query.Count || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 || query.OrderBy != "" || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.SecurityMode != "" || query.UsingScope != "" || query.OptionsBind != "" || query.DataCategoryGroup != "" || query.GroupMode != "" ||
		!query.HasLimit || query.Limit != 1 || query.LimitBind != "" || query.HasOffset || query.Where == nil {
		return false
	}
	where := query.Where
	if !where.SourceAnd || where.Not || where.Parenthesized || where.Field != "" || where.Op != "" || where.Formula != nil || where.Subquery != nil || len(where.Or) != 0 || len(where.And) != len(parent.ChildQueries)-1 {
		return false
	}
	joins := 0
	for _, condition := range where.And {
		if condition.Not || condition.Parenthesized || condition.Field == "" || condition.Range || len(condition.And) != 0 || len(condition.Or) != 0 || condition.Formula != nil {
			return false
		}
		if condition.Subquery != nil {
			if !strings.EqualFold(condition.Field, "Id") || !strings.EqualFold(condition.Op, "IN") || len(condition.Subquery.Fields) != 1 {
				return false
			}
			joins++
		} else if condition.Op != "=" && condition.Op != "!=" {
			return false
		}
	}
	return joins == 1
}
