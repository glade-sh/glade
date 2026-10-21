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
	if standardRTEMEventAssignable(objectName) || StandardFieldAssignmentRequiresNamedCode(objectName, fieldName) {
		return false
	}
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

// These setters compile in named Apex at API 62 and 67. Execute Anonymous
// applies a stricter read-only check to the same fields.
func StandardFieldAssignmentRequiresNamedCode(objectName, fieldName string) bool {
	return strings.EqualFold(objectName, "AccountHistory") && strings.EqualFold(fieldName, "Field") ||
		strings.EqualFold(objectName, "EventLogFile") && strings.EqualFold(fieldName, "LogDate")
}

// Dynamic put follows the RTEM field's describe permissions even though its
// Apex setter accepts assignment. Record identity retains its ordinary rule.
func StandardFieldDynamicPutReadOnly(objectName, fieldName string) bool {
	if !standardRTEMEventAssignable(objectName) {
		return false
	}
	definition, known := StandardObjectDefinition(objectName)
	if !known {
		return false
	}
	name, known := ResolveFieldName(definition, "", fieldName)
	if !known {
		return false
	}
	field := definition.Fields[name]
	return field.Type != FieldID && !FieldFlagValue(field.Createable, false) && !FieldFlagValue(field.Updateable, false)
}

// rtemEventAssignableObjects are Real-Time Event Monitoring event and event
// store objects. Salesforce compiles Apex assignment to every field of these
// objects, including describe-read-only fields such as CreatedDate, at API 62
// and 67 (native deploy rows, 2026-10-18). Dynamic SObject.put stays read-only.
var rtemEventAssignableObjects = map[string]struct{}{
	"apianomalyeventstore": {}, "apievent": {}, "bulkapiresulteventstore": {},
	"credentialstuffingeventstore": {}, "fileeventstore": {},
	"guestuseranomalyeventstore": {}, "identityprovidereventstore": {},
	"identityverificationevent": {}, "lightningurievent": {},
	"listviewevent": {}, "loginanomalyeventstore": {}, "loginasevent": {},
	"loginevent": {}, "logoutevent": {}, "permissionsetevent": {}, "permissionseteventstore": {},
	"reportanomalyeventstore": {}, "reportevent": {},
	"sessionhijackingeventstore": {}, "universalanomalyeventstore": {},
	"urievent": {},
}

func standardRTEMEventAssignable(objectName string) bool {
	_, ok := rtemEventAssignableObjects[strings.ToLower(strings.TrimSpace(objectName))]
	return ok
}
