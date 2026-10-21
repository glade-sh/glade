package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

const jsonCapturedOwnerProjectionField = "__glade_json_captured_owner_projection"

// Ordinary JSON captures cover single-string Case.Owner envelopes for User,
// Group and Account, Task.Owner for User, and the bounded API65 expanded
// envelopes below. Other relationships, targets and strict mode retain their path.
func (vm *VM) capturedOwnerValueFromJSON(containingType, relationship string, raw any, strict bool) (Value, bool, error) {
	if strict || vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" || relationship != "Owner" ||
		(containingType != "Case" && containingType != "Task") {
		return Null, false, nil
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 62) || apexversion.AtLeast(version, 69) {
		return Null, false, nil
	}
	if containingType == "Case" && apexversion.AtLeast(version, 65) && !apexversion.AtLeast(version, 66) {
		if value, handled, err := vm.capturedExpandedOwnerValueFromJSON(raw); handled || err != nil {
			return value, handled, err
		}
	}
	// Retain repeated keys so unmeasured duplicate payloads use the old path.
	fields, ok := jsonSObjectFieldsInTextualOrder(raw)
	if !ok {
		return Null, false, nil
	}
	unknown := func() (Value, bool, error) {
		return Null, true, jsonDeserializeException("Cannot deserialize instance of <unknown> from null value null or request may be missing a required field")
	}
	if containingType == "Case" && len(fields) == 1 && fields[0].name == "FirstName" {
		if text, stringValue := fields[0].value.(string); stringValue && text != "" {
			return unknown()
		}
	}
	if len(fields) != 2 {
		return Null, false, nil
	}
	var payload orderedJSONField
	target := ""
	for _, field := range fields {
		if field.name != "attributes" {
			payload = field
			continue
		}
		attributes, object := jsonSObjectFieldsInTextualOrder(field.value)
		if !object || len(attributes) != 1 || attributes[0].name != "type" {
			return Null, false, nil
		}
		target, _ = attributes[0].value.(string)
	}
	if text, stringValue := payload.value.(string); !stringValue || text == "" || payload.name == "" {
		return Null, false, nil
	}
	accepted := target == "User" && payload.name == "FirstName"
	if containingType == "Case" {
		accepted = accepted || (target == "Group" || target == "Account") && payload.name == "Name"
		if target == "Group" && payload.name == "FirstName" ||
			target == "User" && !jsonAllowedFieldContains(vm.jsonAllowedFields("User"), payload.name) {
			return unknown()
		}
	}
	if !accepted {
		return Null, false, nil
	}
	if resolved, known := vm.resolveObjectName(target); !known || !strings.EqualFold(resolved, target) {
		return Null, false, nil
	}
	value, err := vm.typedValueFromJSON(target, jsonTypedInput{value: raw, sObjectRecord: true}, false)
	return value, true, err
}

// API65 ordinary Case.Owner captures additionally cover an explicit User ID,
// a null or matching URL attribute, and the Name projection with optional Type.
// Keep these measured envelopes separate from the earlier two-field controls.
func (vm *VM) capturedExpandedOwnerValueFromJSON(raw any) (Value, bool, error) {
	fields, ok := jsonSObjectFieldsInTextualOrder(raw)
	if !ok || (len(fields) != 3 && len(fields) != 4) {
		return Null, false, nil
	}
	values := make(map[string]any, len(fields))
	for _, field := range fields {
		if _, duplicate := values[field.name]; duplicate {
			return Null, false, nil
		}
		values[field.name] = field.value
	}
	first, firstOK := values["FirstName"].(string)
	id, idOK := values["Id"].(string)
	attributes, attributesOK := jsonSObjectFieldsInTextualOrder(values["attributes"])
	if !firstOK || first == "" || !idOK || validateApexID(id) != nil || !attributesOK || len(attributes) != 2 {
		return Null, false, nil
	}
	idType, known := vm.sObjectNameForID(id)
	if !known || idType != "User" {
		return Null, false, nil
	}
	target, url, hasURL := "", any(nil), false
	for _, attribute := range attributes {
		switch attribute.name {
		case "type":
			target, _ = attribute.value.(string)
		case "url":
			url, hasURL = attribute.value, true
		default:
			return Null, false, nil
		}
	}
	if !hasURL || (target != "User" && target != "Name") {
		return Null, false, nil
	}
	url65 := "/services/data/v65.0/sobjects/" + idType + "/" + id
	url68 := "/services/data/v68.0/sobjects/" + idType + "/" + id
	generatedURL := "/services/data/v68.0/sobjects/" + idType + "/" + displayIDText(id)
	// Ordinary API65 captures also accept the canonical 18-character ID
	// in a REST65 URL for either lexical ID width and each measured target.
	generatedURL65 := "/services/data/v65.0/sobjects/" + idType + "/" + displayIDText(id)
	allowedURL := len(id) == 15 && (url == nil || url == url68 || url == url65 && (target == "User" || len(fields) == 4))
	if !allowedURL && url != generatedURL && url != generatedURL65 {
		return Null, false, nil
	}
	if len(fields) == 4 {
		if target != "Name" || values["Type"] != idType {
			return Null, false, nil
		}
	} else if _, hasType := values["Type"]; hasType {
		return Null, false, nil
	}
	value, err := vm.typedValueFromJSON(target, jsonTypedInput{value: raw, sObjectRecord: true}, false)
	if err == nil {
		value.Fields[jsonCapturedOwnerProjectionField] = Bool(true)
	}
	return value, true, err
}
