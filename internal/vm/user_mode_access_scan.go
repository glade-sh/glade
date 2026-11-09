package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// A USER_MODE query by a user who cannot read some of the fields it names
// raises one QueryException. Its message names the first denied reference.
// QueryException.getInaccessibleFields() lists every denied field the query
// names, grouped by the object that owns the field. The native rows are in
// apextest/testdata/conformance/user_mode_inaccessible_shapes (API 62, 67, 68;
// static and dynamic forms agree on every shape):
//
//   - The list holds select-list fields (including aggregate arguments, date
//     functions and child subqueries), WHERE, GROUP BY, HAVING and top-level
//     ORDER BY fields, for any WHERE shape, LIMIT, OFFSET, direction and count.
//   - Message order is the select list in source order, GROUP BY, ORDER BY,
//     WHERE in source order, HAVING, then the clauses of child subqueries.
//   - A relationship path lists the denied leaf under the object that owns it.
//     A lookup the user cannot read is listed under its owner, and the message
//     reads "No such relation". A relationship through an object the user
//     cannot read lists the referenced field, Name included, under that object
//     and the message reads "Didn't understand relationship".
//   - A select-list field of an aggregate query, an aggregate argument and a
//     FORMAT argument read "Invalid field: 'F'". FORMAT arguments, ORDER BY
//     relationship paths and semi/anti-join predicates choose the message but
//     are not listed.
//   - A root object the user cannot read lists every non-Id field the query
//     names, and the message reads "sObject type is not supported".
type userModeAccessRef struct {
	message string
	// object and field list the reference in getInaccessibleFields. An empty
	// object means the reference only chooses the message.
	object, field string
}

type userModeDeferredFilters struct {
	query  soql.Query
	object string
}

type userModeAccessScan struct {
	vm       *VM
	rootName string
	// rootDenied marks a root object the user cannot read: every non-Id field
	// of the root counts as denied.
	rootDenied bool
	refs       []userModeAccessRef
	deferred   []userModeDeferredFilters
	// probe only asks whether any reference is denied. It allocates nothing,
	// so a clean query costs permission lookups and no scan state.
	probe, found bool
}

func (s *userModeAccessScan) add(ref userModeAccessRef) {
	if s.probe {
		s.found = true
		return
	}
	s.refs = append(s.refs, ref)
}

// userModeAccessScanError returns the USER_MODE denial for a query, or nil when
// every field it names is readable or the scan cannot classify a reference. The
// bool reports that the scan covered the whole query, child subqueries included.
func (vm *VM) userModeAccessScanError(query soql.Query, objectName string, rootDenied bool) (error, bool) {
	if vm.Org == nil || query.Count && len(query.Fields) == 1 && strings.EqualFold(query.Fields[0], "COUNT()") && query.Where == nil {
		return nil, false
	}
	var probe userModeAccessScan
	probe = userModeAccessScan{vm: vm, rootName: objectName, rootDenied: rootDenied, probe: true}
	ok := probe.query(query, objectName)
	if !probe.found {
		return nil, ok
	}
	scan := &userModeAccessScan{vm: vm, rootName: objectName, rootDenied: rootDenied}
	if !scan.query(query, objectName) {
		return nil, false
	}
	if len(scan.refs) == 0 {
		return nil, true
	}
	return scan.exception(rootDenied, objectName), true
}

func (s *userModeAccessScan) exception(rootDenied bool, objectName string) error {
	message := s.refs[0].message
	if rootDenied {
		message = soql.MissingObjectMessage(objectName)
	}
	objects := []string(nil)
	fields := map[string][]string{}
	seen := map[string]bool{}
	for _, ref := range s.refs {
		if ref.object == "" {
			continue
		}
		key := strings.ToLower(ref.object) + "\x00" + strings.ToLower(ref.field)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := fields[ref.object]; !ok {
			objects = append(objects, ref.object)
		}
		fields[ref.object] = append(fields[ref.object], ref.field)
	}
	if len(objects) == 0 {
		return newExceptionError("QueryException", message)
	}
	inaccessible := Map()
	inaccessible.Type = "Map<String,Set<String>>"
	for _, object := range objects {
		values := make([]Value, len(fields[object]))
		for i, field := range fields[object] {
			values[i] = String(field)
		}
		set := Set(values...)
		set.Type = "Set<String>"
		key := String(object)
		inaccessible.Map[mapKey(key)] = set
		inaccessible.MapKeys[mapKey(key)] = key
	}
	exception := Object("QueryException")
	exception.Fields["message"] = String(message)
	exception.Fields["__inaccessibleFields"] = inaccessible
	return &apexThrowError{value: exception}
}

// query scans the select list and filters of one query. It reports false when
// a child subquery cannot be resolved, so the caller keeps its existing checks.
func (s *userModeAccessScan) query(query soql.Query, objectName string) bool {
	if !s.selectList(query, objectName) {
		return false
	}
	s.filters(query, objectName)
	for len(s.deferred) > 0 {
		next := s.deferred[0]
		s.deferred = s.deferred[1:]
		s.filters(next.query, next.object)
	}
	return true
}

const (
	userModeColumn = iota
	userModeInvalidField
)

func (s *userModeAccessScan) selectList(query soql.Query, objectName string) bool {
	aggregate := len(query.Aggregates) != 0 || len(query.GroupBy) != 0
	children, typeofs := 0, 0
	emit := func(position int) bool {
		for children < len(query.ChildQueries) && query.ChildQueries[children].After <= position {
			if !s.child(objectName, query.ChildQueries[children]) {
				return false
			}
			children++
		}
		for typeofs < len(query.Typeofs) && query.Typeofs[typeofs].After <= position {
			s.typeof(query.Typeofs[typeofs])
			typeofs++
		}
		return true
	}
	for i, field := range query.Fields {
		if !emit(i) {
			return false
		}
		s.selectField(objectName, field, aggregate)
	}
	return emit(len(query.Fields) + 1)
}

func (s *userModeAccessScan) selectField(objectName, expression string, aggregate bool) {
	name, argument, function := userModeSelectExpression(expression)
	if !function {
		if aggregate {
			s.path(objectName, name, userModeInvalidField, true)
		} else {
			s.path(objectName, name, userModeColumn, true)
		}
		return
	}
	if argument == "" || strings.ContainsAny(argument, "(),") || strings.Contains(argument, ".") {
		return
	}
	switch strings.ToUpper(name) {
	case "FORMAT":
		s.path(objectName, argument, userModeInvalidField, false)
	case "COUNT", "SUM", "MIN", "MAX", "AVG", "COUNT_DISTINCT",
		"CALENDAR_MONTH", "CALENDAR_QUARTER", "CALENDAR_YEAR", "DAY_IN_MONTH", "DAY_IN_WEEK", "DAY_IN_YEAR", "DAY_ONLY",
		"FISCAL_MONTH", "FISCAL_QUARTER", "FISCAL_YEAR", "HOUR_IN_DAY", "WEEK_IN_MONTH", "WEEK_IN_YEAR":
		s.path(objectName, argument, userModeInvalidField, true)
	}
}

// userModeSelectExpression splits a select-list entry into its field or
// function name, the single function argument, and whether it is a function.
func userModeSelectExpression(expression string) (name, argument string, function bool) {
	expression = strings.TrimSpace(expression)
	open := strings.IndexByte(expression, '(')
	if open < 0 {
		if space := strings.IndexAny(expression, " \t\r\n"); space >= 0 {
			expression = expression[:space]
		}
		return expression, "", false
	}
	depth, closeAt := 0, -1
	for i := open; i < len(expression) && closeAt < 0; i++ {
		switch expression[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closeAt = i
			}
		}
	}
	if closeAt < 0 {
		return expression, "", true
	}
	return strings.TrimSpace(expression[:open]), strings.TrimSpace(expression[open+1 : closeAt]), true
}

func (s *userModeAccessScan) typeof(spec soql.TypeofSpec) {
	for name, fields := range spec.When {
		objectName, ok := s.vm.resolveObjectName(name)
		if !ok || !s.vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", "") {
			continue
		}
		for _, field := range fields {
			s.field(objectName, field, userModeColumn, true)
		}
	}
}

func (s *userModeAccessScan) child(parent string, child soql.ChildQuery) bool {
	childObject, ok := s.vm.soqlChildRelationshipObject(parent, child.Relationship)
	if !ok {
		return false
	}
	if canonical, found := s.vm.resolveObjectName(childObject); found {
		childObject = canonical
	}
	if !s.vm.currentUserObjectPermissionWithScope(childObject, "isAccessible", "") {
		return false
	}
	if !s.selectList(child.Query, childObject) {
		return false
	}
	if s.probe {
		s.filters(child.Query, childObject)
		return !s.found
	}
	s.deferred = append(s.deferred, userModeDeferredFilters{query: child.Query, object: childObject})
	return true
}

func (s *userModeAccessScan) filters(query soql.Query, objectName string) {
	for _, group := range query.GroupBy {
		name, argument, function := userModeSelectExpression(group)
		if function {
			name = argument
		}
		s.path(objectName, name, userModeColumn, true)
	}
	for _, order := range query.Order {
		name, argument, function := userModeSelectExpression(order.Field)
		if function {
			name = argument
		}
		if strings.Contains(name, ".") {
			s.path(objectName, name, userModeColumn, false)
		} else {
			s.path(objectName, name, userModeColumn, true)
		}
	}
	s.condition(query.Where, objectName, true)
	for _, having := range query.HavingAggregates {
		s.path(objectName, having.Field, userModeColumn, true)
	}
}

func (s *userModeAccessScan) condition(condition *soql.Condition, objectName string, listed bool) {
	if condition == nil {
		return
	}
	for i := range condition.And {
		s.condition(&condition.And[i], objectName, listed)
	}
	for i := range condition.Or {
		s.condition(&condition.Or[i], objectName, listed)
	}
	if condition.Formula != nil || strings.TrimSpace(condition.Field) == "" {
		if condition.Subquery != nil {
			s.semiJoin(condition.Subquery)
		}
		return
	}
	s.path(objectName, condition.Field, userModeColumn, listed)
	if condition.Subquery != nil {
		s.semiJoin(condition.Subquery)
	}
}

// A semi- or anti-join predicate chooses the message but is not listed.
func (s *userModeAccessScan) semiJoin(inner *soql.Query) {
	objectName, ok := s.vm.resolveObjectName(inner.Object)
	if !ok || !s.vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", "") {
		return
	}
	s.condition(inner.Where, objectName, false)
}

// path resolves a field path from objectName and records each denied
// reference. A relationship name is checked through its lookup field.
func (s *userModeAccessScan) path(objectName, path string, kind int, listed bool) {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsAny(path, "() \t") || strings.HasPrefix(path, "\x00") {
		return
	}
	var buffer [8]string
	parts := splitUserModePath(buffer[:0], path)
	if len(parts) >= 2 && s.vm.soqlFieldQualifierMatchesObject(objectName, parts[0]) {
		parts = parts[1:]
	}
	if len(parts) == 1 {
		s.field(objectName, parts[0], kind, listed)
		return
	}
	if s.rootDenied && strings.EqualFold(objectName, s.rootName) {
		return
	}
	current := objectName
	for i, part := range parts[:len(parts)-1] {
		relation, ok := s.vm.parentRelationshipMetadata(current, part)
		if !ok || len(relation.ParentObjects) != 1 {
			return
		}
		target, ok := s.vm.resolveObjectName(relation.ParentObjects[0])
		if !ok {
			return
		}
		if lookup, found := s.vm.soqlRelationshipReferenceFieldName(current, relation); found &&
			!s.vm.currentUserFieldPermissionWithScope(current, lookup, "isAccessible", "") {
			message := strings.Replace(soql.MissingColumnMessage(part, current), "No such column", "No such relation", 1)
			s.add(userModeAccessRef{message: message, object: userModeListedObject(current, listed), field: lookup})
		}
		leaf := parts[len(parts)-1]
		if !s.vm.currentUserObjectPermissionWithScope(target, "isAccessible", "") {
			// Only a custom parent object has native rows.
			if i != len(parts)-2 || !strings.HasSuffix(strings.ToLower(target), "__c") || strings.EqualFold(leaf, "Id") {
				return
			}
			definition := s.vm.Org.Objects[target].Definition
			name, known := storage.ResolveFieldName(definition, s.vm.Org.Namespace, leaf)
			if !known {
				return
			}
			s.add(userModeAccessRef{message: soql.MissingRelationshipMessage(part, "field path"), object: userModeListedObject(target, listed), field: name})
			return
		}
		current = target
	}
	s.field(current, parts[len(parts)-1], kind, listed)
}

func userModeListedObject(objectName string, listed bool) string {
	if listed {
		return objectName
	}
	return ""
}

// field records a denied field of objectName.
func (s *userModeAccessScan) field(objectName, field string, kind int, listed bool) {
	field = strings.TrimSpace(field)
	if field == "" || strings.EqualFold(field, "Id") {
		return
	}
	definition := s.vm.Org.Objects[objectName].Definition
	name, known := storage.ResolveFieldName(definition, s.vm.Org.Namespace, field)
	if !known {
		return
	}
	denied := s.rootDenied && strings.EqualFold(objectName, s.rootName) ||
		!s.vm.currentUserFieldPermissionWithScope(objectName, name, "isAccessible", "")
	if !denied {
		return
	}
	message := soql.MissingColumnMessage(field, objectName)
	if kind == userModeInvalidField {
		message = fmt.Sprintf("Invalid field: '%s'", field)
	}
	s.add(userModeAccessRef{message: message, object: userModeListedObject(objectName, listed), field: name})
}

func splitUserModePath(parts []string, path string) []string {
	for {
		dot := strings.IndexByte(path, '.')
		if dot < 0 {
			return append(parts, path)
		}
		parts = append(parts, path[:dot])
		path = path[dot+1:]
	}
}
