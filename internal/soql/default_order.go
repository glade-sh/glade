package soql

import "strings"

// Default result order without ORDER BY (re_order_default capture, API 62.0 and
// 67.0). A prefix LIKE on the name field is served by the name index, so the
// rows come back in index order. Only the captured shapes fire:
//
//   - an Account query whose entire WHERE clause is Name LIKE 'prefix%' returns
//     ascending Name;
//   - an Account to Contacts child subquery whose entire WHERE clause is
//     LastName LIKE 'prefix%' returns the contacts by descending LastName.
//
// Every other object, field, compound filter (AND, OR, NOT) and unfiltered
// query keeps the Id order. The capture did not fit GROUP BY, custom-object or
// describe order, nor any other child object.

// indexedNamePrefixField returns the canonical field name when the condition
// is a single prefix LIKE on one of fields and is the whole WHERE clause.
func indexedNamePrefixField(condition *Condition, objectName string, fields ...string) (string, bool) {
	if condition == nil || condition.Not || len(condition.Or) > 0 || len(condition.And) > 0 {
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

// defaultChildOrder is the order of a child subquery without ORDER BY. Only
// Contact with a LastName prefix LIKE was captured.
func defaultChildOrder(query Query, childObjectName string) []OrderSpec {
	if len(query.Order) > 0 || !strings.EqualFold(childObjectName, "Contact") {
		return query.Order
	}
	if field, ok := indexedNamePrefixField(query.Where, childObjectName, "LastName"); ok {
		return []OrderSpec{{Field: field, Desc: true}}
	}
	return query.Order
}
