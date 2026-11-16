package soql

import "strings"

// Default result order without ORDER BY (re_order_default capture, API 62.0 and
// 67.0). A prefix LIKE on the name field is served by the name index, so the
// rows come back in index order:
//
//   - an Account query filtered by Name LIKE 'prefix%' returns ascending Name;
//   - a child subquery filtered by Name or LastName LIKE 'prefix%' returns the
//     children by descending Name (LastName).
//
// Unfiltered queries, other filters and other objects keep the Id order. The
// capture did not fit GROUP BY, custom-object or describe order.

// indexedNamePrefixField returns the canonical name of the first field in
// fields that the condition constrains with a prefix LIKE, joined by AND only.
func indexedNamePrefixField(condition *Condition, objectName string, fields ...string) (string, bool) {
	if condition == nil || condition.Not || len(condition.Or) > 0 {
		return "", false
	}
	if len(condition.And) > 0 {
		for i := range condition.And {
			if field, ok := indexedNamePrefixField(&condition.And[i], objectName, fields...); ok {
				return field, true
			}
		}
		return "", false
	}
	if !strings.EqualFold(condition.Op, "LIKE") || condition.Subquery != nil || condition.Formula != nil {
		return "", false
	}
	pattern := condition.Value.String
	if pattern == "" || pattern[0] == '%' || pattern[0] == '_' {
		return "", false
	}
	name := condition.Field
	if prefix := objectName + "."; len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
		name = name[len(prefix):]
	}
	for _, field := range fields {
		if strings.EqualFold(name, field) {
			return field, true
		}
	}
	return "", false
}

// defaultParentOrder is the index order of an Account query without ORDER BY.
func defaultParentOrder(query Query, objectName string) []OrderSpec {
	if len(query.Order) > 0 || !strings.EqualFold(objectName, "Account") || queryHasAggregates(query) {
		return query.Order
	}
	if field, ok := indexedNamePrefixField(query.Where, objectName, "Name"); ok {
		return []OrderSpec{{Field: field}}
	}
	return query.Order
}

// defaultChildOrder is the order of a child subquery without ORDER BY.
func defaultChildOrder(query Query, childObjectName string) []OrderSpec {
	if len(query.Order) > 0 {
		return query.Order
	}
	if field, ok := indexedNamePrefixField(query.Where, childObjectName, "Name", "LastName"); ok {
		return []OrderSpec{{Field: field, Desc: true}}
	}
	return query.Order
}
