package storage

import "strings"

// The owned describe catalog records permission checkboxes as non-nillable
// even when defaultedOnCreate is false. That is not a caller-required value.
// Preserve these native facets separately from derived DML requiredness.
func permissionSetCheckboxSchemaNeedsRepair(definition ObjectDefinition) bool {
	if !strings.EqualFold(definition.APIName, "PermissionSet") {
		return false
	}
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return false
	}
	for _, captured := range describe.Fields {
		if captured.Type != "boolean" || !strings.HasPrefix(captured.Name, "Permissions") {
			continue
		}
		name, exists := ResolveFieldName(definition, "", captured.Name)
		if !exists {
			continue
		}
		field := definition.Fields[name]
		if field.Type == FieldBoolean && (field.Required || field.Nillable == nil || field.DefaultedOnCreate == nil) {
			return true
		}
	}
	return false
}

func applyPermissionSetCheckboxSchema(definition *ObjectDefinition) {
	if definition == nil || !strings.EqualFold(definition.APIName, "PermissionSet") {
		return
	}
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return
	}
	for _, captured := range describe.Fields {
		if captured.Type != "boolean" || !strings.HasPrefix(captured.Name, "Permissions") {
			continue
		}
		name, exists := ResolveFieldName(*definition, "", captured.Name)
		if !exists {
			continue
		}
		field := definition.Fields[name]
		if field.Type != FieldBoolean {
			continue
		}
		field.Required = false
		if field.Nillable == nil {
			field.Nillable = cloneBoolFlag(captured.Nillable)
		}
		if field.DefaultedOnCreate == nil {
			field.DefaultedOnCreate = cloneBoolFlag(captured.DefaultedOnCreate)
		}
		definition.Fields[name] = field
	}
}

// Keep the existing sparse ObjectPermissions insertion shape separate from
// permission-set defaults. These facets do not imply caller-required inputs.
func applyObjectPermissionCheckboxFacets(definition *ObjectDefinition) {
	if definition == nil || !strings.EqualFold(definition.APIName, "ObjectPermissions") {
		return
	}
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return
	}
	for _, captured := range describe.Fields {
		if captured.Type != "boolean" {
			continue
		}
		name, exists := ResolveFieldName(*definition, "", captured.Name)
		if !exists || definition.Fields[name].Type != FieldBoolean {
			continue
		}
		field := definition.Fields[name]
		facets := describeField(captured)
		facets.Required = false
		enrichStandardField(&field, facets)
		definition.Fields[name] = field
	}
}

func objectPermissionCheckboxSchemaNeedsRepair(definition ObjectDefinition) bool {
	if !strings.EqualFold(definition.APIName, "ObjectPermissions") {
		return false
	}
	describe, ok, err := lookupStandardDescribeCatalogV2(definition.APIName)
	if err != nil || !ok {
		return false
	}
	for _, captured := range describe.Fields {
		if captured.Type != "boolean" {
			continue
		}
		name, exists := ResolveFieldName(definition, "", captured.Name)
		if !exists {
			continue
		}
		field := definition.Fields[name]
		if field.Type == FieldBoolean && (field.Nillable == nil || field.Createable == nil || field.DefaultedOnCreate == nil) {
			return true
		}
	}
	return false
}
