package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/soql"
)

// SEMIJOIN, both routes at 62/67/68: the captured custom-parent Id IN query
// cannot use ContentDocumentLink as its inner entity. Other entities, operands,
// anti-joins and query shapes retain their existing compile path.
func contentChildInnerEntityDiagnostics(parent schema.Object, query soql.Query, ctx queryTextContext, cursor int) []diagnostic.Diagnostic {
	if parent.CustomSettingsType != "" || !strings.HasSuffix(strings.ToLower(parent.Name), "__c") ||
		!contentChildInnerEntityPlainQuery(query, "Id") || query.Where == nil ||
		query.Where.Not || len(query.Where.Or) != 0 || len(query.Where.And) != 2 {
		return nil
	}
	var scope, membership *soql.Condition
	for i := range query.Where.And {
		predicate := &query.Where.And[i]
		if predicate.Not || len(predicate.And) != 0 || len(predicate.Or) != 0 || !strings.EqualFold(predicate.Field, "Id") {
			return nil
		}
		switch predicate.Op {
		case "=":
			if predicate.Subquery != nil || predicate.Range {
				return nil
			}
			scope = predicate
		case "IN":
			membership = predicate
		default:
			return nil
		}
	}
	if scope == nil || membership == nil || membership.Subquery == nil {
		return nil
	}
	inner := membership.Subquery
	if !strings.EqualFold(inner.Object, "ContentDocumentLink") || !contentChildInnerEntityPlainQuery(*inner, "LinkedEntityId") || inner.Where == nil {
		return nil
	}
	predicate := inner.Where
	if predicate.Not || predicate.Range || len(predicate.And) != 0 || len(predicate.Or) != 0 ||
		predicate.Subquery != nil || !strings.EqualFold(predicate.Field, "LinkedEntityId") || predicate.Op != "=" {
		return nil
	}
	outerBind, outerBound := soqlBindName(scope.Value)
	innerBind, innerBound := soqlBindName(predicate.Value)
	if !outerBound || !innerBound || !strings.EqualFold(outerBind, innerBind) {
		return nil
	}
	message := "Entity 'ContentDocumentLink' is not supported for semi join inner selects"
	offset := findSOQLFromObject(ctx.queryText, inner.Object, cursor)
	return []diagnostic.Diagnostic{ctx.diagnostic("GLADESEMA_QUERY_CONTRACT", message, inner.Object, offset)}
}

func contentChildInnerEntityPlainQuery(query soql.Query, field string) bool {
	return len(query.Fields) == 1 && strings.EqualFold(query.Fields[0], field) &&
		len(query.ChildQueries) == 0 && len(query.Typeofs) == 0 && query.Having == nil &&
		len(query.Order) == 0 && query.OrderBy == "" && !query.HasLimit && query.LimitBind == "" &&
		!query.HasOffset && query.OffsetBind == "" && !query.Count &&
		len(query.Aggregates) == 0 && len(query.HavingAggregates) == 0 && len(query.GroupBy) == 0 && query.GroupMode == "" &&
		!query.ForUpdate && !query.ForView && !query.ForReference && !query.AllRows && query.UsingScope == "" && query.SecurityMode == ""
}
