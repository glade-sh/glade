package vm

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) displayString(value Value, result *Result) (string, error) {
	if idText, ok := typedIDValueText(value); ok {
		return displayIDText(idText), nil
	}
	switch value.Kind {
	case ValueList:
		return vm.displayList(value.List, result)
	case ValueSet:
		return vm.displaySet(value.Set, result)
	case ValueMap:
		return value.String(), nil
	case ValueObject:
	default:
		return value.String(), nil
	}
	if value.Type == "Type" {
		if text := typeValueText(value); text != "" {
			if objectName, ok := vm.resolveObjectName(text); ok {
				return vm.sObjectTypeDisplayName(objectName), nil
			}
			return text, nil
		}
	}
	if strings.EqualFold(value.Type, "UUID") {
		if text, err := platformScalarText(value, "UUID"); err == nil {
			return text, nil
		}
	}
	if strings.EqualFold(value.Type, "String") {
		if text, err := platformScalarText(value, "String"); err == nil {
			return text, nil
		}
	}
	if _, ok := stubProxyTypeName(value); ok {
		return value.String(), nil
	}
	if strings.EqualFold(value.Type, "Date") {
		if text, err := platformScalarText(value, "Date"); err == nil {
			return text + " 00:00:00", nil
		}
	}
	if strings.EqualFold(value.Type, "Schema.SObjectField") {
		if fieldName, ok := value.Fields["field"]; ok && fieldName.Kind == ValueString {
			return fieldName.Text, nil
		}
	}
	if strings.EqualFold(value.Type, "Schema.SObjectType") {
		if objectName, ok := value.Fields["object"]; ok && objectName.Kind == ValueString {
			return vm.sObjectTypeDisplayName(objectName.Text), nil
		}
	}
	if value.Type == "LoggingLevel" && isLoggingLevelName(value.Text) {
		return value.Text, nil
	}
	if value.Type == "RoundingMode" && isDecimalRoundingModeName(value.Text) {
		return value.Text, nil
	}
	if value.Type == "StatusCode" && value.Text != "" {
		return value.Text, nil
	}
	if class, ok := vm.Classes[value.Type]; ok && len(class.EnumValues) > 0 && value.Text != "" {
		return value.Text, nil
	}
	if value.Text != "" && strings.Contains(value.Type, ".") {
		return value.Text, nil
	}
	if isExceptionType(value.Type) {
		return exceptionToString(value), nil
	}
	if text, ok, err := vm.frameworkQualifiedMethodVerifierDisplayString(value, result); ok || err != nil {
		return text, err
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(value.Type, "toString", nil)
	if ambiguous {
		return "", vm.ambiguousOverloadError("toString", nil)
	}
	if !ok {
		return vm.defaultObjectDisplayString(value), nil
	}
	out, err := vm.callMethodWithReceiver(target, value, nil, result)
	if err != nil {
		return "", err
	}
	if out.Kind != ValueString {
		return "", fmt.Errorf("%s returned %s, want String", target.Name, out.Kind)
	}
	return out.Text, nil
}

func (vm *VM) defaultObjectDisplayString(value Value) string {
	text := value.String()
	if value.Kind != ValueObject || !strings.Contains(value.Type, ".") || vm == nil {
		return text
	}
	if _, ok := vm.Classes[value.Type]; !ok {
		return text
	}
	shortName := value.Type[strings.LastIndex(value.Type, ".")+1:]
	return shortName + strings.TrimPrefix(text, value.Type)
}

func stringValueOfDate(value Value) (string, error) {
	text, err := platformScalarText(value, "Date")
	if err != nil {
		return "", err
	}
	if strings.EqualFold(value.Static, "Object") {
		return text + " 00:00:00", nil
	}
	return text, nil
}

func (vm *VM) sObjectTypeDisplayName(typeName string) string {
	objectName := typeName
	if resolved, ok := vm.resolveObjectName(typeName); ok {
		objectName = resolved
	}
	displayName := vm.describeObjectName(objectName)
	if vm.sObjectTypeDescribeShouldUseLocalName(objectName) {
		return localSchemaName(displayName)
	}
	return displayName
}

func (vm *VM) equalCurrentNamespaceApexStubText(left, right string) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(vm.Org.Namespace) == "" {
		return false
	}
	if !strings.Contains(left, "__sfdc_ApexStub") || !strings.Contains(right, "__sfdc_ApexStub") {
		return false
	}
	return vm.normalizeCurrentNamespaceApexStubText(left) == vm.normalizeCurrentNamespaceApexStubText(right)
}

func (vm *VM) normalizeCurrentNamespaceApexStubText(text string) string {
	namespace := strings.TrimSpace(vm.Org.Namespace)
	if namespace == "" || !strings.Contains(text, "__sfdc_ApexStub") {
		return text
	}
	return strings.ReplaceAll(text, namespace+".", "")
}

func (vm *VM) frameworkQualifiedMethodVerifierDisplayString(value Value, result *Result) (string, bool, error) {
	if value.Kind != ValueObject || !strings.EqualFold(frameworkMockSupportType(value.Type), "QualifiedMethod") {
		return "", false, nil
	}
	if !vm.inFrameworkMethodVerifierFrame() {
		return "", false, nil
	}
	typeName := objectStringField(value, "typeName")
	if before, after, ok := strings.Cut(typeName, "."); ok && before != "" && strings.Contains(after, "__sfdc_ApexStub") {
		if vm != nil && vm.Org != nil && strings.EqualFold(before, strings.TrimSpace(vm.Org.Namespace)) {
			typeName = after
		}
	}
	methodName := objectStringField(value, "methodName")
	_, argTypes, _ := objectFieldValue(value, "methodArgTypes")
	argsText, err := vm.displayString(argTypes, result)
	if err != nil {
		return "", true, err
	}
	return typeName + "." + methodName + argsText, true, nil
}

func (vm *VM) inFrameworkMethodVerifierFrame() bool {
	if strings.EqualFold(frameworkMockSupportType(vm.currentMethod.ClassName), "MethodVerifier") {
		return true
	}
	for _, frame := range vm.callStack {
		if strings.EqualFold(frameworkMockSupportType(classNameFromMethod(frame.Symbol)), "MethodVerifier") {
			return true
		}
	}
	return false
}

func (vm *VM) displayList(values []Value, result *Result) (string, error) {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		text, err := vm.displayString(item, result)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

func (vm *VM) displaySet(values []Value, result *Result) (string, error) {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		text, err := vm.displayString(item, result)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

const (
	sobjectErrorsField                       = "__glade_errors"
	sobjectReadOnlyField                     = "__glade_readonly"
	sobjectQueriedFieldsField                = "__glade_queried_fields"
	sobjectExplicitFieldsField               = "__glade_explicit_fields"
	sobjectSetFieldsField                    = "__glade_set_fields"
	sobjectUserSetFieldsField                = "__glade_user_set_fields"
	sobjectDMLOptionsField                   = "__glade_dml_options"
	sobjectDMLAccessibleField                = "__glade_dml_accessible"
	sobjectTriggerField                      = "__glade_trigger_record"
	sobjectParentProjectionField             = "__glade_parent_projection"
	sobjectPopulatedFieldsAliasContainsField = "__glade_populated_fields_alias_contains"
)

func isInternalSObjectField(field string) bool {
	return field == sobjectErrorsField || field == sobjectReadOnlyField || field == sobjectQueriedFieldsField || field == sobjectExplicitFieldsField || field == sobjectSetFieldsField || field == sobjectUserSetFieldsField || field == sobjectDMLOptionsField || field == sobjectDMLAccessibleField || field == sobjectTriggerField || field == sobjectParentProjectionField
}

func vmImplicitDMLField(field storage.Field) bool {
	if vmCalculatedOrSummaryField(field) {
		return true
	}
	return !storage.FieldFlagValue(field.Createable, true) || !storage.FieldFlagValue(field.Updateable, true)
}

func vmImplicitDMLFieldDefaultValue(field storage.Field, value Value) bool {
	if vmCalculatedOrSummaryField(field) {
		return false
	}
	switch value.Kind {
	case ValueNull:
		return true
	case ValueBool:
		return !value.Bool
	case ValueInt:
		return value.Int == 0
	case ValueDecimal:
		return value.Decimal == 0
	case ValueString:
		return value.Text == ""
	default:
		return false
	}
}

func vmCalculatedOrSummaryField(field storage.Field) bool {
	return field.Type == storage.FieldCalculated || field.Type == storage.FieldSummary || strings.TrimSpace(field.Formula) != ""
}

func vmImplicitGeneratedFieldValue(field string, value Value) bool {
	if !strings.EqualFold(field, "RecordTypeId") {
		return false
	}
	if value.Kind == ValueString {
		return strings.HasPrefix(value.Text, "012")
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		id, ok := sObjectIDFromValue(value)
		return ok && strings.HasPrefix(string(id), "012")
	}
	return false
}

func markExplicitSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || strings.TrimSpace(field) == "" {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		selected = Map()
		selected.Type = "Map<String,Boolean>"
	}
	keyValue := String(strings.ToLower(field))
	encoded := mapKey(keyValue)
	selected.Map[encoded] = Bool(true)
	if selected.MapKeys == nil {
		selected.MapKeys = make(map[string]Value)
	}
	selected.MapKeys[encoded] = keyValue
	filteredOrder := selected.MapOrder[:0]
	for _, key := range selected.MapOrder {
		if key == encoded {
			continue
		}
		filteredOrder = append(filteredOrder, key)
	}
	selected.MapOrder = append([]string{encoded}, filteredOrder...)
	value.Fields[sobjectExplicitFieldsField] = selected
}

func isExplicitSObjectField(value Value, field string) bool {
	field = strings.TrimSpace(field)
	if value.Fields == nil || field == "" {
		return false
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag, ok := selected.Map["string:"+strings.ToLower(field)]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func explicitSObjectFieldNames(value Value) []string {
	if value.Fields == nil {
		return nil
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		return nil
	}
	fields := make([]string, 0, len(selected.Map))
	for _, key := range orderedValueMapKeys(selected) {
		flag := selected.Map[key]
		if flag.Kind != ValueBool || !flag.Bool {
			continue
		}
		if strings.HasPrefix(key, "string:") {
			fields = append(fields, strings.TrimPrefix(key, "string:"))
		}
	}
	return fields
}

func unmarkExplicitSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		return
	}
	delete(selected.Map, mapKey(String(strings.ToLower(field))))
	if len(selected.Map) == 0 {
		delete(value.Fields, sobjectExplicitFieldsField)
		return
	}
	value.Fields[sobjectExplicitFieldsField] = selected
}

func markSetSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || strings.TrimSpace(field) == "" {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		selected = Map()
		selected.Type = "Map<String,Boolean>"
	}
	keyValue := String(strings.ToLower(field))
	encoded := mapKey(keyValue)
	selected.Map[encoded] = Bool(true)
	if selected.MapKeys == nil {
		selected.MapKeys = make(map[string]Value)
	}
	selected.MapKeys[encoded] = keyValue
	value.Fields[sobjectSetFieldsField] = selected
}

func markUserSetSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || strings.TrimSpace(field) == "" {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectUserSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		selected = Map()
		selected.Type = "Map<String,Boolean>"
	}
	keyValue := String(strings.ToLower(field))
	encoded := mapKey(keyValue)
	selected.Map[encoded] = Bool(true)
	if selected.MapKeys == nil {
		selected.MapKeys = make(map[string]Value)
	}
	selected.MapKeys[encoded] = keyValue
	value.Fields[sobjectUserSetFieldsField] = selected
}

func isUserSetSObjectField(value Value, field string) bool {
	if value.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	selected, ok := value.Fields[sobjectUserSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func isSetSObjectField(value Value, field string) bool {
	if value.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	selected, ok := value.Fields[sobjectSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func setExplicitSObjectField(value *Value, field string, fieldValue Value) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields[field] = fieldValue
	markExplicitSObjectField(value, field)
}

func markTriggerSObject(value *Value) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields[sobjectTriggerField] = Bool(true)
}

func isTriggerSObject(value Value) bool {
	if value.Kind != ValueObject || value.Fields == nil {
		return false
	}
	marker, ok := value.Fields[sobjectTriggerField]
	return ok && marker.Kind == ValueBool && marker.Bool
}

func isParentProjectionSObject(value Value) bool {
	if value.Kind != ValueObject || value.Fields == nil {
		return false
	}
	marker, ok := value.Fields[sobjectParentProjectionField]
	return ok && marker.Kind == ValueBool && marker.Bool
}

func queriedSObjectFieldsValue(objectName string, fields map[string]bool) Value {
	value := Map()
	value.Type = "Map<String,Boolean>"
	value.Map[mapKey(String("object"))] = String(objectName)
	value.MapKeys[mapKey(String("object"))] = String("object")
	value.Map[mapKey(String("id"))] = Bool(true)
	value.MapKeys[mapKey(String("id"))] = String("id")
	for field := range fields {
		value.Map[mapKey(String(field))] = Bool(true)
		value.MapKeys[mapKey(String(field))] = String(field)
	}
	return value
}

func markQueriedSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	selected, ok := value.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return
	}
	selected.Map[mapKey(String(strings.ToLower(field)))] = Bool(true)
	value.Fields[sobjectQueriedFieldsField] = selected
}

func unmarkQueriedSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	selected, ok := value.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return
	}
	delete(selected.Map, mapKey(String(strings.ToLower(field))))
	value.Fields[sobjectQueriedFieldsField] = selected
}

func dmlVisibleSObjectFields(value *Value) map[string]bool {
	fields := map[string]bool{"id": true, "lastmodifiedbyid": true}
	if value == nil || value.Kind != ValueObject {
		return fields
	}
	for field := range value.Fields {
		if isInternalSObjectField(field) || isSObjectSystemField(field) {
			continue
		}
		fields[strings.ToLower(field)] = true
	}
	return fields
}

func (vm *VM) unqueriedSObjectFieldError(receiver Value, field string, enforceDML bool) error {
	if receiver.Kind != ValueObject {
		return nil
	}
	if marker, ok := receiver.Fields[sobjectDMLAccessibleField]; ok && marker.Kind == ValueBool && marker.Bool {
		if _, _, exists := objectFieldValue(receiver, field); !exists {
			return nil
		}
	}
	selected, ok := receiver.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return nil
	}
	if vm.queriedSObjectFieldsIncludes(receiver, field) {
		return nil
	}
	if isExplicitSObjectField(receiver, field) {
		return nil
	}
	if vm.loadedChildRelationshipForField(receiver, field) {
		return nil
	}
	if vm.loadedParentRelationshipForField(receiver, field) {
		return nil
	}
	if vm.unqueriedLookupFieldCanDefaultNull(receiver, field) {
		return nil
	}
	if vm.unqueriedStoredMissingFieldCanDefaultNull(receiver, field) {
		return nil
	}
	if _, ok := vm.unqueriedStoredDefaultFieldValue(receiver, field); ok {
		return nil
	}
	if vm.unqueriedParentRelationshipCanDefaultNull(receiver, field) {
		return nil
	}
	if vm.unqueriedChildRelationshipOnParentProjectionCanDefaultEmpty(receiver, field) {
		return nil
	}
	if !enforceDML {
		if _, ok := vm.lazyChildRelationshipValue(receiver, field); ok {
			return nil
		}
	}
	return newExceptionError("SObjectException", fmt.Sprintf("SObject row was retrieved via SOQL without querying the requested field: %s.%s", receiver.Type, field))
}

func (vm *VM) unqueriedStoredDefaultFieldValue(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || receiver.Fields == nil {
		return Null, false
	}
	if marker, ok := receiver.Fields[sobjectDMLAccessibleField]; ok && marker.Kind == ValueBool && marker.Bool {
		return Null, false
	}
	objectName := receiver.Type
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		objectName = resolved
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	canonical := field
	if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
		canonical = resolved
	}
	fieldDef, ok := object.Definition.Fields[canonical]
	if !ok {
		return Null, false
	}
	if unqueriedStoredDefaultFieldMustRemainHidden(canonical) {
		return Null, false
	}
	if strings.TrimSpace(fieldDef.DefaultValue) == "" {
		return Null, false
	}
	record := storage.Record{Object: objectName, Fields: map[string]storage.Value{}}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return Null, false
	}
	stored, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return Null, false
	}
	record = stored
	defaultValue, ok := vm.defaultValueForRecordField(object.Definition, record, fieldDef)
	if !ok || defaultValue.Kind == storage.ValueNull {
		return Null, false
	}
	if rawRecordTypeDefaultStorageValue(defaultValue, fieldDef) {
		return Null, false
	}
	if record.HasExplicitNull(canonical) {
		return Null, false
	}
	if current, ok := record.GetField(canonical); ok && !storageValuesEqualForVM(fieldDef, current, defaultValue) {
		return Null, false
	}
	if _, fieldValue, exists := objectFieldValue(receiver, field); exists {
		current, err := storageValueFromVMForField(fieldValue, fieldDef.Type)
		if err != nil || !storageValuesEqualForVM(fieldDef, current, defaultValue) {
			return Null, false
		}
	}
	return vmValueFromStorage(defaultValue), true
}

func unqueriedStoredDefaultFieldMustRemainHidden(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "createddate", "createdbyid", "lastmodifieddate", "lastmodifiedbyid", "systemmodstamp", "isdeleted":
		return true
	default:
		return false
	}
}

func (vm *VM) unqueriedStoredMissingFieldCanDefaultNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || receiver.Fields == nil {
		return false
	}
	objectName := receiver.Type
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		objectName = resolved
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	canonical := field
	if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
		canonical = resolved
	}
	fieldDef, ok := object.Definition.Fields[canonical]
	if !ok {
		return false
	}
	if fieldDef.Type != storage.FieldReference {
		return false
	}
	if unqueriedStoredDefaultFieldMustRemainHidden(canonical) {
		return false
	}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return false
	}
	stored, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return false
	}
	if stored.HasExplicitNull(canonical) {
		return true
	}
	_, hasStored := stored.GetField(canonical)
	return !hasStored
}

func (vm *VM) unqueriedChildRelationshipOnParentProjectionCanDefaultEmpty(receiver Value, field string) bool {
	if receiver.Kind != ValueObject || receiver.Fields == nil {
		return false
	}
	marker, ok := receiver.Fields[sobjectParentProjectionField]
	if !ok || marker.Kind != ValueBool || !marker.Bool {
		return false
	}
	_, ok = vm.jsonSObjectChildRelationshipType(receiver.Type, field)
	return ok
}

func (vm *VM) lazyChildRelationshipValue(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || receiver.Fields == nil || strings.TrimSpace(receiver.Type) == "" || strings.TrimSpace(field) == "" {
		return Null, false
	}
	parentObject, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		parentObject = receiver.Type
	}
	lookup := vm.lazyChildRelationshipLookup(parentObject, field)
	if !lookup.OK {
		return Null, false
	}
	parentID := sObjectIDFromFields(receiver.Fields)
	children := []Value(nil)
	seen := map[storage.ID]bool{}
	for _, target := range lookup.Targets {
		childState, ok := vm.Org.Objects[target.ChildName]
		if !ok {
			continue
		}
		children = vm.appendLazyChildRelationshipRecords(children, seen, target.ChildName, childState, target.LookupField, parentID)
	}
	list := List(children...)
	list.Type = "List<" + lookup.ChildType + ">"
	return list, true
}

func (vm *VM) lazyChildRelationshipLookup(parentObject, field string) lazyChildRelationshipLookup {
	key := strings.ToLower(strings.TrimSpace(parentObject)) + "\x00" + strings.ToLower(strings.TrimSpace(field))
	if vm.lazyChildRelCache == nil {
		vm.lazyChildRelCache = make(map[string]lazyChildRelationshipLookup)
	}
	if cached, ok := vm.lazyChildRelCache[key]; ok {
		return cached
	}
	lookup := lazyChildRelationshipLookup{}
	childMatches := []string(nil)
	for childName, childState := range vm.Org.Objects {
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) || strings.TrimSpace(relation.Field) == "" {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" && canDeriveChildRelationshipName(parentObject, childName, relation) {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if childRelationshipName == "" || !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
				continue
			}
			childMatches = appendUniqueStringFold(childMatches, childName)
			lookup.Targets = append(lookup.Targets, lazyChildRelationshipTarget{ChildName: childName, LookupField: relation.Field})
		}
		for fieldName, fieldDef := range childState.Definition.Fields {
			if fieldDef.Type != storage.FieldReference || len(fieldDef.ReferenceTo) == 0 {
				continue
			}
			if !relationshipTargetsObject(storage.Relationship{ParentObjects: append([]string(nil), fieldDef.ReferenceTo...)}, parentObject) {
				continue
			}
			if fieldDef.APIName == "" {
				fieldDef.APIName = fieldName
			}
			for _, childRelationshipName := range vmFieldChildRelationshipNames(childState.Definition, fieldDef) {
				if !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
					continue
				}
				childMatches = appendUniqueStringFold(childMatches, childName)
				lookup.Targets = append(lookup.Targets, lazyChildRelationshipTarget{ChildName: childName, LookupField: fieldDef.APIName})
			}
		}
	}
	if childType := vm.bestChildRelationshipObject(childMatches); childType != "" {
		lookup.ChildType = childType
		lookup.Targets = lazyChildRelationshipTargetsForChild(lookup.Targets, childType)
	}
	lookup.OK = lookup.ChildType != ""
	vm.lazyChildRelCache[key] = lookup
	return lookup
}

func lazyChildRelationshipTargetsForChild(targets []lazyChildRelationshipTarget, childType string) []lazyChildRelationshipTarget {
	if childType == "" || len(targets) == 0 {
		return targets
	}
	out := targets[:0]
	for _, target := range targets {
		if strings.EqualFold(target.ChildName, childType) {
			out = append(out, target)
		}
	}
	return out
}

func (vm *VM) clearLazyChildRelationshipCache() {
	if vm.lazyChildRelCache != nil {
		vm.lazyChildRelCache = make(map[string]lazyChildRelationshipLookup)
	}
}

func (vm *VM) appendLazyChildRelationshipRecords(out []Value, seen map[storage.ID]bool, childName string, childState storage.ObjectState, lookupField string, parentID storage.ID) []Value {
	if parentID == "" {
		return out
	}
	ids := make([]string, 0, len(childState.Records))
	if indexed, ok := storage.LookupIndex(childState, lookupField, storage.IDValue(parentID)); ok && len(indexed) > 0 {
		for _, id := range indexed {
			ids = append(ids, string(id))
		}
	} else {
		for id, record := range childState.Records {
			if record.System.IsDeleted {
				continue
			}
			value, ok := record.GetField(lookupField)
			if !ok || !storage.IDsEqual(storageIDFromValue(value), parentID) {
				continue
			}
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	for _, idText := range ids {
		id := storage.ID(idText)
		if seen[id] {
			continue
		}
		record, ok := childState.Records[id]
		if !ok || record.System.IsDeleted {
			continue
		}
		record.Object = childName
		out = append(out, vm.vmValueFromRecord(record))
		seen[id] = true
	}
	return out
}

func (vm *VM) loadedChildRelationshipForField(receiver Value, field string) bool {
	_, _, ok := vm.loadedChildRelationshipValue(receiver, field)
	return ok
}

func (vm *VM) loadedChildRelationshipValue(receiver Value, field string) (string, Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" || strings.TrimSpace(field) == "" {
		return "", Null, false
	}
	parentObject, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		parentObject = receiver.Type
	}
	lookup := vm.loadedChildRelationshipLookup(receiver.Type, parentObject, field)
	if len(lookup.ChildRelationshipNames) == 0 {
		return "", Null, false
	}
	parentRelationshipExists := lookup.ParentRelationshipExists
	for _, candidate := range lookup.CandidateNames {
		if actualName, value, ok := objectFieldValue(receiver, candidate); ok && loadedChildRelationshipRuntimeValue(value) {
			if parentRelationshipExists && value.Kind == ValueNull {
				continue
			}
			return actualName, value, true
		}
	}
	for actualName, value := range receiver.Fields {
		if !loadedChildRelationshipRuntimeValue(value) {
			continue
		}
		for _, childRelationshipName := range lookup.ChildRelationshipNames {
			if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, actualName) {
				if parentRelationshipExists && value.Kind == ValueNull {
					continue
				}
				return actualName, value, true
			}
		}
	}
	return "", Null, false
}

func (vm *VM) loadedChildRelationshipLookup(receiverType, parentObject, field string) loadedChildRelationshipLookup {
	key := strings.ToLower(strings.TrimSpace(receiverType)) + "\x00" + strings.ToLower(strings.TrimSpace(parentObject)) + "\x00" + strings.ToLower(strings.TrimSpace(field))
	if vm.loadedChildRelCache == nil {
		vm.loadedChildRelCache = make(map[string]loadedChildRelationshipLookup)
	}
	if cached, ok := vm.loadedChildRelCache[key]; ok {
		return cached
	}
	lookup := loadedChildRelationshipLookup{
		ParentRelationshipExists: vm.sObjectParentRelationshipField(receiverType, field),
	}
	for childName, childState := range vm.Org.Objects {
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" && canDeriveChildRelationshipName(parentObject, childName, relation) {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if childRelationshipName == "" || !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
				continue
			}
			lookup.ChildRelationshipNames = appendUniqueStringFold(lookup.ChildRelationshipNames, childRelationshipName)
			if vm.Org.Namespace != "" {
				lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, storage.NamespaceTokenName(vm.Org.Namespace, childRelationshipName))
				lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, storage.NamespaceTokenName(vm.Org.Namespace, field))
			}
			lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, childRelationshipName)
			lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, field)
		}
	}
	vm.loadedChildRelCache[key] = lookup
	return lookup
}

func loadedChildRelationshipRuntimeValue(value Value) bool {
	return value.Kind == ValueList || value.Kind == ValueNull
}

func (vm *VM) unqueriedParentRelationshipCanDefaultNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	for name, fieldDef := range object.Definition.Fields {
		apiName := fieldDef.APIName
		if apiName == "" {
			apiName = name
		}
		if fieldDef.Type == storage.FieldReference && vmParentRelationshipNameMatches(vm.Org.Namespace, apiName, field) {
			return true
		}
	}
	return false
}

func (vm *VM) unqueriedLookupFieldCanDefaultNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	fieldName := vm.resolveSObjectFieldName(receiver.Type, field)
	fieldDef, ok := object.Definition.Fields[fieldName]
	return ok && fieldDef.Type == storage.FieldReference
}

func (vm *VM) loadedParentRelationshipForField(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	relationship, ok := vm.parentRelationshipNameForField(object.Definition, field)
	if !ok {
		if strings.HasSuffix(field, "__c") {
			relationship = strings.TrimSuffix(field, "__c") + "__r"
		} else if strings.HasSuffix(field, "Id") && len(field) > len("Id") {
			relationship = strings.TrimSuffix(field, "Id")
		}
	}
	if relationship == "" {
		return false
	}
	_, value, exists := objectFieldValue(receiver, relationship)
	return exists && value.Kind != ValueNull
}

func sobjectReadOnlyReason(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	reason, ok := value.Fields[sobjectReadOnlyField]
	if !ok || reason.Kind != ValueString || reason.Text == "" {
		return "", false
	}
	return reason.Text, true
}

func addSObjectError(value *Value, message string, fields []string) {
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	errorValue := Object("Database.Error")
	errorValue.Fields["message"] = String(message)
	errorValue.Fields["statusCode"] = String("FIELD_CUSTOM_VALIDATION_EXCEPTION")
	fieldsList := List()
	for _, field := range fields {
		fieldsList.List = append(fieldsList.List, String(field))
	}
	errorValue.Fields["fields"] = fieldsList
	errorsList, ok := value.Fields[sobjectErrorsField]
	if !ok || errorsList.Kind != ValueList {
		errorsList = List()
	}
	errorsList.List = append(errorsList.List, errorValue)
	value.Fields[sobjectErrorsField] = errorsList
}

func sobjectErrors(value Value) []Value {
	errorsList, ok := value.Fields[sobjectErrorsField]
	if !ok || errorsList.Kind != ValueList {
		return nil
	}
	return append([]Value(nil), errorsList.List...)
}

func dmlResultsFromSObjectErrors(records []storage.Record, values []Value) []dml.Result {
	results := make([]dml.Result, len(records))
	for i, value := range values {
		if i >= len(results) {
			break
		}
		errors := sobjectErrors(value)
		if len(errors) == 0 {
			continue
		}
		dmlErrors := make([]dml.Error, 0, len(errors))
		messages := make([]string, 0, len(errors))
		aggregateFields := make([]string, 0, len(errors))
		for _, errValue := range errors {
			dmlError := dml.Error{
				Message:    "record blocked by addError",
				StatusCode: "FIELD_CUSTOM_VALIDATION_EXCEPTION",
			}
			if errValue.Kind == ValueObject {
				if value, ok := errValue.Fields["message"]; ok {
					dmlError.Message = value.String()
				}
				if value, ok := errValue.Fields["statusCode"]; ok {
					dmlError.StatusCode = value.String()
				}
				if value, ok := errValue.Fields["fields"]; ok && value.Kind == ValueList {
					for _, field := range value.List {
						dmlError.Fields = append(dmlError.Fields, field.String())
					}
				}
			}
			messages = append(messages, dmlError.Message)
			aggregateFields = append(aggregateFields, dmlError.Fields...)
			dmlErrors = append(dmlErrors, dmlError)
		}
		results[i] = dml.Result{
			ID:         records[i].ID,
			Success:    false,
			Error:      strings.Join(messages, "; "),
			StatusCode: dmlErrors[0].StatusCode,
			Fields:     aggregateFields,
			Errors:     dmlErrors,
		}
	}
	return results
}

func databaseErrorsList(result dml.Result) Value {
	errors := dmlResultErrors(result)
	values := make([]Value, 0, len(errors))
	for _, err := range errors {
		values = append(values, databaseErrorValue(err))
	}
	return List(values...)
}

func dmlResultErrors(result dml.Result) []dml.Error {
	if len(result.Errors) > 0 {
		out := make([]dml.Error, len(result.Errors))
		copy(out, result.Errors)
		return out
	}
	if result.Error == "" {
		return nil
	}
	code := result.StatusCode
	if code == "" {
		code = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	}
	return []dml.Error{{
		Message:    result.Error,
		StatusCode: code,
		Fields:     append([]string(nil), result.Fields...),
	}}
}

func databaseErrorValue(err dml.Error) Value {
	value := Object("Database.Error")
	value.Fields["message"] = String(err.Message)
	code := err.StatusCode
	if code == "" {
		code = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	}
	value.Fields["statusCode"] = String(code)
	fields := List()
	for _, field := range err.Fields {
		fields.List = append(fields.List, String(field))
	}
	value.Fields["fields"] = fields
	value.Fields["extendedErrorDetails"] = List()
	return value
}

func dmlExceptionDetail(receiver Value, method string, args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("%s.%s expects Integer index", receiver.Type, method)
	}
	details, ok := receiver.Fields["__dmlErrors"]
	if !ok || details.Kind != ValueList {
		return Null, fmt.Errorf("%s.%s index out of bounds: %d", receiver.Type, method, args[0].Int)
	}
	index := int(args[0].Int)
	if index < 0 || index >= len(details.List) {
		return Null, fmt.Errorf("%s.%s index out of bounds: %d", receiver.Type, method, args[0].Int)
	}
	detail := details.List[index]
	if detail.Kind != ValueObject {
		return Null, fmt.Errorf("%s.%s detail is not available: %d", receiver.Type, method, args[0].Int)
	}
	return detail, nil
}

func (vm *VM) resolveObjectName(name string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return "", false
	}
	if prefix, rest, ok := strings.Cut(strings.TrimSpace(name), "."); ok && strings.EqualFold(prefix, "Schema") {
		name = rest
		key = strings.ToLower(strings.TrimSpace(name))
	}
	namespace := strings.ToLower(strings.TrimSpace(vm.currentCallerNamespace()))
	cacheKey := namespace + "|" + key
	if vm.objectNameCache == nil {
		vm.objectNameCache = make(map[string]objectNameLookup)
	}
	if cached, ok := vm.objectNameCache[cacheKey]; ok {
		return cached.Name, cached.OK
	}
	if resolved, ok := vm.resolveObjectNameInExecutionNamespace(name); ok {
		vm.objectNameCache[cacheKey] = objectNameLookup{Name: resolved, OK: true}
		return resolved, true
	}
	resolved, ok := storage.ResolveObjectName(*vm.Org, name)
	vm.objectNameCache[cacheKey] = objectNameLookup{Name: resolved, OK: ok}
	return resolved, ok
}

func (vm *VM) resolveObjectNameInExecutionNamespace(name string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	namespace := strings.TrimSpace(vm.currentCallerNamespace())
	if namespace == "" {
		return "", false
	}
	prefixed := storage.NamespaceTokenName(namespace, name)
	if prefixed == name {
		return "", false
	}
	if _, ok := vm.Org.Objects[prefixed]; ok {
		return prefixed, true
	}
	for candidate := range vm.Org.Objects {
		if strings.EqualFold(candidate, prefixed) {
			return candidate, true
		}
	}
	return "", false
}

func (vm *VM) resolveSObjectFieldName(typeName, field string) string {
	if vm.Org == nil {
		return field
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return storage.StripNamespaceToken(vm.Org.Namespace, field)
	}
	if dot := strings.LastIndex(field, "."); dot >= 0 && dot+1 < len(field) {
		prefix := field[:dot]
		if resolvedPrefix, prefixOK := vm.resolveObjectName(prefix); prefixOK && strings.EqualFold(resolvedPrefix, objectName) {
			field = field[dot+1:]
		}
	}
	if canonical, ok := storage.ResolveFieldName(vm.Org.Objects[objectName].Definition, vm.Org.Namespace, field); ok {
		return canonical
	}
	return storage.StripNamespaceToken(vm.Org.Namespace, field)
}

func (vm *VM) hasSObjectField(typeName, field string) bool {
	_, _, ok := vm.sObjectFieldDefinition(typeName, field)
	return ok
}

func dmlAccessibleSObject(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	marker, ok := value.Fields[sobjectDMLAccessibleField]
	return ok && marker.Kind == ValueBool && marker.Bool
}

func shouldEvaluateSObjectFormulaField(value Value, field storage.Field) bool {
	if strings.TrimSpace(field.Formula) == "" {
		return false
	}
	if !dmlAccessibleSObject(value) {
		return true
	}
	switch strings.ToUpper(field.DisplayType) {
	case "STRING", "TEXT", "TEXTAREA", "URL", "EMAIL", "PHONE":
		return true
	default:
		return false
	}
}

func (vm *VM) sObjectFieldDefinition(typeName, field string) (storage.ObjectDefinition, storage.Field, bool) {
	if vm.Org == nil {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	definition := vm.describePreparedDefinition(objectName, vm.Org.Objects[objectName].Definition)
	fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
	if !ok {
		if systemField, systemOK := syntheticSObjectSystemField(field); systemOK {
			return definition, systemField, true
		}
		if isCustomObjectLikeName(objectName) {
			if synthetic := syntheticSchemaField(field); synthetic.APIName != "" {
				return definition, synthetic, true
			}
		}
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	return definition, definition.Fields[fieldName], true
}

func (vm *VM) missingSObjectFieldValue(receiver Value, field string) (Value, bool) {
	if isExplicitSObjectField(receiver, field) {
		if _, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field); ok {
			if defaultValue, ok := storage.DefaultValueForField(fieldDef); ok {
				if rawRecordTypeDefaultStorageValue(defaultValue, fieldDef) {
					return storageFieldNullValue(fieldDef), true
				}
				return vmValueFromStorage(defaultValue), true
			}
			return storageFieldNullValue(fieldDef), true
		}
		return Null, true
	}
	if value, ok := vm.parentRelationshipValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.parentRelationshipValueFromLookupID(receiver, field); ok {
		return value, true
	}
	_, hasQueriedFields := receiver.Fields[sobjectQueriedFieldsField]
	if hasQueriedFields {
		if value, ok := vm.lazyChildRelationshipValue(receiver, field); ok {
			return value, true
		}
	}
	if relationshipType, ok := vm.jsonSObjectChildRelationshipType(receiver.Type, field); ok {
		children := List()
		children.Type = relationshipType
		return children, true
	}
	if !hasQueriedFields {
		if value, ok := vm.lazyChildRelationshipValue(receiver, field); ok {
			return value, true
		}
	}
	definition, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok {
		if value, ok := vm.sObjectCompoundAddressValueByPrefix(receiver, field); ok {
			return value, true
		}
		if value, ok := vm.parentRelationshipValue(receiver, field); ok {
			return value, true
		}
		return Null, false
	}
	if fieldDef.Type == storage.FieldCalculated {
		if shouldEvaluateSObjectFormulaField(receiver, fieldDef) {
			if record, ok := vm.formulaRecordFromSObject(receiver); ok {
				if value, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record); ok {
					formulaValue := vmValueFromStorage(value)
					if calculatedDateFormulaBlankValue(fieldDef, formulaValue) {
						return Null, true
					}
					return formulaValue, true
				}
			}
		}
		switch strings.ToUpper(fieldDef.DisplayType) {
		case "INTEGER":
			return Int(0), true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return Decimal(0), true
		case "BOOLEAN":
			return Bool(false), true
		default:
			return Null, true
		}
	}
	if fieldDef.Type == storage.FieldSummary {
		if dmlAccessibleSObject(receiver) {
			if value, ok := emptySummaryStorageValue(fieldDef); ok {
				return vmValueFromStorage(value), true
			}
			return storageFieldNullValue(fieldDef), true
		}
		if value, ok := vm.evaluateSummaryField(receiver, fieldDef); ok {
			return vmValueFromStorage(value), true
		}
		if value, ok := emptySummaryStorageValue(fieldDef); ok {
			return vmValueFromStorage(value), true
		}
		return storageFieldNullValue(fieldDef), true
	}
	if value, ok := vm.storedSObjectFieldValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.unqueriedStoredDefaultFieldValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.sObjectCompoundAddressValue(receiver, field); ok {
		return value, true
	}
	if fieldDef.Type == storage.FieldReference {
		if isExplicitSObjectField(receiver, field) {
			return Null, true
		}
		if value, ok := vm.lookupIDFromLoadedParentRelationship(receiver, definition, field); ok {
			return value, true
		}
	}
	if storage.IsCustomMetadataDefinition(definition) || storage.IsCustomSettingDefinition(definition) {
		if defaultValue, ok := storage.DefaultValueForField(fieldDef); ok {
			return vmValueFromStorage(defaultValue), true
		}
	}
	if fieldDef.Type == storage.FieldBoolean {
		if storage.IsCustomMetadataDefinition(definition) || storage.IsCustomSettingDefinition(definition) {
			return Value{Kind: ValueNull, Type: "Boolean"}, true
		}
		return Bool(false), true
	}
	return storageFieldNullValue(fieldDef), true
}

func (vm *VM) unknownSObjectFieldError(receiver Value, field string) error {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return nil
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return nil
	}
	definition := vm.describePreparedDefinition(objectName, vm.Org.Objects[objectName].Definition)
	if _, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field); ok {
		return nil
	}
	if _, ok := syntheticSObjectSystemField(field); ok {
		return nil
	}
	if isCustomObjectLikeName(objectName) {
		if synthetic := syntheticSchemaField(field); synthetic.APIName != "" {
			return nil
		}
	}
	return newExceptionError("SObjectException", fmt.Sprintf("Invalid field %s for %s", field, objectName))
}

func calculatedDateFormulaBlankValue(fieldDef storage.Field, value Value) bool {
	if !strings.EqualFold(fieldDef.DisplayType, "DATE") && !strings.EqualFold(fieldDef.DisplayType, "DATETIME") {
		return false
	}
	switch value.Kind {
	case ValueInt:
		return value.Int == 0
	case ValueDecimal:
		return value.Decimal == 0
	case ValueString:
		return strings.TrimSpace(value.Text) == "" || strings.TrimSpace(value.Text) == "0"
	case ValueObject:
		if !strings.EqualFold(value.Type, "Date") && !strings.EqualFold(value.Type, "Datetime") {
			return false
		}
		if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
			text := strings.TrimSpace(raw.Text)
			return text == "" || text == "0"
		}
		return false
	default:
		return false
	}
}

func (vm *VM) storedSObjectFieldValue(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	if marker, ok := receiver.Fields[sobjectTriggerField]; ok && marker.Kind == ValueBool && marker.Bool {
		return Null, false
	}
	if _, hasQueriedFields := receiver.Fields[sobjectQueriedFieldsField]; hasQueriedFields {
		return Null, false
	}
	if isExplicitSObjectField(receiver, "Id") && !isExplicitSObjectField(receiver, field) {
		_, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
		if !ok || strings.TrimSpace(fieldDef.DefaultValue) == "" {
			return Null, false
		}
	}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	record, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return Null, false
	}
	if record.HasExplicitNull(field) {
		if _, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field); ok {
			return storageFieldNullValue(fieldDef), true
		}
		return Null, true
	}
	if value, ok := record.GetField(field); ok {
		if _, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field); ok && rawRecordTypeDefaultStorageValue(value, fieldDef) {
			return storageFieldNullValue(fieldDef), true
		}
		return vmValueFromStorage(value), true
	}
	return Null, false
}

func (vm *VM) sObjectCompoundAddressValue(receiver Value, field string) (Value, bool) {
	definition, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok {
		return vm.sObjectCompoundAddressValueByPrefix(receiver, field)
	}
	if fieldDef.Type != storage.FieldAddress {
		return Null, false
	}
	address := Object("Address")
	for componentName, component := range definition.Fields {
		if !strings.EqualFold(component.CompoundFieldName, fieldDef.APIName) && !strings.EqualFold(component.CompoundFieldName, field) {
			continue
		}
		value, ok := vm.sObjectComponentFieldValue(receiver, componentName)
		if !ok || value.Kind == ValueNull {
			continue
		}
		addressField, ok := compoundAddressComponentField(componentName, fieldDef.APIName)
		if !ok {
			continue
		}
		address.Fields[addressField] = value
	}
	if len(address.Fields) == 0 {
		return vm.sObjectCompoundAddressValueByPrefix(receiver, field)
	}
	return address, true
}

func (vm *VM) sObjectCompoundAddressValueByPrefix(receiver Value, field string) (Value, bool) {
	prefix := strings.TrimSuffix(field, "Address")
	if prefix == field || prefix == "" {
		return Null, false
	}
	address := Object("Address")
	for _, component := range []struct {
		suffix string
		field  string
	}{
		{"Street", "street"},
		{"City", "city"},
		{"State", "state"},
		{"StateCode", "stateCode"},
		{"PostalCode", "postalCode"},
		{"Country", "country"},
		{"CountryCode", "countryCode"},
		{"Latitude", "latitude"},
		{"Longitude", "longitude"},
		{"GeocodeAccuracy", "geocodeAccuracy"},
	} {
		value, ok := vm.sObjectComponentFieldValue(receiver, prefix+component.suffix)
		if !ok || value.Kind == ValueNull {
			continue
		}
		address.Fields[component.field] = value
	}
	if len(address.Fields) == 0 {
		return Null, false
	}
	return address, true
}

func (vm *VM) sObjectComponentFieldValue(receiver Value, field string) (Value, bool) {
	if _, value, ok := objectFieldValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.storedSObjectFieldValueIgnoringProjection(receiver, field); ok {
		return value, true
	}
	return Null, false
}

func (vm *VM) storedSObjectFieldValueIgnoringProjection(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	record, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return Null, false
	}
	if record.HasExplicitNull(field) {
		return Null, true
	}
	if value, ok := record.GetField(field); ok {
		return vmValueFromStorage(value), true
	}
	return Null, false
}

func compoundAddressComponentField(componentName, compoundName string) (string, bool) {
	prefix := strings.TrimSuffix(compoundName, "Address")
	if prefix == "" || !hasPrefixFold(componentName, prefix) {
		return "", false
	}
	suffix := componentName[len(prefix):]
	switch strings.ToLower(suffix) {
	case "street":
		return "street", true
	case "city":
		return "city", true
	case "state":
		return "state", true
	case "statecode":
		return "stateCode", true
	case "postalcode":
		return "postalCode", true
	case "country":
		return "country", true
	case "countrycode":
		return "countryCode", true
	case "latitude":
		return "latitude", true
	case "longitude":
		return "longitude", true
	case "geocodeaccuracy":
		return "geocodeAccuracy", true
	default:
		return "", false
	}
}

func (vm *VM) emptyParentRelationshipShell(receiver Value, field string, relationship Value) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || relationship.Kind != ValueObject {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	isParentRelationship := false
	for _, relation := range object.Definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, field) ||
			vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, field) {
			isParentRelationship = true
			break
		}
	}
	if !isParentRelationship {
		return false
	}
	for fieldName, value := range relationship.Fields {
		if isInternalSObjectField(fieldName) {
			continue
		}
		if value.Kind != ValueNull {
			return false
		}
	}
	return true
}

func (vm *VM) lookupIDFromLoadedParentRelationship(receiver Value, definition storage.ObjectDefinition, field string) (Value, bool) {
	relationship, ok := vm.parentRelationshipNameForField(definition, field)
	if !ok {
		if strings.HasSuffix(field, "__c") {
			relationship = strings.TrimSuffix(field, "__c") + "__r"
		} else if strings.HasSuffix(field, "Id") && len(field) > len("Id") {
			relationship = strings.TrimSuffix(field, "Id")
		}
	}
	if relationship == "" {
		return Null, false
	}
	if !vm.queriedSObjectFieldsIncludes(receiver, field) && !vm.queriedSObjectFieldsIncludes(receiver, relationship) {
		return Null, false
	}
	relationshipAliases := uniqueNonEmptyStrings([]string{relationship, lookupFieldRelationshipName(field)})
	var parent Value
	for _, alias := range relationshipAliases {
		if isExplicitSObjectField(receiver, alias) && !isExplicitSObjectField(receiver, field) {
			return Null, false
		}
		actualName, candidate, candidateOK := objectFieldValue(receiver, alias)
		if candidateOK && candidate.Kind == ValueObject && parent.Kind == "" {
			if isExplicitSObjectField(receiver, actualName) && !isExplicitSObjectField(receiver, field) {
				return Null, false
			}
			parent = candidate
		}
	}
	ok = parent.Kind == ValueObject
	if !ok || parent.Kind != ValueObject {
		return Null, false
	}
	if id := sObjectIDFromFields(parent.Fields); id != "" {
		return String(string(id)), true
	}
	return Null, false
}

func (vm *VM) queriedSObjectFieldsIncludes(receiver Value, field string) bool {
	selected, ok := receiver.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	if queriedSObjectFieldsMapIncludes(selected, field) {
		return true
	}
	objectName := receiver.Type
	if rawObject, ok := selected.Map[mapKey(String("object"))]; ok && rawObject.Kind == ValueString {
		objectName = rawObject.Text
	}
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" {
		return false
	}
	if canonicalObject, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonicalObject
	}
	if queriedSObjectFieldsMapIncludes(selected, storage.NamespaceTokenName(vm.Org.Namespace, field)) {
		return true
	}
	if vm.Org.Namespace != "" && queriedSObjectFieldsMapIncludes(selected, storage.StripNamespaceToken(vm.Org.Namespace, field)) {
		return true
	}
	if object, ok := vm.Org.Objects[objectName]; ok {
		if storage.IsCustomMetadataDefinition(object.Definition) && customMetadataSyntheticFieldCanDefaultNull(field) {
			return true
		}
		if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
			if storage.IsCustomMetadataDefinition(object.Definition) && customMetadataSyntheticFieldCanDefaultNull(canonical) {
				return true
			}
			return queriedSObjectFieldsMapIncludes(selected, canonical) ||
				queriedSObjectFieldsMapIncludes(selected, storage.NamespaceTokenName(vm.Org.Namespace, canonical)) ||
				queriedSObjectFieldsMapIncludes(selected, storage.StripAnyNamespaceToken(canonical)) ||
				queriedSObjectFieldsMapIncludes(selected, storage.StripAnyNamespaceToken(field)) ||
				(vm.Org.Namespace != "" && queriedSObjectFieldsMapIncludes(selected, storage.StripNamespaceToken(vm.Org.Namespace, canonical)))
		}
	}
	return false
}

func customMetadataSyntheticFieldCanDefaultNull(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "name", "ownerid", "isdeleted", "createddate", "createdbyid", "lastmodifieddate", "lastmodifiedbyid", "systemmodstamp":
		return true
	default:
		return false
	}
}

func queriedSObjectFieldsMapIncludes(selected Value, field string) bool {
	_, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok
}

func (vm *VM) parentRelationshipValue(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		return vm.parentRelationshipValueForRelationName(receiver, relation, relationshipName), true
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		return vm.parentRelationshipValueForRelationName(receiver, relation, relationshipName), true
	}
	return Null, false
}

func (vm *VM) hydrateQueriedRecordTypeRelationships(value Value) {
	if vm == nil || vm.Org == nil || value.Kind != ValueObject {
		return
	}
	objectName, ok := vm.resolveObjectName(value.Type)
	if !ok {
		objectName = value.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return
	}
	for _, relation := range object.Definition.Relations {
		if !relationReferencesObject(relation, "RecordType") ||
			!vm.queriedSObjectFieldsIncludes(value, relation.ParentRelationship) {
			continue
		}
		_, lookupValue, lookupOK := objectFieldValue(value, relation.Field)
		if (!lookupOK || lookupValue.Kind == ValueNull) && value.Kind == ValueObject {
			if storedValue, storedOK := vm.storedSObjectFieldValueIgnoringProjection(value, relation.Field); storedOK {
				lookupValue = storedValue
				lookupOK = true
			}
		}
		if !lookupOK || lookupValue.Kind == ValueNull {
			continue
		}
		lookupID, idOK := sObjectIDFromValue(lookupValue)
		if !idOK {
			continue
		}
		if recordType, ok := vm.recordTypeRelationshipValue(object.Definition, lookupID); ok {
			value.Fields[relation.ParentRelationship] = recordType
		}
	}
}

func (vm *VM) parentRelationshipValueForRelation(receiver Value, relation storage.Relationship) Value {
	return vm.parentRelationshipValueForRelationName(receiver, relation, relation.ParentRelationship)
}

func (vm *VM) parentRelationshipValueForRelationName(receiver Value, relation storage.Relationship, relationshipName string) Value {
	for _, alias := range parentRelationshipFieldAliases(relation, relationshipName) {
		if _, relationship, ok := objectFieldValue(receiver, alias); ok && relationship.Kind == ValueObject {
			if !vm.isSObjectLikeType(relationship.Type) && len(relation.ParentObjects) > 0 {
				relationship.Type = relation.ParentObjects[0]
			}
			return relationship
		}
	}
	if _, relationship, ok := objectFieldValue(receiver, relation.ParentRelationship); ok && relationship.Kind == ValueObject {
		if !vm.isSObjectLikeType(relationship.Type) && len(relation.ParentObjects) > 0 {
			relationship.Type = relation.ParentObjects[0]
		}
		return relationship
	}
	_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
	if (!ok || lookupValue.Kind == ValueNull) && receiver.Kind == ValueObject {
		if storedValue, storedOK := vm.storedSObjectFieldValueIgnoringProjection(receiver, relation.Field); storedOK {
			lookupValue = storedValue
			ok = true
		}
	}
	if !ok || lookupValue.Kind == ValueNull {
		return vm.parentRelationshipTypedNull(relation)
	}
	if relationReferencesObject(relation, "RecordType") && !vm.queriedSObjectFieldsIncludes(receiver, relation.ParentRelationship) {
		return vm.parentRelationshipTypedNull(relation)
	}
	return vm.parentRelationshipTypedNull(relation)
}

func parentRelationshipFieldAliases(relation storage.Relationship, relationshipName string) []string {
	aliases := []string{relationshipName, relation.ParentRelationship, lookupFieldRelationshipName(relation.Field)}
	return uniqueNonEmptyStrings(aliases)
}

func (vm *VM) syntheticParentRelationship(definition storage.ObjectDefinition, relationshipName string) (storage.Relationship, bool) {
	if strings.EqualFold(definition.APIName, "RelationshipDomain") {
		switch strings.ToLower(strings.TrimSpace(relationshipName)) {
		case "childsobject":
			return storage.Relationship{Field: "ChildSobjectId", ParentObjects: []string{"EntityDefinition"}, ParentRelationship: "ChildSobject"}, true
		case "field":
			return storage.Relationship{Field: "FieldId", ParentObjects: []string{"FieldDefinition"}, ParentRelationship: "Field"}, true
		}
	}
	for name, field := range definition.Fields {
		apiName := field.APIName
		if apiName == "" {
			apiName = name
		}
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		if !vmParentRelationshipNameMatches(vm.Org.Namespace, apiName, relationshipName) {
			continue
		}
		parentRelationship := vm.parentRelationshipNameForReferenceField(definition, field)
		return storage.Relationship{Field: apiName, ParentObjects: append([]string(nil), field.ReferenceTo...), ParentRelationship: parentRelationship, Polymorphic: len(field.ReferenceTo) > 1}, true
	}
	for _, fieldName := range []string{relationshipName + "Id", lookupFieldRelationshipName(relationshipName)} {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" {
			continue
		}
		_, field, ok := vm.sObjectFieldDefinition(definition.APIName, fieldName)
		if !ok || field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		parentRelationship := vm.parentRelationshipNameForReferenceField(definition, field)
		if !vmRelationshipNameMatches(vm.Org.Namespace, parentRelationship, relationshipName) {
			continue
		}
		return storage.Relationship{Field: field.APIName, ParentObjects: append([]string(nil), field.ReferenceTo...), ParentRelationship: parentRelationship, Polymorphic: len(field.ReferenceTo) > 1}, true
	}
	return storage.Relationship{}, false
}

func (vm *VM) parentRelationshipValueFromLookupID(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		if marker, ok := receiver.Fields[sobjectDMLAccessibleField]; ok && marker.Kind == ValueBool && marker.Bool {
			return vm.parentRelationshipTypedNull(relation), true
		}
		_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
		if (!ok || lookupValue.Kind == ValueNull) && receiver.Kind == ValueObject {
			if storedValue, storedOK := vm.storedSObjectFieldValueIgnoringProjection(receiver, relation.Field); storedOK {
				lookupValue = storedValue
				ok = true
			}
		}
		if !ok || lookupValue.Kind == ValueNull {
			return Null, false
		}
		if sObjectIDFromFields(receiver.Fields) == "" {
			return Null, false
		}
		return vm.parentRelationshipShellFromLookupID(relation, lookupValue)
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
		if !ok || lookupValue.Kind == ValueNull {
			return Null, false
		}
		return vm.parentRelationshipShellFromLookupID(relation, lookupValue)
	}
	return Null, false
}

func (vm *VM) parentRelationshipShellFromLookupID(relation storage.Relationship, lookupValue Value) (Value, bool) {
	if vm == nil || vm.Org == nil {
		return Null, false
	}
	lookupID, ok := sObjectIDFromValue(lookupValue)
	if !ok || lookupID == "" {
		return Null, false
	}
	for _, parentName := range relation.ParentObjects {
		parentObject, ok := vm.resolveObjectName(parentName)
		if !ok {
			parentObject = parentName
		}
		if strings.TrimSpace(parentObject) == "" {
			continue
		}
		parent := Object(parentObject)
		parent.Fields["Id"] = platformScalar("Id", string(lookupID))
		if strings.EqualFold(parentObject, "User") && storage.ID(vm.currentUserInfoField("Id", "")) == lookupID {
			parent.Fields["Name"] = String(vm.currentUserInfoField("Name", "Test User"))
		}
		return parent, true
	}
	return Null, false
}

func (vm *VM) recordTypeRelationshipValue(definition storage.ObjectDefinition, id storage.ID) (Value, bool) {
	if id == "" {
		return Null, false
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && !apexIDTextEqual(string(recordType.ID), string(id)) {
			continue
		}
		value := Object("RecordType")
		value.Fields["Id"] = platformScalar("Id", string(id))
		name := recordType.Name
		if name == "" {
			name = recordType.DeveloperName
		}
		value.Fields["Name"] = String(name)
		value.Fields["DeveloperName"] = String(recordType.DeveloperName)
		value.Fields["SObjectType"] = String(definition.APIName)
		return value, true
	}
	return Null, false
}

func (vm *VM) parentRelationshipTypedNull(relation storage.Relationship) Value {
	if vm == nil || vm.Org == nil || len(relation.ParentObjects) == 0 {
		return Null
	}
	parentObject, ok := vm.resolveObjectName(relation.ParentObjects[0])
	if !ok {
		parentObject = relation.ParentObjects[0]
	}
	value := Null
	value.Type = parentObject
	value.Runtime = relationshipNullRuntime
	return value
}

func (vm *VM) parentRelationshipTypedShell(relation storage.Relationship) Value {
	if vm == nil || vm.Org == nil || len(relation.ParentObjects) == 0 {
		return Null
	}
	parentObject, ok := vm.resolveObjectName(relation.ParentObjects[0])
	if !ok {
		parentObject = relation.ParentObjects[0]
	}
	if strings.TrimSpace(parentObject) == "" {
		return Null
	}
	return Object(parentObject)
}

func (vm *VM) parentRelationshipShell(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		for _, parentName := range relation.ParentObjects {
			parentObject, ok := vm.resolveObjectName(parentName)
			if !ok {
				parentObject = parentName
			}
			return Object(parentObject), true
		}
		return Null, false
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		for _, parentName := range relation.ParentObjects {
			parentObject, ok := vm.resolveObjectName(parentName)
			if !ok {
				parentObject = parentName
			}
			return Object(parentObject), true
		}
		return Null, false
	}
	return Null, false
}

func (vm *VM) findOrgRecord(objectName string, id storage.ID) (storage.Record, bool) {
	if vm == nil || vm.Org == nil || id == "" {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	if record, ok := object.Records[id]; ok {
		return record, true
	}
	for candidateID, record := range object.Records {
		if apexIDTextEqual(string(candidateID), string(id)) {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) formulaRecordFromSObject(value Value) (storage.Record, bool) {
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return storage.Record{}, false
	}
	if vm.Org == nil || record.ID == "" {
		return record, true
	}
	objectName, ok := vm.resolveObjectName(record.Object)
	if !ok {
		return record, true
	}
	if persisted, ok := vm.Org.Objects[objectName].Records[record.ID]; ok {
		for field, fieldValue := range persisted.Fields {
			if _, exists := record.GetField(field); !exists && !record.HasExplicitNull(field) {
				record.Fields[field] = fieldValue.Clone()
			}
		}
	}
	return record, true
}

func (vm *VM) evaluateSummaryField(receiver Value, fieldDef storage.Field) (storage.Value, bool) {
	operation := strings.ToLower(strings.TrimSpace(fieldDef.SummaryOperation))
	if vm.Org == nil || (operation != "sum" && operation != "count") {
		return storage.Value{}, false
	}
	parent, ok := vm.formulaRecordFromSObject(receiver)
	if !ok || parent.ID == "" {
		return storage.Value{}, false
	}
	childObject, childField := splitQualifiedField(fieldDef.SummarizedField)
	fkObject, fkField := splitQualifiedField(fieldDef.SummaryForeignKey)
	if childObject == "" && operation == "count" {
		childObject = fkObject
	}
	if childObject == "" || fkObject == "" || fkField == "" || !strings.EqualFold(childObject, fkObject) {
		return storage.Value{}, false
	}
	if operation != "count" && childField == "" {
		return storage.Value{}, false
	}
	canonicalChild, ok := vm.resolveObjectName(childObject)
	if !ok {
		return storage.Value{}, false
	}
	childState := vm.Org.Objects[canonicalChild]
	childFieldName := ""
	if childField != "" {
		var ok bool
		childFieldName, ok = storage.ResolveFieldName(childState.Definition, vm.Org.Namespace, childField)
		if !ok {
			return storage.Value{}, false
		}
	}
	fkFieldName, ok := storage.ResolveFieldName(childState.Definition, vm.Org.Namespace, fkField)
	if !ok {
		return storage.Value{}, false
	}
	count := int64(0)
	total := 0.0
	matched := false
	for _, child := range childState.Records {
		if child.System.IsDeleted {
			continue
		}
		if !apexIDTextEqual(storageValueIDText(child.Fields[fkFieldName]), string(parent.ID)) {
			continue
		}
		if !vm.summaryFiltersMatch(childState.Definition, child, fieldDef.SummaryFilterItems) {
			continue
		}
		if operation == "count" {
			count++
			matched = true
			continue
		}
		value, ok := vm.summaryRecordFieldValue(childState.Definition, child, childFieldName)
		if !ok {
			continue
		}
		number, ok := storageNumericValue(value)
		if !ok {
			continue
		}
		total += number
		matched = true
	}
	if operation == "count" {
		return storage.IntegerValue(count), true
	}
	if !matched {
		return storage.DecimalValue("0"), true
	}
	return storage.DecimalValue(strconv.FormatFloat(total, 'f', -1, 64)), true
}

func emptySummaryStorageValue(fieldDef storage.Field) (storage.Value, bool) {
	switch strings.ToLower(strings.TrimSpace(fieldDef.SummaryOperation)) {
	case "count":
		return storage.IntegerValue(0), true
	case "sum":
		return storage.DecimalValue("0"), true
	default:
		return storage.Value{}, false
	}
}

func splitQualifiedField(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "", name
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func (vm *VM) summaryFiltersMatch(definition storage.ObjectDefinition, record storage.Record, filters []storage.SummaryFilterItem) bool {
	for _, filter := range filters {
		_, fieldName := splitQualifiedField(filter.Field)
		if fieldName == "" {
			fieldName = filter.Field
		}
		canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
		if !ok {
			return false
		}
		value, ok := vm.summaryRecordFieldValue(definition, record, canonical)
		if !ok {
			value = storage.NullValue()
		}
		if !summaryFilterMatches(value, filter) {
			return false
		}
	}
	return true
}

func (vm *VM) summaryRecordFieldValue(definition storage.ObjectDefinition, record storage.Record, fieldName string) (storage.Value, bool) {
	if value, ok := record.GetField(fieldName); ok {
		return value, true
	}
	fieldDef, ok := definition.Fields[fieldName]
	if !ok || fieldDef.Type != storage.FieldCalculated || strings.TrimSpace(fieldDef.Formula) == "" {
		return storage.Value{}, false
	}
	value, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record)
	return value, ok
}

func summaryFilterMatches(value storage.Value, filter storage.SummaryFilterItem) bool {
	switch strings.ToLower(strings.TrimSpace(filter.Operation)) {
	case "", "equals":
		return storageValueMatchesText(value, filter.Value)
	default:
		return false
	}
}

func storageValueMatchesText(value storage.Value, text string) bool {
	text = strings.TrimSpace(text)
	switch value.Kind {
	case storage.ValueBoolean:
		return strings.EqualFold(strconv.FormatBool(value.Boolean), text)
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	case storage.ValueID:
		return apexIDTextEqual(string(value.ID), text)
	case storage.ValueInteger:
		parsed, err := strconv.ParseInt(text, 10, 64)
		return err == nil && value.Integer == parsed
	case storage.ValueDecimal:
		return strings.TrimRight(strings.TrimRight(value.Decimal, "0"), ".") == strings.TrimRight(strings.TrimRight(text, "0"), ".")
	case storage.ValueNull:
		return strings.EqualFold(text, "null") || text == ""
	default:
		return false
	}
}

func storageNumericValue(value storage.Value) (float64, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		return parsed, err == nil
	case storage.ValueString:
		parsed, err := strconv.ParseFloat(value.String, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (vm *VM) sObjectFieldArg(receiverType string, value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && isSObjectFieldTokenType(value.Type) {
		if objectValue, ok := value.Fields["object"]; ok && objectValue.Kind == ValueString && receiverType != "" && !strings.EqualFold(receiverType, "SObject") {
			if vm.Org != nil {
				if receiverObject, ok := vm.resolveObjectName(receiverType); ok {
					if tokenObject, ok := vm.resolveObjectName(objectValue.Text); ok && !strings.EqualFold(tokenObject, receiverObject) {
						return "", fmt.Errorf("%w: field token belongs to %s, not %s", errSObjectFieldTokenWrongObject, objectValue.Text, receiverType)
					}
				}
			}
		}
		field, ok := value.Fields["field"]
		if !ok || field.Kind != ValueString {
			return "", fmt.Errorf("field token missing field name")
		}
		return field.Text, nil
	}
	if value.Kind == ValueNull && isSObjectFieldTokenType(value.Type) {
		return "", errSObjectFieldTokenNull
	}
	return "", fmt.Errorf("expected field name")
}

var errSObjectFieldTokenWrongObject = errors.New("field token belongs to another SObject type")

var errSObjectFieldTokenNull = errors.New("field token is null")

func isSObjectFieldTokenType(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.SObjectField") || strings.EqualFold(typeName, "SObjectField")
}
