package storage

import "strings"

// StandardFieldAssignmentRequiresEmptyID records standard fields whose Apex
// setter rejects an ID-bearing receiver, including a synthetic or sparse ID.
// This is an assignment contract, separate from DML create/update permissions.
func StandardFieldAssignmentRequiresEmptyID(objectName, fieldName string) bool {
	return strings.EqualFold(objectName, "PricebookEntry") && strings.EqualFold(fieldName, "Pricebook2Id")
}

// StandardFieldAssignmentReadOnly combines retained accessor metadata with
// fields assignable when creating or updating a record. An insert-only field
// can still be assigned in Apex even when its stub omits a setter.
// Record identity remains assignable even though its describe flags are false.
func StandardFieldAssignmentReadOnly(objectName, fieldName string) bool {
	fields, known := standardSObjectStubReadOnlyFieldsFor(objectName)
	if !known {
		return false
	}
	found := false
	for _, name := range fields {
		if strings.EqualFold(name, fieldName) {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	definition, known := StandardObjectDefinition(objectName)
	if !known {
		return false
	}
	for name, field := range definition.Fields {
		if strings.EqualFold(name, fieldName) {
			return field.Type != FieldID && !FieldFlagValue(field.Createable, false) && !FieldFlagValue(field.Updateable, false)
		}
	}
	return false
}
