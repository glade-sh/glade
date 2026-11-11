package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// The static field is absent at source API 62 and present from 63 onward.
// Native observations: case289_followups ST1-ST4 and static-boundary.tsv.
func objectPermissionsFieldHidden(version, object, field string) bool {
	return apexversion.AtLeast(version, 62) && !apexversion.AtLeast(version, 63) &&
		strings.EqualFold(object, "ObjectPermissions") && strings.EqualFold(field, "PermissionsViewAllFields")
}

// An object-qualified SOQL projection names the same standard field.
func objectPermissionsQueryField(object, field string) string {
	prefix := object + "."
	if len(field) > len(prefix) && strings.EqualFold(field[:len(prefix)], prefix) {
		return field[len(prefix):]
	}
	return field
}

// semaObjectPermissionsHiddenFieldPath finds the hidden static field in a dotted
// path that names it through the object token, through Schema, or through an
// SObjectType fields member, with any trailing member such as getDescribe.
// Native: object_permissions_schema_qualified_token/org.tsv (source API 62.0
// rejects, 67.0 accepts). A local variable or a project type that shadows the
// root keeps its own meaning.
//
// The diagnostic text follows the native rows: a plain member read names the
// field, a chained call names the whole path, and the Schema.SObjectType
// fields member names the field in both forms.
func semaObjectPermissionsHiddenFieldPath(version, path string, chained bool, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	shapes := [][]string{
		{"ObjectPermissions", "PermissionsViewAllFields"},
		{"Schema", "ObjectPermissions", "PermissionsViewAllFields"},
		{"ObjectPermissions", "SObjectType", "fields", "PermissionsViewAllFields"},
		{"Schema", "ObjectPermissions", "SObjectType", "fields", "PermissionsViewAllFields"},
		{"Schema", "SObjectType", "ObjectPermissions", "fields", "PermissionsViewAllFields"},
	}
	for _, shape := range shapes {
		if len(parts) < len(shape) {
			continue
		}
		matched := true
		for i, want := range shape {
			if !strings.EqualFold(strings.TrimSpace(parts[i]), want) {
				matched = false
				break
			}
		}
		if !matched || !objectPermissionsFieldHidden(version, "ObjectPermissions", "PermissionsViewAllFields") {
			continue
		}
		root := strings.TrimSpace(parts[0])
		if _, bound := scope.lookup(root); bound || semaProjectTypeShadowsPlatform(model, root) {
			return "", false
		}
		if strings.EqualFold(root, "Schema") && len(shape) > 2 && semaProjectTypeShadowsPlatform(model, parts[1]) {
			return "", false
		}
		if !chained || strings.EqualFold(shape[1], "SObjectType") {
			return "PermissionsViewAllFields", true
		}
		return strings.Join(parts[:len(shape)], "."), true
	}
	return "", false
}
