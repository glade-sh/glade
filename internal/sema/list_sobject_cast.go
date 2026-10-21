package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

// A direct query keeps singleton assignment compatibility. A stored list or
// Object bridge has a different operand form and retains ordinary cast checks.
func semaListSObjectDirectQueryCast(target, value string, expr ir.Expr, model *semaTypeMemberView) bool {
	if !expr.CastQueryOperand || len(expr.Args) != 1 || !isSemaSObjectLike(target, model) || semaProjectTypeShadowsPlatform(model, target) {
		return false
	}
	operand := expr.Args[0]
	if operand.Kind == ir.ExprSOQL {
		return semaSOQLSingletonAssignable(target, value, "["+operand.Value+"]", model)
	}
	return strings.EqualFold(value, "Database.QueryResult")
}

// CL047-055, both routes at API 62/67/68: preserve the existing rejection,
// but give the captured direct typed casts their native diagnostic text.
// Object bridges and list-to-list casts never enter this rejected-cast path.
func semaListSObjectCastMessage(target, value string, model *semaTypeMemberView) string {
	target = semaCanonicalAssignableType(target)
	base, args := semaGenericBaseAndArgs(semaCanonicalAssignableType(value))
	if !strings.EqualFold(base, "List") || len(args) != 1 ||
		semaProjectTypeShadowsPlatform(model, target) || semaProjectTypeShadowsPlatform(model, args[0]) {
		return ""
	}
	for _, pair := range [][2]string{{"Account", "Account"}, {"SObject", "Account"}, {"SObject", "SObject"}} {
		if strings.EqualFold(args[0], pair[0]) && strings.EqualFold(target, pair[1]) {
			return "Incompatible types since an instance of List<" + pair[0] + "> is never an instance of " + pair[1]
		}
	}
	return ""
}
