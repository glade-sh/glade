package vm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) callSObjectFieldAddError(path []string, args []Value) (Value, bool, error) {
	if len(path) != 2 {
		return Null, false, nil
	}
	root, ok := vm.Globals[path[0]]
	if !ok {
		lookedUp, err := vm.lookup(path[0])
		if err == nil {
			root = lookedUp
			ok = true
		}
	}
	if !ok || root.Kind != ValueObject || !vm.isSObjectType(root.Type) {
		return Null, false, nil
	}
	field := vm.resolveSObjectFieldName(root.Type, path[1])
	if !vm.sObjectFieldExists(root.Type, field) {
		return Null, false, nil
	}
	message, err := sObjectAddErrorMessage(args, "SObject field addError")
	if err != nil {
		return Null, true, err
	}
	addSObjectError(&root, message, []string{field})
	if err := vm.storeReceiver(path[0], root); err != nil {
		return Null, true, err
	}
	return Null, true, nil
}
func listAppendSObjects(receiver Value, field string, args []Value, context string) (Value, error) {
	if len(args) != 1 || (args[0].Kind != ValueObject && args[0].Kind != ValueList) {
		return Null, fmt.Errorf("%s expects SObject or List<SObject>", context)
	}
	values := []Value{args[0]}
	if args[0].Kind == ValueList {
		values = args[0].List
	}
	if receiver.Fields == nil {
		receiver.Fields = make(map[string]Value)
	}
	list := receiver.Fields[field]
	if list.Kind != ValueList {
		list = List()
		list.Type = "List<SObject>"
	}
	for _, value := range values {
		if value.Kind != ValueObject || !listRelationshipSObjectValue(value) {
			return Null, fmt.Errorf("%s expects SObject or List<SObject>", context)
		}
		list.List = append(list.List, value)
	}
	receiver.Fields[field] = list
	return receiver, nil
}
func listRelationshipSObjectValue(value Value) bool {
	return strings.EqualFold(value.Type, "SObject") ||
		isCommonSObjectTypeName(value.Type) ||
		strings.HasSuffix(value.Type, "__c") ||
		strings.HasSuffix(value.Type, "__e") ||
		strings.HasSuffix(value.Type, "__mdt")
}
func sObjectAddErrorMessage(args []Value, name string) (string, error) {
	message, _, err := sObjectAddErrorArgs(args, name)
	return message, err
}
func sObjectAddErrorArgs(args []Value, name string) (string, []string, error) {
	if len(args) < 1 || len(args) > 3 {
		return "", nil, fmt.Errorf("%s expects message, optional field, and optional escapeHtml", name)
	}
	messageArg := args[0]
	fields := []string(nil)
	escapeIndex := 1
	if len(args) >= 2 && args[1].Kind != ValueBool {
		fieldName, ok := sObjectAddErrorFieldName(args[0])
		if !ok {
			return "", nil, fmt.Errorf("%s field expects String or Schema.SObjectField", name)
		}
		fields = []string{fieldName}
		messageArg = args[1]
		escapeIndex = 2
	}
	if len(args) > escapeIndex && args[escapeIndex].Kind != ValueBool {
		return "", nil, fmt.Errorf("%s escapeHtml expects Boolean", name)
	}
	message := messageArg.String()
	if messageArg.Kind == ValueObject {
		if value, ok := messageArg.Fields["message"]; ok {
			message = value.String()
		}
	}
	return message, fields, nil
}
func sObjectAddErrorFieldName(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		return value.Text, true
	case ValueObject:
		if strings.EqualFold(value.Type, "Schema.SObjectField") {
			if field, ok := value.Fields["field"]; ok && field.Kind == ValueString {
				return field.Text, true
			}
			if name, ok := value.Fields["Name"]; ok && name.Kind == ValueString {
				return name.Text, true
			}
			if name, ok := value.Fields["name"]; ok && name.Kind == ValueString {
				return name.Text, true
			}
		}
	}
	return "", false
}
func (vm *VM) sObjectFieldExists(typeName, field string) bool {
	if vm.Org == nil {
		return true
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return true
	}
	if field == "Id" {
		return true
	}
	if _, ok = storage.ResolveFieldName(vm.Org.Objects[objectName].Definition, vm.Org.Namespace, field); ok {
		return true
	}
	return isCustomObjectLikeName(objectName) && isCustomFieldOrRelationshipType(field)
}
func (vm *VM) userClassShadowsSObjectType(typeName string) bool {
	class, ok := vm.lookupClass(typeName)
	if !ok || class.SuperClass == "" {
		return false
	}
	return !strings.EqualFold(class.SuperClass, "SObject")
}
func (vm *VM) sObjectFieldMapKeyIsChildRelationship(objectName, fieldName string) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(fieldName) == "" {
		return false
	}
	aliases := vm.sObjectFieldMapLookupAliases(fieldName)
	for _, relationship := range vm.describeChildRelationships(objectName) {
		if relationship.Kind != ValueObject {
			continue
		}
		name, ok := relationship.Fields["relationshipName"]
		if !ok || name.Kind != ValueString || strings.TrimSpace(name.Text) == "" {
			continue
		}
		for _, alias := range aliases {
			if vmRelationshipNameMatches(vm.Org.Namespace, name.Text, alias) {
				return true
			}
		}
	}
	return false
}
func (vm *VM) sObjectFieldMapDirectValueMatchesKey(receiver Value, encodedKey, fieldName string) bool {
	value, ok := receiver.Map[encodedKey]
	if !ok {
		return false
	}
	canonical, ok := sObjectFieldMapCanonicalFieldName(value)
	if !ok {
		return false
	}
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	for _, alias := range vm.sObjectFieldMapLookupAliases(fieldName) {
		if schemaSObjectFieldMapKeyMatches(namespace, canonical, alias) {
			return true
		}
	}
	return false
}
func (vm *VM) canSynthesizeSObjectFieldMapField(objectName string) bool {
	if vm == nil || vm.Org == nil {
		return true
	}
	canonical, ok := vm.resolveObjectName(objectName)
	if !ok {
		canonical = objectName
	}
	state, ok := vm.Org.Objects[canonical]
	if !ok {
		return true
	}
	return len(state.Definition.Fields) == 0 || isCustomObjectLikeName(state.Definition.APIName)
}
func (vm *VM) canSynthesizeCustomSObjectFieldMapField(objectName, fieldName string) bool {
	if lookupField, ok := customRelationshipLookupFieldName(fieldName); ok {
		if vm != nil && vm.Org != nil {
			return vm.inferredCustomFieldReferenceTarget(objectName, lookupField) != ""
		}
		return false
	}
	if !isCustomFieldOrRelationshipType(fieldName) {
		return false
	}
	if vm != nil && vm.Org != nil {
		if target := vm.inferredCustomFieldReferenceTarget(objectName, fieldName); target != "" {
			return true
		}
	}
	return isLikelyNumericCustomField(fieldName)
}
func sObjectFieldMapObjectName(value Value) (string, bool) {
	const prefix = "sobjectfieldmap:"
	if !strings.HasPrefix(value.Runtime, prefix) {
		return "", false
	}
	objectName := strings.TrimSpace(strings.TrimPrefix(value.Runtime, prefix))
	return objectName, objectName != ""
}
func isSObjectFieldMapValue(value Value) bool {
	if value.Kind != ValueMap {
		return false
	}
	if _, ok := sObjectFieldMapObjectName(value); ok {
		return true
	}
	return value.Type == "Schema.SObjectFieldMap"
}
func (vm *VM) sObjectFieldMapCanonicalKeySet(value Value) (Value, bool) {
	if !isSObjectFieldMapValue(value) {
		return Null, false
	}
	out := Set()
	out.Type = "Set<String>"
	seen := map[string]bool{}
	for _, rawKey := range orderedValueMapKeys(value) {
		fieldName, ok := sObjectFieldMapCanonicalFieldName(value.Map[rawKey])
		if !ok {
			continue
		}
		key := strings.ToLower(fieldName)
		if seen[key] {
			continue
		}
		seen[key] = true
		if vm != nil {
			fieldName = vm.describeFieldName(fieldName)
		}
		out.Set = append(out.Set, String(strings.ToLower(fieldName)))
	}
	return out, true
}
func sObjectFieldMapCanonicalValues(value Value) (Value, bool) {
	if !isSObjectFieldMapValue(value) {
		return Null, false
	}
	out := List()
	out.Type = "List<Schema.SObjectField>"
	seen := map[string]bool{}
	for _, rawKey := range orderedValueMapKeys(value) {
		item := value.Map[rawKey]
		fieldName, ok := sObjectFieldMapCanonicalFieldName(item)
		if !ok {
			continue
		}
		key := strings.ToLower(fieldName)
		if seen[key] {
			continue
		}
		seen[key] = true
		out.List = append(out.List, item)
	}
	return out, true
}
func sObjectFieldMapCanonicalSize(value Value) (int, bool) {
	if !isSObjectFieldMapValue(value) {
		return 0, false
	}
	seen := map[string]bool{}
	for _, item := range value.Map {
		fieldName, ok := sObjectFieldMapCanonicalFieldName(item)
		if !ok {
			continue
		}
		seen[strings.ToLower(fieldName)] = true
	}
	return len(seen), true
}
func sObjectFieldMapCanonicalFieldName(value Value) (string, bool) {
	if value.Kind != ValueObject || !strings.EqualFold(value.Type, "Schema.SObjectField") {
		return "", false
	}
	field, ok := value.Fields["field"]
	if !ok || field.Kind != ValueString || strings.TrimSpace(field.Text) == "" {
		return "", false
	}
	return field.Text, true
}
func schemaSObjectFieldMapKeyMatches(namespace, canonical, candidate string) bool {
	if schemaDescribeMapKeyMatches(namespace, canonical, candidate) {
		return true
	}
	if dot := strings.LastIndex(candidate, "."); dot >= 0 && dot+1 < len(candidate) {
		return schemaDescribeMapKeyMatches(namespace, canonical, candidate[dot+1:])
	}
	return false
}
func mapContainsOnlySObjectFieldTokens(receiver Value) bool {
	if receiver.Kind != ValueMap || len(receiver.Map) == 0 {
		return false
	}
	for _, value := range receiver.Map {
		if value.Kind != ValueObject || value.Type != "Schema.SObjectField" {
			return false
		}
	}
	return true
}
func (vm *VM) isSObjectType(typeName string) bool {
	if vm.Org == nil {
		return false
	}
	_, ok := vm.resolveObjectName(typeName)
	return ok
}
func (vm *VM) isSObjectLikeType(typeName string) bool {
	if strings.EqualFold(typeName, "sObject") {
		return true
	}
	if strings.EqualFold(typeName, "AggregateResult") {
		return true
	}
	if isCommonSObjectTypeName(typeName) || isCustomObjectLikeName(typeName) {
		return true
	}
	return vm.isSObjectType(typeName)
}
func sObjectMemberCallShapeSupported(method string, args []Value) bool {
	method = canonicalStdlibMemberName(method,
		"addError", "hasErrors", "getErrors", "get", "put", "putSObject", "isSet", "clear",
		"getPopulatedFieldsAsMap", "getSObjectType", "getSObjects", "getQuickActionName",
		"getAll", "getInstance", "getOrgDefaults", "getValues", "recalculateFormulas",
	)
	switch method {
	case "addError":
		return len(args) >= 1 && len(args) <= 3
	case "hasErrors", "getErrors", "clear", "getPopulatedFieldsAsMap", "getSObjectType", "getAll", "getOrgDefaults", "getValues", "recalculateFormulas":
		return len(args) == 0
	case "get", "isSet", "getSObjects", "getQuickActionName", "getInstance":
		return len(args) == 1
	case "put", "putSObject":
		return len(args) == 2
	default:
		return false
	}
}
func withConcreteSObjectListRuntime(records Value) Value {
	if records.Kind != ValueList {
		return records
	}
	objectName := ""
	if elementType, ok := collectionElementType(records.Type); ok && !strings.EqualFold(elementType, "SObject") {
		objectName = elementType
	}
	if objectName == "" {
		for _, item := range records.List {
			if item.Kind == ValueObject && item.Type != "" && !strings.EqualFold(item.Type, "Object") && !strings.EqualFold(item.Type, "SObject") {
				objectName = item.Type
				break
			}
		}
	}
	if objectName == "" {
		return records
	}
	records.Runtime = "List<" + objectName + ">"
	if records.Static == "" {
		records.Static = records.Type
	}
	return records
}
func (vm *VM) canonicalSObjectValueType(record Value) string {
	objectName := record.Type
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			return canonical
		}
	}
	return objectName
}
func sObjectIDValue(record Value) Value {
	if value, ok := record.Fields["Id"]; ok && value.Kind != ValueNull {
		return value
	}
	for field, value := range record.Fields {
		if strings.EqualFold(field, "Id") && value.Kind != ValueNull {
			return value
		}
	}
	return Null
}
func mergeSparseSObjectUpdate(existing, update Value) Value {
	for field, value := range update.Fields {
		if strings.EqualFold(field, "Id") || isInternalSObjectField(field) {
			continue
		}
		setExplicitSObjectField(&existing, field, value)
	}
	return existing
}
func sObjectTypeTokenObjectName(value Value) (string, bool) {
	_, objectName, ok := objectFieldValue(value, "object")
	return objectName.Text, ok && objectName.Kind == ValueString
}
func (vm *VM) callSObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method,
		"addError", "hasErrors", "getErrors", "get", "put", "putSObject", "isSet", "clear",
		"getPopulatedFieldsAsMap", "getSObjectType", "getSObjects", "getQuickActionName",
		"getAll", "getInstance", "getOrgDefaults", "getValues", "recalculateFormulas", "clone",
	)
	switch method {
	case "addError":
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		message, fields, err := sObjectAddErrorArgs(args, "SObject.addError")
		if err != nil {
			return Null, true, err
		}
		addSObjectError(&receiver, message, fields)
		return Null, true, nil
	case "hasErrors":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.hasErrors expects 0 arguments")
		}
		return Bool(len(sobjectErrors(receiver)) > 0), true, nil
	case "getErrors":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getErrors expects 0 arguments")
		}
		return List(sobjectErrors(receiver)...), true, nil
	case "get":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("SObject.get expects field name String or Schema.SObjectField")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			if errors.Is(err, errSObjectFieldTokenWrongObject) {
				return Null, true, newExceptionError("SObjectException", err.Error())
			}
			if errors.Is(err, errSObjectFieldTokenNull) {
				return Null, true, newExceptionError("SObjectException", err.Error())
			}
			return Null, true, fmt.Errorf("SObject.get expects field name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		if err := vm.unqueriedSObjectFieldError(receiver, field, true); err != nil {
			return Null, true, err
		}
		_, value, ok := objectFieldValue(receiver, field)
		if !ok && vm.Org != nil {
			if stripped := storage.StripNamespaceToken(vm.Org.Namespace, field); !strings.EqualFold(stripped, field) {
				_, value, ok = objectFieldValue(receiver, stripped)
			}
		}
		if !ok {
			if value, ok := vm.missingSObjectFieldValue(receiver, field); ok {
				return value, true, nil
			}
			return Null, true, nil
		}
		if value.Kind == ValueNull {
			if addressValue, hasAddress := vm.sObjectCompoundAddressValue(receiver, field); hasAddress {
				return addressValue, true, nil
			}
		}
		if definition, fieldDef, exists := vm.sObjectFieldDefinition(receiver.Type, field); exists {
			if strings.TrimSpace(fieldDef.Formula) != "" && calculatedDateFormulaBlankValue(fieldDef, value) {
				return Null, true, nil
			}
			if fieldDef.Type == storage.FieldCalculated &&
				!isExplicitSObjectField(receiver, field) &&
				!vm.queriedSObjectFieldsIncludes(receiver, field) &&
				shouldEvaluateSObjectFormulaField(receiver, fieldDef) {
				if record, ok := vm.formulaRecordFromSObject(receiver); ok {
					if evaluated, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record); ok {
						formulaValue := vmValueFromStorage(evaluated)
						if calculatedDateFormulaBlankValue(fieldDef, formulaValue) {
							return Null, true, nil
						}
						return formulaValue, true, nil
					}
				}
			}
		}
		if value.Kind == ValueNull {
			if isExplicitSObjectField(receiver, field) {
				return value, true, nil
			}
			if definition, fieldDef, exists := vm.sObjectFieldDefinition(receiver.Type, field); exists && fieldDef.Type == storage.FieldReference {
				if value, ok := vm.lookupIDFromLoadedParentRelationship(receiver, definition, field); ok {
					return value, true, nil
				}
			}
		}
		return value, true, nil
	case "put":
		if len(args) != 2 {
			return Null, true, fmt.Errorf("SObject.put expects field name String or Schema.SObjectField and value")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			if errors.Is(err, errSObjectFieldTokenWrongObject) {
				return Null, true, newExceptionError("SObjectException", err.Error())
			}
			if errors.Is(err, errSObjectFieldTokenNull) {
				return Null, true, newExceptionError("System.NullPointerException", "Argument cannot be null.")
			}
			return Null, true, fmt.Errorf("SObject.put expects field name String or Schema.SObjectField and value")
		}
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		previousReceiver := snapshotAlias(receiver)
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		actualField, previous, ok := objectFieldValue(receiver, field)
		if !ok {
			actualField = field
			previous = Null
		}
		value := args[1]
		setExplicitSObjectField(&receiver, actualField, value)
		markSetSObjectField(&receiver, actualField)
		markUserSetSObjectField(&receiver, actualField)
		markQueriedSObjectField(&receiver, actualField)
		vm.propagateAliasSnapshotToScope(vm.Globals, previousReceiver, receiver)
		vm.propagateAliasSnapshotToStatics(previousReceiver, receiver)
		return previous, true, nil
	case "putSObject":
		if len(args) != 2 {
			return Null, true, fmt.Errorf("SObject.putSObject expects relationship name String or Schema.SObjectField and SObject value")
		}
		fieldTokenArg := args[0].Kind == ValueObject && isSObjectFieldTokenType(args[0].Type)
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.putSObject expects relationship name String or Schema.SObjectField and SObject value")
		}
		if args[1].Kind != ValueNull && (args[1].Kind != ValueObject || !vm.isSObjectLikeType(args[1].Type)) {
			return Null, true, fmt.Errorf("SObject.putSObject expects SObject value")
		}
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		previousReceiver := snapshotAlias(receiver)
		relationshipName := fieldArg
		if fieldTokenArg {
			field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
			if definition, fieldDef, exists := vm.sObjectFieldDefinition(receiver.Type, field); exists && fieldDef.Type == storage.FieldReference {
				relationshipName = vm.parentRelationshipNameForReferenceField(definition, fieldDef)
			} else if derived := lookupFieldRelationshipName(field); derived != "" {
				relationshipName = derived
			}
		}
		if relationshipName == "" {
			return Null, true, fmt.Errorf("SObject.putSObject relationship name is blank")
		}
		setExplicitSObjectField(&receiver, relationshipName, args[1])
		markQueriedSObjectField(&receiver, relationshipName)
		vm.propagateAliasSnapshotToScope(vm.Globals, previousReceiver, receiver)
		vm.propagateAliasSnapshotToStatics(previousReceiver, receiver)
		return Null, true, nil
	case "isSet":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("SObject.isSet expects field name String or Schema.SObjectField")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.isSet expects field name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		if _, _, ok := objectFieldValue(receiver, field); ok {
			return Bool(true), true, nil
		}
		return Bool(isSetSObjectField(receiver, field)), true, nil
	case "clear":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.clear expects 0 arguments")
		}
		if reason, ok := sobjectReadOnlyReason(receiver); ok {
			return Null, true, fmt.Errorf("cannot modify read-only %s", reason)
		}
		for field := range receiver.Fields {
			delete(receiver.Fields, field)
		}
		delete(receiver.Fields, sobjectExplicitFieldsField)
		return Null, true, nil
	case "getPopulatedFieldsAsMap":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getPopulatedFieldsAsMap expects 0 arguments")
		}
		out := Map()
		out.Type = "Map<String,Object>"
		out.Runtime = "sobject-populated-fields:" + receiver.Type
		added := make(map[string]struct{}, len(receiver.Fields))
		addField := func(field string, value Value, includeSystem bool) {
			if isInternalSObjectField(field) || (!includeSystem && isSObjectSystemField(field)) {
				return
			}
			encoded := mapKey(String(field))
			if _, exists := out.Map[encoded]; exists {
				return
			}
			out.Map[encoded] = value
			out.MapKeys[encoded] = String(field)
			out.MapOrder = append(out.MapOrder, encoded)
			added[strings.ToLower(field)] = struct{}{}
		}
		for _, explicit := range explicitSObjectFieldNames(receiver) {
			actual, value, ok := objectFieldValue(receiver, explicit)
			if !ok {
				continue
			}
			addField(actual, value, true)
		}
		for field, value := range receiver.Fields {
			if _, ok := added[strings.ToLower(field)]; ok {
				continue
			}
			addField(field, value, false)
		}
		if selected, ok := receiver.Fields[sobjectQueriedFieldsField]; ok && selected.Kind == ValueMap {
			out.Fields = map[string]Value{sobjectPopulatedFieldsAliasContainsField: Bool(true)}
			objectName := receiver.Type
			if rawObject, ok := selected.Map[mapKey(String("object"))]; ok && rawObject.Kind == ValueString {
				objectName = rawObject.Text
			}
			for _, key := range selected.MapKeys {
				if key.Kind != ValueString {
					continue
				}
				field := key.Text
				if strings.Contains(field, ".") {
					vm.addQueriedRelationshipFieldToPopulatedMap(&out, receiver, field)
					continue
				}
				if strings.EqualFold(field, "object") || isInternalSObjectField(field) {
					continue
				}
				if vm.Org != nil {
					if object, ok := vm.Org.Objects[objectName]; ok {
						if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
							field = canonical
						}
					}
				}
				value := Null
				if actual, existing, ok := objectFieldValue(receiver, field); ok {
					field = actual
					value = existing
				}
				encoded := mapKey(String(field))
				if _, exists := out.Map[encoded]; !exists {
					out.Map[encoded] = value
					out.MapKeys[encoded] = String(field)
					out.MapOrder = append(out.MapOrder, encoded)
				}
			}
		}
		return out, true, nil
	case "getSObjectType":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getSObjectType expects 0 arguments")
		}
		typeName := runtimeObjectType(receiver)
		token, ok := vm.sObjectTypeTokenForName(typeName)
		if !ok {
			return Null, false, nil
		}
		return token, true, nil
	case "getQuickActionName":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.getQuickActionName expects 0 arguments")
		}
		for _, field := range []string{"QuickActionName", "quickActionName"} {
			if _, value, ok := objectFieldValue(receiver, field); ok {
				if value.Kind == ValueNull || value.Kind == ValueString {
					return value, true, nil
				}
				return Null, true, fmt.Errorf("SObject.getQuickActionName field %s is not a String", field)
			}
		}
		return Null, true, nil
	case "getAll", "getInstance", "getOrgDefaults", "getValues":
		return vm.callCustomDataStaticMember(receiver.Type, method, args)
	case "recalculateFormulas":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("SObject.recalculateFormulas expects 0 arguments")
		}
		result := vm.recalculateFormulaSObject(receiver)
		if value, ok := result.Fields["sobject"]; ok && value.Kind == ValueObject {
			for field, fieldValue := range value.Fields {
				receiver.Fields[field] = fieldValue
			}
		}
		return result, true, nil
	case "clone":
		if len(args) > 4 {
			return Null, true, fmt.Errorf("SObject.clone expects 0 to 4 arguments")
		}
		for _, arg := range args {
			if arg.Kind != ValueBool {
				return Null, true, fmt.Errorf("SObject.clone preserve flags must be Boolean")
			}
		}
		cloned := cloneValue(receiver)
		if cloned.Fields == nil {
			cloned.Fields = make(map[string]Value)
		}
		vm.hydrateCloneRecordTypeID(receiver, &cloned)
		preserveID := len(args) > 0 && args[0].Bool
		if !preserveID {
			deleteObjectField(cloned.Fields, "Id")
		}
		deleteObjectField(cloned.Fields, sobjectErrorsField)
		deleteObjectField(cloned.Fields, sobjectReadOnlyField)
		return cloned, true, nil
	case "getSObject":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("SObject.getSObject expects relationship name String or Schema.SObjectField")
		}
		fieldTokenArg := args[0].Kind == ValueObject && isSObjectFieldTokenType(args[0].Type)
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.getSObject expects relationship name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		_, value, ok := objectFieldValue(receiver, field)
		if !ok || value.Kind == ValueNull {
			if fieldTokenArg {
				if relationshipName := lookupFieldRelationshipName(field); relationshipName != "" {
					if _, relationship, hasRelationship := objectFieldValue(receiver, relationshipName); hasRelationship && relationship.Kind == ValueObject {
						return relationship, true, nil
					}
				}
			}
			if relationship, hasRelationship := vm.parentRelationshipValue(receiver, field); hasRelationship {
				if relationship.Kind != ValueNull {
					return relationship, true, nil
				}
				return relationship, true, nil
			}
			return Null, true, nil
		}
		if fieldTokenArg {
			if definition, fieldDef, exists := vm.sObjectFieldDefinition(receiver.Type, field); exists && fieldDef.Type == storage.FieldReference {
				relationshipName := vm.parentRelationshipNameForReferenceField(definition, fieldDef)
				if relationshipName != "" {
					if relationship, ok := vm.parentRelationshipValue(receiver, relationshipName); ok {
						return relationship, true, nil
					}
				}
			}
		}
		if value.Kind != ValueObject || !vm.isSObjectLikeType(value.Type) {
			if fieldTokenArg {
				return Null, true, nil
			}
			return Null, true, fmt.Errorf("SObject.getSObject field %s is not an SObject", field)
		}
		return value, true, nil
	case "getSObjects":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("SObject.getSObjects expects relationship name String or Schema.SObjectField")
		}
		fieldArg, err := vm.sObjectFieldArg(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("SObject.getSObjects expects relationship name String or Schema.SObjectField")
		}
		field := vm.resolveSObjectFieldName(receiver.Type, fieldArg)
		if err := vm.unqueriedSObjectFieldError(receiver, field, true); err != nil {
			return Null, true, err
		}
		if _, value, ok := vm.loadedChildRelationshipValue(receiver, field); ok {
			if value.Kind == ValueNull {
				return List(), true, nil
			}
			if value.Kind != ValueList {
				return Null, true, fmt.Errorf("SObject.getSObjects field %s is not a List", field)
			}
			return value, true, nil
		}
		if value, ok := vm.lazyChildRelationshipValue(receiver, field); ok {
			return value, true, nil
		}
		_, value, ok := objectFieldValue(receiver, field)
		if !ok || value.Kind == ValueNull {
			return List(), true, nil
		}
		if value.Kind != ValueList {
			return Null, true, fmt.Errorf("SObject.getSObjects field %s is not a List", field)
		}
		return value, true, nil
	default:
		return Null, false, nil
	}
}
