package soql

import "strings"

// The direct ID-filtered subquery captures distinguish inline grammar recovery
// from Database.query's missing-value exception. FOR and other window shapes
// retain their previous parser recovery.
func semiJoinWindowRejection(query Query, clause string) error {
	legacy := querySyntaxError("missing value at '"+clause+"'", "semi-join subqueries do not support ORDER BY, LIMIT, or FOR UPDATE")
	where := query.Where
	if query.ForUpdate || where == nil || where.Not || where.Subquery != nil || len(where.And) != 0 || len(where.Or) != 0 || !strings.EqualFold(where.Field, "Id") || !strings.EqualFold(where.Op, "IN") {
		return legacy
	}
	limit := clause == "LIMIT" && query.HasLimit && query.Limit == 1 && query.LimitBind == "" && len(query.Order) == 0 && (!query.HasOffset || query.Offset == 1 && query.OffsetBind == "")
	order := clause == "ORDER" && len(query.Order) == 1 && !query.Order[0].Desc && !query.HasLimit && !query.HasOffset
	if !limit && !order {
		return legacy
	}
	return &SemiJoinError{QueryError: legacy.(*QueryError), CompileMessage: "Missing ')' at '" + clause + "'"}
}
