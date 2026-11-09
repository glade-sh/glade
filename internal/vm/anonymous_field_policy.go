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

// policyHiddenField reports an existing ordinary custom field the principal
// cannot read under the field policy.
func (vm *VM) policyHiddenField(objectName, fieldName string) bool {
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
	if !vm.anonymousFieldPolicyScope() || !vm.ordinaryCustomPermissionObject(query.Object) {
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
	for _, fields := range hidden {
		sort.Strings(fields)
	}
	return hidden
}
