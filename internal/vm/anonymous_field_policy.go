package vm

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
)

// Anonymous Apex that runs as the default administrator without a field grant
// cannot see an ordinary custom field the API 62+ field policy denies: the
// describe field map omits it, SObject.put rejects it as an invalid field, and
// dynamic SOQL reports no such column (SObject.get has no captured row and is
// unchanged). Static references to it do
// not compile (sema receives AnonymousPolicyHiddenFields). Native rows:
// admin_optional_field_api_matrix anonymous cells at 62.0, 66.0, 67.0 and 68.0.
// Named classes, tests and triggers keep the describe-flag behavior.

// anonymousFieldPolicyScope reports an anonymous frame at a policy API whose
// principal is the System Administrator.
func (vm *VM) anonymousFieldPolicyScope() bool {
	if vm == nil || vm.Org == nil || vm.testContext != nil || vm.currentClass != "" || vm.currentMethod.ClassName != "" || vm.currentTrigger {
		return false
	}
	return adminOptionalFieldPolicyAPI(vm.currentMethod.APIVersion) && vm.currentUserProfileName(vm.fieldPermissionPrincipal()) == "System Administrator"
}

// adminFLSStandardFields are the standard fields the default administrator
// needs an explicit field grant for, in every baseline org (native
// case_standard_field_fls rows, API 62.0 and 67.0): anonymous Apex does not see
// them, API 67.0 named queries fail with "No such column", and a permission set
// field grant makes every route see and query them. Keyed by object, lowercase.
var adminFLSStandardFields = map[string]map[string]bool{
	"case": {
		"entitlementid": true, "businesshoursid": true, "milestonestatus": true,
		"slastartdate": true, "slaexitdate": true, "isstopped": true,
		"stopstartdate": true, "isclosedoncreate": true, "productid": true,
		"servicecontractid": true, "assetwarrantyid": true,
	},
}

// adminFLSStandardField reports a standard field that needs an explicit grant.
func adminFLSStandardField(objectName, fieldName string) bool {
	return adminFLSStandardFields[strings.ToLower(strings.TrimSpace(objectName))][strings.ToLower(strings.TrimSpace(fieldName))]
}

func hasAdminFLSStandardFields(objectName string) bool {
	return len(adminFLSStandardFields[strings.ToLower(strings.TrimSpace(objectName))]) > 0
}

// adminFieldNeedsExplicitGrant reports a field whose access the default
// administrator owes to a profile or permission set grant: an optional custom
// field on an ordinary custom object, or one of adminFLSStandardFields.
func (vm *VM) adminFieldNeedsExplicitGrant(objectName, fieldName string) bool {
	if adminFLSStandardField(objectName, fieldName) {
		// Only the routes the native rows prove apply the standard field rule:
		// anonymous visibility and default-mode queries (adminStandardFLSRoute).
		// Describe flags, stripInaccessible, explicit USER_MODE and user-mode DML
		// keep their earlier behavior (no rows; parked).
		return vm.adminStandardFLSRoute
	}
	return vm.ordinaryCustomPermissionObject(objectName) && isCustomSchemaName(fieldName) && !vm.customFieldHasIntrinsicAccess(objectName, fieldName)
}

// policyHiddenField reports a field the principal cannot read under the field
// policy: an existing optional custom field of an ordinary custom object or a
// standard field in adminFLSStandardFields.
func (vm *VM) policyHiddenField(objectName, fieldName string) bool {
	if adminFLSStandardField(objectName, fieldName) {
		saved := vm.adminStandardFLSRoute
		vm.adminStandardFLSRoute = true
		defer func() { vm.adminStandardFLSRoute = saved }()
		return !vm.currentUserFieldPermission(objectName, fieldName, "isAccessible")
	}
	if !vm.ordinaryCustomPermissionObject(objectName) {
		return false
	}
	_, field, ok := vm.sObjectFieldDefinition(objectName, fieldName)
	if !ok || !isCustomSchemaName(field.APIName) || vm.customFieldHasIntrinsicAccess(objectName, field.APIName) {
		return false
	}
	return !vm.currentUserFieldPermission(objectName, field.APIName, "isAccessible")
}

func (vm *VM) anonymousPolicyHiddenField(objectName, fieldName string) bool {
	return vm.anonymousFieldPolicyScope() && vm.policyHiddenField(objectName, fieldName)
}

// anonymousPolicyHiddenQueryField returns the first direct field of query
// that anonymous Apex cannot see.
func (vm *VM) anonymousPolicyHiddenQueryField(query soql.Query) (string, bool) {
	if !vm.anonymousFieldPolicyScope() || !(vm.ordinaryCustomPermissionObject(query.Object) || hasAdminFLSStandardFields(query.Object)) {
		return "", false
	}
	fields := append([]string(nil), query.Fields...)
	fields = append(fields, soqlConditionFields(query.Where)...)
	for _, order := range query.Order {
		fields = append(fields, order.Field)
	}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || strings.ContainsAny(field, ". ()\t\r\n") {
			continue
		}
		if vm.policyHiddenField(query.Object, field) {
			return field, true
		}
	}
	return "", false
}

// AnonymousPolicyHiddenFields lists, per object, the custom fields named in
// source that anonymous Apex at apiVersion cannot see when it runs as the
// default administrator without a field grant. Names are local to the org
// namespace. A field whose name does not occur in source cannot be referenced
// statically, so it is not checked.
func (vm *VM) AnonymousPolicyHiddenFields(apiVersion, source string) map[string][]string {
	if vm == nil || vm.Org == nil || vm.testContext != nil || !adminOptionalFieldPolicyAPI(apiVersion) {
		return nil
	}
	saved := vm.currentMethod.APIVersion
	vm.currentMethod.APIVersion = apiVersion
	defer func() { vm.currentMethod.APIVersion = saved }()
	if !vm.anonymousFieldPolicyScope() {
		return nil
	}
	defer vm.beginPermissionCheckMemo()()
	lower := strings.ToLower(source)
	prefix := ""
	if vm.Org.Namespace != "" {
		prefix = strings.ToLower(vm.Org.Namespace) + "__"
	}
	local := func(name string) string {
		if prefix != "" && strings.HasPrefix(strings.ToLower(name), prefix) {
			return name[len(prefix):]
		}
		return name
	}
	var hidden map[string][]string
	for objectName, state := range vm.Org.Objects {
		if !isCustomSchemaName(objectName) || !vm.ordinaryCustomPermissionObject(objectName) {
			continue
		}
		for fieldName, field := range state.Definition.Fields {
			name := field.APIName
			if name == "" {
				name = fieldName
			}
			if !isCustomSchemaName(name) || !strings.Contains(lower, strings.ToLower(local(name))) {
				continue
			}
			if vm.policyHiddenField(objectName, name) {
				if hidden == nil {
					hidden = make(map[string][]string)
				}
				hidden[local(objectName)] = append(hidden[local(objectName)], local(name))
			}
		}
	}
	for objectName, fieldNames := range adminFLSStandardFields {
		for fieldName := range fieldNames {
			if !strings.Contains(lower, fieldName) {
				continue
			}
			canonical, ok := vm.standardPolicyFieldName(objectName, fieldName)
			if !ok || !vm.policyHiddenField(objectName, canonical) {
				continue
			}
			if hidden == nil {
				hidden = make(map[string][]string)
			}
			object, _ := vm.resolveObjectName(objectName)
			hidden[object] = append(hidden[object], canonical)
		}
	}
	for _, fields := range hidden {
		sort.Strings(fields)
	}
	return hidden
}

// standardPolicyFieldName returns the catalog spelling of a standard field that
// the org defines.
func (vm *VM) standardPolicyFieldName(objectName, fieldName string) (string, bool) {
	_, field, ok := vm.sObjectFieldDefinition(objectName, fieldName)
	if !ok {
		return "", false
	}
	return field.APIName, true
}
