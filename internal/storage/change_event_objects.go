package storage

import "strings"

// EnsureChangeEventObject installs a change event's field schema from its base
// object. It returns true only when it adds an object; existing records and
// definitions are retained. This does not enable or deliver change events.
func EnsureChangeEventObject(org *OrgState, objectName string) bool {
	const suffix = "ChangeEvent"
	if org == nil || len(objectName) <= len(suffix) || !strings.EqualFold(objectName[len(objectName)-len(suffix):], suffix) {
		return false
	}
	if _, exists := ResolveObjectName(*org, objectName); exists {
		return false
	}
	baseName := objectName[:len(objectName)-len(suffix)]
	if strings.HasSuffix(baseName, "__") {
		baseName += "c"
	}
	var base ObjectDefinition
	if canonical, ok := ResolveObjectName(*org, baseName); ok {
		base = org.Objects[canonical].Definition.Clone()
	} else {
		var ok bool
		base, ok = StandardObjectDefinition(baseName)
		if !ok {
			return false
		}
	}
	objectName = base.APIName + suffix
	if strings.HasSuffix(base.APIName, "__c") {
		objectName = strings.TrimSuffix(base.APIName, "__c") + "__ChangeEvent"
	}
	if _, exists := ResolveObjectName(*org, objectName); exists {
		return false
	}
	EnsureStandardObjectFieldsForFeatures(&base, org.Features)
	fields := make(map[string]Field, len(base.Fields)+4)
	for key, field := range base.Fields {
		name := field.APIName
		if name == "" {
			name = key
		}
		if changeEventExcludedBaseFields[strings.ToLower(name)] {
			continue
		}
		// Event objects never own a parent's child relationship.
		field.ChildRelationshipName = ""
		fields[key] = field
	}
	for _, added := range changeEventAddedFields[strings.ToLower(base.APIName)] {
		if _, exists := ResolveFieldName(ObjectDefinition{Fields: fields}, "", added.field); exists {
			continue
		}
		field := Field{APIName: added.field, Type: FieldString}
		if source, ok := StandardObjectDefinition(added.source); ok {
			if canonical, ok := ResolveFieldName(source, "", added.field); ok {
				field = source.Fields[canonical]
			}
		}
		field.ChildRelationshipName = ""
		field.Nillable = BoolFlag(added.nillable)
		fields[field.APIName] = field
	}
	fields["ChangeEventHeader"] = Field{APIName: "ChangeEventHeader", Type: FieldAny}
	fields["ReplayId"] = Field{APIName: "ReplayId", Type: FieldString}
	if org.Objects == nil {
		org.Objects = make(map[string]ObjectState)
	}
	org.Objects[objectName] = ObjectState{
		Definition: ObjectDefinition{APIName: objectName, Label: objectName, PluralLabel: objectName, Fields: fields, Metadata: map[string]string{"kind": changeEventKind, "parentObject": base.APIName}},
		Records:    make(map[ID]Record),
		Indexes:    make(map[string]IndexSet),
	}
	org.ClearRuntimeSchemaStamp()
	return true
}

const changeEventKind = "changeEvent"

// IsChangeEventObject reports whether definition was installed by
// EnsureChangeEventObject.
func IsChangeEventObject(definition ObjectDefinition) bool {
	return definition.Metadata["kind"] == changeEventKind && definition.Metadata["parentObject"] != ""
}

// Native describe rows (change_event_describe, APIs 62.0, 67.0 and 68.0)
// show these base fields absent from the events. Feature-enabled followup
// rows also exclude IsCustomerPortal even when Account exposes it.
var changeEventExcludedBaseFields = map[string]bool{
	"isdeleted": true, "systemmodstamp": true, "masterrecordid": true, "lastactivitydate": true,
	"lastreferenceddate": true, "lastvieweddate": true, "photourl": true,
	"iscustomerportal": true,
}

// ChangeEventExcludesBaseField reports whether a base field is absent from
// the base object's change event. Sema shares this rule.
func ChangeEventExcludesBaseField(name string) bool {
	return changeEventExcludedBaseFields[strings.ToLower(strings.TrimSpace(name))]
}

type changeEventAddedField struct {
	field, source string
	nillable      bool
}

// AccountChangeEvent also lists the name components below although its base
// describe does not. Native followup rows confirm Contact's type, length and
// label, but all three event fields are nillable, including LastName.
var changeEventAddedFields = map[string][]changeEventAddedField{
	"account": {{"FirstName", "Contact", true}, {"LastName", "Contact", true}, {"Salutation", "Contact", true}},
}
