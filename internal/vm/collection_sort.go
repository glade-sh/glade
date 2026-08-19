package vm

import (
	"fmt"
	"sort"
	"strings"
)

func missingMapValue(receiver Value) Value {
	_, valueType, ok := mapTypeArgs(receiver.Type)
	if !ok || strings.TrimSpace(valueType) == "" {
		return Null
	}
	return Value{Kind: ValueNull, Type: valueType}
}
func (vm *VM) mapFromSObjectList(mapType string, list Value) (Value, error) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok {
		return Null, unsupportedCallError("Map constructor from SObject list")
	}
	out := Map()
	out.Type = mapType
	trustDeclaredValueType := sObjectListDeclaresMapValueType(list, valueType)
	for i, item := range list.List {
		if item.Kind == ValueNull {
			return Null, fmt.Errorf("Map constructor from SObject list requires non-null SObject at index %d", i)
		}
		if item.Kind != ValueObject {
			return Null, fmt.Errorf("Map constructor from SObject list requires SObject values at index %d", i)
		}
		coerced := item
		if trustDeclaredValueType {
			if coerced.Runtime == "" && coerced.Type != "" && !strings.EqualFold(coerced.Type, valueType) {
				coerced.Runtime = coerced.Type
			}
			if !strings.EqualFold(valueType, "SObject") {
				coerced.Static = valueType
			}
		} else {
			var err error
			coerced, err = vm.coerceAssignable(valueType, item)
			if err != nil {
				return Null, fmt.Errorf("Map constructor from SObject list: value at index %d: %w", i, err)
			}
		}
		keyValue, ok := mapConstructorKeyValue(keyType, coerced)
		if !ok || keyValue.Kind == ValueNull {
			return Null, fmt.Errorf("Map constructor from SObject list requires non-null %s at index %d", keyType, i)
		}
		key, err := vm.coerceAssignable(keyType, keyValue)
		if err != nil {
			return Null, fmt.Errorf("Map constructor from SObject list: key at index %d: %w", i, err)
		}
		encodedKey := mapKey(key)
		if _, exists := out.Map[encodedKey]; !exists {
			out.MapOrder = append(out.MapOrder, encodedKey)
		}
		out.Map[encodedKey] = coerced
		out.MapKeys[encodedKey] = key
	}
	return out, nil
}
func sObjectListDeclaresMapValueType(list Value, valueType string) bool {
	for _, sourceType := range []string{list.Type, list.Static} {
		elementType, ok := collectionElementType(sourceType)
		if !ok {
			continue
		}
		if strings.EqualFold(elementType, valueType) {
			return true
		}
		if strings.EqualFold(valueType, "SObject") && isCommonSObjectTypeName(elementType) {
			return true
		}
	}
	return false
}
func mapConstructorKeyValue(keyType string, record Value) (Value, bool) {
	if strings.EqualFold(keyType, "Id") || strings.EqualFold(keyType, "String") {
		value, ok := record.Fields["Id"]
		if ok {
			return value, true
		}
		if strings.EqualFold(keyType, "Id") {
			return value, false
		}
	}
	for _, preferred := range []string{"Name", "DeveloperName", "MasterLabel"} {
		if value, ok := record.Fields[preferred]; ok {
			return value, true
		}
	}
	for _, value := range record.Fields {
		return value, true
	}
	return Null, false
}
func typedNullCollectionBase(value Value) string {
	if value.Kind != ValueNull || value.Type == "" {
		return ""
	}
	return collectionBase(value.Type)
}
func valueShape(value Value) string {
	shape := string(value.Kind)
	if value.Type != "" {
		shape += ":" + value.Type
	}
	if value.Static != "" {
		shape += ":static=" + value.Static
	}
	if value.Runtime != "" {
		shape += ":runtime=" + value.Runtime
	}
	return shape
}
func (vm *VM) mapLookupKey(receiver Value, key Value) string {
	keyType, _, ok := mapTypeArgs(receiver.Type)
	if !ok || strings.TrimSpace(keyType) == "" {
		return vm.mapKey(key)
	}
	coerced, err := vm.coerceAssignable(keyType, key)
	if err != nil {
		return vm.mapKey(key)
	}
	return vm.mapKey(coerced)
}
func caseInsensitiveStringMapStoredKey(receiver Value, key Value) (string, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString {
		return "", false
	}
	_, valueType, typed := mapTypeArgs(receiver.Type)
	if !caseInsensitiveStringMap(receiver) && (!typed || !strings.EqualFold(valueType, "String")) {
		return "", false
	}
	fold := caseInsensitiveStringMap(receiver)
	for rawKey, storedKey := range receiver.MapKeys {
		if storedKey.Kind == ValueString && (storedKey.Text == key.Text || fold && strings.EqualFold(storedKey.Text, key.Text)) {
			return rawKey, true
		}
		if storedKey.String() == key.Text || fold && strings.EqualFold(storedKey.String(), key.Text) {
			return rawKey, true
		}
	}
	for rawKey := range receiver.Map {
		storedKey := mapStoredKey(receiver, rawKey)
		if storedKey.Kind == ValueString && (storedKey.Text == key.Text || fold && strings.EqualFold(storedKey.Text, key.Text)) {
			return rawKey, true
		}
		if storedKey.String() == key.Text || fold && strings.EqualFold(storedKey.String(), key.Text) {
			return rawKey, true
		}
	}
	return "", false
}
func caseInsensitiveStringMap(receiver Value) bool {
	if receiver.Runtime == "pagereference-parameters" {
		return true
	}
	if receiver.MapKeys != nil {
		if flag, ok := receiver.MapKeys["__glade_case_insensitive_string_keys"]; ok && flag.Kind == ValueBool && flag.Bool {
			return true
		}
	}
	if receiver.Fields == nil {
		return false
	}
	flag, ok := receiver.Fields["__caseInsensitiveStringKeys"]
	return ok && flag.Kind == ValueBool && flag.Bool
}
func (vm *VM) putAllSObjectList(receiver Value, list Value) (Value, error) {
	value, err := vm.mapFromSObjectList(receiver.Type, list)
	if err != nil {
		return receiver, err
	}
	for key, item := range value.Map {
		if keyValue, ok := value.MapKeys[key]; ok {
			vm.markCollectionRefsEscaped(keyValue, item)
		} else {
			vm.markCollectionRefsEscaped(item)
		}
		receiver.Map[key] = item
	}
	if receiver.MapKeys == nil {
		receiver.MapKeys = make(map[string]Value)
	}
	for key, item := range value.MapKeys {
		receiver.MapKeys[key] = item
	}
	return receiver, nil
}
func collectionMembers(value Value) []Value {
	switch value.Kind {
	case ValueList:
		return value.List
	case ValueSet:
		return value.Set
	default:
		return nil
	}
}
func (vm *VM) collectionContainsValue(values []Value, needle Value, result *Result) (bool, error) {
	index, err := vm.collectionIndexOfValue(values, needle, result)
	return index >= 0, err
}
func (vm *VM) collectionIndexOfValue(values []Value, needle Value, result *Result) (int, error) {
	for i, value := range values {
		equal, err := vm.apexCollectionElementEquals(value, needle, result)
		if err != nil {
			return -1, err
		}
		if equal {
			return i, nil
		}
	}
	return -1, nil
}
func (vm *VM) apexCollectionElementEquals(left, right Value, result *Result) (bool, error) {
	if collectionStringLike(left) || collectionStringLike(right) {
		return left.Equal(right), nil
	}
	return vm.apexEquals(left, right, result)
}
func collectionStringLike(value Value) bool {
	return value.Kind == ValueString || (value.Kind == ValueObject && strings.EqualFold(value.Type, "String"))
}
func (vm *VM) iterableCollectionMembers(value Value, result *Result, context string) ([]Value, error) {
	switch value.Kind {
	case ValueList, ValueSet:
		return collectionMembers(value), nil
	case ValueObject:
		iterator := value
		if !isIteratorValue(iterator) {
			var err error
			iterator, err = vm.iteratorForObject(value, result)
			if err != nil {
				return nil, fmt.Errorf("%s expects List, Set, or Iterable: %w", context, err)
			}
		}
		const iteratorName = "__glade_add_all_iterator"
		previousIterator, hadIterator := vm.Globals[iteratorName]
		previousIteratorType, hadIteratorType := vm.VarTypes[iteratorName]
		defer func() {
			if hadIterator {
				vm.Globals[iteratorName] = previousIterator
			} else {
				delete(vm.Globals, iteratorName)
			}
			if hadIteratorType {
				vm.VarTypes[iteratorName] = previousIteratorType
			} else {
				delete(vm.VarTypes, iteratorName)
			}
		}()
		vm.Globals[iteratorName] = iterator
		vm.VarTypes[iteratorName] = iterator.Type
		values := make([]Value, 0)
		for iteration := 0; ; iteration++ {
			if iteration >= maxLoopIterations {
				return nil, fmt.Errorf("%s iterable exceeded %d iterations", context, maxLoopIterations)
			}
			hasNext, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "hasNext", nil, result)
			if err != nil {
				return nil, err
			}
			if !handled || hasNext.Kind != ValueBool {
				return nil, fmt.Errorf("%s iterable requires Boolean hasNext", context)
			}
			if !hasNext.Bool {
				return values, nil
			}
			next, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "next", nil, result)
			if err != nil {
				return nil, err
			}
			if !handled {
				return nil, fmt.Errorf("%s iterable requires next", context)
			}
			values = append(values, next)
		}
	default:
		return nil, fmt.Errorf("%s expects List, Set, or Iterable", context)
	}
}
func collectionIterator(value Value) Value {
	snapshot := List(append([]Value(nil), collectionMembers(value)...)...)
	iterator := Object(collectionIteratorType(value.Type))
	iterator.Fields["__values"] = snapshot
	iterator.Fields["__index"] = Int(0)
	return iterator
}
func collectionIteratorType(collectionType string) string {
	if elementType, ok := collectionElementType(collectionType); ok {
		return "Iterator<" + elementType + ">"
	}
	return "Iterator"
}
func isIteratorValue(value Value) bool {
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Iterator") ||
		hasPrefixFold(value.Type, "iterator<") ||
		hasPrefixFold(value.Type, "system.iterator<") ||
		value.Type == "Database.QueryLocatorIterator" ||
		value.Type == "Database.QueryLocatorChunkIterator")
}
func callIteratorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	values, ok := receiver.Fields["__values"]
	if !ok || values.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("Iterator missing snapshot")
	}
	indexValue, ok := receiver.Fields["__index"]
	if !ok || indexValue.Kind != ValueInt {
		return Null, receiver, false, true, fmt.Errorf("Iterator missing index")
	}
	index := int(indexValue.Int)
	switch method {
	case "hasNext":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.hasNext expects 0 arguments")
		}
		return Bool(index < len(values.List)), receiver, false, true, nil
	case "next":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.next expects 0 arguments")
		}
		if index >= len(values.List) {
			return Null, receiver, false, true, newExceptionError("NoSuchElementException", "Iterator has no more elements")
		}
		receiver.Fields["__index"] = Int(int64(index + 1))
		return values.List[index], receiver, true, true, nil
	case "remove":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Iterator.remove expects 0 arguments")
		}
		return Null, receiver, false, true, unsupportedCallError("Iterator.remove")
	default:
		return Null, receiver, false, false, nil
	}
}
func (vm *VM) sortComparableValues(values []Value, result *Result) error {
	for _, value := range values {
		switch value.Kind {
		case ValueNull, ValueInt, ValueDecimal, ValueString, ValueBool:
		case ValueObject:
		default:
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
	}
	if listSortHasObjects(values) {
		return vm.sortApexComparableValues(values, result)
	}
	sort.SliceStable(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if compare, ok := compareNullSortValues(left, right); ok {
			return compare < 0
		}
		if collectionNumericKind(left.Kind) && collectionNumericKind(right.Kind) {
			return collectionNumericLess(left, right)
		}
		if left.Kind != right.Kind {
			return collectionSortKindRank(left.Kind) < collectionSortKindRank(right.Kind)
		}
		switch left.Kind {
		case ValueInt:
			return left.Int < right.Int
		case ValueDecimal:
			return left.Decimal < right.Decimal
		case ValueString:
			return left.Text < right.Text
		case ValueBool:
			return !left.Bool && right.Bool
		default:
			return false
		}
	})
	return nil
}
func (vm *VM) sortValuesWithComparator(values []Value, comparator Value, result *Result) error {
	if comparator.Kind != ValueObject {
		return fmt.Errorf("List.sort comparator must be an object")
	}
	comparatorType := runtimeObjectType(comparator)
	if comparatorType == "" {
		return fmt.Errorf("List.sort comparator type is required")
	}
	var sortErr error
	sort.SliceStable(values, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		compare, err := vm.compareValuesWithComparator(comparator, comparatorType, values[i], values[j], result)
		if err != nil {
			sortErr = err
			return false
		}
		return compare < 0
	})
	return sortErr
}
func (vm *VM) compareValuesWithComparator(comparator Value, comparatorType string, left, right Value, result *Result) (int64, error) {
	args := []Value{left, right}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(comparatorType, "compare", args)
	if ambiguous {
		return 0, vm.ambiguousOverloadError(comparatorType+".compare", args)
	}
	if !ok {
		return 0, unsupportedCallError("List.sort comparator without compare method")
	}
	value, err := vm.callMethodWithReceiver(target, comparator, args, result)
	if err != nil {
		return 0, err
	}
	switch value.Kind {
	case ValueInt:
		return value.Int, nil
	case ValueDecimal:
		return int64(value.Decimal), nil
	default:
		return 0, fmt.Errorf("%s returned %s, want Integer", target.Name, valueTypeName(value))
	}
}
func listSortHasObjects(values []Value) bool {
	for _, value := range values {
		if value.Kind == ValueObject {
			return true
		}
	}
	return false
}
func (vm *VM) sortApexComparableValues(values []Value, result *Result) error {
	hasSObject := false
	hasComparable := false
	hasPlatformComparable := false
	for _, value := range values {
		if value.Kind == ValueNull {
			continue
		}
		if value.Kind != ValueObject {
			return unsupportedCallError("List.sort for mixed primitive and Comparable values")
		}
		runtimeType := runtimeObjectType(value)
		if vm.isSortableSObjectValue(value) {
			hasSObject = true
			continue
		}
		if isSortablePlatformValue(value) {
			hasPlatformComparable = true
			continue
		}
		if _, ok, ambiguous := vm.resolveInstanceMethodForArgs(runtimeType, "compareTo", []Value{value}); ambiguous {
			return vm.ambiguousOverloadError(runtimeType+".compareTo", []Value{value})
		} else if !ok {
			return unsupportedCallError("List.sort for non-primitive comparable values")
		}
		hasComparable = true
	}
	kinds := 0
	if hasSObject {
		kinds++
	}
	if hasComparable {
		kinds++
	}
	if hasPlatformComparable {
		kinds++
	}
	if kinds > 1 {
		return unsupportedCallError("List.sort for mixed object comparable values")
	}
	var sortErr error
	sort.SliceStable(values, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		if compare, ok := compareNullSortValues(values[i], values[j]); ok {
			return compare < 0
		}
		if hasSObject {
			return compareSObjectSortValues(values[i], values[j]) < 0
		}
		if hasPlatformComparable {
			return comparePlatformSortValues(values[i], values[j]) < 0
		}
		compare, err := vm.compareApexComparableValues(values[i], values[j], result)
		if err != nil {
			sortErr = err
			return false
		}
		if compare < 0 {
			reverse, err := vm.compareApexComparableValues(values[j], values[i], result)
			if err != nil {
				sortErr = err
				return false
			}
			if reverse < 0 {
				return false
			}
		}
		return compare < 0
	})
	return sortErr
}
func compareNullSortValues(left, right Value) (int, bool) {
	if left.Kind == ValueNull && right.Kind == ValueNull {
		return 0, true
	}
	if left.Kind == ValueNull {
		return -1, true
	}
	if right.Kind == ValueNull {
		return 1, true
	}
	return 0, false
}
func isSortablePlatformValue(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	runtimeType := runtimeObjectType(value)
	return strings.EqualFold(runtimeType, "SelectOption") || platformScalarObject(runtimeType)
}
func comparePlatformSortValues(left, right Value) int {
	if cmp := strings.Compare(strings.ToLower(runtimeObjectType(left)), strings.ToLower(runtimeObjectType(right))); cmp != 0 {
		return cmp
	}
	if strings.EqualFold(runtimeObjectType(left), "SelectOption") && strings.EqualFold(runtimeObjectType(right), "SelectOption") {
		if cmp := strings.Compare(selectOptionSortText(left, "label"), selectOptionSortText(right, "label")); cmp != 0 {
			return cmp
		}
		return strings.Compare(selectOptionSortText(left, "value"), selectOptionSortText(right, "value"))
	}
	if leftText, ok := platformScalarObjectText(left); ok {
		if rightText, ok := platformScalarObjectText(right); ok {
			return strings.Compare(leftText, rightText)
		}
	}
	return strings.Compare(sObjectStableSortKey(left), sObjectStableSortKey(right))
}
func selectOptionSortText(value Value, field string) string {
	_, fieldValue, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	return strings.ToLower(fieldValue.String())
}
func (vm *VM) isSortableSObjectValue(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	runtimeType := runtimeObjectType(value)
	if _, ok := vm.lookupClass(runtimeType); ok {
		return false
	}
	return vm.isSObjectLikeType(runtimeType)
}
func compareSObjectSortValues(left, right Value) int {
	if cmp := strings.Compare(strings.ToLower(runtimeObjectType(left)), strings.ToLower(runtimeObjectType(right))); cmp != 0 {
		return cmp
	}
	if leftName, rightName, ok := sObjectSortFieldPair(left, right, "Name"); ok {
		return strings.Compare(leftName, rightName)
	}
	if leftLabel, rightLabel, ok := sObjectSortFieldPair(left, right, "MasterLabel"); ok {
		return strings.Compare(leftLabel, rightLabel)
	}
	if leftDeveloperName, rightDeveloperName, ok := sObjectSortFieldPair(left, right, "DeveloperName"); ok {
		return strings.Compare(leftDeveloperName, rightDeveloperName)
	}
	if leftID, rightID, ok := sObjectSortFieldPair(left, right, "Id"); ok {
		return strings.Compare(leftID, rightID)
	}
	return strings.Compare(sObjectStableSortKey(left), sObjectStableSortKey(right))
}
func sObjectSortFieldPair(left, right Value, field string) (string, string, bool) {
	_, leftValue, leftOK := objectFieldValue(left, field)
	_, rightValue, rightOK := objectFieldValue(right, field)
	if !leftOK || !rightOK || leftValue.Kind == ValueNull || rightValue.Kind == ValueNull {
		return "", "", false
	}
	return leftValue.String(), rightValue.String(), true
}
func sObjectStableSortKey(value Value) string {
	fields := make([]string, 0, len(value.Fields))
	for field := range value.Fields {
		if isInternalSObjectField(field) {
			continue
		}
		fields = append(fields, field)
	}
	sort.Strings(fields)
	var out strings.Builder
	for _, field := range fields {
		out.WriteString(strings.ToLower(field))
		out.WriteByte('=')
		out.WriteString(value.Fields[field].String())
		out.WriteByte(';')
	}
	return out.String()
}
func (vm *VM) compareApexComparableValues(left, right Value, result *Result) (int64, error) {
	if left.Kind != ValueObject {
		return 0, unsupportedCallError("List.sort for mixed primitive and Comparable values")
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(runtimeObjectType(left), "compareTo", []Value{right})
	if ambiguous {
		return 0, vm.ambiguousOverloadError(runtimeObjectType(left)+".compareTo", []Value{right})
	}
	if !ok {
		return 0, unsupportedCallError("List.sort for non-primitive comparable values")
	}
	value, err := vm.callMethodWithReceiver(target, left, []Value{right}, result)
	if err != nil {
		return 0, err
	}
	switch value.Kind {
	case ValueInt:
		return value.Int, nil
	case ValueDecimal:
		return int64(value.Decimal), nil
	default:
		return 0, fmt.Errorf("%s returned %s, want Integer", target.Name, valueTypeName(value))
	}
}
func collectionNumericKind(kind ValueKind) bool {
	return kind == ValueInt || kind == ValueDecimal
}
func collectionNumericValue(value Value) float64 {
	if value.Kind == ValueInt {
		return float64(value.Int)
	}
	return value.Decimal
}
func collectionNumericLess(left, right Value) bool {
	leftRat, leftOK := valueDecimalRat(left)
	rightRat, rightOK := valueDecimalRat(right)
	if leftOK && rightOK {
		return leftRat.Cmp(rightRat) < 0
	}
	return collectionNumericValue(left) < collectionNumericValue(right)
}
func collectionSortKindRank(kind ValueKind) int {
	switch kind {
	case ValueBool:
		return 0
	case ValueInt, ValueDecimal:
		return 1
	case ValueString:
		return 2
	default:
		return 3
	}
}
func valueFromMapKey(key string) Value {
	if strings.HasPrefix(key, string(ValueObject)+":") {
		rest := strings.TrimPrefix(key, string(ValueObject)+":")
		typeName, text, ok := strings.Cut(rest, ":")
		if ok && typeName == "Schema.SObjectType" {
			return sObjectTypeToken(text)
		}
		if ok && typeName == "Schema.SObjectField" {
			objectName, fieldName, hasField := strings.Cut(text, ".")
			if hasField {
				return sObjectFieldToken(objectName, fieldName)
			}
		}
		if ok && typeName == "Schema.ChildRelationship" {
			relationshipName, rest, hasRelationship := strings.Cut(text, "|")
			childName, fieldName, hasField := strings.Cut(rest, "|")
			if hasRelationship && hasField {
				value := Object("Schema.ChildRelationship")
				value.Fields["relationshipName"] = String(relationshipName)
				value.Fields["childSObject"] = sObjectTypeToken(childName)
				value.Fields["field"] = sObjectFieldToken(childName, fieldName)
				value.Fields["cascadeDelete"] = Bool(false)
				value.Fields["restrictedDelete"] = Bool(false)
				return value
			}
		}
		if ok && typeName == "Type" {
			return Value{Kind: ValueObject, Type: "Type", Text: text}
		}
		if ok && platformScalarObject(typeName) {
			return platformScalar(typeName, text)
		}
		if ok && typeName != "" {
			value := Value{Kind: ValueObject, Type: typeName, Text: text, Fields: make(map[string]Value)}
			if looksLikeID(text) {
				value.Fields["Id"] = String(text)
			}
			return value
		}
	}
	kind, text, ok := strings.Cut(key, ":")
	if !ok {
		return String(key)
	}
	switch ValueKind(kind) {
	case ValueNull:
		return Null
	case ValueInt:
		var parsed int64
		if _, err := fmt.Sscan(text, &parsed); err == nil {
			return Int(parsed)
		}
	case ValueDecimal:
		var parsed float64
		if _, err := fmt.Sscan(text, &parsed); err == nil {
			return Decimal(parsed)
		}
	case ValueBool:
		return Bool(strings.EqualFold(text, "true"))
	case ValueString:
		return String(text)
	}
	return String(text)
}
