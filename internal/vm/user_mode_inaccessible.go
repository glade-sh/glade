package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// The captured rows capture a runAs test user's flat,
// identity-scoped SECURITY_ENFORCED denial at 59/62. Other versions, permission
// scopes and query shapes retain their existing diagnostics pending captures.
func (vm *VM) userModeInaccessibleSecureDenial(query soql.Query, permissionSetID string) bool {
	if vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		(vm.currentMethod.APIVersion != "59.0" && vm.currentMethod.APIVersion != "62.0") ||
		!strings.EqualFold(query.SecurityMode, "SECURITY_ENFORCED") {
		return false
	}
	return vm.userModeInaccessibleFlatIDQuery(query)
}

// The captured rows capture the same flat USER_MODE denial
// map at 59/62/67/68. C138_FIELD_*/OBJECT_* bracket named-route answers at
// every API 59-67; the same rule therefore covers 59-68. Anonymous twins reject
// runAs. Documented USER_MODE access details also cover the narrowly admitted
// ordered four-field projection below; its exact native maps remain pending.
// Keep scoped permissions, relationships and other shapes unchanged.
func (vm *VM) userModeInaccessibleDenial(query soql.Query, permissionSetID string) error {
	objectName, fields, objectDenied := vm.userModeInaccessibleDeniedFields(query, permissionSetID)
	if len(fields) == 0 {
		return nil
	}
	message := soql.MissingColumnMessage(fields[0], objectName)
	if objectDenied {
		message = soql.MissingObjectMessage(objectName)
	}
	values := make([]Value, len(fields))
	for i, field := range fields {
		values[i] = String(field)
	}
	set := Set(values...)
	set.Type = "Set<String>"
	inaccessible := Map()
	inaccessible.Type = "Map<String,Set<String>>"
	key := String(objectName)
	inaccessible.Map[mapKey(key)] = set
	inaccessible.MapKeys[mapKey(key)] = key
	exception := Object("QueryException")
	exception.Fields["message"] = String(message)
	exception.Fields["__inaccessibleFields"] = inaccessible
	return &apexThrowError{value: exception}
}

// Selecting the details route must not change execution of readable queries.
// This shared predicate allocates no Apex values and applies the same captured
// permission/shape checks as exception construction.
func (vm *VM) userModeInaccessibleDeniedFields(query soql.Query, permissionSetID string) (string, []string, bool) {
	if vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		!strings.EqualFold(query.SecurityMode, "USER_MODE") {
		return "", nil, false
	}
	projection := vm.userModeInaccessibleProjectionQuery(query)
	expanded := apexversion.AtLeast(vm.currentMethod.APIVersion, 62) && vm.userModeInaccessibleExpandedIDQuery(query)
	if !projection && !expanded && !vm.userModeInaccessibleFlatIDQuery(query) {
		return "", nil, false
	}
	if !(apexversion.Range{Since: 59, Until: 69}).Allows(vm.currentMethod.APIVersion) {
		return "", nil, false
	}
	if projection && apexversion.Before(vm.currentMethod.APIVersion, 62) {
		return "", nil, false // Preserve the older captured flat-query boundary.
	}
	objectName, _ := vm.resolveObjectName(query.Object) // Validated by the shape guards.
	objectDenied := !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", permissionSetID)
	if objectDenied && !projection && !expanded && len(query.Fields) != 2 {
		return "", nil, false // The object-denial capture projects one string field.
	}
	definition := vm.Org.Objects[objectName].Definition
	var fields []string
	selected := query.Fields[1:]
	if projection || expanded {
		selected = query.Fields
	}
	for _, field := range selected {
		name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
		if !ok {
			return "", nil, false
		}
		if (projection || expanded) && strings.EqualFold(name, "Id") {
			continue // Identity fields retain their existing readable boundary.
		}
		if objectDenied || !vm.currentUserFieldPermissionWithScope(objectName, name, "isAccessible", permissionSetID) {
			fields = append(fields, name)
		} else if !projection && !expanded {
			return "", nil, false // Mixed readable/denied projections have no native row.
		}
	}
	return objectName, fields, objectDenied
}

// USER_MODE reports the full set of access errors, rather than only the first
// denied projection (Salesforce Secure Coding Guide, CRUD/FLS protection).
// Admit only four flat fields, one scalar identity membership condition, and
// one ascending text-field ordering. Field types and permissions come from
// schema/permission metadata, never from application or field names.
// Exact native maps, order-clause denials and API63 helper twins are pending in
// apextest/testdata/conformance/user_mode_projection_pending.json.
func (vm *VM) userModeInaccessibleProjectionQuery(query soql.Query) bool {
	if vm.Org == nil || len(query.Fields) != 4 || len(query.Order) != 1 || query.Order[0].Desc ||
		(query.Order[0].Nulls != "" && !strings.EqualFold(query.Order[0].Nulls, "FIRST")) ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 ||
		query.Having != nil || query.HasLimit || query.HasOffset || query.ForUpdate || query.ForView ||
		query.ForReference || query.AllRows || query.UsingScope != "" || query.Count {
		return false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || !strings.EqualFold(where.Op, "IN") ||
		where.Not || len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	identity, text, number, date := 0, 0, 0, 0
	orderedText := false
	seen := make(map[string]bool, len(query.Fields))
	for _, field := range query.Fields {
		if strings.ContainsAny(field, ". ()\t\r\n") {
			return false
		}
		name, known := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
		if !known || seen[name] {
			return false
		}
		seen[name] = true
		if strings.EqualFold(name, "Id") {
			identity++
			continue
		}
		switch definition.Fields[name].Type {
		case storage.FieldString:
			text++
			orderedText = orderedText || strings.EqualFold(query.Order[0].Field, name)
		case storage.FieldDecimal:
			number++
		case storage.FieldDate:
			date++
		default:
			return false
		}
	}
	return identity == 1 && number == 1 && orderedText && (text == 2 && date == 0 || text == 1 && date == 1)
}

// USER_MODE exposes all FLS errors through getInaccessibleFields. Keep the
// additional route limited to flat Id-IN queries with two to four scalar
// projections and at most one selected sort field. Relationships, aggregates,
// other predicates and scoped permissions retain their existing routes.
func (vm *VM) userModeInaccessibleExpandedIDQuery(query soql.Query) bool {
	if vm.Org == nil || len(query.Fields) < 2 || len(query.Fields) > 4 ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 ||
		query.Having != nil || len(query.Order) > 1 || query.HasLimit || query.HasOffset ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" || query.Count {
		return false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || !strings.EqualFold(where.Op, "IN") ||
		where.Not || len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	hasID := false
	sortSelected := query.OrderBy == ""
	for _, field := range query.Fields {
		if strings.ContainsAny(field, ". ()\t\r\n") {
			return false
		}
		name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
		if !ok {
			return false
		}
		if strings.EqualFold(name, "Id") {
			hasID = true
		} else {
			switch definition.Fields[name].Type {
			case storage.FieldString, storage.FieldPicklist, storage.FieldDate:
			default:
				return false
			}
		}
		if strings.EqualFold(name, query.OrderBy) {
			sortSelected = true
		}
		if len(query.Order) == 1 && !strings.EqualFold(query.Order[0].Field, query.OrderBy) {
			return false
		}
	}
	return hasID && sortSelected
}

func (vm *VM) userModeInaccessibleFlatIDQuery(query soql.Query) bool {
	if vm.Org == nil || len(query.Fields) < 2 || len(query.Fields) > 3 || !strings.EqualFold(query.Fields[0], "Id") ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 ||
		query.Having != nil || len(query.Order) != 0 || query.OrderBy != "" || query.HasLimit || query.HasOffset ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" || query.Count {
		return false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || where.Op != "=" ||
		where.Not || len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	for _, field := range query.Fields[1:] {
		if strings.ContainsAny(field, ". ()\t\r\n") {
			return false
		}
		name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
		if !ok || definition.Fields[name].Type != storage.FieldString {
			return false
		}
	}
	return true
}

// The captured rows reject the single-argument Database.query SYSTEM_MODE
// suffix on the anonymous route at every captured API. Named tests 034/040
// succeed. Do not extend this boundary to queryWithBinds, handles or other APIs.
func (vm *VM) userModeInaccessibleAnonymousSystemQuery(raw string) error {
	if vm.testContext != nil || vm.currentClass != "" || vm.currentMethod.ClassName != "" || vm.currentTrigger {
		return nil
	}
	switch vm.currentMethod.APIVersion {
	case "59.0", "62.0", "67.0", "68.0":
	default:
		return nil
	}
	query, err := vm.parseSOQLAt(raw)
	if err != nil || !strings.EqualFold(query.SecurityMode, "SYSTEM_MODE") || !(vm.userModeInaccessibleFlatIDQuery(query) || vm.anonymousSystemProjectionDenied(query)) {
		return nil
	}
	return newExceptionError("SecurityException", "Cannot use SYSTEM_MODE access level in anonymous execution of Apex.")
}
