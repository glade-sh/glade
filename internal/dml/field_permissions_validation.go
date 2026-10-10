package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
)

// FieldPermissions uses a schema-backed restricted field selector. Missing
// metadata keeps its existing behavior; only an explicit catalog denial is known.
func (e *Engine) validateFieldPermissionTarget(record storage.Record) error {
	if !strings.EqualFold(record.Object, "FieldPermissions") || e.Org == nil {
		return nil
	}
	value, ok := record.GetField("Field")
	if !ok || value.Kind != storage.ValueString {
		return nil
	}
	objectName, fieldName, qualified := strings.Cut(value.String, ".")
	if !qualified || fieldName == "" {
		return nil
	}
	canonicalObject, ok := storage.ResolveObjectName(*e.Org, objectName)
	if !ok {
		return nil
	}
	definition := e.Org.Objects[canonicalObject].Definition
	canonicalField, ok := storage.ResolveFieldName(definition, e.Org.Namespace, fieldName)
	if !ok {
		return nil
	}
	field := definition.Fields[canonicalField]
	if field.Permissionable != nil && !*field.Permissionable {
		return dmlErrorf("INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST", []string{"Field"}, "Field Name: bad value for restricted picklist field: %s", value.String)
	}
	return nil
}
