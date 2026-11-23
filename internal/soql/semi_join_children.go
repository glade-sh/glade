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

// Captured child semi-joins (62.0 and 67.0, anonymous and @IsTest) accept a
// top-level conjunction in the child WHERE holding one or two IN or NOT IN
// subqueries on a single-field select, beside simple scalar conjuncts, with no
// LIMIT or a literal LIMIT, for any number of sibling child queries. OR forms
// and nested groups are rejected by Salesforce and stay rejected here. Other
// shapes retain their boundary.
func conjunctiveChildSemiJoinSupported(parent, query Query) bool {
	if parent.HasLimit || parent.HasOffset || len(parent.Order) != 0 || parent.OrderBy != "" || parent.Count || len(parent.Aggregates) != 0 || len(parent.GroupBy) != 0 || parent.Having != nil || len(parent.Typeofs) != 0 || parent.ForUpdate || parent.ForView || parent.ForReference || parent.AllRows || parent.SecurityMode != "" || parent.UsingScope != "" || parent.OptionsBind != "" || parent.DataCategoryGroup != "" || parent.GroupMode != "" || (parent.Where != nil && conditionContainsSubquery(*parent.Where)) ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || query.Count || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 || query.OrderBy != "" || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.SecurityMode != "" || query.UsingScope != "" || query.OptionsBind != "" || query.DataCategoryGroup != "" || query.GroupMode != "" ||
		query.LimitBind != "" || query.HasOffset || query.Offset != 0 || (query.HasLimit && query.Limit < 1) || query.Where == nil {
		return false
	}
	withJoins := 0
	for _, sibling := range parent.ChildQueries {
		if queryHasSemiAntiJoin(sibling.Query) {
			withJoins++
		}
	}
	if withJoins != 1 {
		return false
	}
	terms := []Condition{*query.Where}
	if query.Where.Subquery == nil {
		where := query.Where
		if !where.SourceAnd || where.Not || where.Parenthesized || where.Field != "" || where.Op != "" || where.Formula != nil || len(where.Or) != 0 || len(where.And) < 2 {
			return false
		}
		terms = where.And
	}
	joins := 0
	for _, condition := range terms {
		if condition.Not || condition.Parenthesized || condition.Field == "" || condition.Range || len(condition.And) != 0 || len(condition.Or) != 0 || condition.Formula != nil {
			return false
		}
		if condition.Subquery != nil {
			if !strings.EqualFold(condition.Op, "IN") && !strings.EqualFold(condition.Op, "NOT IN") || len(condition.Subquery.Fields) != 1 || strings.Contains(condition.Field, ".") {
				return false
			}
			joins++
		} else if condition.Op != "=" && condition.Op != "!=" {
			return false
		}
	}
	return joins == 1 || joins == 2
}
