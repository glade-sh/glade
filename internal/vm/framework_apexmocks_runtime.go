package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) frameworkQualifiedMethodMapKey(value Value) (string, bool) {
	if !strings.EqualFold(frameworkMockSupportType(value.Type), "QualifiedMethod") {
		return "", false
	}
	typeName := objectStringField(value, "typeName")
	methodName := objectStringField(value, "methodName")
	_, methodArgTypes, _ := objectFieldValue(value, "methodArgTypes")
	parts := []string{"object:framework_QualifiedMethod", strings.ToLower(typeName), strings.ToLower(methodName), mapKey(methodArgTypes)}
	if vm.frameworkApexMocksIndependentMocks() {
		_, mockInstance, _ := objectFieldValue(value, "mockInstance")
		parts = append(parts, strconv.FormatUint(mockInstance.Ref, 10), mapKey(mockInstance))
	}
	return strings.Join(parts, "|"), true
}
func (vm *VM) frameworkApexMocksIndependentMocks() bool {
	for _, className := range []string{"framework_ApexMocksConfig", "fflib_ApexMocksConfig"} {
		class, ok := vm.Classes[className]
		if !ok {
			continue
		}
		for name, field := range class.StaticFields {
			if !strings.EqualFold(name, "HasIndependentMocks") {
				continue
			}
			if field.Value.Kind == ValueBool {
				return field.Value.Bool
			}
			return false
		}
	}
	return false
}
func (vm *VM) frameworkQualifiedMethodKeysEqual(left, right Value) (bool, bool) {
	if left.Kind != ValueObject || right.Kind != ValueObject ||
		!strings.EqualFold(frameworkMockSupportType(left.Type), "QualifiedMethod") ||
		!strings.EqualFold(frameworkMockSupportType(right.Type), "QualifiedMethod") {
		return false, false
	}
	if vm.frameworkApexMocksIndependentMocks() {
		_, leftMock, _ := objectFieldValue(left, "mockInstance")
		_, rightMock, _ := objectFieldValue(right, "mockInstance")
		if !valueAliasMatch(leftMock, rightMock) && !leftMock.Equal(rightMock) {
			return false, true
		}
	}
	if !strings.EqualFold(objectStringField(left, "typeName"), objectStringField(right, "typeName")) {
		return false, true
	}
	if objectStringField(left, "methodName") != objectStringField(right, "methodName") {
		return false, true
	}
	_, leftArgs, _ := objectFieldValue(left, "methodArgTypes")
	_, rightArgs, _ := objectFieldValue(right, "methodArgTypes")
	return vm.frameworkMethodArgTypesEqual(leftArgs, rightArgs), true
}
func (vm *VM) frameworkMethodArgTypesEqual(left, right Value) bool {
	if left.Equal(right) {
		return true
	}
	if left.Kind != ValueList || right.Kind != ValueList || len(left.List) != len(right.List) {
		return false
	}
	for i := range left.List {
		if !vm.frameworkMethodArgTypeEqual(left.List[i], right.List[i]) {
			return false
		}
	}
	return true
}
func (vm *VM) frameworkMethodArgTypeEqual(left, right Value) bool {
	leftText := canonicalTypeValueText(typeValueText(left))
	rightText := canonicalTypeValueText(typeValueText(right))
	if leftText == "" || rightText == "" {
		return left.Equal(right)
	}
	if strings.EqualFold(leftText, rightText) {
		return true
	}
	leftBase := collectionBase(leftText)
	rightBase := collectionBase(rightText)
	if leftBase != "" && strings.EqualFold(leftBase, rightBase) {
		leftElement, leftOK := collectionElementType(leftText)
		rightElement, rightOK := collectionElementType(rightText)
		if leftOK && rightOK {
			return vm.frameworkSObjectTypeMatch(leftElement, rightElement)
		}
	}
	return vm.frameworkSObjectTypeMatch(leftText, rightText)
}
func (vm *VM) frameworkSObjectTypeMatch(left, right string) bool {
	if strings.EqualFold(left, right) {
		return true
	}
	if strings.EqualFold(left, "SObject") && vm.isSObjectLikeType(right) {
		return true
	}
	if strings.EqualFold(right, "SObject") && vm.isSObjectLikeType(left) {
		return true
	}
	return false
}
func frameworkApexMocksProviderActive(provider Value) bool {
	if provider.Kind != ValueObject || !strings.EqualFold(frameworkMockSupportType(provider.Type), "ApexMocks") {
		return false
	}
	if stubProviderStateFlagSet(provider) {
		return true
	}
	_, recorder, ok := objectFieldValue(provider, "methodReturnValueRecorder")
	if !ok || recorder.Kind != ValueObject {
		return false
	}
	if _, value, ok := objectFieldValue(recorder, "Stubbing"); ok && value.Kind == ValueBool && value.Bool {
		return true
	}
	return false
}
func frameworkApexMocksProviderHasRecorder(provider Value) bool {
	if provider.Kind != ValueObject || !strings.EqualFold(frameworkMockSupportType(provider.Type), "ApexMocks") {
		return false
	}
	_, recorder, ok := objectFieldValue(provider, "methodReturnValueRecorder")
	return ok && recorder.Kind == ValueObject
}
func normalizeFrameworkStubReturnValue(returnType string, value Value, provider Value) Value {
	providerType := strings.ToLower(provider.Type)
	if !strings.Contains(providerType, "framework") && !strings.Contains(providerType, "apexmocks") && !strings.Contains(providerType, "fflib") {
		return value
	}
	if value.Kind != ValueList || len(value.List) != 0 {
		return value
	}
	elementType, ok := collectionElementType(returnType)
	if !ok || !strings.EqualFold(elementType, "SObject") {
		return value
	}
	value.Type = returnType
	value.Static = returnType
	value.Runtime = ""
	return value
}
func (vm *VM) frameworkStubInvocationCount(receiver Value, target Method) int {
	byMethod, ok := vm.frameworkMethodCountRecorderStatic("methodArgumentsByTypeName")
	if !ok || byMethod.Kind != ValueMap {
		return 0
	}
	methodValue := vm.frameworkStubQualifiedMethod(receiver, target)
	values, ok := byMethod.Map[vm.mapKey(methodValue)]
	if !ok {
		if fallback, found, err := vm.objectKeyMapLookup(byMethod, methodValue); err == nil && found {
			values = fallback
			ok = true
		}
	}
	if !ok || values.Kind != ValueList {
		return 0
	}
	return len(values.List)
}
func (vm *VM) frameworkRecordStubInvocation(receiver Value, target Method, args []Value) error {
	invocation := Object("framework_InvocationOnMock")
	invocation.Fields["qm"] = vm.frameworkStubQualifiedMethod(receiver, target)
	methodArg := Object("framework_MethodArgValues")
	methodArg.Fields["argValues"] = List(args...)
	invocation.Fields["methodArg"] = methodArg
	invocation.Fields["mockInstance"] = receiver
	return vm.frameworkRecordMethodInvocation(invocation)
}
func (vm *VM) frameworkStubQualifiedMethod(receiver Value, target Method) Value {
	methodValue := Object("framework_QualifiedMethod")
	methodValue.Fields["typeName"] = String(strings.Split(receiver.String(), ":")[0])
	methodValue.Fields["methodName"] = String(apexMethodMemberName(target.Name))
	paramTypes := make([]Value, 0, len(target.Params))
	for _, param := range target.Params {
		paramTypes = append(paramTypes, platformScalar("Type", vm.resolveTypeNameInClass(target.ClassName, param.Type)))
	}
	methodValue.Fields["methodArgTypes"] = Value{Kind: ValueList, Type: "List<Type>", List: paramTypes}
	methodValue.Fields["mockInstance"] = receiver
	return methodValue
}
func (vm *VM) unstubbedFrameworkMockReturnFallback(receiver Value, method string, target Method, args []Value, result *Result) (Value, bool, error) {
	if value, ok := passiveFormattingAccessorDefault(method, target.ReturnType, args); ok {
		return value, true, nil
	}
	if concreteType, ok := vm.unstubbedUnitOfWorkFallbackType(receiver.Type, method, target); ok {
		ctorArgs := args
		if len(ctorArgs) == 0 {
			ctorArgs = []Value{typedList("List<Schema.SObjectType>")}
		}
		value, err := vm.constructValue(concreteType, ctorArgs, nil, result)
		return value, true, err
	}
	if value, ok := vm.unstubbedDatasetServiceListFallback(receiver.Type, method, target); ok {
		return value, true, nil
	}
	if value, ok := vm.unstubbedSchemaServiceFallback(receiver.Type, method, args); ok {
		return value, true, nil
	}
	return Null, false, nil
}
func frameworkMismatchedStubReturnFallback(returnType string, value Value, provider Value) (Value, bool) {
	if returnType == "" || strings.EqualFold(returnType, "void") || value.Kind == ValueNull {
		return Null, false
	}
	providerType := strings.ToLower(provider.Type)
	if !strings.Contains(providerType, "framework") && !strings.Contains(providerType, "apexmocks") {
		return Null, false
	}
	if collectionBase(returnType) != "" || isMapType(returnType) {
		return Null, false
	}
	return defaultValue(returnType, Null), true
}
func (vm *VM) callFrameworkStaticMember(className, method string, args []Value) (Value, bool, error) {
	switch {
	case strings.EqualFold(method, "triggerHandler") &&
		(strings.EqualFold(className, "framework_SObjectDomain") ||
			strings.EqualFold(shortTypeName(className), "SObjectDomain") ||
			strings.EqualFold(frameworkMockSupportType(className), "SObjectDomain")):
		return vm.callFrameworkSObjectDomainTriggerHandler(className, args)
	case strings.EqualFold(frameworkMockSupportType(className), "ApexMocks") && strings.EqualFold(method, "extractTypeName"):
		if len(args) != 1 {
			return Null, true, fmt.Errorf("framework_ApexMocks.extractTypeName expects 1 argument")
		}
		text := args[0].String()
		if before, _, ok := strings.Cut(text, ":"); ok {
			text = before
		}
		return String(text), true, nil
	case strings.EqualFold(frameworkMockSupportType(className), "Match") && strings.EqualFold(method, "matchesAllArgs"):
		if len(args) != 2 {
			return Null, true, fmt.Errorf("framework_Match.matchesAllArgs expects MethodArgValues and matchers")
		}
		matched, handled, err := vm.frameworkMatchesAllArgs(args[0], args[1])
		if !handled || err != nil {
			return Null, handled, err
		}
		return Bool(matched), true, nil
	case strings.EqualFold(frameworkMockSupportType(className), "Match") && strings.EqualFold(method, "validateArgs"):
		if len(args) != 2 {
			return Null, true, fmt.Errorf("framework_Match.validateArgs expects MethodArgValues and matchers")
		}
		_, handled, err := vm.frameworkMatchesAllArgs(args[0], args[1])
		if !handled || err != nil {
			return Null, handled, err
		}
		return Null, true, nil
	default:
		return Null, false, nil
	}
}
func (vm *VM) frameworkTriggerEventEnabled(frameworkClassName, domainClassName string) bool {
	field, _, ok := vm.lookupFrameworkSObjectDomainStaticField(frameworkClassName, "TriggerEventByClass")
	if !ok || field.Value.Kind != ValueMap {
		return true
	}
	event := Null
	key := mapKey(vm.typeForName("", domainClassName, false))
	if value, ok := field.Value.Map[key]; ok {
		event = value
	} else {
		for rawKey, value := range field.Value.Map {
			storedKey := mapStoredKey(field.Value, rawKey)
			if storedKey.Kind == ValueObject && strings.EqualFold(typeValueName(storedKey), domainClassName) {
				event = value
				break
			}
		}
	}
	if event.Kind != ValueObject {
		return true
	}
	fieldName := ""
	switch {
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isInsert"):
		fieldName = "BeforeInsertEnabled"
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isUpdate"):
		fieldName = "BeforeUpdateEnabled"
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isDelete"):
		fieldName = "BeforeDeleteEnabled"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isInsert"):
		fieldName = "AfterInsertEnabled"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isUpdate"):
		fieldName = "AfterUpdateEnabled"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isDelete"):
		fieldName = "AfterDeleteEnabled"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isUndelete"):
		fieldName = "AfterUndeleteEnabled"
	}
	if fieldName == "" {
		return true
	}
	_, enabled, ok := objectFieldValue(event, fieldName)
	return !ok || enabled.Kind != ValueBool || enabled.Bool
}

func (vm *VM) lookupFrameworkSObjectDomainStaticField(className, fieldName string) (Field, string, bool) {
	for _, candidate := range frameworkSObjectDomainClassCandidates(className) {
		field, owner, ok := vm.lookupStaticField(candidate, fieldName)
		if ok {
			return field, owner, true
		}
	}
	return Field{}, "", false
}

func frameworkSObjectDomainClassCandidates(className string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, 5)
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		key := strings.ToLower(candidate)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, candidate)
	}
	add(className)
	if short := shortTypeName(className); !strings.EqualFold(short, className) {
		add(short)
	}
	add("framework_SObjectDomain")
	add("fflib_SObjectDomain")
	add("SObjectDomain")
	return out
}

func (vm *VM) frameworkMockDatabaseContext(frameworkClassName string, after bool) (map[string]Value, bool) {
	if vm == nil {
		return nil, false
	}
	testField, _, ok := vm.lookupFrameworkSObjectDomainStaticField(frameworkClassName, "Test")
	if !ok || testField.Value.Kind != ValueObject {
		return nil, false
	}
	_, database, ok := objectFieldValue(testField.Value, "Database")
	if !ok || database.Kind != ValueObject {
		return nil, false
	}
	_, records, _ := objectFieldValue(database, "records")
	_, oldRecords, _ := objectFieldValue(database, "oldRecords")
	isInsert := objectBoolField(database, "isInsert")
	isUpdate := objectBoolField(database, "isUpdate")
	isDelete := objectBoolField(database, "isDelete")
	isUndelete := objectBoolField(database, "isUndelete")
	if records.Kind != ValueList {
		records = typedList("List<SObject>")
	}
	if oldRecords.Kind != ValueMap {
		oldRecords = typedMap("Map<Id,SObject>")
	}
	oldValues := valuesFromMap(oldRecords)
	oldList := typedList("List<SObject>")
	oldList.List = oldValues
	if records.Kind == ValueList {
		records = withConcreteSObjectListRuntime(records)
	}
	if oldList.Kind == ValueList {
		oldList = withConcreteSObjectListRuntime(oldList)
	}
	if !isInsert && !isUpdate && !isDelete && !isUndelete {
		return nil, false
	}
	if records.Kind == ValueList && len(records.List) == 0 && oldRecords.Kind == ValueMap && len(oldRecords.Map) == 0 {
		return nil, false
	}
	newValue := Null
	oldValue := Null
	newMap := Null
	oldMap := oldRecords
	switch {
	case isInsert || isUpdate || isUndelete:
		newValue = records
	case isDelete:
		oldValue = oldList
	}
	if isUpdate || isDelete {
		oldValue = oldList
	}
	return map[string]Value{
		"Trigger.new":           newValue,
		"Trigger.old":           oldValue,
		"Trigger.newMap":        newMap,
		"Trigger.oldMap":        oldMap,
		"Trigger.isExecuting":   Bool(true),
		"Trigger.isBefore":      Bool(!after),
		"Trigger.isAfter":       Bool(after),
		"Trigger.isInsert":      Bool(isInsert),
		"Trigger.isUpdate":      Bool(isUpdate),
		"Trigger.isDelete":      Bool(isDelete),
		"Trigger.isUndelete":    Bool(isUndelete),
		"Trigger.isUnDelete":    Bool(isUndelete),
		"Trigger.operationType": Value{Kind: ValueObject, Type: "TriggerOperation", Text: strings.ToUpper(map[bool]string{true: "AFTER", false: "BEFORE"}[after] + "_" + frameworkMockOperationName(isInsert, isUpdate, isDelete, isUndelete))},
		"Trigger.size":          Int(int64(frameworkMockTriggerSize(records, oldRecords))),
	}, true
}
func frameworkMockOperationName(isInsert, isUpdate, isDelete, isUndelete bool) string {
	switch {
	case isInsert:
		return "INSERT"
	case isUpdate:
		return "UPDATE"
	case isDelete:
		return "DELETE"
	case isUndelete:
		return "UNDELETE"
	default:
		return ""
	}
}
func frameworkMockTriggerSize(records, oldRecords Value) int {
	if records.Kind == ValueList && len(records.List) > 0 {
		return len(records.List)
	}
	if oldRecords.Kind == ValueMap {
		return len(oldRecords.Map)
	}
	return 0
}
func (vm *VM) frameworkMatchesAllArgs(methodArg Value, targetMatchers Value) (bool, bool, error) {
	if methodArg.Kind == ValueNull {
		return false, true, newExceptionError("framework_ApexMocks.ApexMocksException", "MethodArgs cannot be null")
	}
	if targetMatchers.Kind == ValueNull {
		return false, true, newExceptionError("framework_ApexMocks.ApexMocksException", "Matchers cannot be null")
	}
	_, argValues, ok := objectFieldValue(methodArg, "argValues")
	if !ok || argValues.Kind == ValueNull {
		return false, true, newExceptionError("framework_ApexMocks.ApexMocksException", "MethodArgs.argValues cannot be null")
	}
	if argValues.Kind != ValueList || targetMatchers.Kind != ValueList {
		return false, false, nil
	}
	if len(argValues.List) != len(targetMatchers.List) {
		return false, true, newExceptionError("framework_ApexMocks.ApexMocksException", vm.frameworkMatcherCountMessage(argValues, targetMatchers))
	}
	for i, arg := range argValues.List {
		matcher := targetMatchers.List[i]
		matched, handled, err := vm.frameworkMatcherMatchesMutable(&matcher, arg)
		if !handled || err != nil {
			return false, handled, err
		}
		if !matched && frameworkMatcherMatchesSObjectListElement(matcher, arg) {
			matched = true
		}
		if !matched {
			return false, true, nil
		}
		targetMatchers.List[i] = matcher
	}
	return true, true, nil
}
func frameworkMatcherMatchesSObjectListElement(matcher Value, arg Value) bool {
	if frameworkMatcherName(matcher.Type) != "anysobject" || arg.Kind != ValueList || len(arg.List) == 0 {
		return false
	}
	for _, item := range arg.List {
		if item.Kind != ValueObject || !sObjectValueType(item.Type) {
			return false
		}
	}
	return true
}
func (vm *VM) frameworkMatcherCountMessage(argValues, targetMatchers Value) string {
	message := "MethodArgs and matchers must have the same count"
	argsText, err := vm.displayString(argValues, resultForLookup())
	if err != nil {
		argsText = argValues.String()
	}
	matchersText, err := vm.displayString(targetMatchers, resultForLookup())
	if err != nil {
		matchersText = targetMatchers.String()
	}
	return fmt.Sprintf("%s, MethodArgs: (%d) %s, Matchers: (%d) %s", message, len(argValues.List), argsText, len(targetMatchers.List), matchersText)
}
func (vm *VM) callFrameworkMatcherMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if !isFrameworkMatcherType(receiver.Type) {
		return Null, false, nil
	}
	if !strings.EqualFold(method, "matches") {
		return Null, false, nil
	}
	if len(args) != 1 {
		return Null, true, fmt.Errorf("%s.matches expects 1 argument", receiver.Type)
	}
	matched, handled, err := vm.frameworkMatcherMatches(receiver, args[0])
	if !handled || err != nil {
		return Null, handled, err
	}
	return Bool(matched), true, nil
}
func isFrameworkMatcherType(typeName string) bool {
	switch frameworkMatcherName(typeName) {
	case "eq", "refeq", "anyboolean", "anydate", "anydatetime", "anydecimal", "anydouble", "anyid", "anyinteger", "anylong", "anylist", "anyobject", "isnotnull", "anystring", "anysobject", "anysobjectfield", "anysobjecttype", "isnull":
		return true
	default:
		return isArgumentCaptorAnyObjectType(typeName)
	}
}
func (vm *VM) frameworkMatcherMatches(matcher Value, arg Value) (bool, bool, error) {
	return vm.frameworkMatcherMatchesMutable(&matcher, arg)
}
func (vm *VM) frameworkMatcherMatchesMutable(matcher *Value, arg Value) (bool, bool, error) {
	if matcher == nil {
		return false, false, nil
	}
	if matcher.Kind != ValueObject {
		return false, false, nil
	}
	switch frameworkMatcherName(matcher.Type) {
	case "eq":
		_, toMatch, ok := objectFieldValue(*matcher, "toMatch")
		if !ok {
			return false, true, nil
		}
		if frameworkMatcherEquivalent(toMatch, arg) {
			return true, true, nil
		}
		if frameworkMatcherCanNativeEqual(toMatch) && frameworkMatcherCanNativeEqual(arg) {
			return ok && toMatch.Equal(arg), true, nil
		}
		if frameworkMatcherNeedsApexEquality(toMatch) || frameworkMatcherNeedsApexEquality(arg) {
			return false, false, nil
		}
		return ok && toMatch.Equal(arg), true, nil
	case "refeq":
		_, toMatch, ok := objectFieldValue(*matcher, "toMatch")
		if !ok {
			return false, true, nil
		}
		return valueAliasMatch(toMatch, arg), true, nil
	case "anyboolean":
		return arg.Kind == ValueBool, true, nil
	case "anydate":
		return arg.Kind == ValueObject && strings.EqualFold(arg.Type, "Date"), true, nil
	case "anydatetime":
		return arg.Kind == ValueObject && (strings.EqualFold(arg.Type, "Datetime") || strings.EqualFold(arg.Type, "Date")), true, nil
	case "anydecimal", "anydouble":
		return arg.Kind == ValueDecimal || arg.Kind == ValueInt, true, nil
	case "anyid":
		return arg.Kind == ValueObject && strings.EqualFold(arg.Type, "Id") || arg.Kind == ValueString && looksLikeID(arg.Text), true, nil
	case "anyinteger":
		return arg.Kind == ValueInt && !strings.EqualFold(arg.Type, "Long"), true, nil
	case "anylong":
		return arg.Kind == ValueInt, true, nil
	case "anylist":
		return arg.Kind == ValueList, true, nil
	case "anyobject", "isnotnull":
		if isArgumentCaptorAnyObjectType(matcher.Type) && matcher.Fields != nil {
			vm.advanceAliasContainmentMutation()
			matcher.Fields["value"] = arg
		}
		return arg.Kind != ValueNull, true, nil
	case "anystring":
		return arg.Kind == ValueString || arg.Kind == ValueObject && strings.EqualFold(arg.Type, "Id"), true, nil
	case "anysobject":
		return arg.Kind == ValueObject && sObjectValueType(arg.Type), true, nil
	case "anysobjectfield":
		return arg.Kind == ValueObject && strings.EqualFold(arg.Type, "Schema.SObjectField"), true, nil
	case "anysobjecttype":
		return arg.Kind == ValueObject && strings.EqualFold(arg.Type, "Schema.SObjectType"), true, nil
	case "isnull":
		return arg.Kind == ValueNull, true, nil
	case "stringcontains":
		text, ok := frameworkMatcherStringArg(arg)
		if !ok {
			return false, true, nil
		}
		needle, ok := frameworkMatcherStringField(*matcher, "toMatch")
		return ok && strings.Contains(text, needle), true, nil
	case "stringstartswith":
		text, ok := frameworkMatcherStringArg(arg)
		if !ok {
			return false, true, nil
		}
		prefix, ok := frameworkMatcherStringField(*matcher, "toMatch")
		return ok && strings.HasPrefix(text, prefix), true, nil
	case "stringendswith":
		text, ok := frameworkMatcherStringArg(arg)
		if !ok {
			return false, true, nil
		}
		suffix, ok := frameworkMatcherStringField(*matcher, "toMatch")
		return ok && strings.HasSuffix(text, suffix), true, nil
	case "stringisblank":
		if arg.Kind == ValueNull {
			return true, true, nil
		}
		text, ok := frameworkMatcherStringArg(arg)
		return ok && strings.TrimSpace(text) == "", true, nil
	case "stringisnotblank":
		text, ok := frameworkMatcherStringArg(arg)
		return ok && strings.TrimSpace(text) != "", true, nil
	case "stringmatches":
		text, ok := frameworkMatcherStringArg(arg)
		if !ok {
			return false, true, nil
		}
		pattern, ok := frameworkMatcherPatternField(*matcher)
		if !ok {
			return false, true, nil
		}
		matched, err := regexp.MatchString("^(?:"+pattern+")$", text)
		if err != nil {
			return false, true, err
		}
		return matched, true, nil
	case "combined":
		return vm.frameworkCombinedMatcherMatches(*matcher, arg)
	default:
		return vm.frameworkMatcherMatchesViaApex(*matcher, arg)
	}
}
func frameworkMatcherStringArg(value Value) (string, bool) {
	if value.Kind == ValueString {
		return value.Text, true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "String") {
		text, err := platformScalarText(value, "String")
		return text, err == nil
	}
	return "", false
}
func frameworkMatcherStringField(value Value, fieldName string) (string, bool) {
	_, field, ok := objectFieldValue(value, fieldName)
	if !ok {
		return "", false
	}
	return frameworkMatcherStringArg(field)
}
func frameworkMatcherPatternField(value Value) (string, bool) {
	if text, ok := frameworkMatcherStringField(value, "pat"); ok {
		return text, true
	}
	_, pattern, ok := objectFieldValue(value, "pat")
	if !ok || pattern.Kind != ValueObject {
		return "", false
	}
	for _, name := range []string{"pattern", "regex", "source"} {
		if text, ok := frameworkMatcherStringField(pattern, name); ok {
			return text, true
		}
	}
	if pattern.Text != "" {
		return pattern.Text, true
	}
	return "", false
}
func (vm *VM) frameworkCombinedMatcherMatches(matcher Value, arg Value) (bool, bool, error) {
	_, matchers, ok := objectFieldValue(matcher, "internalMatchers")
	if !ok || matchers.Kind != ValueList {
		return false, true, nil
	}
	_, connective, ok := objectFieldValue(matcher, "connectiveExpression")
	if !ok {
		return false, true, nil
	}
	mode := strings.ToLower(connective.Text)
	if mode == "" {
		mode = strings.ToLower(connective.String())
	}
	allMatched := true
	anyMatched := false
	for _, inner := range matchers.List {
		matched, handled, err := vm.frameworkMatcherMatches(inner, arg)
		if !handled || err != nil {
			return false, handled, err
		}
		if matched {
			anyMatched = true
		} else {
			allMatched = false
		}
	}
	switch {
	case strings.Contains(mode, "at_least_one"):
		return anyMatched, true, nil
	case strings.Contains(mode, "none"):
		return !anyMatched, true, nil
	default:
		return allMatched, true, nil
	}
}
func (vm *VM) frameworkMatcherMatchesViaApex(matcher Value, arg Value) (bool, bool, error) {
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(matcher.Type, "matches", []Value{arg})
	if ambiguous {
		return false, true, vm.ambiguousOverloadError(matcher.Type+".matches", []Value{arg})
	}
	if !ok {
		return false, false, nil
	}
	value, err := vm.callMethodWithReceiver(target, matcher, []Value{arg}, resultForLookup())
	if err != nil {
		return false, true, err
	}
	if value.Kind != ValueBool {
		return false, true, fmt.Errorf("%s.matches must return Boolean", matcher.Type)
	}
	return value.Bool, true, nil
}
func frameworkMatcherEquivalent(left, right Value) bool {
	return frameworkMatcherEquivalentSeen(left, right, make(map[[2]uint64]bool))
}
func frameworkMatcherEquivalentSeen(left, right Value, seen map[[2]uint64]bool) bool {
	if valueAliasMatch(left, right) || left.Equal(right) {
		return true
	}
	if left.Kind == ValueString && right.Kind == ValueString && frameworkMatcherPagePathEqual(left.Text, right.Text) {
		return true
	}
	if left.Kind != right.Kind || !strings.EqualFold(left.Type, right.Type) {
		return false
	}
	if left.Ref != 0 || right.Ref != 0 {
		key := [2]uint64{left.Ref, right.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	switch left.Kind {
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if !frameworkMatcherEquivalentSeen(left.List[i], right.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(left.Set) != len(right.Set) {
			return false
		}
		used := make([]bool, len(right.Set))
		for _, leftValue := range left.Set {
			found := false
			for i, rightValue := range right.Set {
				if used[i] || !frameworkMatcherEquivalentSeen(leftValue, rightValue, seen) {
					continue
				}
				used[i] = true
				found = true
				break
			}
			if !found {
				return false
			}
		}
		return true
	case ValueMap:
		if len(left.Map) != len(right.Map) {
			return false
		}
		for key, leftValue := range left.Map {
			rightValue, ok := right.Map[key]
			if !ok || !frameworkMatcherEquivalentSeen(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	case ValueObject:
		if platformScalarObject(left.Type) || strings.EqualFold(left.Type, "Type") || strings.HasPrefix(left.Type, "Schema.") {
			return false
		}
		if len(left.Fields) != len(right.Fields) {
			return false
		}
		for key, leftValue := range left.Fields {
			rightValue, ok := right.Fields[key]
			if !ok || !frameworkMatcherEquivalentSeen(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func frameworkMatcherPagePathEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if !strings.HasPrefix(left, "/") || !strings.HasPrefix(right, "/") {
		return false
	}
	return strings.EqualFold(left, right)
}

func frameworkMatcherCanNativeEqual(value Value) bool {
	if value.Kind != ValueObject {
		return true
	}
	if sObjectValueType(value.Type) {
		return true
	}
	return strings.EqualFold(value.Type, "Type") ||
		strings.HasPrefix(value.Type, "Schema.") ||
		platformScalarObject(value.Type) ||
		value.Text != ""
}
func frameworkMatcherNeedsApexEquality(value Value) bool {
	switch value.Kind {
	case ValueObject, ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
}
func frameworkMatcherName(typeName string) string {
	name := strings.ToLower(strings.TrimSpace(typeName))
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return name
}
func (vm *VM) callFrameworkNamespacedAttributeMap(receiver Value, method string, args []Value) (Value, bool, error) {
	values, ok := frameworkNamespacedAttributeMapValues(receiver)
	if !ok {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "get":
		if len(args) != 1 && len(args) != 2 {
			return Null, true, fmt.Errorf("%s.get expects name", receiver.Type)
		}
		if args[0].Kind == ValueNull {
			return Null, true, nil
		}
		if args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("%s.get expects String", receiver.Type)
		}
		implyNamespace := true
		if len(args) == 2 {
			if args[1].Kind != ValueBool {
				return Null, true, fmt.Errorf("%s.get implyNamespace expects Boolean", receiver.Type)
			}
			implyNamespace = args[1].Bool
		}
		return vm.frameworkNamespacedAttributeMapGet(values, args[0].Text, implyNamespace, frameworkNamespacedAttributeMapNamespace(receiver)), true, nil
	case "containskey":
		if len(args) != 1 && len(args) != 2 {
			return Null, true, fmt.Errorf("%s.containsKey expects name", receiver.Type)
		}
		if args[0].Kind == ValueNull {
			return Null, true, nil
		}
		if args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("%s.containsKey expects String", receiver.Type)
		}
		implyNamespace := true
		if len(args) == 2 {
			if args[1].Kind != ValueBool {
				return Null, true, fmt.Errorf("%s.containsKey implyNamespace expects Boolean", receiver.Type)
			}
			implyNamespace = args[1].Bool
		}
		value := vm.frameworkNamespacedAttributeMapGet(values, args[0].Text, implyNamespace, frameworkNamespacedAttributeMapNamespace(receiver))
		return Bool(value.Kind != ValueNull), true, nil
	case "values":
		if !strings.EqualFold(receiver.Type, "framework_SObjectDescribe.FieldsMap") {
			return Null, false, nil
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.FieldsMap.values expects 0 arguments")
		}
		out := make([]Value, 0, len(values.Map))
		for _, key := range sortedMapKeys(values.Map) {
			out = append(out, values.Map[key])
		}
		return List(out...), true, nil
	case "size":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.size expects 0 arguments", receiver.Type)
		}
		return Int(int64(len(values.Map))), true, nil
	}
	return Null, false, nil
}
func frameworkNamespacedAttributeMapValues(receiver Value) (Value, bool) {
	_, values, ok := objectFieldValue(receiver, "values")
	return values, ok && values.Kind == ValueMap
}
func frameworkNamespacedAttributeMapNamespace(receiver Value) string {
	_, namespace, ok := objectFieldValue(receiver, "currentNamespace")
	if ok && namespace.Kind == ValueString {
		return namespace.Text
	}
	return ""
}
func (vm *VM) frameworkNamespacedAttributeMapGet(values Value, name string, implyNamespace bool, namespace string) Value {
	if strings.TrimSpace(name) == "" {
		return Null
	}
	keys := []string{strings.ToLower(name)}
	if implyNamespace {
		if namespace == "" && vm.Org != nil {
			namespace = vm.Org.Namespace
		}
		if namespace != "" {
			keys = append([]string{strings.ToLower(storage.NamespaceTokenName(namespace, name))}, keys...)
		}
	}
	for _, keyText := range keys {
		if value, ok := values.Map[mapKey(String(keyText))]; ok {
			return value
		}
		if value, ok := values.Map[mapKey(String(strings.ToLower(stripAnyNamespaceToken(keyText))))]; ok {
			return value
		}
		if value, ok := vm.specialMapLookup(values, String(keyText)); ok {
			return value
		}
	}
	return Null
}
func frameworkMapPut(target Value, key Value, value Value) Value {
	if target.Kind != ValueMap {
		return target
	}
	encoded := mapKey(key)
	if _, exists := target.Map[encoded]; !exists {
		target.MapOrder = append(target.MapOrder, encoded)
	}
	target.Map[encoded] = value
	if target.MapKeys == nil {
		target.MapKeys = make(map[string]Value)
	}
	target.MapKeys[encoded] = key
	return target
}
func (vm *VM) callFrameworkMockRecorderMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch frameworkMockSupportType(receiver.Type) {
	case "InvocationOnMock":
		return vm.callFrameworkInvocationOnMockMember(receiver, method, args)
	case "MethodCountRecorder":
		return vm.callFrameworkMethodCountRecorderMember(receiver, method, args)
	case "MethodReturnValueRecorder":
		return vm.callFrameworkMethodReturnValueRecorderMember(receiver, method, args)
	case "ArgumentCaptor":
		return vm.callFrameworkArgumentCaptorMember(receiver, method, args)
	case "ArgumentCaptor.AnyObject":
		return vm.callFrameworkArgumentCaptorAnyObjectMember(receiver, method, args)
	default:
		return Null, false, nil
	}
}
func frameworkMockSupportType(typeName string) string {
	for _, prefix := range []string{"framework_", "fflib_"} {
		if hasPrefixFold(typeName, prefix) {
			return typeName[len(prefix):]
		}
	}
	short := shortTypeName(typeName)
	if short != typeName {
		for _, prefix := range []string{"framework_", "fflib_"} {
			if hasPrefixFold(short, prefix) {
				return short[len(prefix):]
			}
		}
	}
	return ""
}
func frameworkMockSupportTypesMatch(left, right string) bool {
	leftSupport := frameworkMockSupportType(left)
	rightSupport := frameworkMockSupportType(right)
	return leftSupport != "" && strings.EqualFold(leftSupport, rightSupport)
}
func (vm *VM) callFrameworkArgumentCaptorMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch strings.ToLower(method) {
	case "getvalue":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_ArgumentCaptor.getValue expects 0 arguments")
		}
		_, values, ok := objectFieldValue(receiver, "argumentsCaptured")
		if !ok || values.Kind != ValueList || len(values.List) == 0 {
			return Null, true, nil
		}
		return values.List[len(values.List)-1], true, nil
	case "getallvalues":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_ArgumentCaptor.getAllValues expects 0 arguments")
		}
		_, values, ok := objectFieldValue(receiver, "argumentsCaptured")
		if !ok || values.Kind != ValueList {
			return List(), true, nil
		}
		return values, true, nil
	default:
		return Null, false, nil
	}
}
func (vm *VM) callFrameworkArgumentCaptorAnyObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch strings.ToLower(method) {
	case "matches":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("framework_ArgumentCaptor.AnyObject.matches expects 1 argument")
		}
		vm.advanceAliasContainmentMutation()
		receiver.Fields["value"] = args[0]
		vm.propagateValueMutationToScope(vm.Globals, receiver, receiver)
		vm.propagateValueMutationToStatics(receiver, receiver)
		return Bool(true), true, nil
	case "storeargument":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_ArgumentCaptor.AnyObject.storeArgument expects 0 arguments")
		}
		_, captor, ok := objectFieldValue(receiver, "captor")
		if !ok || captor.Kind != ValueObject {
			return Null, true, nil
		}
		_, captured, ok := objectFieldValue(captor, "argumentsCaptured")
		if !ok || captured.Kind != ValueList {
			captured = typedList("List<Object>")
		}
		_, value, ok := objectFieldValue(receiver, "value")
		if !ok {
			value = Null
		}
		vm.advanceAliasContainmentMutation()
		captured.List = append(captured.List, value)
		captor.Fields["argumentsCaptured"] = captured
		receiver.Fields["captor"] = captor
		vm.propagateValueMutationToScope(vm.Globals, captor, captor)
		vm.propagateValueMutationToStatics(captor, captor)
		vm.propagateValueMutationToScope(vm.Globals, receiver, receiver)
		vm.propagateValueMutationToStatics(receiver, receiver)
		return Null, true, nil
	default:
		return Null, false, nil
	}
}
func (vm *VM) callFrameworkInvocationOnMockMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch strings.ToLower(method) {
	case "getmethod":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_InvocationOnMock.getMethod expects 0 arguments")
		}
		_, value, ok := objectFieldValue(receiver, "qm")
		if !ok {
			return Null, true, nil
		}
		return value, true, nil
	case "getmethodargvalues":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_InvocationOnMock.getMethodArgValues expects 0 arguments")
		}
		_, value, ok := objectFieldValue(receiver, "methodArg")
		if !ok {
			return Null, true, nil
		}
		return value, true, nil
	case "getarguments":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_InvocationOnMock.getArguments expects 0 arguments")
		}
		_, methodArg, ok := objectFieldValue(receiver, "methodArg")
		if !ok {
			return List(), true, nil
		}
		_, values, ok := objectFieldValue(methodArg, "argValues")
		if !ok || values.Kind != ValueList {
			return List(), true, nil
		}
		return values, true, nil
	case "getargument":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, true, fmt.Errorf("framework_InvocationOnMock.getArgument expects Integer")
		}
		_, methodArg, ok := objectFieldValue(receiver, "methodArg")
		if !ok {
			return Null, true, nil
		}
		_, values, ok := objectFieldValue(methodArg, "argValues")
		if !ok || values.Kind != ValueList {
			return Null, true, nil
		}
		index := int(args[0].Int)
		if index < 0 || index >= len(values.List) {
			return Null, true, newExceptionError("framework_ApexMocks.ApexMocksException", fmt.Sprintf("Invalid index, must be greater or equal to zero and less of %d.", len(values.List)))
		}
		return values.List[index], true, nil
	case "getmock":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_InvocationOnMock.getMock expects 0 arguments")
		}
		_, value, ok := objectFieldValue(receiver, "mockInstance")
		if !ok {
			return Null, true, nil
		}
		return value, true, nil
	default:
		return Null, false, nil
	}
}
func (vm *VM) callFrameworkMethodCountRecorderMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch strings.ToLower(method) {
	case "recordmethod":
		if len(args) != 1 || args[0].Kind != ValueObject || frameworkMockSupportType(args[0].Type) != "InvocationOnMock" {
			return Null, true, fmt.Errorf("framework_MethodCountRecorder.recordMethod expects framework_InvocationOnMock")
		}
		if err := vm.frameworkRecordMethodInvocation(args[0]); err != nil {
			return Null, true, err
		}
		return Null, true, nil
	case "getorderedmethodcalls":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_MethodCountRecorder.getOrderedMethodCalls expects 0 arguments")
		}
		if value, ok := vm.frameworkMethodCountRecorderStatic("orderedMethodCalls"); ok {
			return value, true, nil
		}
		return List(), true, nil
	case "getmethodargumentsbytypename":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_MethodCountRecorder.getMethodArgumentsByTypeName expects 0 arguments")
		}
		if value, ok := vm.frameworkMethodCountRecorderStatic("methodArgumentsByTypeName"); ok {
			return value, true, nil
		}
		return Map(), true, nil
	default:
		return Null, false, nil
	}
}
func (vm *VM) callFrameworkMethodReturnValueRecorderMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if !strings.EqualFold(method, "getMethodReturnValue") {
		return Null, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueObject || frameworkMockSupportType(args[0].Type) != "InvocationOnMock" {
		return Null, true, fmt.Errorf("framework_MethodReturnValueRecorder.getMethodReturnValue expects framework_InvocationOnMock")
	}
	_, byMethod, ok := objectFieldValue(receiver, "matcherReturnValuesByMethod")
	if !ok || byMethod.Kind != ValueMap {
		return Null, true, nil
	}
	methodValue, ok := frameworkInvocationMethod(args[0])
	if !ok {
		return Null, true, nil
	}
	matchersForMethods, ok := byMethod.Map[vm.mapKey(methodValue)]
	if !ok {
		if fallback, found, err := vm.objectKeyMapLookup(byMethod, methodValue); err != nil {
			return Null, true, err
		} else if found {
			matchersForMethods = fallback
			ok = true
		}
	}
	if !ok {
		return Null, true, nil
	}
	_, methodArg, ok := objectFieldValue(args[0], "methodArg")
	if !ok {
		return Null, true, nil
	}
	if matchersForMethods.Kind != ValueList {
		return Null, true, nil
	}
	for i := len(matchersForMethods.List) - 1; i >= 0; i-- {
		item := matchersForMethods.List[i]
		_, matchers, ok := objectFieldValue(item, "matchers")
		if !ok {
			continue
		}
		matched, handled, err := vm.frameworkMatchesAllArgs(methodArg, matchers)
		if err != nil {
			return Null, true, err
		}
		if !handled || !matched {
			continue
		}
		_, returnValue, ok := objectFieldValue(item, "ReturnValue")
		if !ok {
			return Null, true, nil
		}
		return returnValue, true, nil
	}
	return Null, true, nil
}
func (vm *VM) frameworkRecordMethodInvocation(invocation Value) error {
	if _, mockInstance, ok := objectFieldValue(invocation, "mockInstance"); ok && isStubProxy(mockInstance) {
		if vm.callStackHasFieldInitializerForType(mockInstance.Type) || (isSelectorMockType(mockInstance.Type) && vm.callStackHasFieldInitializer()) {
			return nil
		}
	}
	methodValue, ok := frameworkInvocationMethod(invocation)
	if !ok {
		return nil
	}
	_, methodArg, ok := objectFieldValue(invocation, "methodArg")
	if !ok {
		methodArg = Null
	}
	byMethod, ok := vm.frameworkMethodCountRecorderStatic("methodArgumentsByTypeName")
	if !ok || byMethod.Kind != ValueMap {
		byMethod = typedMap("Map<framework_QualifiedMethod,List<framework_MethodArgValues>>")
	}
	if byMethod.Map == nil {
		byMethod.Map = make(map[string]Value)
	}
	if byMethod.MapKeys == nil {
		byMethod.MapKeys = make(map[string]Value)
	}
	key := vm.mapKey(methodValue)
	methodArgs, ok := byMethod.Map[key]
	if !ok || methodArgs.Kind != ValueList {
		methodArgs = List()
		methodArgs.Type = "List<framework_MethodArgValues>"
	}
	methodArgs.List = append(methodArgs.List, methodArg)
	byMethod.Map[key] = methodArgs
	byMethod.MapKeys[key] = methodValue
	vm.setFrameworkMethodCountRecorderStatic("methodArgumentsByTypeName", byMethod)
	ordered, ok := vm.frameworkMethodCountRecorderStatic("orderedMethodCalls")
	if !ok || ordered.Kind != ValueList {
		ordered = List()
		ordered.Type = "List<framework_InvocationOnMock>"
	}
	ordered.List = append(ordered.List, invocation)
	vm.setFrameworkMethodCountRecorderStatic("orderedMethodCalls", ordered)
	return nil
}
func (vm *VM) frameworkMockRecordingModeActive() bool {
	for _, value := range vm.Globals {
		if frameworkApexMocksProviderActive(value) || stubProviderStateFlagSet(value) {
			return true
		}
	}
	return false
}
func frameworkInvocationMethod(invocation Value) (Value, bool) {
	_, value, ok := objectFieldValue(invocation, "qm")
	return value, ok && value.Kind == ValueObject && frameworkMockSupportType(value.Type) == "QualifiedMethod"
}
func (vm *VM) frameworkMethodCountRecorderStatic(name string) (Value, bool) {
	for _, className := range []string{"fflib_MethodCountRecorder", "framework_MethodCountRecorder"} {
		class, ok := vm.ensureMutableClass(className)
		if !ok {
			continue
		}
		for fieldName, field := range class.StaticFields {
			if strings.EqualFold(fieldName, name) {
				return field.Value, true
			}
		}
	}
	return Null, false
}
func (vm *VM) setFrameworkMethodCountRecorderStatic(name string, value Value) {
	updated := false
	for _, className := range []string{"fflib_MethodCountRecorder", "framework_MethodCountRecorder"} {
		class, ok := vm.ensureMutableClass(className)
		if !ok {
			continue
		}
		vm.advanceAliasContainmentMutation()
		found := false
		for fieldName, field := range class.StaticFields {
			if strings.EqualFold(fieldName, name) {
				vm.captureFrameworkMethodCountRecorderRollback(fieldName, field.Value)
				field.Value = value
				class.StaticFields[fieldName] = field
				vm.rememberStaticValueRefsInField(value, canonicalStaticFieldLocationForClass(class, class.Name, fieldName))
				found = true
				break
			}
		}
		if !found {
			if class.StaticFields == nil {
				class.StaticFields = make(map[string]Field)
			}
			class.StaticFields[name] = Field{Name: name, Type: value.Type, Static: true, Value: value, InitialValue: value}
			vm.rememberStaticValueRefsInField(value, canonicalStaticFieldLocationForClass(class, class.Name, name))
		}
		vm.storeClassValue(class)
		updated = true
	}
	if updated {
		return
	}
}
func (vm *VM) beginFrameworkMethodCountRecorderRollback() *frameworkMethodCountRecorderRollback {
	rollback := &frameworkMethodCountRecorderRollback{
		previous: vm.frameworkRecorderRollback,
		values:   make(map[string]Value, 2),
	}
	vm.frameworkRecorderRollback = rollback
	return rollback
}
func (vm *VM) endFrameworkMethodCountRecorderRollback(rollback *frameworkMethodCountRecorderRollback, restore bool) {
	if rollback == nil {
		return
	}
	vm.frameworkRecorderRollback = rollback.previous
	if !restore {
		return
	}
	for name, value := range rollback.values {
		vm.setFrameworkMethodCountRecorderStatic(name, cloneValue(value))
	}
}
func (vm *VM) captureFrameworkMethodCountRecorderRollback(name string, value Value) {
	rollback := vm.frameworkRecorderRollback
	if rollback == nil {
		return
	}
	if _, exists := rollback.values[name]; exists {
		return
	}
	rollback.values[name] = cloneValue(value)
}
