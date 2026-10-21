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
	return !vm.currentTrigger && vm.testContext != nil && vm.testContext.RunAsDepth > vm.testContext.PackageRunAsDepth &&
		apexversion.AtLeast(vm.currentMethod.APIVersion, 67)
}

// Owned sharing_update_observations rows report DmlException, rather than the
// explicit USER_MODE preflight exception, for a non-owner scalar text update
// on a Private custom object from source API 67. CRUD/FLS checks precede this
// route choice. Other payloads and access modes retain their existing paths.
func (vm *VM) defaultRunAsPrivateTextUpdate(op string, value Value) bool {
	if op != "update" || value.Kind != ValueObject || !vm.runAsSecureDefaults() || vm.Org == nil {
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
	if !vm.runAsSecureDefaults() {
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
		if vm.currentUserObjectPermission(objectName, "isCreateable") ||
			vm.currentUserObjectPermission(objectName, "isUpdateable") ||
			vm.currentUserObjectPermission(objectName, "isDeletable") {
			continue // The readable denial capture has no create/edit/delete grants.
		}
		if op == "upsert" {
			message := fmt.Sprintf("Operation failed due to fields being inaccessible on Sobject %s, check errors on Exception or Result! ", objectName)
			exception := Object("DmlException")
			exception.Fields["message"] = String(message)
			exception.Fields["__dmlErrors"] = dmlExceptionErrorDetails([]dml.Result{{Error: message, StatusCode: "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY"}}, []string{objectName})
			return &apexThrowError{value: exception}
		}
		return newExceptionError("TypeException", fmt.Sprintf("DML operation %s not allowed on %s", strings.ToUpper(op), objectName))
	}
	return nil
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
// scoped by Id. COUNT denies entity access; projections report a missing object.
func (vm *VM) defaultRunAsSOQLDenial(query soql.Query) error {
	if !vm.runAsSecureDefaults() || len(query.ChildQueries) != 0 || len(query.Typeofs) != 0 ||
		(!query.Count && len(query.Aggregates) != 0) || len(query.GroupBy) != 0 || query.Having != nil ||
		len(query.Order) != 0 || query.HasLimit || query.HasOffset || query.ForUpdate ||
		query.ForView || query.ForReference || query.AllRows || query.UsingScope != "" {
		return nil
	}
	where := query.Where
	if where == nil || !strings.EqualFold(where.Field, "Id") || where.Op != "=" || where.Not ||
		len(where.And) != 0 || len(where.Or) != 0 || where.Range || where.Subquery != nil {
		return nil
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
