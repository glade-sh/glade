package vm

import (
	"fmt"
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

// api62DynamicSecurePermissionDenial covers the dynamic projections captured
// for an inaccessible ReadWrite custom object at source API 62. It does not
// change static SOQL, other APIs, field-only denials, or other projections.
func (vm *VM) api62DynamicSecurePermissionDenial(query soql.Query, permissionSetID string) bool {
	if vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		vm.currentMethod.APIVersion != "62.0" || vm.dynamicSOQLBindRoute != "Database.query" ||
		!strings.EqualFold(query.SecurityMode, "SECURITY_ENFORCED") ||
		query.Where != nil || query.Having != nil || len(query.Order) != 0 || query.OrderBy != "" ||
		query.HasLimit || query.HasOffset || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 ||
		len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Count || query.ForUpdate ||
		query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return false
	}
	user := vm.executionUser
	if vm.testContext.CurrentUser.Kind != "" {
		user = vm.testContext.CurrentUser
	}
	if !strings.EqualFold(vm.currentUserProfileName(user), "Standard User") || vm.Org == nil ||
		!isCustomSchemaName(query.Object) {
		return false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	if !vm.ordinaryCustomPermissionObject(objectName) || !strings.EqualFold(definition.SharingModel, "ReadWrite") {
		return false
	}
	fields := make([]string, len(query.Fields))
	var customText string
	for i, field := range query.Fields {
		fields[i] = strings.TrimSpace(field)
		if fields[i] == "" || strings.ContainsAny(fields[i], ". ()\t\r\n") {
			return false
		}
		canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fields[i])
		if !ok {
			return false
		}
		fieldDefinition := definition.Fields[canonical]
		if isCustomSchemaName(canonical) && fieldDefinition.Type == storage.FieldString && strings.EqualFold(fieldDefinition.DisplayType, "STRING") {
			if customText != "" {
				return false
			}
			customText = canonical
		} else if !strings.EqualFold(canonical, "Id") && !strings.EqualFold(canonical, "Name") {
			return false
		}
	}
	switch {
	case len(fields) == 1 && strings.EqualFold(fields[0], "Id"):
		return true
	case len(fields) == 1 && customText != "":
		return true
	case len(fields) == 2 && strings.EqualFold(fields[0], "Name") && customText != "" && strings.EqualFold(fields[1], customText):
		return true
	default:
		return false
	}
}

// The captured rows capture the same flat USER_MODE denial
// map at 59/62/67/68. C138_FIELD_*/OBJECT_* bracket named-route answers at
// every API 59-67; the same rule therefore covers 59-68. Anonymous twins reject
// runAs. Documented USER_MODE access details also cover the narrowly admitted
// ordered four-field projection below; its exact native maps remain pending.
// At API 62 to 68 the user_mode_inaccessible_shapes rows replace these shape
// gates with one rule (user_mode_access_scan.go). Scoped permissions and
// API 59 to 61 keep the shape-gated checks.
func (vm *VM) userModeInaccessibleDenial(query soql.Query, permissionSetID string) error {
	if (apexversion.Range{Since: 62, Until: 69}).Allows(vm.currentMethod.APIVersion) {
		// Native rows for 104 query shapes (user_mode_inaccessible_shapes) show
		// one general rule at API 62 to 68, applied by userModeAccessScanError
		// in enforceSOQLSecurity. The shape-gated checks below remain for the
		// older APIs and for the child-to-parent lookup row, which the scan
		// does not model.
		if vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" || !strings.EqualFold(query.SecurityMode, "USER_MODE") {
			return nil
		}
		if objectName, field, relationship, denied := vm.userModeInaccessibleChildLookup(query, permissionSetID); denied {
			return userModeFieldExceptionMessage(objectName, []string{field}, soql.MissingRelationshipMessage(relationship, "FROM part of query call"))
		}
		return nil
	}
	if err := vm.userModeInaccessiblePredicateDenial(query, permissionSetID); err != nil {
		return err
	}
	if objectName, field, denied := vm.userModeInaccessibleAggregateField(query, permissionSetID); denied {
		return userModeFieldException(objectName, []string{field}, false)
	}
	if objectName, field, denied := vm.userModeInaccessibleSumField(query, permissionSetID); denied {
		return userModeFieldExceptionMessage(objectName, []string{field}, fmt.Sprintf("Invalid field: '%s'", field))
	}
	if objectName, field, denied := vm.userModeInaccessibleTypeofField(query, permissionSetID); denied {
		return userModeFieldException(objectName, []string{field}, false)
	}
	if objectName, field, relationship, denied := vm.userModeInaccessibleChildLookup(query, permissionSetID); denied {
		return userModeFieldExceptionMessage(objectName, []string{field}, soql.MissingRelationshipMessage(relationship, "FROM part of query call"))
	}
	objectName, fields, objectDenied := vm.userModeInaccessibleDeniedFields(query, permissionSetID)
	if len(fields) == 0 {
		return nil
	}
	return userModeFieldException(objectName, fields, objectDenied)
}

// A captured USER_MODE child query rejects an inaccessible child-to-parent
// lookup, even when the lookup was omitted from the child SELECT list.
func (vm *VM) userModeInaccessibleChildLookup(query soql.Query, permissionSetID string) (string, string, string, bool) {
	if vm.Org == nil || vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		!(apexversion.Range{Since: 62, Until: 68}).Allows(vm.currentMethod.APIVersion) || !strings.EqualFold(query.SecurityMode, "USER_MODE") ||
		len(query.Fields) != 1 || !strings.EqualFold(query.Fields[0], "Id") || len(query.ChildQueries) != 1 || len(query.Typeofs) != 0 ||
		len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 || query.HasLimit || query.HasOffset ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return "", "", "", false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || where.Op != "=" || where.Not || where.Range || where.Subquery != nil || len(where.And) != 0 || len(where.Or) != 0 {
		return "", "", "", false
	}
	child := query.ChildQueries[0]
	if len(child.Query.Fields) != 2 || !strings.EqualFold(child.Query.Fields[0], "Id") || !strings.EqualFold(child.Query.Fields[1], "Name") ||
		child.Query.Where != nil || child.Query.HasLimit || child.Query.HasOffset || len(child.Query.ChildQueries) != 0 || len(child.Query.Typeofs) != 0 {
		return "", "", "", false
	}
	parent, ok := vm.resolveObjectName(query.Object)
	if !ok || !vm.currentUserObjectPermissionWithScope(parent, "isAccessible", "") {
		return "", "", "", false
	}
	childName := vm.childRelationshipLookup(parent, child.Relationship).ChildType
	state, ok := vm.Org.Objects[childName]
	if !ok || !vm.currentUserObjectPermissionWithScope(childName, "isAccessible", "") {
		return "", "", "", false
	}
	for _, relation := range state.Definition.Relations {
		if relation.Field != "" && relation.ChildRelationship != "" && relationshipTargetsObject(relation, parent) &&
			vmRelationshipNameMatches(vm.Org.Namespace, relation.ChildRelationship, child.Relationship) &&
			!vm.currentUserFieldPermissionWithScope(childName, relation.Field, "isAccessible", "") {
			return childName, relation.Field, child.Relationship, true
		}
	}
	return "", "", "", false
}

// Captured Task.What custom-object TYPEOF queries reject an inaccessible WHEN
// projection with the target object and field in the exception details.
func (vm *VM) userModeInaccessibleTypeofField(query soql.Query, permissionSetID string) (string, string, bool) {
	if vm.Org == nil || vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		!(apexversion.Range{Since: 62, Until: 69}).Allows(vm.currentMethod.APIVersion) || !strings.EqualFold(query.SecurityMode, "USER_MODE") ||
		!strings.EqualFold(query.Object, "Task") || len(query.Fields) != 1 || !strings.EqualFold(query.Fields[0], "Id") ||
		len(query.Typeofs) != 1 || len(query.ChildQueries) != 0 || len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 ||
		query.HasLimit || query.HasOffset || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return "", "", false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || where.Op != "=" || where.Not || where.Range || where.Subquery != nil || len(where.And) != 0 || len(where.Or) != 0 {
		return "", "", false
	}
	spec := query.Typeofs[0]
	if !strings.EqualFold(spec.Relationship, "What") || len(spec.When) != 1 || len(spec.Else) != 0 {
		return "", "", false
	}
	for name, fields := range spec.When {
		objectName, ok := vm.resolveObjectName(name)
		if !ok || len(fields) != 1 || !strings.HasSuffix(strings.ToLower(objectName), "__c") ||
			!strings.EqualFold(vm.Org.Objects[objectName].Definition.Metadata["enableActivities"], "true") ||
			!vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", "") {
			return "", "", false
		}
		definition := vm.Org.Objects[objectName].Definition
		field, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fields[0])
		if ok && !vm.currentUserFieldPermissionWithScope(objectName, field, "isAccessible", "") {
			return objectName, field, true
		}
	}
	return "", "", false
}

func userModeFieldException(objectName string, fields []string, objectDenied bool) error {
	message := soql.MissingColumnMessage(fields[0], objectName)
	if objectDenied {
		message = soql.MissingObjectMessage(objectName)
	}
	return userModeFieldExceptionMessage(objectName, fields, message)
}

func userModeFieldExceptionMessage(objectName string, fields []string, message string) error {
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

// The C11 capture records an inaccessible numeric SUM over an Id-IN scope,
// together with readable and SYSTEM_MODE controls at both supported endpoints.
func (vm *VM) userModeInaccessibleSumField(query soql.Query, permissionSetID string) (string, string, bool) {
	if vm.Org == nil || vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		!(apexversion.Range{Since: 62, Until: 68}).Allows(vm.currentMethod.APIVersion) || !strings.EqualFold(query.SecurityMode, "USER_MODE") ||
		len(query.Aggregates) != 1 || !strings.EqualFold(query.Aggregates[0].Func, "SUM") || len(query.GroupBy) != 0 || query.Having != nil ||
		len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.Order) != 0 || query.HasLimit || query.HasOffset ||
		query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return "", "", false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || !strings.EqualFold(where.Op, "IN") || where.Not || where.Range || where.Subquery != nil || len(where.And) != 0 || len(where.Or) != 0 {
		return "", "", false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok || !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", "") {
		return "", "", false
	}
	definition := vm.Org.Objects[objectName].Definition
	field, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, query.Aggregates[0].Field)
	if !ok || definition.Fields[field].Type != storage.FieldDecimal || vm.currentUserFieldPermissionWithScope(objectName, field, "isAccessible", "") {
		return "", "", false
	}
	return objectName, field, true
}

// Q14 captures denied grouping and HAVING fields on a grouped COUNT(Id)
// with an identity membership filter, together with readable and system controls.
func (vm *VM) userModeInaccessibleAggregateField(query soql.Query, permissionSetID string) (string, string, bool) {
	if vm.Org == nil || vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		!(apexversion.Range{Since: 62, Until: 68}).Allows(vm.currentMethod.APIVersion) || !strings.EqualFold(query.SecurityMode, "USER_MODE") ||
		len(query.GroupBy) != 1 || len(query.Aggregates) != 1 || !strings.EqualFold(query.Aggregates[0].Func, "COUNT") ||
		!strings.EqualFold(query.Aggregates[0].Field, "Id") || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 ||
		len(query.Order) != 0 || query.HasLimit || query.HasOffset || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return "", "", false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || !strings.EqualFold(where.Op, "IN") || where.Not || where.Range || where.Subquery != nil || len(where.And) != 0 || len(where.Or) != 0 {
		return "", "", false
	}
	if (query.Having == nil && len(query.HavingAggregates) != 0) || (query.Having != nil && (len(query.HavingAggregates) != 1 || !strings.EqualFold(query.HavingAggregates[0].Func, "SUM"))) {
		return "", "", false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok || !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", "") {
		return "", "", false
	}
	definition := vm.Org.Objects[objectName].Definition
	candidates := []string{query.GroupBy[0]}
	if query.Having != nil {
		candidates = append(candidates, query.HavingAggregates[0].Field)
	}
	for _, candidate := range candidates {
		field, exists := storage.ResolveFieldName(definition, vm.Org.Namespace, candidate)
		if !exists {
			return "", "", false
		}
		if !vm.currentUserFieldPermissionWithScope(objectName, field, "isAccessible", "") {
			return objectName, field, true
		}
	}
	return "", "", false
}

// C11 Q4 captures an inaccessible text equality in an identity-scoped
// predicate, including the same predicate inside a semi- or anti-join.
func (vm *VM) userModeInaccessiblePredicateDenial(query soql.Query, permissionSetID string) error {
	if vm.Org == nil || vm.testContext == nil || vm.testContext.RunAsDepth == 0 || permissionSetID != "" ||
		!(apexversion.Range{Since: 62, Until: 68}).Allows(vm.currentMethod.APIVersion) || !strings.EqualFold(query.SecurityMode, "USER_MODE") ||
		len(query.Fields) != 1 || !strings.EqualFold(query.Fields[0], "Id") || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 ||
		len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 ||
		query.HasLimit || query.HasOffset || query.Count || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return nil
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok || !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", "") {
		return nil
	}
	other, ok := userModeIdentityAndOther(query.Where)
	if !ok {
		return nil
	}
	if other.Subquery != nil {
		inner := other.Subquery
		if !strings.EqualFold(other.Field, "Id") || (other.Op != "IN" && other.Op != "NOT IN") || other.Not ||
			len(inner.Fields) != 1 || inner.Where == nil || len(inner.ChildQueries) != 0 || len(inner.Typeofs) != 0 ||
			len(inner.Aggregates) != 0 || len(inner.GroupBy) != 0 || inner.Having != nil || len(inner.Order) != 0 ||
			inner.HasLimit || inner.HasOffset || inner.Count || inner.ForUpdate || inner.ForView || inner.ForReference || inner.AllRows || inner.UsingScope != "" {
			return nil
		}
		innerName, exists := vm.resolveObjectName(inner.Object)
		if !exists || !vm.currentUserObjectPermissionWithScope(innerName, "isAccessible", "") {
			return nil
		}
		definition := vm.Org.Objects[innerName].Definition
		joinField, exists := storage.ResolveFieldName(definition, vm.Org.Namespace, inner.Fields[0])
		if !exists || definition.Fields[joinField].Type != storage.FieldReference || !vm.currentUserFieldPermissionWithScope(innerName, joinField, "isAccessible", "") {
			return nil
		}
		condition, valid := userModeIdentityAndOther(inner.Where)
		if !valid {
			return nil
		}
		field, denied := vm.userModeDeniedTextEquality(innerName, condition)
		if denied {
			return newExceptionError("QueryException", soql.MissingColumnMessage(field, innerName))
		}
		return nil
	}
	field, denied := vm.userModeDeniedTextEquality(objectName, other)
	if !denied {
		return nil
	}
	set := Set(String(field))
	set.Type = "Set<String>"
	inaccessible := Map()
	inaccessible.Type = "Map<String,Set<String>>"
	key := String(objectName)
	inaccessible.Map[mapKey(key)] = set
	inaccessible.MapKeys[mapKey(key)] = key
	exception := Object("QueryException")
	exception.Fields["message"] = String(soql.MissingColumnMessage(field, objectName))
	exception.Fields["__inaccessibleFields"] = inaccessible
	return &apexThrowError{value: exception}
}

func userModeIdentityAndOther(condition *soql.Condition) (*soql.Condition, bool) {
	if condition == nil || condition.Not || len(condition.Or) != 0 || len(condition.And) != 2 {
		return nil, false
	}
	for i := range condition.And {
		c := &condition.And[i]
		if strings.EqualFold(c.Field, "Id") && c.Op == "=" && !c.Not && c.Subquery == nil && len(c.And) == 0 && len(c.Or) == 0 && !c.Range {
			return &condition.And[1-i], true
		}
	}
	return nil, false
}

func (vm *VM) userModeDeniedTextEquality(objectName string, condition *soql.Condition) (string, bool) {
	if condition == nil || condition.Op != "=" || condition.Not || condition.Range || condition.Subquery != nil || len(condition.And) != 0 || len(condition.Or) != 0 {
		return "", false
	}
	definition := vm.Org.Objects[objectName].Definition
	field, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, condition.Field)
	if !ok || definition.Fields[field].Type != storage.FieldString {
		return "", false
	}
	return field, !vm.currentUserFieldPermissionWithScope(objectName, field, "isAccessible", "")
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
	single := (apexversion.Range{Since: 62, Until: 68}).Allows(vm.currentMethod.APIVersion) && vm.userModeInaccessibleSingleFieldQuery(query, permissionSetID)
	if !projection && !expanded && !single && !vm.userModeInaccessibleFlatIDQuery(query) {
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
	if projection || expanded || single {
		selected = query.Fields
	}
	for _, field := range selected {
		name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
		if !ok {
			return "", nil, false
		}
		if (projection || expanded || single) && strings.EqualFold(name, "Id") {
			continue // Identity fields retain their existing readable boundary.
		}
		if objectDenied || !vm.currentUserFieldPermissionWithScope(objectName, name, "isAccessible", permissionSetID) {
			fields = append(fields, name)
		} else if !projection && !expanded && !single {
			return "", nil, false // Mixed readable/denied projections have no native row.
		}
	}
	return objectName, fields, objectDenied
}

// C11 Q5 and corrected Q14 capture direct single-field denials for a
// readable object with an identity equality or membership predicate.
func (vm *VM) userModeInaccessibleSingleFieldQuery(query soql.Query, permissionSetID string) bool {
	if vm.Org == nil || len(query.Fields) != 1 || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 ||
		len(query.Aggregates) != 0 || len(query.GroupBy) != 0 || query.Having != nil || len(query.Order) != 0 ||
		query.HasLimit || query.HasOffset || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" || query.Count {
		return false
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || (where.Op != "=" && !strings.EqualFold(where.Op, "IN")) ||
		where.Not || len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok || !vm.currentUserObjectPermissionWithScope(objectName, "isAccessible", permissionSetID) {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	name, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, query.Fields[0])
	if !ok {
		return false
	}
	return definition.Fields[name].Type == storage.FieldString || definition.Fields[name].Type == storage.FieldDecimal
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
