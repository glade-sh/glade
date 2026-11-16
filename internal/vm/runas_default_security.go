package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) runAsSecureDefaults() bool {
	return !vm.currentTrigger && vm.runAsSecureDefaultsAnyContext()
}

// runAsSecureDefaultsAnyContext is runAsSecureDefaults without the trigger
// exclusion. Native api67leads rows run the same unqualified statements inside
// a trigger body under runAs at API 67 and 68 and report the class-route
// outcomes for a non-owner text update and for a delete without the grant.
func (vm *VM) runAsSecureDefaultsAnyContext() bool {
	return vm.testContext != nil && vm.testContext.RunAsDepth > vm.testContext.PackageRunAsDepth &&
		apexversion.AtLeast(vm.currentMethod.APIVersion, 67)
}

// Owned sharing_update_observations rows report DmlException, rather than the
// explicit USER_MODE preflight exception, for a non-owner scalar text update
// on a Private custom object from source API 67. CRUD/FLS checks precede this
// route choice. Other payloads and access modes retain their existing paths.
func (vm *VM) defaultRunAsPrivateTextUpdate(op string, value Value) bool {
	if op != "update" || value.Kind != ValueObject || !vm.runAsSecureDefaultsAnyContext() || vm.Org == nil {
		return false
	}
	objectName, ok := vm.resolveObjectName(value.Type)
	if !ok || !hasSuffixFold(objectName, "__c") {
		return false
	}
	definition := vm.Org.Objects[objectName].Definition
	if !strings.EqualFold(strings.TrimSpace(definition.SharingModel), "Private") {
		return false
	}
	_, id, present := objectFieldValue(value, "Id")
	text, valid := idValueText(id)
	if !present || !valid {
		return false
	}
	stored, found := vm.findOrgRecord(objectName, storage.ID(text))
	if !found || stored.System.OwnerID == "" || vm.currentUserCanWriteRecord(objectName, stored, vm.currentUserID(), op) {
		return false
	}
	// Custom OwnerId metadata can also name Group, but these observations
	// retain a User owner. Do not extend the exception rule to queue owners.
	ownerType, found := vm.sObjectNameForID(string(stored.System.OwnerID))
	if !found || !strings.EqualFold(ownerType, "User") {
		return false
	}
	hasText := false
	for name, fieldValue := range value.Fields {
		if isInternalSObjectField(name) || strings.EqualFold(name, "Id") {
			continue
		}
		// Insert callers carry local bookkeeping system fields that are not
		// user writes. Keep explicit system-field and queried payloads out.
		if dmlAccessibleSObject(value) && isSObjectSystemField(name) && !strings.EqualFold(name, "OwnerId") && !isExplicitSObjectField(value, name) && !isUserSetSObjectField(value, name) {
			continue
		}
		field, exists := definition.Fields[vm.resolveSObjectFieldName(objectName, name)]
		if !exists {
			return false
		}
		if strings.EqualFold(field.APIName, "OwnerId") && field.Type == storage.FieldReference && strings.EqualFold(field.RelationshipName, "Owner") {
			ownerText, ownerValid := idValueText(fieldValue)
			if !ownerValid || !storage.IDsEqual(storage.ID(ownerText), stored.System.OwnerID) {
				return false
			}
			userTarget := false
			for _, target := range field.ReferenceTo {
				userTarget = userTarget || strings.EqualFold(target, "User")
			}
			if userTarget {
				continue
			}
		}
		if field.Type != storage.FieldString || fieldValue.Kind != ValueString {
			return false
		}
		hasText = true
	}
	if !hasText {
		return false
	}
	return true
}

// C137 DEFAULT denials distinguish unreadable objects from readable objects
// without write permission. Explicit modes keep their own enforcement path.
func (vm *VM) defaultRunAsDMLDenial(op string, value Value) error {
	if !vm.runAsSecureDefaults() && !(op == "delete" && vm.runAsSecureDefaultsAnyContext()) {
		return nil
	}
	switch op {
	case "insert", "update", "delete", "upsert":
	default:
		return nil
	}
	permission, _ := userModeDMLPermissions(op)
	for _, record := range dmlAccessRecords(value) {
		if record.Kind != ValueObject {
			continue
		}
		objectName := record.Type
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			objectName = canonical
		}
		if !vm.currentUserObjectPermission(objectName, "isAccessible") {
			return newExceptionError("SecurityException", fmt.Sprintf("Access to entity '%s' denied", objectName))
		}
		if vm.currentUserObjectPermission(objectName, permission) {
			continue
		}
		// Native api67leads rows: a readable object that lacks only the delete
		// grant reports the same TypeException as the no-grant capture.
		if op != "delete" && (vm.currentUserObjectPermission(objectName, "isCreateable") ||
			vm.currentUserObjectPermission(objectName, "isUpdateable") ||
			vm.currentUserObjectPermission(objectName, "isDeletable")) {
			continue // The readable denial capture has no create/edit/delete grants.
		}
		return readonlyObjectDMLException(op, objectName)
	}
	return nil
}

// readonlyObjectDMLException is the statement failure for a readable object
// without the operation's grant: upsert reports the field-access DmlException,
// the other operations a TypeException.
func readonlyObjectDMLException(op, objectName string) error {
	if op == "upsert" {
		message := fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result! ", objectName)
		exception := Object("DmlException")
		exception.Fields["message"] = String(message)
		exception.Fields["__dmlErrors"] = dmlExceptionErrorDetails([]dml.Result{{Error: message, StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}}, []string{objectName})
		return &apexThrowError{value: exception}
	}
	return newExceptionError("TypeException", fmt.Sprintf("DML operation %s not allowed on %s", strings.ToUpper(op), objectName))
}

// explicitRunAsUserMode reports a runAs block at a source API with explicit
// USER_MODE DML captures (C137 rows, APIs 59.0 through 68.0).
func (vm *VM) explicitRunAsUserMode() bool {
	return vm.testContext != nil && vm.testContext.RunAsDepth > vm.testContext.PackageRunAsDepth && vm.Org != nil &&
		apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 59)
}

// explicitRunAsMissingGrant resolves a record's object when the running user
// can read it but lacks the grant the operation needs: create for insert, edit
// for update, delete for delete, and both create and edit for upsert.
func (vm *VM) explicitRunAsMissingGrant(op string, record Value) (string, bool) {
	if record.Kind != ValueObject {
		return "", false
	}
	objectName, ok := vm.resolveObjectName(record.Type)
	if !ok || !vm.currentUserObjectPermission(objectName, "isAccessible") {
		return "", false
	}
	switch op {
	case "insert":
		return objectName, !vm.currentUserObjectPermission(objectName, "isCreateable")
	case "update":
		return objectName, !vm.currentUserObjectPermission(objectName, "isUpdateable")
	case "delete":
		return objectName, !vm.currentUserObjectPermission(objectName, "isDeletable")
	case "upsert":
		return objectName, !vm.currentUserObjectPermission(objectName, "isCreateable") && !vm.currentUserObjectPermission(objectName, "isUpdateable")
	case "undelete":
		return objectName, !vm.currentUserObjectPermission(objectName, "isDeletable")
	}
	return "", false
}

// undeleteGrantFailure is the row failure for an undelete without the delete
// grant (native sharing_access_follow f_undelete_obj rows, APIs 62.0-68.0).
func undeleteGrantFailure(record Value) dml.Result {
	_, id, _ := objectFieldValue(record, "Id")
	text, _ := idValueText(id)
	return dml.Result{ID: storage.ID(text), Error: "Entity type is not undeletable", StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}
}

// Native C137_353-359 and C137_374-380 (readonly grant, APIs 59.0-68.0) and
// sharing_access_modes d1 rows (readonly, create-only and edit-only grants,
// APIs 62.0-68.0), all in runAs: explicit USER_MODE DML on a readable object
// without the operation's grant raises TypeException "DML operation <OP> not
// allowed on <object>"; upsert raises the field-access DmlException naming the
// record's written fields. Unreadable objects keep the SecurityException
// (C137_234).
func (vm *VM) explicitRunAsObjectDMLDenial(op string, value Value) error {
	switch op {
	case "insert", "update", "delete", "upsert", "undelete":
	default:
		return nil
	}
	if !vm.explicitRunAsUserMode() || op == "undelete" && !apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 62) {
		return nil
	}
	records := dmlAccessRecords(value)
	if len(records) == 0 {
		return nil
	}
	objectName := ""
	for _, record := range records {
		name, denied := vm.explicitRunAsMissingGrant(op, record)
		if !denied || objectName != "" && name != objectName {
			return nil
		}
		objectName = name
	}
	if op == "undelete" {
		results := make([]dml.Result, len(records))
		for i, record := range records {
			results[i] = undeleteGrantFailure(record)
		}
		return statementDMLException(op, results, []string{objectName})
	}
	if op == "upsert" {
		var fields []string
		for _, field := range explicitSObjectFieldNamesInInsertionOrder(records[0]) {
			if !strings.EqualFold(field, "Id") {
				fields = append(fields, vm.resolveSObjectFieldName(objectName, field))
			}
		}
		return userModeFieldDMLException(op, objectName, records[0], fields, 0)
	}
	return readonlyObjectDMLException(op, objectName)
}

// explicitRunAsPartialFailures is the allOrNone=false counterpart in runAs at
// APIs 59.0-68.0: each row fails without writes. Insert, update and new-record
// upsert rows carry CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY and the TypeException
// text (an upsert of a new record names INSERT); delete rows carry
// DELETE_FAILED. An unreadable object fails insert rows with the denied-entity
// text (C137_198/240, d1_10). Mixed lists keep the old path.
func (vm *VM) explicitRunAsPartialFailures(op string, value Value) ([]dml.Result, bool) {
	if !vm.explicitRunAsUserMode() {
		return nil, false
	}
	records := dmlAccessRecords(value)
	if len(records) == 0 {
		return nil, false
	}
	results := make([]dml.Result, len(records))
	objectName := ""
	for i, record := range records {
		if record.Kind != ValueObject {
			return nil, false
		}
		canonical, ok := vm.resolveObjectName(record.Type)
		if !ok || (objectName != "" && objectName != canonical) {
			return nil, false
		}
		objectName = canonical
		_, id, hasID := objectFieldValue(record, "Id")
		if op == "upsert" && hasID && id.Kind != ValueNull {
			return nil, false
		}
		if !vm.currentUserObjectPermission(canonical, "isAccessible") {
			if op != "insert" {
				return nil, false
			}
			results[i] = dml.Result{Error: fmt.Sprintf("Access to entity '%s' denied", canonical), StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}
			continue
		}
		if _, denied := vm.explicitRunAsMissingGrant(op, record); !denied {
			return nil, false
		}
		switch op {
		case "insert", "upsert":
			results[i] = dml.Result{Error: fmt.Sprintf("DML operation INSERT not allowed on %s", canonical), StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}
		case "update":
			results[i] = dml.Result{Error: fmt.Sprintf("DML operation UPDATE not allowed on %s", canonical), StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}
		case "delete":
			text, _ := idValueText(id)
			results[i] = dml.Result{ID: storage.ID(text), Error: fmt.Sprintf("DML operation DELETE not allowed on %s", canonical), StatusCode: "DELETE_FAILED"}
		case "undelete":
			if !apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 62) {
				return nil, false
			}
			results[i] = undeleteGrantFailure(record)
		default:
			return nil, false
		}
	}
	return results, true
}

// C137_184/226 and C137_352/373 capture allOrNone=false insert on homogeneous
// unreadable or readonly lists: each row is a failed SaveResult without writes.
// Mixed permission lists and partially writable objects retain their old path.
func (vm *VM) defaultRunAsInsertFailures(value Value) ([]dml.Result, bool) {
	if !vm.runAsSecureDefaults() || value.Kind != ValueList || len(value.List) == 0 {
		return nil, false
	}
	results := make([]dml.Result, len(value.List))
	objectName := ""
	for i, record := range value.List {
		if record.Kind != ValueObject {
			return nil, false
		}
		canonical, ok := vm.resolveObjectName(record.Type)
		if !ok || (objectName != "" && objectName != canonical) {
			return nil, false
		}
		objectName = canonical
		message := fmt.Sprintf("Access to entity '%s' denied", canonical)
		if vm.currentUserObjectPermission(canonical, "isAccessible") {
			if vm.currentUserObjectPermission(canonical, "isCreateable") ||
				vm.currentUserObjectPermission(canonical, "isUpdateable") ||
				vm.currentUserObjectPermission(canonical, "isDeletable") {
				return nil, false
			}
			message = fmt.Sprintf("DML operation INSERT not allowed on %s", canonical)
		}
		results[i] = dml.Result{Error: message, StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}
	}
	return results, true
}

// C137_185-191/227-233 and the captured rows capture ordinary flat queries
// scoped by Id. The default_permissions_capture rows also cover unfiltered
// custom-object Database.countQuery calls. COUNT denies entity access;
// projections report a missing object.
func (vm *VM) defaultRunAsSOQLDenial(query soql.Query) error {
	if !vm.runAsSecureDefaults() || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 ||
		(!query.Count && len(query.Aggregates) != 0) || len(query.GroupBy) != 0 || query.Having != nil ||
		len(query.Order) != 0 || query.HasLimit || query.HasOffset || query.ForUpdate ||
		query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return nil
	}
	unfilteredCustomCount := vm.dynamicSOQLBindRoute == "Database.countQuery" &&
		query.Count && query.Where == nil && vm.ordinaryCustomPermissionObject(query.Object)
	// no_account_read_profile_user (API 67.0): a static SELECT COUNT() FROM
	// Account by a profile without Account access raises the entity denial.
	unfilteredCustomCount = unfilteredCustomCount || query.Count && query.Where == nil && strings.EqualFold(query.Object, "Account")
	// sharing_access_modes d6 rows (API 67.0 and 68.0): static COUNT() and the
	// default Database.countQuery, unfiltered or with one Name equality, on an
	// unreadable custom object or Account raise the same entity denial.
	unfilteredCustomCount = unfilteredCustomCount || (vm.dynamicSOQLBindRoute == "" || vm.dynamicSOQLBindRoute == "Database.countQuery") &&
		runAsCountDenialShape(query) && (query.Where == nil || strings.EqualFold(query.Where.Field, "Name")) &&
		(vm.ordinaryCustomPermissionObject(query.Object) || strings.EqualFold(query.Object, "Account"))
	if !unfilteredCustomCount {
		where := query.Where
		if where == nil || !strings.EqualFold(where.Field, "Id") || where.Op != "=" || where.Not ||
			len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
			return nil
		}
	}
	for _, field := range query.Fields {
		if !query.Count && strings.ContainsAny(field, ". ()\t\r\n") {
			return nil
		}
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok || vm.currentUserObjectPermission(objectName, "isAccessible") {
		return nil
	}
	if !query.Count {
		if len(query.Fields) < 1 || len(query.Fields) > 3 || !strings.EqualFold(query.Fields[0], "Id") {
			return nil
		}
		definition := vm.Org.Objects[objectName].Definition
		for _, field := range query.Fields[1:] {
			canonical, found := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
			if !found || definition.Fields[canonical].Type != storage.FieldString {
				return nil
			}
		}
	}
	if query.Count {
		return newExceptionError("SecurityException", fmt.Sprintf("Access to entity '%s' denied", objectName))
	}
	return newExceptionError("QueryException", soql.MissingObjectMessage(objectName))
}

// Native api67leads rows: Database.delete(ids, false) without an access level
// on a readable object that lacks the delete grant returns a DELETE_FAILED row
// for each record, with the statement TypeException's message.
func (vm *VM) defaultRunAsDeleteFailures(value Value) ([]dml.Result, bool) {
	if !vm.runAsSecureDefaults() || value.Kind != ValueList || len(value.List) == 0 {
		return nil, false
	}
	results := make([]dml.Result, len(value.List))
	for i, record := range value.List {
		if record.Kind != ValueObject {
			return nil, false
		}
		objectName, ok := vm.resolveObjectName(record.Type)
		if !ok || !vm.currentUserObjectPermission(objectName, "isAccessible") || vm.currentUserObjectPermission(objectName, "isDeletable") {
			return nil, false
		}
		_, id, _ := objectFieldValue(record, "Id")
		text, _ := idValueText(id)
		results[i] = dml.Result{ID: storage.ID(text), Error: fmt.Sprintf("DML operation DELETE not allowed on %s", objectName), StatusCode: "DELETE_FAILED"}
	}
	return results, true
}

// runAsSharingDenialText is the row text for a record the running user cannot
// write through sharing. Native sharing_access_modes d2 and d6_07 rows (runAs
// Standard User, custom objects, APIs 62.0-68.0) and C137_180/208/222/250
// (runAs, Account and a custom object, APIs 59.0-68.0): update reports the
// cross-reference id, delete the object id. Other contexts keep their text.
func (vm *VM) runAsSharingDenialText(op, objectName string, id storage.ID) (string, bool) {
	if vm.testContext == nil || vm.testContext.RunAsDepth <= vm.testContext.PackageRunAsDepth ||
		!apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 59) ||
		!(hasSuffixFold(objectName, "__c") || strings.EqualFold(objectName, "Account")) {
		return "", false
	}
	switch op {
	case "update":
		text := string(id)
		if len(text) > 15 {
			text = text[:15]
		}
		return runAsCrossReferenceDenial + text, true
	case "delete":
		return runAsObjectIDDenial, true
	}
	return "", false
}

const (
	runAsCrossReferenceDenial = "insufficient access rights on cross-reference id: "
	runAsObjectIDDenial       = "insufficient access rights on object id"
)

func isRunAsSharingDenialText(text string) bool {
	return strings.HasPrefix(text, runAsCrossReferenceDenial) || text == runAsObjectIDDenial
}

func (vm *VM) explicitRunAsSharingRowFailure(op, objectName string) bool {
	if (op != "update" && op != "delete") || !hasSuffixFold(objectName, "__c") || vm.testContext == nil ||
		vm.testContext.RunAsDepth <= vm.testContext.PackageRunAsDepth || !apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 62) {
		return false
	}
	permission, _ := userModeDMLPermissions(op)
	return vm.currentUserObjectPermission(objectName, permission)
}

// runAsCountDenialShape is a flat COUNT() with no filter or one positive
// equality on Name or Id: the captured entity-denial shapes.
func runAsCountDenialShape(query soql.Query) bool {
	if !query.Count || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 || len(query.GroupBy) != 0 || query.Having != nil ||
		len(query.Order) != 0 || query.HasLimit || query.HasOffset || query.ForUpdate || query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return false
	}
	where := query.Where
	if where == nil {
		return true
	}
	return (strings.EqualFold(where.Field, "Name") || strings.EqualFold(where.Field, "Id")) && where.Op == "=" && !where.Not &&
		len(where.And) == 0 && len(where.Or) == 0 && !where.Range && where.Subquery == nil
}

const accessLevelClauseConflictMessage = "Cannot use the WITH AccessLevel clause in dynamic queries that also specify an access level."

// runAsAccessLevelClauseConflict orders a WITH USER_MODE or WITH SYSTEM_MODE
// clause against an AccessLevel argument inside runAs at APIs 62.0-68.0
// (native sharing_access_modes u3 and sharing_access_follow f_u3 rows). The
// clash raises the SecurityException, except that Database.query and
// queryWithBinds first check a selected field the user cannot read: under a
// USER_MODE clause the QueryException lists it, under a SYSTEM_MODE clause with
// a USER_MODE argument the QueryException has no inaccessible map.
// Database.countQuery with a USER_MODE argument always raises the clash. Two
// SYSTEM_MODE selections and other contexts keep their existing routes.
func (vm *VM) runAsAccessLevelClauseConflict(route, queryText string, accessLevel Value) (error, bool) {
	if vm.testContext == nil || vm.testContext.RunAsDepth <= vm.testContext.PackageRunAsDepth || vm.Org == nil ||
		!apexversion.AtLeast(vm.currentSourceExecutionPolicy().APIVersion, 62) || !isDatabaseAccessLevelValue(accessLevel) || accessLevelPermissionSetID(accessLevel) != "" {
		return nil, false
	}
	query, err := vm.parseSOQLAt(queryText)
	if err != nil {
		return nil, false
	}
	clause := strings.ToUpper(strings.TrimSpace(query.SecurityMode))
	argument := databaseAccessLevelSecurityMode(accessLevel)
	if clause != "USER_MODE" && clause != "SYSTEM_MODE" || clause == "SYSTEM_MODE" && argument != "USER_MODE" {
		return nil, false
	}
	conflict := newExceptionError("SecurityException", accessLevelClauseConflictMessage)
	if route == "Database.countQuery" {
		if argument != "USER_MODE" {
			return nil, false
		}
		return conflict, true
	}
	objectName, ok := vm.resolveObjectName(query.Object)
	if !ok || !vm.currentUserObjectPermission(objectName, "isAccessible") || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 {
		return nil, false
	}
	var denied []string
	for _, field := range query.Fields {
		if strings.ContainsAny(field, ". ()") {
			return nil, false
		}
		canonical := vm.resolveSObjectFieldName(objectName, field)
		if strings.EqualFold(canonical, "Id") {
			continue
		}
		if !vm.currentUserFieldPermission(objectName, canonical, "isAccessible") {
			denied = append(denied, canonical)
		}
	}
	if len(denied) == 0 {
		return conflict, true
	}
	if clause == "USER_MODE" {
		return userModeFieldException(objectName, denied, false), true
	}
	return newExceptionError("QueryException", soql.MissingColumnMessage(denied[0], objectName)), true
}
