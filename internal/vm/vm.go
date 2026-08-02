package vm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func industryMapResult() Value {
	result := typedMap("Map<String,Object>")
	for key, value := range map[string]Value{
		"success": Bool(true),
		"records": typedList("List<Object>"),
		"errors":  typedList("List<Object>"),
	} {
		encoded := mapKey(String(key))
		result.Map[encoded] = value
		result.MapKeys[encoded] = String(key)
	}
	return result
}

func (vm *VM) orgID() string {
	if vm.Org != nil && vm.Org.OrgID != "" {
		return vm.Org.OrgID
	}
	return "00D000000000001"
}

func firstStringField(record storage.Record, names ...string) string {
	for _, name := range names {
		value, ok := record.GetField(name)
		if ok && value.Kind == storage.ValueString {
			return value.String
		}
	}
	return ""
}

func userInfoField(user Value, field, fallback string) string {
	if value, ok := userInfoFieldValue(user, field); ok {
		return value
	}
	return fallback
}

func userInfoFieldValue(user Value, field string) (string, bool) {
	if user.Kind == ValueObject {
		_, value, ok := objectFieldValue(user, field)
		if ok {
			if value.Kind == ValueString {
				return value.Text, true
			}
			if value.Kind == ValueObject {
				if raw, err := platformScalarText(value, value.Type); err == nil {
					return raw, true
				}
			}
		}
		return "", false
	}
	if user.Kind == ValueString {
		if !strings.EqualFold(field, "Id") && !strings.EqualFold(field, "Username") {
			return "", false
		}
		return user.Text, true
	}
	return "", false
}

func recordFieldString(record storage.Record, field string) string {
	if record.Fields == nil {
		return ""
	}
	value, ok := record.Fields[field]
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return value.String
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueDecimal:
		return value.Decimal
	default:
		return ""
	}
}

func (vm *VM) currentUserInfoField(field, fallback string) string {
	if vm.testContext != nil {
		if value, ok := userInfoFieldValue(vm.testContext.CurrentUser, field); ok {
			return value
		}
		if value, ok := vm.currentUserStoredField(vm.testContext.CurrentUser, field); ok {
			return value
		}
		if strings.EqualFold(field, "UserType") {
			return vm.userTypeFromCurrentUserProfile(vm.testContext.CurrentUser, fallback)
		}
		return fallback
	}
	if vm.executionUser.Kind != "" && vm.executionUser.Kind != ValueNull {
		if value, ok := userInfoFieldValue(vm.executionUser, field); ok {
			return value
		}
		if value, ok := vm.currentUserStoredField(vm.executionUser, field); ok {
			return value
		}
		if strings.EqualFold(field, "UserType") {
			return vm.userTypeFromCurrentUserProfile(vm.executionUser, fallback)
		}
		return fallback
	}
	return fallback
}

func (vm *VM) currentUserStoredField(user Value, field string) (string, bool) {
	if vm == nil || vm.Org == nil || strings.TrimSpace(field) == "" {
		return "", false
	}
	userID := userInfoField(user, "Id", "")
	if strings.TrimSpace(userID) == "" {
		return "", false
	}
	users, ok := vm.Org.Objects["User"]
	if !ok {
		return "", false
	}
	_, record, ok := storage.LookupRecordByID(users.Records, storage.ID(userID))
	if !ok {
		return "", false
	}
	if strings.EqualFold(field, "Id") {
		return string(record.ID), true
	}
	if value, ok := record.GetField(field); ok {
		switch value.Kind {
		case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
			return value.String, true
		case storage.ValueID:
			return string(value.ID), true
		case storage.ValueDecimal:
			return value.Decimal, true
		case storage.ValueBoolean:
			if value.Boolean {
				return "true", true
			}
			return "false", true
		}
	}
	return "", false
}

func (vm *VM) userTypeFromCurrentUserProfile(user Value, fallback string) string {
	if vm == nil || vm.Org == nil {
		return fallback
	}
	profileID := userInfoField(user, "ProfileId", "")
	if strings.TrimSpace(profileID) == "" {
		if value, ok := vm.currentUserStoredField(user, "ProfileId"); ok {
			profileID = value
		}
	}
	if strings.TrimSpace(profileID) == "" {
		return fallback
	}
	profiles, ok := vm.Org.Objects["Profile"]
	if !ok {
		return fallback
	}
	profile, ok := profiles.Records[storage.ID(profileID)]
	if !ok {
		return fallback
	}
	if value, ok := profile.GetField("UserType"); ok && strings.TrimSpace(value.String) != "" {
		return value.String
	}
	name := strings.ToLower(strings.TrimSpace(recordFieldString(profile, "Name")))
	switch {
	case strings.Contains(name, "guest"):
		return "Guest"
	case strings.Contains(name, "community hub login user plus"):
		return "PowerCustomerSuccess"
	case strings.Contains(name, "community hub login"):
		return "CspLitePortal"
	case strings.Contains(name, "customer community plus"):
		return "PowerCustomerSuccess"
	case strings.Contains(name, "customer community"):
		return "CspLitePortal"
	default:
		return fallback
	}
}

func (vm *VM) currentUserTimeZoneID() string {
	return vm.currentUserInfoField("TimeZoneSidKey", "UTC")
}

func stringField(value Value, field string) string {
	if value.Kind != ValueObject {
		return ""
	}
	_, raw, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	switch raw.Kind {
	case ValueString:
		return raw.Text
	case ValueObject:
		if strings.EqualFold(raw.Type, "Id") && raw.Text != "" {
			return raw.Text
		}
		if raw.Text != "" {
			return raw.Text
		}
		if nested, ok := raw.Fields["value"]; ok && nested.Kind == ValueString {
			return nested.Text
		}
	}
	return ""
}

func storageIDValueEquals(value storage.Value, text string) bool {
	return storageValueIDText(value) == text
}

func storageValueIDText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}

func storageStringValueEquals(value storage.Value, text string) bool {
	return value.Kind == storage.ValueString && strings.EqualFold(value.String, text)
}

func (vm *VM) shouldEnqueueFuture(method Method) bool {
	if vm.testContext == nil || vm.testContext.Draining {
		return false
	}
	return methodHasModifier(method.Modifiers, "future")
}

func (vm *VM) enqueueFuture(method Method, args []Value, result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, nil
	}
	if !method.IsStatic {
		return Null, fmt.Errorf("@future method %s must be static", method.Name)
	}
	if err := vm.incrementLimit("asyncJobs", 1); err != nil {
		return Null, err
	}
	if err := vm.incrementLimit("futureCalls", 1); err != nil {
		return Null, err
	}
	if len(args) != len(method.Params) {
		return Null, fmt.Errorf("%s expects %d arguments", method.Name, len(method.Params))
	}
	coercedArgs := make([]Value, len(args))
	resolutionClass := vm.methodTypeResolutionClass(method)
	for i, param := range method.Params {
		paramType := vm.resolveTypeNameInClass(resolutionClass, param.Type)
		coerced, err := vm.coerceAssignable(paramType, vm.valueWithTypesResolvedInClass(resolutionClass, args[i]))
		if err != nil {
			return Null, fmt.Errorf("%s parameter %s: %w", method.Name, param.Name, err)
		}
		coerced.Static = paramType
		coercedArgs[i] = coerced
	}
	job := AsyncJob{
		ID:     vm.nextAsyncJobID(),
		Kind:   "Future",
		Method: method,
		Args:   coercedArgs,
	}
	vm.testContext.AsyncJobs = append(vm.testContext.AsyncJobs, job)
	vm.recordAsyncJob(job, "Queued", "")
	appendTrace(result, "apex.async.enqueue", "apex.async", map[string]any{
		"kind":   job.Kind,
		"jobId":  job.ID,
		"method": method.Name,
	})
	return Null, nil
}

func (vm *VM) assertError(message string) error {
	return &RuntimeError{
		Type:    "System.AssertException",
		Message: message,
		Stack:   vm.stackFrames(),
	}
}

func (vm *VM) apexPagesMessagesFromValue(value Value, result *Result) ([]Value, error) {
	if value.Kind == ValueList {
		messages := make([]Value, 0, len(value.List))
		for _, item := range value.List {
			nested, err := vm.apexPagesMessagesFromValue(item, result)
			if err != nil {
				return nil, err
			}
			messages = append(messages, nested...)
		}
		return messages, nil
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "ApexPages.Message") {
		return []Value{value}, nil
	}
	if value.Kind == ValueObject && isExceptionType(value.Type) {
		summary := ""
		if _, message, ok := objectFieldValue(value, "message"); ok {
			text, err := vm.displayString(message, result)
			if err != nil {
				return nil, err
			}
			summary = text
		}
		if summary == "" {
			summary = value.String()
		}
		message := Object("ApexPages.Message")
		severity, _ := apexPagesSeverityStaticValue("ApexPages.Severity.ERROR")
		message.Fields["severity"] = severity
		message.Fields["summary"] = String(summary)
		message.Fields["detail"] = String(summary)
		return []Value{message}, nil
	}
	return nil, fmt.Errorf("ApexPages.addMessages expects Exception or ApexPages.Message list")
}

func (vm *VM) addApexPageMessages(messages []Value) {
	for _, message := range messages {
		vm.addApexPageMessage(message)
	}
}

func (vm *VM) addApexPageMessage(message Value) {
	for _, existing := range vm.pageMessages {
		if apexPagesMessagesEquivalent(existing, message) {
			return
		}
	}
	vm.pageMessages = append(vm.pageMessages, message)
}

func apexPagesMessagesEquivalent(left, right Value) bool {
	if left.Kind != ValueObject || right.Kind != ValueObject ||
		!strings.EqualFold(left.Type, "ApexPages.Message") ||
		!strings.EqualFold(right.Type, "ApexPages.Message") {
		return left.Equal(right)
	}
	if !apexPagesMessageSeverityEqual(left, right) {
		return false
	}
	return apexPagesMessageFieldEqual(left, right, "summary") &&
		apexPagesMessageFieldEqual(left, right, "detail")
}

func apexPagesMessageSeverityEqual(left, right Value) bool {
	leftSeverity, leftOK := apexPagesSeverityName(left.Fields["severity"])
	rightSeverity, rightOK := apexPagesSeverityName(right.Fields["severity"])
	if leftOK || rightOK {
		return leftOK && rightOK && strings.EqualFold(leftSeverity, rightSeverity)
	}
	return apexPagesMessageFieldEqual(left, right, "severity")
}

func apexPagesMessageFieldEqual(left, right Value, field string) bool {
	leftValue, leftOK := left.Fields[field]
	rightValue, rightOK := right.Fields[field]
	if !leftOK || !rightOK {
		return leftOK == rightOK
	}
	return leftValue.Equal(rightValue)
}

func (vm *VM) requireTestContext(callee string) error {
	if vm.testContext == nil {
		return fmt.Errorf("%s is only available in test context", callee)
	}
	return nil
}

func (vm *VM) calleeStartsWithRuntimeReceiver(callee string, args []Value) bool {
	root, _, ok := strings.Cut(callee, ".")
	if !ok || root == "" {
		return false
	}
	if _, ok := vm.Globals[root]; ok {
		return true
	}
	if !vm.hasExactRuntimeClass(root) {
		if _, ok := vm.lookupGlobalName(root); ok {
			return true
		}
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if _, _, ok := objectFieldValue(this, root); ok {
			return true
		}
		if _, _, ok := vm.lookupReceiverField(this.Type, root); ok {
			return true
		}
	}
	if vm.currentClass != "" {
		if _, _, ok := vm.lookupStaticField(vm.currentClass, root); ok {
			return true
		}
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if _, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args); ok || ambiguous {
			return false
		}
	}
	return false
}

func (vm *VM) hasExactRuntimeClass(name string) bool {
	if vm == nil || strings.TrimSpace(name) == "" {
		return false
	}
	if _, ok := vm.Classes[name]; ok {
		return true
	}
	if _, ok := generatedPlatformTypes()[strings.ToLower(name)]; ok {
		return true
	}
	if class, ok := vm.resolveEnumClass(name); ok && class.Name == name {
		return true
	}
	return isBuiltinTypeName(name)
}

func (vm *VM) webServiceCalloutInvoke(args []Value, result *Result) (Value, error) {
	if len(args) != 4 {
		return Null, fmt.Errorf("WebServiceCallout.invoke expects stub, request, response map, and options")
	}
	if err := vm.incrementLimit("callouts", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.callout.webservice", "apex.callout", map[string]any{"operation": "WebServiceCallout.invoke"})
	if args[2].Kind != ValueMap {
		return Null, fmt.Errorf("WebServiceCallout.invoke expects response map")
	}
	if args[3].Kind != ValueList || len(args[3].List) != 7 {
		return Null, fmt.Errorf("WebServiceCallout.invoke expects 7 option strings")
	}
	for _, option := range args[3].List {
		if option.Kind != ValueString {
			return Null, fmt.Errorf("WebServiceCallout.invoke expects 7 option strings")
		}
	}
	responseType := scalarText(args[3].List[6])
	responseKey := mapKey(String("response_x"))
	if _, ok := args[2].Map[responseKey]; !ok {
		response := Object(responseType)
		if responseType != "" {
			vm.initializeFields(&response, responseType)
		}
		args[2].Map[responseKey] = response
		args[2].MapKeys[responseKey] = String("response_x")
	}
	if vm.testContext == nil || vm.testContext.WebServiceMock.Kind != ValueObject {
		operation := scalarText(args[3].List[3])
		response := args[2].Map[responseKey]
		if strings.EqualFold(operation, "renameMetadata") {
			saveResult := Object("MetadataService.SaveResult")
			saveResult.Fields["success"] = Bool(true)
			response.Fields["result"] = saveResult
		} else {
			response.Fields["result"] = List()
		}
		args[2].Map[responseKey] = response
		args[2].MapKeys[responseKey] = String("response_x")
		return Null, nil
	}
	mockArgs := []Value{
		args[0],
		args[1],
		args[2],
		String(scalarText(args[3].List[0])),
		String(scalarText(args[3].List[1])),
		String(scalarText(args[3].List[3])),
		String(scalarText(args[3].List[4])),
		String(scalarText(args[3].List[5])),
		String(scalarText(args[3].List[6])),
	}
	mock := vm.testContext.WebServiceMock
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(mock.Type, "doInvoke", mockArgs)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(mock.Type+".doInvoke", mockArgs)
	}
	if !ok {
		return Null, fmt.Errorf("WebServiceMock %s must implement doInvoke", mock.Type)
	}
	_, err := vm.callMethodWithReceiver(target, mock, mockArgs, result)
	return Null, err
}

func (vm *VM) emptyChildRelationshipForEach(expr ir.Expr) (Value, bool) {
	path := expr.Name
	if path == "" {
		path = expr.Callee
	}
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return Null, false
	}
	rootName := parts[0]
	if actual, ok := vm.lookupGlobalName(rootName); ok {
		rootName = actual
	}
	receiver, ok := vm.Globals[rootName]
	if !ok || receiver.Kind != ValueObject {
		return Null, false
	}
	if len(parts) > 2 {
		value, err := vm.lookupPath(receiver, parts[1:len(parts)-1])
		if err != nil || value.Kind != ValueObject {
			return Null, false
		}
		receiver = value
	}
	relationshipName := parts[len(parts)-1]
	if relationshipType, ok := vm.jsonSObjectChildRelationshipType(receiver.Type, relationshipName); ok {
		children := List()
		children.Type = relationshipType
		return children, true
	}
	return Null, false
}

func forEachExprContext(expr ir.Expr) string {
	switch {
	case expr.Name != "":
		return ": " + expr.Name
	case expr.Callee != "":
		return ": " + expr.Callee
	default:
		return ""
	}
}

func (vm *VM) executeObjectForEach(source string, inst ir.Instruction, result *Result, iterable Value) (execOutcome, error) {
	iterator := iterable
	if !isIteratorValue(iterator) {
		var err error
		iterator, err = vm.iteratorForObject(iterable, result)
		if err != nil {
			return execOutcome{}, err
		}
	}
	_, existed := vm.Globals[inst.Name]
	previous := vm.Globals[inst.Name]
	previousType, hadType := vm.VarTypes[inst.Name]
	const iteratorName = "__glade_for_each_iterator"
	previousIterator, hadIterator := vm.Globals[iteratorName]
	previousIteratorType, hadIteratorType := vm.VarTypes[iteratorName]
	defer func() {
		if existed {
			vm.Globals[inst.Name] = previous
		} else {
			delete(vm.Globals, inst.Name)
		}
		if hadType {
			vm.VarTypes[inst.Name] = previousType
		} else {
			delete(vm.VarTypes, inst.Name)
		}
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
	for iteration := 0; ; iteration++ {
		if iteration >= maxLoopIterations {
			return execOutcome{}, fmt.Errorf("enhanced for loop exceeded %d iterations", maxLoopIterations)
		}
		hasNext, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "hasNext", nil, result)
		if err != nil {
			return execOutcome{}, err
		}
		if !handled || hasNext.Kind != ValueBool {
			return execOutcome{}, fmt.Errorf("enhanced for iterator requires Boolean hasNext")
		}
		if !hasNext.Bool {
			return execOutcome{}, nil
		}
		value, handled, err := vm.callValueMember(iteratorName, vm.Globals[iteratorName], "next", nil, result)
		if err != nil {
			return execOutcome{}, err
		}
		if !handled {
			return execOutcome{}, fmt.Errorf("enhanced for iterator requires next")
		}
		coerced, err := vm.coerceAssignable(inst.Type, value)
		if err != nil {
			return execOutcome{}, fmt.Errorf("%s %s: %w", inst.Type, inst.Name, err)
		}
		vm.Globals[inst.Name] = coerced
		vm.VarTypes[inst.Name] = inst.Type
		out, err := vm.executeProgram(ir.Program{Instructions: inst.Then, Source: source}, result)
		if err != nil {
			return execOutcome{}, err
		}
		switch out.signal {
		case signalNone:
		case signalContinue:
		case signalBreak:
			return execOutcome{}, nil
		default:
			return out, nil
		}
	}
}

func (vm *VM) iteratorForObject(iterable Value, result *Result) (Value, error) {
	if iterable.Kind == ValueNull {
		return Null, newNullDereferenceError("enhanced for over null collection")
	}
	dispatchType := runtimeObjectType(iterable)
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchType, "iterator", nil)
	if ambiguous {
		return Null, vm.ambiguousOverloadError(dispatchType+".iterator", nil)
	}
	if !ok && iterable.Static != "" && !strings.EqualFold(iterable.Static, dispatchType) {
		target, ok, ambiguous = vm.resolveInstanceMethodForArgs(iterable.Static, "iterator", nil)
		if ambiguous {
			return Null, vm.ambiguousOverloadError(iterable.Static+".iterator", nil)
		}
	}
	if !ok {
		return Null, fmt.Errorf("enhanced for requires List, Set, or Iterable, got %s", iterable.Kind)
	}
	iterator, err := vm.callMethodWithReceiver(target, iterable, nil, result)
	if err != nil {
		return Null, err
	}
	if iterator.Kind == ValueNull {
		return Null, newNullDereferenceError("enhanced for over null iterator")
	}
	return iterator, nil
}

func ensureObject(org *storage.OrgState, definition storage.ObjectDefinition) {
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	if existing, ok := org.Objects[definition.APIName]; ok {
		existing.Definition = existing.Definition.Clone()
		if existing.Records == nil {
			existing.Records = make(map[storage.ID]storage.Record)
		}
		if existing.Definition.Fields == nil {
			existing.Definition.Fields = definition.Fields
		} else {
			for name, field := range definition.Fields {
				if _, ok := existing.Definition.Fields[name]; !ok {
					existing.Definition.Fields[name] = field
				}
			}
		}
		for _, relation := range definition.Relations {
			found := false
			for _, existingRelation := range existing.Definition.Relations {
				if existingRelation.Field == relation.Field {
					found = true
					break
				}
			}
			if !found {
				existing.Definition.Relations = append(existing.Definition.Relations, relation)
			}
		}
		org.Objects[definition.APIName] = existing
		return
	}
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: make(map[storage.ID]storage.Record)}
}

func (vm *VM) recordApexClass(className string) storage.ID {
	vm.ensureAsyncObjects()
	return vm.recordApexClassRecord(className)
}

func (vm *VM) recordApexClassRecord(className string) storage.ID {
	fallbackID := storage.ID(asyncApexClassID(className))
	if vm.Org == nil || className == "" {
		return fallbackID
	}
	object := vm.Org.Objects["ApexClass"]
	namespace, localName := splitApexClassName(className)
	if namespace == "" {
		namespace = vm.apexClassNamespace(localName)
	}
	if id, ok := findApexClassRecordID(object, localName, namespace); ok {
		return id
	}
	if _, ok := object.Records[fallbackID]; ok {
		return fallbackID
	}
	vm.recordIsolationJournalMutation("ApexClass", fallbackID, storage.Record{}, false)
	object.Records[fallbackID] = storage.Record{
		ID:     fallbackID,
		Object: "ApexClass",
		Fields: map[string]storage.Value{
			"Name":            storage.StringValue(localName),
			"NamespacePrefix": apexClassNamespaceValue(namespace),
		},
	}
	vm.Org.Objects["ApexClass"] = object
	return fallbackID
}

func findApexClassRecordID(object storage.ObjectState, localName, namespace string) (storage.ID, bool) {
	for id, record := range object.Records {
		if !storageStringValueEquals(record.Fields["Name"], localName) {
			continue
		}
		nsValue := record.Fields["NamespacePrefix"]
		if strings.TrimSpace(namespace) == "" {
			if nsValue.Kind == storage.ValueNull || (nsValue.Kind == storage.ValueString && strings.TrimSpace(nsValue.String) == "") {
				return id, true
			}
			continue
		}
		if storageStringValueEquals(nsValue, namespace) {
			return id, true
		}
	}
	return "", false
}

func (vm *VM) apexClassNamespace(localName string) string {
	if vm == nil {
		return ""
	}
	if class, ok := vm.lookupClass(localName); ok && strings.TrimSpace(class.Namespace) != "" {
		return class.Namespace
	}
	if vm.Org != nil {
		return strings.TrimSpace(vm.Org.Namespace)
	}
	return ""
}

func splitApexClassName(className string) (string, string) {
	namespace, localName, ok := strings.Cut(className, ".")
	if !ok || strings.TrimSpace(namespace) == "" || strings.TrimSpace(localName) == "" {
		return "", className
	}
	return namespace, localName
}

func apexClassNamespaceValue(namespace string) storage.Value {
	if strings.TrimSpace(namespace) == "" {
		return storage.NullValue()
	}
	return storage.StringValue(namespace)
}

func (vm *VM) sObjectNameForIDPrefix(prefix string) (string, bool) {
	if vm.Org != nil {
		names := make([]string, 0, len(vm.Org.Objects))
		for name := range vm.Org.Objects {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if vm.Org.Objects[name].Definition.KeyPrefix == prefix {
				return name, true
			}
		}
	}
	name, ok := standardSObjectPrefixes[prefix]
	return name, ok
}

func (vm *VM) sObjectNameForID(idText string) (string, bool) {
	if len(idText) < 3 {
		return "", false
	}
	if vm.Org != nil {
		names := make([]string, 0, len(vm.Org.Objects))
		for name := range vm.Org.Objects {
			names = append(names, name)
		}
		sort.Strings(names)
		id := storage.ID(idText)
		for _, name := range names {
			object := vm.Org.Objects[name]
			if _, _, ok := storage.LookupRecordByID(object.Records, id); ok {
				return name, true
			}
		}
	}
	return vm.sObjectNameForIDPrefix(idText[:3])
}

func idTextFromValue(value Value) (string, bool) {
	switch value.Kind {
	case ValueString:
		return value.Text, true
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			return platformScalarObjectText(value)
		}
	}
	return "", false
}

func idPrefix(idText string) string {
	if len(idText) < 3 {
		return idText
	}
	return idText[:3]
}

var standardSObjectPrefixes = map[string]string{
	"001": "Account",
	"003": "Contact",
	"005": "User",
	"006": "Opportunity",
	"00G": "Group",
	"00Q": "Lead",
	"00T": "Task",
	"00U": "Event",
	"00D": "Organization",
	"500": "Case",
	"701": "Campaign",
}

var commonSObjectTypeNames []string
var commonSObjectTypeNameSet map[string]bool
var generatedPlatformTypeIndex map[string]generatedPlatformType
var generatedPlatformMethodIndex map[string]map[string][]Method

func init() {
	for objectName, prefix := range storage.StandardKeyPrefixes() {
		if prefix != "" {
			standardSObjectPrefixes[prefix] = objectName
		}
	}
}

type generatedPlatformType struct {
	Name             string
	Kind             apexast.DeclarationKind
	SuperClass       string
	Fields           map[string]Field
	FieldOrder       []string
	StaticFields     map[string]Field
	StaticFieldOrder []string
	Constructors     []Method
}

func datetimeLegacyIsoFractionalTruncate(value Value) bool {
	flag, ok := value.Fields["legacyIsoFractionalTruncate"]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func defaultURLPort(scheme string) int64 {
	switch strings.ToLower(scheme) {
	case "http":
		return 80
	case "https":
		return 443
	case "ftp":
		return 21
	default:
		return -1
	}
}

func parsePlatformDate(value Value) (time.Time, error) {
	text, err := platformScalarText(value, "Date")
	if err != nil {
		return time.Time{}, err
	}
	date, err := parseDateText(text)
	if err != nil {
		return parsePlatformDateText(text)
	}
	return date, nil
}

func parsePlatformDatetime(value Value) (time.Time, error) {
	text, err := platformScalarText(value, "Datetime")
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := parseDatetimeTextAllowDateOnly(text)
	if err != nil {
		return parsePlatformDatetimeText(text)
	}
	return parsed.UTC(), nil
}

func datetimeFromLocalParts(year, month, day, hour, minute, second, millisecond int, zoneID string) (time.Time, error) {
	canonical, offset, ok := parseFixedTimeZoneID(zoneID)
	if ok {
		return time.Date(year, time.Month(month), day, hour, minute, second, millisecond*int(time.Millisecond), time.FixedZone(canonical, int(offset/time.Second))).UTC(), nil
	}
	zone, ok := supportedNamedTimeZone(zoneID)
	if !ok {
		return time.Time{}, unsupportedCallError("Datetime.newInstance timezone " + zoneID)
	}
	return zone.instantFromLocal(year, time.Month(month), day, hour, minute, second, millisecond), nil
}

func addMonthsClamped(value time.Time, months int) time.Time {
	year, month, day := value.Date()
	monthIndex := year*12 + int(month) - 1 + months
	targetYear := monthIndex / 12
	targetMonthIndex := monthIndex % 12
	if targetMonthIndex < 0 {
		targetMonthIndex += 12
		targetYear--
	}
	targetMonth := time.Month(targetMonthIndex + 1)
	if maxDay := daysInMonth(targetYear, targetMonth); day > maxDay {
		day = maxDay
	}
	return time.Date(targetYear, targetMonth, day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func parseDatetimeText(text string) (time.Time, error) {
	text = normalizeDatetimeShortTimezoneOffset(strings.TrimSpace(text))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
	} {
		if value, err := time.Parse(layout, text); err == nil {
			year := value.Year()
			if err := validateDateParts(year, int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
}

var datetimeShortTimezoneOffsetPattern = regexp.MustCompile(`^(.+[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?)([+-])([0-9]{1,2})$`)
var datetimeUnsignedShortTimezoneOffsetPattern = regexp.MustCompile(`^(.+[T ][0-9]{2}:[0-9]{2}:[0-9]{2})([0-9]{1,2})$`)

func normalizeDatetimeShortTimezoneOffset(text string) string {
	matches := datetimeShortTimezoneOffsetPattern.FindStringSubmatch(text)
	if matches != nil {
		hour, err := strconv.Atoi(matches[3])
		if err != nil {
			return text
		}
		return fmt.Sprintf("%s%s%02d:00", matches[1], matches[2], hour)
	}
	matches = datetimeUnsignedShortTimezoneOffsetPattern.FindStringSubmatch(text)
	if matches == nil {
		return text
	}
	hour, err := strconv.Atoi(matches[2])
	if err != nil || hour > 14 {
		return text
	}
	return fmt.Sprintf("%s+%02d:00", matches[1], hour)
}

func parseDatetimeTextAllowDateOnly(text string) (time.Time, error) {
	if value, err := parseDatetimeText(text); err == nil {
		return value, nil
	}
	return parseDateText(text)
}

func parseDatetimeParseText(text, zoneID string) (time.Time, error) {
	text = strings.TrimSpace(text)
	if value, err := parseDatetimeText(text); err == nil {
		return value, nil
	}
	location := time.UTC
	if strings.TrimSpace(zoneID) != "" {
		if loaded, err := time.LoadLocation(zoneID); err == nil {
			location = loaded
		}
	}
	for _, layout := range []string{
		"1/2/2006, 3:04:05 PM",
		"1/2/2006, 3:04 PM",
		"01/02/2006, 03:04:05 PM",
		"01/02/2006, 03:04 PM",
		"1/2/06, 3:04:05 PM",
		"1/2/06, 3:04 PM",
		"01/02/06, 03:04:05 PM",
		"01/02/06, 03:04 PM",
		"1/2/2006 3:04:05 PM",
		"1/2/2006 3:04 PM",
		"01/02/2006 03:04:05 PM",
		"01/02/2006 03:04 PM",
	} {
		if value, err := time.ParseInLocation(layout, text, location); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return value, nil
		}
	}
	date, err := parseDateParseText(text)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location), nil
}

func parseDateText(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, ","); idx > 0 {
		if value, err := parseDateText(text[:idx]); err == nil {
			return value, nil
		}
	}
	for _, layout := range []string{
		"2006-01-02", "2006-1-2",
		"2006-01-02 15:04:05", "2006-1-2 15:04:05",
		"2006-01-02T15:04:05", "2006-1-2T15:04:05",
	} {
		if value, err := time.Parse(layout, text); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	if value, err := parseDatetimeText(text); err == nil {
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
}

func parsePlatformDateText(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range []string{"2006-01-02", "2006-1-2"} {
		if value, err := time.Parse(layout, text); err == nil {
			if value.Year() < 0 || value.Year() > 9999 {
				return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
			}
			if value.Format("2006-01-02") != text {
				return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported Date value %q", text)
}

func parsePlatformDatetimeText(text string) (time.Time, error) {
	normalized := normalizeDatetimeShortTimezoneOffset(strings.TrimSpace(text))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z0700",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
	} {
		if value, err := time.Parse(layout, normalized); err == nil {
			if value.Year() < 0 || value.Year() > 9999 {
				return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
			}
			return value, nil
		}
	}
	if value, err := parsePlatformDateText(text); err == nil {
		return value, nil
	}
	return time.Time{}, fmt.Errorf("unsupported Datetime value %q", text)
}

func parseDateParseText(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range []string{"1/2/2006", "01/02/2006", "1/2/06", "01/02/06"} {
		if value, err := time.Parse(layout, text); err == nil {
			if err := validateDateParts(value.Year(), int(value.Month()), value.Day()); err != nil {
				return time.Time{}, err
			}
			return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return parseDateText(text)
}

func formatPlatformDatetime(value time.Time) string {
	utc := value.UTC()
	ms := utc.Nanosecond() / int(time.Millisecond)
	if ms == 0 {
		return utc.Format("2006-01-02T15:04:05Z")
	}
	frac := strings.TrimRight(fmt.Sprintf("%03d", ms), "0")
	return fmt.Sprintf("%s.%sZ", utc.Format("2006-01-02T15:04:05"), frac)
}

func formatApexDatetimePattern(value time.Time, pattern, zoneID, zoneLabel string, offset time.Duration) (string, error) {
	var b strings.Builder
	for i := 0; i < len(pattern); {
		ch := pattern[i]
		if ch == '\'' {
			next, literal, err := readApexDatePatternLiteral(pattern, i)
			if err != nil {
				return "", err
			}
			b.WriteString(literal)
			i = next
			continue
		}
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			b.WriteByte(ch)
			i++
			continue
		}
		j := i + 1
		for j < len(pattern) && pattern[j] == ch {
			j++
		}
		token := pattern[i:j]
		text, err := formatApexDatetimeToken(value, token, zoneID, zoneLabel, offset)
		if err != nil {
			return "", err
		}
		b.WriteString(text)
		i = j
	}
	return b.String(), nil
}

func readApexDatePatternLiteral(pattern string, start int) (int, string, error) {
	var b strings.Builder
	for i := start + 1; i < len(pattern); i++ {
		if pattern[i] != '\'' {
			b.WriteByte(pattern[i])
			continue
		}
		if i+1 < len(pattern) && pattern[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		return i + 1, b.String(), nil
	}
	return 0, "", fmt.Errorf("Datetime.format unsupported unterminated quoted literal")
}

func formatApexDatetimeToken(value time.Time, token, zoneID, zoneLabel string, offset time.Duration) (string, error) {
	count := len(token)
	switch token[0] {
	case 'y', 'Y':
		year := value.Year()
		if count == 2 {
			return fmt.Sprintf("%02d", year%100), nil
		}
		return fmt.Sprintf("%0*d", maxInt(count, 4), year), nil
	case 'M':
		month := value.Month()
		switch {
		case count >= 4:
			return month.String(), nil
		case count == 3:
			return month.String()[:3], nil
		case count == 2:
			return fmt.Sprintf("%02d", int(month)), nil
		default:
			return strconv.Itoa(int(month)), nil
		}
	case 'd':
		return formatPaddedDateNumber(value.Day(), count), nil
	case 'H':
		return formatPaddedDateNumber(value.Hour(), count), nil
	case 'h':
		hour := value.Hour() % 12
		if hour == 0 {
			hour = 12
		}
		return formatPaddedDateNumber(hour, count), nil
	case 'm':
		return formatPaddedDateNumber(value.Minute(), count), nil
	case 's':
		return formatPaddedDateNumber(value.Second(), count), nil
	case 'S':
		if count > 3 {
			return "", fmt.Errorf("Datetime.format unsupported pattern token %q", token)
		}
		millisecond := value.Nanosecond() / int(time.Millisecond)
		if count <= 1 {
			return strconv.Itoa(millisecond), nil
		}
		return fmt.Sprintf("%0*d", minInt(count, 3), millisecond), nil
	case 'a':
		if value.Hour() < 12 {
			return "AM", nil
		}
		return "PM", nil
	case 'E':
		name := value.Weekday().String()
		if count >= 4 {
			return name, nil
		}
		return name[:3], nil
	case 'u':
		weekday := int(value.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return formatPaddedDateNumber(weekday, count), nil
	case 'w':
		_, week := value.ISOWeek()
		return formatPaddedDateNumber(week, count), nil
	case 'G', 'L', 'c', 'e':
		return "", unsupportedCallError(fmt.Sprintf("Datetime.format locale-dependent pattern token %q", token))
	case 'Z':
		return formatRFC822Offset(offset), nil
	case 'z':
		if zoneID == "UTC" {
			return "UTC", nil
		}
		return zoneLabel, nil
	default:
		return "", fmt.Errorf("Datetime.format unsupported pattern token %q", token)
	}
}

func formatPaddedDateNumber(value, count int) string {
	if count >= 2 {
		return fmt.Sprintf("%02d", value)
	}
	return strconv.Itoa(value)
}

func formatRFC822Offset(offset time.Duration) string {
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	totalMinutes := int(offset / time.Minute)
	return fmt.Sprintf("%s%02d%02d", sign, totalMinutes/60, totalMinutes%60)
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func normalizeDateNewInstanceParts(year, month, day int) (int, int, int) {
	if validateDateParts(year, month, day) == nil {
		return year, month, day
	}
	if year < 1 || year > 12 || month < 1 || month > 31 || day < 1000 {
		return year, month, day
	}
	if validateDateParts(day, year, month) == nil {
		return day, year, month
	}
	return year, month, day
}

func dateFromNewInstanceParts(year, month, day int) (time.Time, error) {
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Year() < 0 || value.Year() > 9999 {
		return time.Time{}, newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	return value, nil
}

func validateDateParts(year, month, day int) error {
	if year < 1 || year > 9999 {
		return newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if value.Year() != year || int(value.Month()) != month || value.Day() != day {
		return newExceptionError("System.TypeException", fmt.Sprintf("invalid Date parts: year=%d month=%d day=%d", year, month, day))
	}
	return nil
}

func validateTimeParts(hour, minute, second int) error {
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return fmt.Errorf("invalid Time parts: hour=%d minute=%d second=%d", hour, minute, second)
	}
	return nil
}

func parseTimeText(text string) (string, error) {
	for _, layout := range []string{"15:04:05.000", "15:04:05.000Z", "15:04:05", "15:04:05Z"} {
		if value, err := time.Parse(layout, text); err == nil {
			return formatPlatformTimeWithMillis(value.Hour(), value.Minute(), value.Second(), value.Nanosecond()/int(time.Millisecond)), nil
		}
	}
	return "", fmt.Errorf("unsupported Time value %q", text)
}

func parsePlatformTime(value Value) (time.Duration, error) {
	text, err := platformScalarText(value, "Time")
	if err != nil {
		return 0, err
	}
	parsed, err := parseTimeText(text)
	if err != nil {
		return 0, err
	}
	t, err := time.Parse("15:04:05.000", ensureTimeMillis(parsed))
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute +
		time.Duration(t.Second())*time.Second +
		time.Duration(t.Nanosecond()), nil
}

func ensureTimeMillis(text string) string {
	if strings.Contains(text, ".") {
		return text
	}
	return text + ".000"
}

func formatPlatformTime(hour, minute, second, millisecond int) string {
	base := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	if millisecond == 0 {
		return base
	}
	return fmt.Sprintf("%s.%03d", base, millisecond)
}

func formatPlatformTimeWithMillis(hour, minute, second, millisecond int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hour, minute, second, millisecond)
}

func formatPlatformTimeZulu(hour, minute, second, millisecond int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03dZ", hour, minute, second, millisecond)
}

func toTwelveHour(hour int) int {
	value := hour % 12
	if value == 0 {
		return 12
	}
	return value
}

func ampm(hour int) string {
	if hour < 12 {
		return "AM"
	}
	return "PM"
}

func platformTimeFromDuration(value time.Duration) Value {
	day := 24 * time.Hour
	value %= day
	if value < 0 {
		value += day
	}
	hour := int(value / time.Hour)
	value %= time.Hour
	minute := int(value / time.Minute)
	value %= time.Minute
	second := int(value / time.Second)
	value %= time.Second
	millisecond := int(value / time.Millisecond)
	return platformScalar("Time", formatPlatformTimeWithMillis(hour, minute, second, millisecond))
}

func fixedTimeZone(id string) (Value, error) {
	canonical, offset, ok := parseFixedTimeZoneID(id)
	locationName := ""
	if !ok {
		location, locationOK := supportedNamedTimeZone(id)
		if !locationOK {
			return Null, unsupportedCallError("TimeZone.getTimeZone " + id)
		}
		canonical = id
		offset = location.standardOffset
		locationName = location.id
	} else if canonical == "UTC" {
		locationName = "UTC"
	}
	out := Object("TimeZone")
	out.Fields["id"] = String(canonical)
	out.Fields["offsetMillis"] = Int(int64(offset / time.Millisecond))
	out.Fields["location"] = String(locationName)
	return out, nil
}

type modeledTimeZone struct {
	id                  string
	standardOffset      time.Duration
	daylightOffset      time.Duration
	standardLabel       string
	daylightLabel       string
	standardDisplayName string
	daylightDisplayName string
	daylightRule        string
}

var supportedNamedTimeZones = map[string]modeledTimeZone{
	"America/Los_Angeles": {id: "America/Los_Angeles", standardOffset: -8 * time.Hour, daylightOffset: -7 * time.Hour, standardLabel: "PST", daylightLabel: "PDT", standardDisplayName: "Pacific Standard Time", daylightDisplayName: "Pacific Daylight Time", daylightRule: "us"},
	"America/New_York":    {id: "America/New_York", standardOffset: -5 * time.Hour, daylightOffset: -4 * time.Hour, standardLabel: "EST", daylightLabel: "EDT", standardDisplayName: "Eastern Standard Time", daylightDisplayName: "Eastern Daylight Time", daylightRule: "us"},
	"America/Chicago":     {id: "America/Chicago", standardOffset: -6 * time.Hour, daylightOffset: -5 * time.Hour, standardLabel: "CST", daylightLabel: "CDT", standardDisplayName: "Central Standard Time", daylightDisplayName: "Central Daylight Time", daylightRule: "us"},
	"America/Denver":      {id: "America/Denver", standardOffset: -7 * time.Hour, daylightOffset: -6 * time.Hour, standardLabel: "MST", daylightLabel: "MDT", standardDisplayName: "Mountain Standard Time", daylightDisplayName: "Mountain Daylight Time", daylightRule: "us"},
	"America/Panama":      {id: "America/Panama", standardOffset: -5 * time.Hour, standardLabel: "EST", standardDisplayName: "Eastern Standard Time"},
	"Europe/London":       {id: "Europe/London", standardOffset: 0, daylightOffset: time.Hour, standardLabel: "GMT", daylightLabel: "BST", standardDisplayName: "Greenwich Mean Time", daylightDisplayName: "British Summer Time", daylightRule: "europe"},
	"Europe/Berlin":       {id: "Europe/Berlin", standardOffset: time.Hour, daylightOffset: 2 * time.Hour, standardLabel: "CET", daylightLabel: "CEST", standardDisplayName: "Central European Standard Time", daylightDisplayName: "Central European Summer Time", daylightRule: "europe"},
	"Asia/Ho_Chi_Minh":    {id: "Asia/Ho_Chi_Minh", standardOffset: 7 * time.Hour, standardLabel: "ICT", standardDisplayName: "Indochina Time"},
	"Asia/Tokyo":          {id: "Asia/Tokyo", standardOffset: 9 * time.Hour, standardLabel: "JST", standardDisplayName: "Japan Standard Time"},
	"Pacific/Honolulu":    {id: "Pacific/Honolulu", standardOffset: -10 * time.Hour, standardLabel: "HST", standardDisplayName: "Hawaii-Aleutian Standard Time"},
	"Pacific/Pago_Pago":   {id: "Pacific/Pago_Pago", standardOffset: -11 * time.Hour, standardLabel: "SST", standardDisplayName: "Samoa Standard Time"},
	"Australia/Sydney":    {id: "Australia/Sydney", standardOffset: 10 * time.Hour, daylightOffset: 11 * time.Hour, standardLabel: "AEST", daylightLabel: "AEDT", standardDisplayName: "Australian Eastern Standard Time", daylightDisplayName: "Australian Eastern Daylight Time", daylightRule: "sydney"},
}

func supportedNamedTimeZone(id string) (modeledTimeZone, bool) {
	location, ok := supportedNamedTimeZones[id]
	return location, ok
}

func resolveTimeZoneForInstant(id string, instant time.Time) (string, time.Duration, time.Time, string, bool) {
	canonical, offset, ok := parseFixedTimeZoneID(id)
	if ok {
		local := instant.UTC().In(time.FixedZone(canonical, int(offset/time.Second)))
		return canonical, offset, local, canonical, true
	}
	location, ok := supportedNamedTimeZone(id)
	if !ok {
		return "", 0, time.Time{}, "", false
	}
	offset, label := location.offsetAt(instant)
	local := instant.UTC().In(time.FixedZone(label, int(offset/time.Second)))
	return id, offset, local, label, true
}

func timeZoneOffsetMillis(receiver Value, instant time.Time) (Value, error) {
	locationValue := receiver.Fields["location"]
	if locationValue.Kind == ValueString && locationValue.Text != "" && locationValue.Text != "UTC" {
		location, ok := supportedNamedTimeZone(locationValue.Text)
		if !ok {
			return Null, unsupportedCallError("TimeZone.getOffset " + locationValue.Text)
		}
		offset, _ := location.offsetAt(instant)
		return Int(int64(offset / time.Millisecond)), nil
	}
	offsetValue := receiver.Fields["offsetMillis"]
	if offsetValue.Kind != ValueInt {
		return Null, fmt.Errorf("TimeZone offset is missing")
	}
	return offsetValue, nil
}

func (vm *VM) timeZoneDisplayName(receiver Value) Value {
	idValue := receiver.Fields["id"]
	if idValue.Kind != ValueString {
		return idValue
	}
	offset := time.Duration(0)
	longName := "Pacific Standard Time"
	locationValue := receiver.Fields["location"]
	if locationValue.Kind == ValueString && locationValue.Text != "" {
		if locationValue.Text == "UTC" {
			longName = "Coordinated Universal Time"
		} else if location, ok := supportedNamedTimeZone(locationValue.Text); ok {
			offset, longName = location.displayNameAt(vm.fakeNow)
		}
	} else if offsetValue := receiver.Fields["offsetMillis"]; offsetValue.Kind == ValueInt {
		offset = time.Duration(offsetValue.Int) * time.Millisecond
	}
	return String(fmt.Sprintf("(GMT%s) %s (%s)", formatTimeZoneDisplayOffset(offset), longName, idValue.Text))
}

func formatTimeZoneDisplayOffset(offset time.Duration) string {
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	totalMinutes := int(offset / time.Minute)
	return fmt.Sprintf("%s%02d:%02d", sign, totalMinutes/60, totalMinutes%60)
}

func (zone modeledTimeZone) offsetAt(instant time.Time) (time.Duration, string) {
	if zone.daylightRule == "" || !zone.isDaylight(instant.UTC()) {
		return zone.standardOffset, zone.standardLabel
	}
	return zone.daylightOffset, zone.daylightLabel
}

func (zone modeledTimeZone) displayNameAt(instant time.Time) (time.Duration, string) {
	offset, _ := zone.offsetAt(instant)
	if zone.daylightRule != "" && zone.isDaylight(instant.UTC()) && zone.daylightDisplayName != "" {
		return offset, zone.daylightDisplayName
	}
	return offset, zone.standardDisplayName
}

func (zone modeledTimeZone) instantFromLocal(year int, month time.Month, day, hour, minute, second, millisecond int) time.Time {
	local := time.Date(year, month, day, hour, minute, second, millisecond*int(time.Millisecond), time.UTC)
	offsets := []time.Duration{zone.standardOffset}
	if zone.daylightRule != "" && zone.daylightOffset != zone.standardOffset {
		offsets = append(offsets, zone.daylightOffset)
	}
	var matches []time.Time
	for _, offset := range offsets {
		candidate := local.Add(-offset)
		actualOffset, _ := zone.offsetAt(candidate)
		if candidate.Add(actualOffset).Equal(local) {
			matches = append(matches, candidate.UTC())
		}
	}
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool { return matches[i].Before(matches[j]) })
		return matches[0]
	}
	return local.Add(-zone.standardOffset).UTC()
}

func (zone modeledTimeZone) isDaylight(instant time.Time) bool {
	year := instant.Year()
	switch zone.daylightRule {
	case "us":
		start := localRuleTransitionUTC(year, time.March, nthWeekdayOfMonth(year, time.March, time.Sunday, 2), 2, zone.standardOffset)
		end := localRuleTransitionUTC(year, time.November, nthWeekdayOfMonth(year, time.November, time.Sunday, 1), 2, zone.daylightOffset)
		return !instant.Before(start) && instant.Before(end)
	case "europe":
		start := time.Date(year, time.March, lastWeekdayOfMonth(year, time.March, time.Sunday), 1, 0, 0, 0, time.UTC)
		end := time.Date(year, time.October, lastWeekdayOfMonth(year, time.October, time.Sunday), 1, 0, 0, 0, time.UTC)
		return !instant.Before(start) && instant.Before(end)
	case "sydney":
		start := localRuleTransitionUTC(year, time.October, nthWeekdayOfMonth(year, time.October, time.Sunday, 1), 2, zone.standardOffset)
		end := localRuleTransitionUTC(year, time.April, nthWeekdayOfMonth(year, time.April, time.Sunday, 1), 3, zone.daylightOffset)
		return !instant.Before(start) || instant.Before(end)
	default:
		return false
	}
}

func localRuleTransitionUTC(year int, month time.Month, day, hour int, offsetBefore time.Duration) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.UTC).Add(-offsetBefore)
}

func nthWeekdayOfMonth(year int, month time.Month, weekday time.Weekday, n int) int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	delta := (int(weekday) - int(first.Weekday()) + 7) % 7
	return 1 + delta + 7*(n-1)
}

func lastWeekdayOfMonth(year int, month time.Month, weekday time.Weekday) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC)
	delta := (int(last.Weekday()) - int(weekday) + 7) % 7
	return last.Day() - delta
}

func parseFixedTimeZoneID(id string) (string, time.Duration, bool) {
	trimmed := strings.TrimSpace(id)
	if trimmed != id {
		return "", 0, false
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "UTC", "GMT", "ETC/UTC", "Z":
		return "UTC", 0, true
	}
	if !strings.HasPrefix(upper, "GMT+") && !strings.HasPrefix(upper, "GMT-") && !strings.HasPrefix(upper, "UTC+") && !strings.HasPrefix(upper, "UTC-") {
		return "", 0, false
	}
	prefix := upper[:3]
	signText := upper[3:4]
	rest := upper[4:]
	if prefix == "UTC" {
		rest = upper[4:]
	}
	parts := strings.Split(rest, ":")
	if len(parts) > 2 || parts[0] == "" {
		return "", 0, false
	}
	if len(parts[0]) > 2 || !allASCIIDigits(parts[0]) {
		return "", 0, false
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", 0, false
	}
	minutes := 0
	if len(parts) == 2 {
		if len(parts[1]) != 2 || !allASCIIDigits(parts[1]) {
			return "", 0, false
		}
		minutes, err = strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, false
		}
	}
	if hours > 14 || minutes > 59 || (hours == 14 && minutes != 0) {
		return "", 0, false
	}
	offset := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
	if signText == "-" {
		offset = -offset
	}
	return fmt.Sprintf("GMT%s%02d:%02d", signText, hours, minutes), offset, true
}

func allASCIIDigits(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return text != ""
}

const (
	defaultHttpTimeoutMillis int64 = 10000
	maxHttpTimeoutMillis     int64 = 120000
)

func validateHttpRequest(request Value) error {
	endpoint, ok := request.Fields["endpoint"]
	if !ok || endpoint.Kind != ValueString {
		return fmt.Errorf("HttpRequest endpoint is required before Http.send")
	}
	if strings.TrimSpace(endpoint.Text) == "" {
		return fmt.Errorf("HttpRequest endpoint is required before Http.send")
	}
	if err := validateHttpEndpoint(endpoint.Text); err != nil {
		return err
	}
	method, ok := request.Fields["method"]
	if !ok || method.Kind != ValueString {
		return fmt.Errorf("HttpRequest method is required before Http.send")
	}
	if strings.TrimSpace(method.Text) == "" {
		return fmt.Errorf("HttpRequest method is required before Http.send")
	}
	if _, err := normalizeHttpMethod(method.Text); err != nil {
		return err
	}
	if timeout, ok := request.Fields["timeout"]; ok {
		if timeout.Kind != ValueInt {
			return fmt.Errorf("HttpRequest timeout must be Integer")
		}
		return validateHttpTimeout(timeout.Int)
	}
	return nil
}

func validateHttpEndpoint(endpoint string) error {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return fmt.Errorf("HttpRequest endpoint is required")
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "callout:") {
		if strings.TrimSpace(trimmed[len("callout:"):]) == "" {
			return fmt.Errorf("HttpRequest endpoint named credential is required")
		}
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("HttpRequest endpoint must be an absolute http, https, or callout URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("HttpRequest endpoint must use http, https, or callout scheme")
	}
	return nil
}

func normalizeHttpMethod(method string) (string, error) {
	trimmed := strings.TrimSpace(method)
	if trimmed == "" {
		return "", fmt.Errorf("HttpRequest method is required")
	}
	upper := strings.ToUpper(trimmed)
	switch upper {
	case "DELETE", "GET", "HEAD", "PATCH", "POST", "PUT", "TRACE":
		return upper, nil
	default:
		return "", fmt.Errorf("HttpRequest method %q is not supported", method)
	}
}

func validateHttpTimeout(timeout int64) error {
	if timeout < 1 || timeout > maxHttpTimeoutMillis {
		return fmt.Errorf("HttpRequest timeout must be between 1 and %d milliseconds", maxHttpTimeoutMillis)
	}
	return nil
}

func httpSetHeader(receiver Value, name string, value Value) {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		headers = Map()
	}
	headers.Map[mapKey(String(strings.ToLower(name)))] = value
	receiver.Fields["headers"] = headers
}

func httpGetHeader(receiver Value, name string) Value {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		return Null
	}
	if value, ok := headers.Map[mapKey(String(strings.ToLower(name)))]; ok {
		return value
	}
	return Null
}

func httpHeaderKeys(receiver Value) Value {
	headers, ok := receiver.Fields["headers"]
	if !ok || headers.Kind != ValueMap {
		return List()
	}
	keys := make([]string, 0, len(headers.Map))
	for rawKey := range headers.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString {
			keys = append(keys, decoded.Text)
		}
	}
	sort.Strings(keys)
	out := make([]Value, 0, len(keys))
	for _, key := range keys {
		out = append(out, String(key))
	}
	return List(out...)
}

func (vm *VM) assertMessage(base string, extra []Value, result *Result) (string, error) {
	if len(extra) == 0 {
		return base, nil
	}
	message, err := vm.displayString(extra[0], result)
	if err != nil {
		return "", err
	}
	return base + ": " + message, nil
}

func blobStringArg(name string, args []Value) (string, error) {
	if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
		return "", fmt.Errorf("%s expects Blob", name)
	}
	return args[0].Fields["value"].String(), nil
}

func urlEncodeWithCharset(name, text, charset string) (string, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return url.QueryEscape(text), nil
	case "us-ascii":
		return urlEncodeASCII(name, text)
	case "iso-8859-1":
		return urlEncodeLatin1(name, text)
	case "utf-16":
		return urlEncodeUTF16(text), nil
	default:
		return "", unsupportedCallError(fmt.Sprintf("%s charset %q", name, charset))
	}
}

func urlDecodeWithCharset(name, text, charset string) (string, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return url.QueryUnescape(text)
	case "us-ascii":
		return urlDecodeASCII(name, text)
	case "iso-8859-1":
		return urlDecodeLatin1(text)
	case "utf-16":
		return urlDecodeUTF16(text)
	default:
		return "", unsupportedCallError(fmt.Sprintf("%s charset %q", name, charset))
	}
}

func normalizeURLCharset(charset string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(charset), "_", "-"))
	switch normalized {
	case "utf-8", "utf8":
		return "utf-8"
	case "us-ascii", "usascii", "ascii":
		return "us-ascii"
	case "iso-8859-1", "iso8859-1", "iso-88591", "iso88591", "latin1", "latin-1":
		return "iso-8859-1"
	case "utf-16", "utf16":
		return "utf-16"
	default:
		return normalized
	}
}

func urlEncodeASCII(_ string, text string) (string, error) {
	var out strings.Builder
	for _, r := range text {
		if r > 0x7f {
			r = '?'
		}
		writeURLEncodedByte(&out, byte(r))
	}
	return out.String(), nil
}

func urlEncodeLatin1(_ string, text string) (string, error) {
	var out strings.Builder
	for _, r := range text {
		if r > 0xff {
			r = '?'
		}
		writeURLEncodedByte(&out, byte(r))
	}
	return out.String(), nil
}

func urlEncodeUTF16(text string) string {
	var out strings.Builder
	unsafeRunes := make([]rune, 0, len(text))
	flushUnsafeRunes := func() {
		if len(unsafeRunes) == 0 {
			return
		}
		for _, b := range utf16BytesForRunes(unsafeRunes) {
			writeURLEncodedByte(&out, b)
		}
		unsafeRunes = unsafeRunes[:0]
	}
	for _, r := range text {
		if isURLEncodedSafeASCII(r) {
			flushUnsafeRunes()
			writeURLEncodedByte(&out, byte(r))
			continue
		}
		if r == ' ' {
			flushUnsafeRunes()
			out.WriteByte('+')
			continue
		}
		unsafeRunes = append(unsafeRunes, r)
	}
	flushUnsafeRunes()
	return out.String()
}

func isURLEncodedSafeASCII(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '*'
}

func utf16BytesForRunes(runes []rune) []byte {
	units := utf16.Encode(runes)
	out := make([]byte, 0, 2+len(units)*2)
	out = append(out, 0xfe, 0xff)
	for _, unit := range units {
		out = append(out, byte(unit>>8), byte(unit))
	}
	return out
}

func writeURLEncodedByte(out *strings.Builder, b byte) {
	switch {
	case b == ' ':
		out.WriteByte('+')
	case (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '*':
		out.WriteByte(b)
	default:
		const hexDigits = "0123456789ABCDEF"
		out.WriteByte('%')
		out.WriteByte(hexDigits[b>>4])
		out.WriteByte(hexDigits[b&0x0f])
	}
}

func urlDecodeASCII(_ string, text string) (string, error) {
	decoded, err := urlDecodeBytes(text)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, b := range decoded {
		if b > 0x7f {
			out.WriteRune('\ufffd')
			continue
		}
		out.WriteByte(b)
	}
	return out.String(), nil
}

func urlDecodeLatin1(text string) (string, error) {
	decoded, err := urlDecodeBytes(text)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, b := range decoded {
		out.WriteRune(rune(b))
	}
	return out.String(), nil
}

func urlDecodeUTF16(text string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		switch ch := text[i]; ch {
		case '+':
			out.WriteByte(' ')
		case '%':
			bytes := make([]byte, 0)
			for i < len(text) && text[i] == '%' {
				if i+2 >= len(text) {
					return "", fmt.Errorf("invalid URL escape %q", text[i:])
				}
				hi, ok := fromHex(text[i+1])
				if !ok {
					return "", fmt.Errorf("invalid URL escape %q", text[i:i+3])
				}
				lo, ok := fromHex(text[i+2])
				if !ok {
					return "", fmt.Errorf("invalid URL escape %q", text[i:i+3])
				}
				bytes = append(bytes, hi<<4|lo)
				i += 3
			}
			i--
			out.WriteString(decodeUTF16Bytes(bytes))
		default:
			out.WriteByte(ch)
		}
	}
	return out.String(), nil
}

func decodeUTF16Bytes(decoded []byte) string {
	if len(decoded) == 0 {
		return ""
	}
	bigEndian := true
	start := 0
	if len(decoded) >= 2 {
		switch {
		case decoded[0] == 0xfe && decoded[1] == 0xff:
			start = 2
		case decoded[0] == 0xff && decoded[1] == 0xfe:
			bigEndian = false
			start = 2
		}
	}
	if (len(decoded)-start)%2 != 0 {
		decoded = append(decoded, 0)
	}
	units := make([]uint16, 0, (len(decoded)-start)/2)
	for i := start; i+1 < len(decoded); i += 2 {
		if bigEndian {
			units = append(units, uint16(decoded[i])<<8|uint16(decoded[i+1]))
		} else {
			units = append(units, uint16(decoded[i+1])<<8|uint16(decoded[i]))
		}
	}
	return string(utf16.Decode(units))
}

func urlDecodeBytes(text string) ([]byte, error) {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		ch := text[i]
		switch ch {
		case '+':
			out = append(out, ' ')
		case '%':
			if i+2 >= len(text) {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:])
			}
			hi, ok := fromHex(text[i+1])
			if !ok {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:i+3])
			}
			lo, ok := fromHex(text[i+2])
			if !ok {
				return nil, fmt.Errorf("invalid URL escape %q", text[i:i+3])
			}
			out = append(out, hi<<4|lo)
			i += 2
		default:
			out = append(out, ch)
		}
	}
	return out, nil
}

func fromHex(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}

func normalizeCryptoAlgorithm(algorithm string) string {
	normalized := strings.ToUpper(strings.TrimSpace(algorithm))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	return normalized
}

func generateDigest(algorithm string, data []byte) ([]byte, error) {
	normalized := strings.ToUpper(algorithm)
	switch normalized {
	case "MD5":
		sum := md5.Sum(data)
		return sum[:], nil
	case "SHA1", "SHA-1":
		sum := sha1.Sum(data)
		return sum[:], nil
	case "SHA256", "SHA-256":
		sum := sha256.Sum256(data)
		return sum[:], nil
	case "SHA384", "SHA-384":
		sum := sha512.Sum384(data)
		return sum[:], nil
	case "SHA512", "SHA-512":
		sum := sha512.Sum512(data)
		return sum[:], nil
	case "SHA3-256":
		sum := sha3.Sum256(data)
		return sum[:], nil
	case "SHA3-384":
		sum := sha3.Sum384(data)
		return sum[:], nil
	case "SHA3-512":
		sum := sha3.Sum512(data)
		return sum[:], nil
	default:
		return nil, newExceptionError("SecurityException", fmt.Sprintf("%s MessageDigest not available", algorithm))
	}
}

func generateMac(algorithm string, input, privateKey []byte) ([]byte, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	var mac hash.Hash
	switch normalized {
	case "HMACMD5":
		mac = hmac.New(md5.New, privateKey)
	case "HMACSHA1":
		mac = hmac.New(sha1.New, privateKey)
	case "HMACSHA256":
		mac = hmac.New(sha256.New, privateKey)
	case "HMACSHA512":
		mac = hmac.New(sha512.New, privateKey)
	default:
		return nil, fmt.Errorf("unsupported MAC algorithm %q", algorithm)
	}
	if _, err := mac.Write(input); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

func encryptAESCBC(algorithm string, privateKey, initializationVector, clearText []byte) ([]byte, error) {
	keySize, err := aesKeySizeForAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	if len(privateKey) != keySize {
		return nil, fmt.Errorf("Crypto.encrypt %s privateKey expects %d bytes, got %d", normalizeCryptoAlgorithm(algorithm), keySize, len(privateKey))
	}
	if len(initializationVector) != aes.BlockSize {
		return nil, fmt.Errorf("Crypto.encrypt initializationVector expects %d bytes, got %d", aes.BlockSize, len(initializationVector))
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(clearText, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, initializationVector).CryptBlocks(out, padded)
	return out, nil
}

func decryptAESCBC(algorithm string, privateKey, initializationVector, cipherText []byte) ([]byte, error) {
	keySize, err := aesKeySizeForAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	if len(privateKey) != keySize {
		return nil, fmt.Errorf("Crypto.decrypt %s privateKey expects %d bytes, got %d", normalizeCryptoAlgorithm(algorithm), keySize, len(privateKey))
	}
	if len(initializationVector) != aes.BlockSize {
		return nil, fmt.Errorf("Crypto.decrypt initializationVector expects %d bytes, got %d", aes.BlockSize, len(initializationVector))
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("Crypto.decrypt cipherText must be a positive multiple of %d bytes", aes.BlockSize)
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	padded := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, initializationVector).CryptBlocks(padded, cipherText)
	return pkcs7Unpad(padded, aes.BlockSize)
}

func managedIV(privateKey, clearText []byte) []byte {
	sum := sha256.Sum256(append(append([]byte("glade-managed-iv:"), privateKey...), clearText...))
	iv := make([]byte, aes.BlockSize)
	copy(iv, sum[:aes.BlockSize])
	return iv
}

func encryptAESGCM(algorithm string, privateKey, initializationVector, clearText, additionalData []byte) ([]byte, error) {
	if normalizeCryptoAlgorithm(algorithm) != "AES256GCM" {
		return nil, fmt.Errorf("Crypto.encryptWithManagedIV authenticated data only supports AES256-GCM")
	}
	if len(privateKey) != 32 {
		return nil, fmt.Errorf("Crypto.encryptWithManagedIV AES256-GCM privateKey expects 32 bytes, got %d", len(privateKey))
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(initializationVector) != aead.NonceSize() {
		return nil, fmt.Errorf("Crypto.encryptWithManagedIV AES256-GCM initializationVector expects %d bytes, got %d", aead.NonceSize(), len(initializationVector))
	}
	return aead.Seal(nil, initializationVector, clearText, additionalData), nil
}

func decryptAESGCM(algorithm string, privateKey, initializationVector, cipherText, additionalData []byte) ([]byte, error) {
	if normalizeCryptoAlgorithm(algorithm) != "AES256GCM" {
		return nil, fmt.Errorf("Crypto.decryptWithManagedIV authenticated data only supports AES256-GCM")
	}
	if len(privateKey) != 32 {
		return nil, fmt.Errorf("Crypto.decryptWithManagedIV AES256-GCM privateKey expects 32 bytes, got %d", len(privateKey))
	}
	block, err := aes.NewCipher(privateKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(initializationVector) != aead.NonceSize() {
		return nil, fmt.Errorf("Crypto.decryptWithManagedIV AES256-GCM initializationVector expects %d bytes, got %d", aead.NonceSize(), len(initializationVector))
	}
	return aead.Open(nil, initializationVector, cipherText, additionalData)
}

func localCryptoSignature(algorithm string, input []byte) ([]byte, error) {
	digestAlgorithm, err := signatureDigestAlgorithm(algorithm)
	if err != nil {
		return nil, err
	}
	return generateDigest(digestAlgorithm, input)
}

func signatureDigestAlgorithm(algorithm string) (string, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	switch normalized {
	case "RSA", "RSASHA1", "ECDSASHA1":
		return "SHA1", nil
	case "RSASHA256", "ECDSASHA256":
		return "SHA256", nil
	case "RSASHA384", "ECDSASHA384":
		return "SHA384", nil
	case "RSASHA512", "ECDSASHA512":
		return "SHA512", nil
	default:
		return "", fmt.Errorf("unsupported signature algorithm %q", algorithm)
	}
}

func aesKeySizeForAlgorithm(algorithm string) (int, error) {
	normalized := normalizeCryptoAlgorithm(algorithm)
	if strings.HasSuffix(normalized, "CBC") {
		normalized = strings.TrimSuffix(normalized, "CBC")
	}
	switch normalized {
	case "AES128":
		return 16, nil
	case "AES192":
		return 24, nil
	case "AES256":
		return 32, nil
	default:
		return 0, fmt.Errorf("unsupported encryption algorithm %q", algorithm)
	}
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS7 padding length")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("invalid PKCS7 padding")
		}
	}
	return data[:len(data)-padding], nil
}

func mathUnary(callee string, args []Value) (Value, error) {
	if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueDecimal) {
		return Null, fmt.Errorf("%s expects numeric argument", callee)
	}
	n := numericFloat(args[0])
	if math.IsInf(n, 0) || math.IsNaN(n) {
		return Null, fmt.Errorf("%s argument must be finite", callee)
	}
	switch callee {
	case "Math.abs":
		if args[0].Kind == ValueInt {
			if args[0].Int == math.MinInt64 {
				return Null, fmt.Errorf("Math.abs integer overflow")
			}
			if args[0].Int < 0 {
				return Int(-args[0].Int), nil
			}
			return args[0], nil
		}
		return decimalAbsValue(args[0])
	case "Math.floor", "Math.ceil", "Math.rint":
		switch callee {
		case "Math.floor":
			return Decimal(math.Floor(n)), nil
		case "Math.ceil":
			return Decimal(math.Ceil(n)), nil
		default:
			return Decimal(roundHalfEven(n)), nil
		}
	case "Math.round":
		rounded, err := int64FromFloat("Math.round", roundHalfEven(n))
		if err != nil {
			return Null, err
		}
		return Int(rounded), nil
	case "Math.roundToLong":
		rounded, err := int64FromFloat("Math.roundToLong", roundHalfEven(n))
		if err != nil {
			return Null, err
		}
		return Int(rounded), nil
	case "Math.signum":
		switch {
		case n > 0:
			return Int(1), nil
		case n < 0:
			return Int(-1), nil
		default:
			return Int(0), nil
		}
	case "Math.sqrt":
		if n < 0 {
			return Null, fmt.Errorf("Math.sqrt argument out of domain")
		}
		return finiteDecimalResult(callee, math.Sqrt(n))
	case "Math.cbrt":
		return finiteDecimalResult(callee, math.Cbrt(n))
	case "Math.acos":
		if n < -1 || n > 1 {
			return Null, newExceptionError("System.MathException", "Math.acos argument out of domain")
		}
		return finiteDecimalResult(callee, math.Acos(n))
	case "Math.asin":
		if n < -1 || n > 1 {
			return Null, newExceptionError("System.MathException", "Math.asin argument out of domain")
		}
		return finiteDecimalResult(callee, math.Asin(n))
	case "Math.atan":
		return finiteDecimalResult(callee, math.Atan(n))
	case "Math.cos":
		return finiteDecimalResult(callee, math.Cos(n))
	case "Math.sin":
		return finiteDecimalResult(callee, math.Sin(n))
	case "Math.tan":
		return finiteDecimalResult(callee, math.Tan(n))
	case "Math.cosh":
		return finiteDecimalResult(callee, math.Cosh(n))
	case "Math.sinh":
		return finiteDecimalResult(callee, math.Sinh(n))
	case "Math.tanh":
		return finiteDecimalResult(callee, math.Tanh(n))
	case "Math.exp":
		return finiteDecimalResult(callee, math.Exp(n))
	case "Math.log":
		if n <= 0 {
			return Null, fmt.Errorf("Math.log argument out of domain")
		}
		return finiteDecimalResult(callee, math.Log(n))
	case "Math.log10":
		if n <= 0 {
			return Null, fmt.Errorf("Math.log10 argument out of domain")
		}
		return finiteDecimalResult(callee, math.Log10(n))
	default:
		return Null, unsupportedCallError(callee)
	}
}

func mathBinary(callee string, args []Value) (Value, error) {
	if len(args) != 2 || !isMathNumeric(args[0]) || !isMathNumeric(args[1]) {
		return Null, fmt.Errorf("%s expects two numeric arguments", callee)
	}
	left := numericFloat(args[0])
	right := numericFloat(args[1])
	if math.IsInf(left, 0) || math.IsNaN(left) || math.IsInf(right, 0) || math.IsNaN(right) {
		return Null, fmt.Errorf("%s arguments must be finite", callee)
	}
	switch callee {
	case "Math.max":
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(int64(math.Max(left, right))), nil
		}
		return Decimal(math.Max(left, right)), nil
	case "Math.min":
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(int64(math.Min(left, right))), nil
		}
		return Decimal(math.Min(left, right)), nil
	case "Math.mod":
		if right == 0 {
			return Null, fmt.Errorf("Math.mod divisor cannot be zero")
		}
		if args[0].Kind == ValueInt && args[1].Kind == ValueInt {
			return Int(args[0].Int % args[1].Int), nil
		}
		return Decimal(math.Mod(left, right)), nil
	case "Math.pow":
		return finiteDecimalResult(callee, math.Pow(left, right))
	case "Math.atan2":
		return finiteDecimalResult(callee, math.Atan2(left, right))
	default:
		return Null, unsupportedCallError(callee)
	}
}

func finiteDecimalResult(callee string, value float64) (Value, error) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return Null, fmt.Errorf("%s result must be finite", callee)
	}
	return Decimal(value), nil
}

func isMathNumeric(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueDecimal
}

func numericFloat(value Value) float64 {
	if value.Kind == ValueInt {
		return float64(value.Int)
	}
	return value.Decimal
}

func builtinEnumStaticValue(typeName, memberName string) (Value, bool) {
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	if canonical, names, ok := coreEnumSpec(typeName); ok {
		return namedEnumStaticValue(canonical, names, canonical+"."+memberName)
	}
	switch {
	case strings.EqualFold(typeName, "AccessLevel"):
		for _, known := range []string{"USER_MODE", "SYSTEM_MODE"} {
			if strings.EqualFold(memberName, known) {
				return Value{Kind: ValueObject, Type: "AccessLevel", Text: known}, true
			}
		}
	case strings.EqualFold(typeName, "AccessType"):
		return namedEnumStaticValue("AccessType", accessTypeNames, "AccessType."+memberName)
	case strings.EqualFold(typeName, "RoundingMode"):
		if mode, ok := canonicalDecimalRoundingModeName(memberName); ok {
			return Value{Kind: ValueObject, Type: "RoundingMode", Text: mode}, true
		}
	case strings.EqualFold(typeName, "LoggingLevel"):
		if level, ok := canonicalLoggingLevelName(memberName); ok {
			return Value{Kind: ValueObject, Type: "LoggingLevel", Text: level}, true
		}
	case strings.EqualFold(typeName, "TriggerOperation"):
		if operation, ok := canonicalTriggerOperationName(memberName); ok {
			return Value{Kind: ValueObject, Type: "TriggerOperation", Text: operation}, true
		}
	case strings.EqualFold(typeName, "StatusCode"):
		if statusCode, ok := canonicalStatusCodeName(memberName); ok {
			return Value{Kind: ValueObject, Type: "StatusCode", Text: statusCode}, true
		}
	case strings.EqualFold(typeName, "JSONToken"):
		if token, ok := canonicalJSONTokenName(memberName); ok {
			return Value{Kind: ValueObject, Type: "JSONToken", Text: token}, true
		}
	case strings.EqualFold(typeName, "XmlTag"):
		if tag, ok := canonicalXmlTagName(memberName); ok {
			return Value{Kind: ValueObject, Type: "XmlTag", Text: tag}, true
		}
	case strings.EqualFold(typeName, "DisplayType") || strings.EqualFold(typeName, "Schema.DisplayType"):
		return schemaDisplayTypeStaticValue("Schema.DisplayType." + memberName)
	case strings.EqualFold(typeName, "SOAPType") || strings.EqualFold(typeName, "SoapType") || strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "Schema.SoapType"):
		return schemaSOAPTypeStaticValue("Schema.SOAPType." + memberName)
	case strings.EqualFold(typeName, "FieldDescribeOptions") || strings.EqualFold(typeName, "Schema.FieldDescribeOptions"):
		for _, known := range schemaFieldDescribeOptionNames {
			if strings.EqualFold(memberName, known) {
				return Value{Kind: ValueObject, Type: "Schema.FieldDescribeOptions", Text: known}, true
			}
		}
	case strings.EqualFold(typeName, "SObjectDescribeOptions") || strings.EqualFold(typeName, "Schema.SObjectDescribeOptions"):
		for _, known := range schemaSObjectDescribeOptionNames {
			if strings.EqualFold(memberName, known) {
				return Value{Kind: ValueObject, Type: "Schema.SObjectDescribeOptions", Text: known}, true
			}
		}
	}
	return Null, false
}

func canonicalTriggerOperationName(name string) (string, bool) {
	for _, candidate := range triggerOperationNames {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
}

func canonicalStatusCodeName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", false
	}
	return strings.ToUpper(trimmed), true
}

func jsonSuppressNulls(callee string, args []Value) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueBool {
		return false, fmt.Errorf("%s expects suppressApexObjectNulls Boolean", callee)
	}
	return args[0].Bool, nil
}

func typedJSONMapKey(typeName, key string) (Value, error) {
	if strings.EqualFold(typeName, "String") || strings.EqualFold(typeName, "Object") {
		return String(key), nil
	}
	value, ok, err := typedScalarFromJSON(typeName, key)
	if err != nil {
		return Null, err
	}
	if ok {
		return value, nil
	}
	return Null, jsonDeserializeException("JSON.deserialize supports Map keys only for scalar/String/Object targets, got %s", typeName)
}

func (vm *VM) schemaGlobalDescribe() Value {
	if vm.globalDescribeCache != nil {
		return *vm.globalDescribeCache
	}
	out := Map()
	out.Type = "Schema.GlobalDescribeMap"
	if vm.Org == nil {
		return out
	}
	names := make([]string, 0, len(vm.Org.Objects))
	seen := make(map[string]bool, cap(names))
	for name := range vm.Org.Objects {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			continue
		}
		names = append(names, name)
		seen[key] = true
	}
	sort.Strings(names)
	for _, name := range names {
		token := sObjectTypeToken(name)
		for _, alias := range vm.schemaDescribeMapAliases(name) {
			vm.putSchemaGlobalDescribeAlias(&out, alias, name, token)
		}
	}
	vm.globalDescribeCache = &out
	return out
}

func (vm *VM) putSchemaGlobalDescribeAlias(out *Value, alias, objectName string, token Value) {
	key := mapKey(String(alias))
	if existing, ok := out.Map[key]; ok && !vm.schemaGlobalDescribeAliasShouldReplace(alias, objectName, existing) {
		return
	}
	out.Map[key] = token
}

var standardDescribeTabObjects = map[string]struct{}{
	"account":     {},
	"campaign":    {},
	"case":        {},
	"contact":     {},
	"contract":    {},
	"event":       {},
	"lead":        {},
	"opportunity": {},
	"order":       {},
	"pricebook2":  {},
	"product2":    {},
	"task":        {},
	"user":        {},
}

// foldClassKeyBuf is the max identifier length handled allocation-free by the
// fold-lookup helpers. Apex class/namespace names are far shorter; longer names
// fall back to the allocating canonicalClassLookupKey path.
const foldClassKeyBuf = 256

func resultForLookup() *Result {
	return &Result{}
}

func newRestRequest() Value {
	request := Object("RestRequest")
	request.Fields["requestURI"] = String("")
	request.Fields["resourcePath"] = String("")
	request.Fields["httpMethod"] = String("")
	request.Fields["remoteAddress"] = String("")
	request.Fields["headers"] = typedMap("Map<String,String>")
	request.Fields["params"] = typedMap("Map<String,String>")
	request.Fields["requestBody"] = nullBlob()
	return request
}

func nullBlob() Value {
	blob := Object("Blob")
	blob.Fields["value"] = Null
	return blob
}

func newPageReference(rawURL string) Value {
	page := Object("PageReference")
	page.Fields["url"] = String(rawURL)
	page.Fields["parameters"] = pageReferenceParameters(rawURL)
	page.Fields["headers"] = typedMap("Map<String,String>")
	page.Fields["cookies"] = typedMap("Map<String,Cookie>")
	return page
}

func newCookie(args []Value) (Value, error) {
	if len(args) != 5 && len(args) != 6 && len(args) != 7 {
		return Null, fmt.Errorf("Cookie constructor expects 5, 6, or 7 arguments")
	}
	if args[0].Kind != ValueString || args[1].Kind != ValueString || (args[2].Kind != ValueString && args[2].Kind != ValueNull) || args[3].Kind != ValueInt || args[4].Kind != ValueBool {
		return Null, fmt.Errorf("Cookie constructor expects name, value, path, maxAge, and isSecure")
	}
	cookie := Object("Cookie")
	cookie.Runtime = "System.Cookie"
	cookie.Fields["name"] = args[0]
	cookie.Fields["value"] = args[1]
	cookie.Fields["path"] = args[2]
	cookie.Fields["maxAge"] = args[3]
	cookie.Fields["secure"] = args[4]
	cookie.Fields["sameSite"] = Null
	cookie.Fields["httpOnly"] = Bool(false)
	if len(args) >= 6 {
		if args[5].Kind != ValueString {
			return Null, fmt.Errorf("Cookie constructor sameSite expects String")
		}
		cookie.Fields["sameSite"] = args[5]
	}
	if len(args) == 7 {
		if args[6].Kind != ValueBool {
			return Null, fmt.Errorf("Cookie constructor isHttpOnly expects Boolean")
		}
		cookie.Fields["httpOnly"] = args[6]
	}
	return cookie, nil
}

func newLocation(latitude, longitude Value) Value {
	location := Object("Location")
	location.Fields["latitude"] = Decimal(numericFloat(latitude))
	location.Fields["longitude"] = Decimal(numericFloat(longitude))
	return location
}

func newDomainFromHostname(hostname string) Value {
	host := strings.ToLower(strings.TrimSpace(hostname))
	host = strings.TrimSuffix(host, ".")
	domain := Object("Domain")
	domain.Fields["hostname"] = String(host)
	domain.Fields["domainType"] = domainTypeForHostname(host)
	domain.Fields["myDomainName"] = String(domainLabel(host))
	domain.Fields["packageName"] = String(domainPackageName(host))
	domain.Fields["sandboxName"] = Null
	domain.Fields["sitesSubdomainName"] = String(domainLabel(host))
	return domain
}

func domainParserHost(value Value) (string, error) {
	switch value.Kind {
	case ValueString:
		return hostFromURLText(value.Text), nil
	case ValueObject:
		if strings.EqualFold(value.Type, "URL") || strings.EqualFold(value.Type, "Url") {
			raw, err := platformScalarText(value, "URL")
			if err != nil {
				return "", err
			}
			return hostFromURLText(raw), nil
		}
	}
	return "", fmt.Errorf("DomainParser.parse expects hostname String or URL")
}

func hostFromURLText(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if parsed, err := url.Parse(text); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	if parsed, err := url.Parse("https://" + text); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return text
}

func domainLabel(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	first := strings.Split(host, ".")[0]
	if before, _, ok := strings.Cut(first, "--"); ok {
		first = before
	}
	return first
}

func domainPackageName(host string) string {
	first := strings.Split(strings.TrimSpace(host), ".")[0]
	before, _, ok := strings.Cut(first, "--")
	if !ok {
		return ""
	}
	return before
}

func domainTypeForHostname(host string) Value {
	normalized := strings.ToLower(host)
	name := "ORG_MY_DOMAIN"
	switch {
	case strings.Contains(normalized, "content"):
		name = "CONTENT_DOMAIN"
	case strings.Contains(normalized, "builder"):
		name = "EXPERIENCE_CLOUD_SITES_BUILDER_DOMAIN"
	case strings.Contains(normalized, "live-preview"):
		name = "EXPERIENCE_CLOUD_SITES_LIVE_PREVIEW_DOMAIN"
	case strings.Contains(normalized, "preview"):
		name = "EXPERIENCE_CLOUD_SITES_PREVIEW_DOMAIN"
	case strings.Contains(normalized, "site"):
		name = "EXPERIENCE_CLOUD_SITES_DOMAIN"
	case strings.Contains(normalized, "visualforce") || strings.Contains(normalized, "--"):
		name = "VISUALFORCE_DOMAIN"
	case strings.Contains(normalized, "lightning-container"):
		name = "LIGHTNING_CONTAINER_COMPONENT_DOMAIN"
	case strings.Contains(normalized, "lightning"):
		name = "LIGHTNING_DOMAIN"
	case strings.Contains(normalized, "setup"):
		name = "SETUP_DOMAIN"
	}
	return Value{Kind: ValueObject, Type: "DomainType", Text: name}
}

func localDomainHostname(kind, packageName string) string {
	normalizedPackage := strings.ToLower(strings.TrimSpace(packageName))
	packagePrefix := ""
	if normalizedPackage != "" {
		packagePrefix = normalizedPackage + "--"
	}
	switch strings.ToLower(kind) {
	case "contenthostname":
		return "glade.content.local"
	case "experiencecloudsitesbuilderhostname":
		return "glade.builder.sites.local"
	case "experiencecloudsiteshostname":
		return "glade.sites.local"
	case "experiencecloudsiteslivepreviewhostname":
		return "glade.live-preview.sites.local"
	case "experiencecloudsitespreviewhostname":
		return "glade.preview.sites.local"
	case "lightningcontainercomponenthostname":
		return packagePrefix + "glade.lightning-container.local"
	case "lightninghostname":
		return "glade.lightning.local"
	case "orgmydomainhostname", "org_my_domain":
		return "glade.my.salesforce.local"
	case "salesforcesiteshostname":
		return "glade.salesforce-sites.local"
	case "setuphostname":
		return "glade.setup.local"
	case "visualforcehostname":
		return packagePrefix + "glade.visualforce.local"
	default:
		return "glade.my.salesforce.local"
	}
}

func locationCoordinate(location Value, field string) (float64, bool) {
	if location.Kind != ValueObject || (!strings.EqualFold(location.Type, "Location") && !strings.EqualFold(location.Type, "Address")) {
		return 0, false
	}
	_, value, ok := objectFieldValue(location, field)
	if !ok || !isMathNumeric(value) {
		return 0, false
	}
	return numericFloat(value), true
}

func locationDistance(left, right Value, unit string) (Value, error) {
	leftLat, ok := locationCoordinate(left, "latitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects Location latitude")
	}
	leftLon, ok := locationCoordinate(left, "longitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects Location longitude")
	}
	rightLat, ok := locationCoordinate(right, "latitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects other Location latitude")
	}
	rightLon, ok := locationCoordinate(right, "longitude")
	if !ok {
		return Null, fmt.Errorf("Location.getDistance expects other Location longitude")
	}
	const earthKm = 6371.0088
	lat1 := leftLat * math.Pi / 180
	lat2 := rightLat * math.Pi / 180
	dLat := (rightLat - leftLat) * math.Pi / 180
	dLon := (rightLon - leftLon) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	distance := earthKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "mi", "mile", "miles":
		distance *= 0.621371192237334
	case "m", "meter", "meters":
		distance *= 1000
	case "km", "kilometer", "kilometers", "":
	default:
		return Null, fmt.Errorf("Location.getDistance unit must be mi, km, or m")
	}
	return Decimal(distance), nil
}

func newQueueableDuplicateSignatureBuilder() Value {
	builder := Object("QueueableDuplicateSignature.Builder")
	builder.Fields["parts"] = typedList("List<String>")
	return builder
}

func newCurrencyValue(amount Value, isoCode string) Value {
	value := Object("CURRENCY")
	value.Fields["amount"] = Decimal(numericFloat(amount))
	value.Fields["isoCode"] = String(strings.ToUpper(strings.TrimSpace(isoCode)))
	return value
}

func currencyAmountText(value Value) string {
	if amount, ok := value.Fields["amount"]; ok {
		switch amount.Kind {
		case ValueDecimal:
			return strconv.FormatFloat(amount.Decimal, 'f', -1, 64)
		case ValueInt:
			return strconv.FormatInt(amount.Int, 10)
		}
	}
	return "0"
}

func currencyISOCode(value Value) string {
	if iso, ok := value.Fields["isoCode"]; ok && iso.Kind == ValueString && iso.Text != "" {
		return iso.Text
	}
	return "USD"
}

func formatLocalThreadingToken(id string) string {
	return "ref:_00Dlocal._" + id + ":ref"
}

func idText(value Value) string {
	id := scalarText(value)
	if id == "" {
		if text, ok := typedIDValueText(value); ok {
			id = text
		}
	}
	return strings.TrimSpace(id)
}

func recordIDFromEmail(subject, textBody, htmlBody Value) Value {
	for _, value := range []Value{subject, textBody, htmlBody} {
		if value.Kind != ValueString {
			continue
		}
		if id := recordIDFromThreadID(value.Text); id.Kind != ValueNull {
			return id
		}
	}
	return Null
}

func caseIDFromThreadID(text string) Value {
	if id := recordIDFromThreadID(text); id.Kind != ValueNull {
		if idText, ok := typedIDValueText(id); ok && strings.HasPrefix(idText, "500") {
			return id
		}
		return Null
	}
	start := strings.Index(text, "500")
	if start < 0 {
		return Null
	}
	end := start
	for end < len(text) {
		ch := text[end]
		if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
			break
		}
		end++
	}
	if end-start < 15 {
		return Null
	}
	id := text[start:end]
	if len(id) > 18 {
		id = id[:18]
	}
	return platformScalar("Id", id)
}

func recordIDFromThreadID(text string) Value {
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return r <= ' ' || r == '<' || r == '>' || r == '"' || r == '\'' || r == '[' || r == ']' || r == '(' || r == ')'
	}) {
		if !strings.Contains(strings.ToLower(token), "ref:") {
			continue
		}
		if id := idFromThreadingToken(token); id != "" {
			return platformScalar("Id", id)
		}
	}
	return Null
}

func idFromThreadingToken(token string) string {
	found := ""
	for i := 0; i < len(token); i++ {
		if !isSalesforceIDChar(token[i]) {
			continue
		}
		end := i
		for end < len(token) && isSalesforceIDChar(token[end]) {
			end++
		}
		if end-i >= 15 {
			id := token[i:end]
			if len(id) > 18 {
				id = id[:18]
			}
			found = id
		}
		i = end
	}
	return found
}

func isSalesforceIDChar(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func collatorCompare(left, right string) int64 {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func newPageTokenReference(rawURL string) Value {
	page := newPageReference(rawURL)
	page.Fields["__pageToken"] = Bool(true)
	return page
}

func pageReferenceParameters(rawURL string) Value {
	params := typedMap("Map<String,String>")
	params.Runtime = "pagereference-parameters"
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return params
	}
	for key, values := range parsed.Query() {
		if key == "" || len(values) == 0 {
			continue
		}
		encodedKey := mapKey(String(key))
		params.Map[encodedKey] = String(values[len(values)-1])
		params.MapKeys[encodedKey] = String(key)
	}
	return params
}

func pageReferenceURL(page Value) Value {
	raw, ok := page.Fields["url"]
	if !ok {
		return String("")
	}
	if raw.Kind == ValueNull {
		return raw
	}
	if raw.Kind != ValueString {
		return String("")
	}
	params, ok := page.Fields["parameters"]
	parsed, err := url.Parse(raw.Text)
	if err != nil {
		return raw
	}
	if !ok || params.Kind != ValueMap || params.Equal(pageReferenceParameters(raw.Text)) {
		if strings.Contains(parsed.RawQuery, "?") {
			query := url.Values{}
			for key, values := range parsed.Query() {
				if len(values) > 0 {
					query.Set(key, values[len(values)-1])
				}
			}
			parsed.RawQuery = query.Encode()
			return String(parsed.String())
		}
		return String(parsed.String())
	}
	query := url.Values{}
	for rawKey, value := range params.Map {
		key := mapStoredKey(params, rawKey)
		if key.Kind != ValueString || key.Text == "" || value.Kind == ValueNull {
			continue
		}
		if value.Kind == ValueString {
			query.Set(key.Text, value.Text)
			continue
		}
		query.Set(key.Text, value.String())
	}
	parsed.RawQuery = query.Encode()
	return String(parsed.String())
}

func pageReferenceAnchor(page Value) Value {
	urlValue := pageReferenceURL(page)
	if urlValue.Kind == ValueNull {
		return urlValue
	}
	if urlValue.Kind != ValueString || !strings.Contains(urlValue.Text, "#") {
		return Null
	}
	parsed, err := url.Parse(urlValue.Text)
	if err != nil {
		return Null
	}
	return String(parsed.Fragment)
}

func setPageReferenceAnchor(page *Value, anchor Value) error {
	if anchor.Kind != ValueString && anchor.Kind != ValueNull {
		return fmt.Errorf("PageReference.setAnchor expects String")
	}
	urlValue := pageReferenceURL(*page)
	if urlValue.Kind == ValueNull {
		return nil
	}
	rawURL := ""
	if urlValue.Kind == ValueString {
		rawURL = urlValue.Text
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if anchor.Kind == ValueNull {
		parsed.Fragment = ""
	} else {
		parsed.Fragment = strings.TrimPrefix(anchor.Text, "#")
	}
	page.Fields["url"] = String(parsed.String())
	return nil
}

var htmlVoidElementPattern = regexp.MustCompile(`(?i)<(area|base|br|col|embed|hr|img|input|link|meta|param|source|track|wbr)([^<>]*?)>`)

func (vm *VM) newPageReference(rawURL string) Value {
	return newPageReference(vm.normalizePageReferenceURL(rawURL))
}

func (vm *VM) normalizePageReferenceURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if !hasPrefixFold(rawURL, "page.") {
		return rawURL
	}
	rest := rawURL[len("Page."):]
	pageName := rest
	suffix := ""
	for _, sep := range []string{"?", "#"} {
		if idx := strings.Index(pageName, sep); idx >= 0 {
			suffix = pageName[idx:]
			pageName = pageName[:idx]
			break
		}
	}
	if pageName == "" || vm.pageReferences == nil {
		return rawURL
	}
	registered, ok := vm.pageReferences[strings.ToLower(pageName)]
	if !ok {
		return rawURL
	}
	return "/apex/" + registered + suffix
}

func newAuthVerificationResult(redirect, success, message Value) Value {
	result := Object("Auth.VerificationResult")
	result.Fields["redirect"] = redirect
	result.Fields["success"] = success
	result.Fields["message"] = message
	return result
}

func newSelectOption(value, label Value, disabled, escapeItem Value) Value {
	option := Object("SelectOption")
	option.Fields["value"] = value
	option.Fields["label"] = label
	option.Fields["disabled"] = disabled
	option.Fields["escapeItem"] = escapeItem
	return option
}

func newHttpRequest() Value {
	request := Object("HttpRequest")
	request.Fields["endpoint"] = String("")
	request.Fields["method"] = String("")
	request.Fields["headers"] = typedMap("Map<String,String>")
	request.Fields["body"] = String("")
	request.Fields["compressed"] = Bool(false)
	request.Fields["timeout"] = Int(defaultHttpTimeoutMillis)
	return request
}

func newHttpResponse() Value {
	response := Object("HttpResponse")
	response.Fields["statusCode"] = Int(200)
	response.Fields["status"] = String("OK")
	response.Fields["headers"] = typedMap("Map<String,String>")
	response.Fields["body"] = String("")
	return response
}

func newContinuation(args []Value, namedArgs map[string]Value) (Value, error) {
	if len(args) != 1 || len(namedArgs) != 0 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("Continuation constructor expects timeout Integer")
	}
	continuation := Object("Continuation")
	continuation.Fields["timeout"] = args[0]
	continuation.Fields["Timeout"] = args[0]
	continuation.Fields["ContinuationMethod"] = Null
	continuation.Fields["state"] = Null
	continuation.Fields["requests"] = typedMap("Map<String,HttpRequest>")
	return continuation, nil
}

func newFormulaBuilder() Value {
	builder := Object("formulaeval.FormulaBuilder")
	builder.Fields["formulaText"] = String("")
	builder.Fields["referencedFields"] = typedSet("Set<String>")
	return builder
}

func newFormulaInstance(builder Value) Value {
	instance := Object("formulaeval.FormulaInstance")
	if formulaText, ok := builder.Fields["formulaText"]; ok {
		instance.Fields["formulaText"] = formulaText
	}
	if referencedFields, ok := builder.Fields["referencedFields"]; ok {
		instance.Fields["referencedFields"] = referencedFields
	} else {
		instance.Fields["referencedFields"] = typedSet("Set<String>")
	}
	return instance
}

func newDatacloudFindDuplicatesResult() Value {
	result := Object("Datacloud.FindDuplicatesResult")
	result.Fields["duplicateResults"] = typedList("List<Datacloud.DuplicateResult>")
	result.Fields["errors"] = typedList("List<Database.Error>")
	result.Fields["success"] = Bool(true)
	return result
}

func newActionResult() Value {
	result := Object("Messaging.ActionResult")
	result.Fields["success"] = Bool(false)
	result.Fields["message"] = Null
	result.Fields["errorCode"] = Null
	return result
}

func newActionResultBuilder() Value {
	builder := Object("Messaging.ActionResult.Builder")
	builder.Fields["success"] = Bool(false)
	builder.Fields["message"] = Null
	builder.Fields["errorCode"] = Null
	return builder
}

func newActionableNotification() Value {
	notification := Object("Messaging.ActionableNotification")
	for _, field := range []string{"actionIdentifier", "notificationTypeId", "recipientId", "senderId", "targetId", "targetPageRef"} {
		notification.Fields[field] = Null
	}
	return notification
}

func newActionableNotificationBuilder() Value {
	builder := Object("Messaging.ActionableNotification.Builder")
	for _, field := range []string{"actionIdentifier", "notificationTypeId", "recipientId", "senderId", "targetId", "targetPageRef"} {
		builder.Fields[field] = Null
	}
	return builder
}

func newCustomNotification(args []Value) Value {
	notification := Object("Messaging.CustomNotification")
	for _, field := range []string{"notificationTypeId", "senderId", "title", "body", "targetId", "targetPageRef"} {
		notification.Fields[field] = Null
	}
	if len(args) == 6 {
		notification.Fields["notificationTypeId"] = args[0]
		notification.Fields["senderId"] = args[1]
		notification.Fields["title"] = args[2]
		notification.Fields["body"] = args[3]
		notification.Fields["targetId"] = args[4]
		notification.Fields["targetPageRef"] = args[5]
	}
	return notification
}

func newPushNotification(args []Value) Value {
	notification := Object("Messaging.PushNotification")
	notification.Fields["payload"] = typedMap("Map<String,Object>")
	notification.Fields["ttl"] = Null
	if len(args) == 1 {
		notification.Fields["payload"] = args[0]
	}
	return notification
}

func stringsFromList(value Value) []string {
	if value.Kind != ValueList {
		return nil
	}
	out := make([]string, 0, len(value.List))
	for _, item := range value.List {
		if item.Kind == ValueString {
			out = append(out, item.Text)
		}
	}
	return out
}

func stringValue(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	if text, ok := platformScalarObjectText(value); ok {
		return text
	}
	return ""
}

func boolValue(value Value) bool {
	return value.Kind == ValueBool && value.Bool
}

func (vm *VM) storageRecordStringField(record storage.Record, field string) string {
	if strings.EqualFold(field, "Id") {
		return string(record.ID)
	}
	if vm.Org != nil {
		if objectName, ok := vm.resolveObjectName(record.Object); ok {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
					field = resolved
				}
			}
		}
	}
	return storageStringField(record, field)
}

func (vm *VM) recordByIDValue(value Value) (storage.Record, bool) {
	idText, ok := idValueText(value)
	if !ok || idText == "" || vm.Org == nil {
		return storage.Record{}, false
	}
	id := storage.ID(idText)
	if len(idText) >= 3 {
		if objectName, ok := vm.sObjectNameForIDPrefix(idText[:3]); ok {
			if object, ok := vm.Org.Objects[objectName]; ok {
				if record, ok := object.Records[id]; ok {
					return record, true
				}
			}
		}
	}
	for _, object := range vm.Org.Objects {
		if record, ok := object.Records[id]; ok {
			return record, true
		}
		for _, record := range object.Records {
			if record.ID == id {
				return record, true
			}
			if fieldID, ok := record.GetField("Id"); ok && storageIDFromValue(fieldID) == id {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}

func storageStringField(record storage.Record, field string) string {
	value, ok := record.GetField(field)
	if !ok {
		return ""
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueInteger:
		return strconv.FormatInt(value.Integer, 10)
	case storage.ValueBoolean:
		if value.Boolean {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func newRestResponse() Value {
	response := Object("RestResponse")
	response.Fields["statusCode"] = Int(200)
	response.Fields["headers"] = typedMap("Map<String,String>")
	response.Fields["responseBody"] = Null
	return response
}

func typedMap(typeName string) Value {
	value := Map()
	value.Type = typeName
	return value
}

func typedList(typeName string) Value {
	value := List()
	value.Type = typeName
	return value
}

func typedSet(typeName string) Value {
	value := Set()
	value.Type = typeName
	return value
}

var canonicalRuntimeTypeNames = []string{
	"HttpRequest", "HttpResponse", "StaticResourceCalloutMock", "MultiStaticResourceCalloutMock",
	"RestRequest", "RestResponse", "Continuation", "PageReference", "VisualEditor.DataRow",
	"VisualEditor.DynamicPickListRows", "Dom.Document", "Auth.UserData", "Auth.VerificationResult",
	"Auth.AuthConfiguration", "Auth.JWT", "Metadata.DeployContainer", "Metadata.CustomMetadata",
	"Metadata.CustomMetadataValue", "Metadata.CustomObject", "Metadata.CustomField", "Metadata.Metadata",
	"Metadata.DeployResult", "Metadata.DeployDetails", "Metadata.DeployMessage", "Metadata.DeployCallbackContext",
	"Metadata.AsyncResult", "SelectOption", "ApexPages.StandardController", "ApexPages.StandardSetController",
	"ApexPages.Message", "Messaging.SendEmailResult", "Messaging.EmailFileAttachment", "Messaging.SingleEmailMessage",
	"Messaging.MassEmailMessage", "Messaging.SendEmailOptions", "Messaging.CustomNotification",
	"Messaging.PushNotification", "Messaging.ActionResult", "Messaging.ActionableNotification",
	"Messaging.ActionResult.Builder", "Messaging.ActionableNotification.Builder", "Messaging.Builder",
	"Messaging.InboundEmail", "Messaging.InboundEnvelope", "Messaging.InboundEmailResult",
	"Messaging.RenderEmailTemplateBodyResult", "Messaging.RenderEmailTemplateError", "URL", "Version", "InstallContext", "UninstallContext",
}

func isCanonicalRuntimeTypeName(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	for _, known := range canonicalRuntimeTypeNames {
		if strings.EqualFold(typeName, known) {
			return true
		}
	}
	return false
}

func canonicalRuntimeTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	for _, known := range canonicalRuntimeTypeNames {
		if strings.EqualFold(typeName, known) {
			return known
		}
	}
	return typeName
}

func (vm *VM) lookupRestContextField(name string) (Value, bool, error) {
	canonical, ok := canonicalRestContextPath(name)
	if !ok {
		return Null, false, nil
	}
	switch canonical {
	case "RestContext.request":
		if vm.restRequest.Kind == "" {
			return Null, true, nil
		}
		return vm.restRequest, true, nil
	case "RestContext.response":
		if vm.restResponse.Kind == "" || vm.restResponse.Kind == ValueNull {
			vm.restResponse = newRestResponse()
		}
		return vm.restResponse, true, nil
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if strings.HasPrefix(canonical, root+".") {
				value, _, err := vm.lookupRestContextField(root)
				if err != nil {
					return Null, true, err
				}
				out, err := vm.lookupPath(value, strings.Split(strings.TrimPrefix(canonical, root+"."), "."))
				if err != nil {
					return Null, true, err
				}
				return out, true, nil
			}
		}
		return Null, false, nil
	}
}

func (vm *VM) assignRestContextField(name string, value Value) (bool, error) {
	canonical, ok := canonicalRestContextPath(name)
	if !ok {
		return false, nil
	}
	switch canonical {
	case "RestContext.request":
		if value.Kind != ValueNull && (value.Kind != ValueObject || value.Type != "RestRequest") {
			return true, fmt.Errorf("RestContext.request expects RestRequest")
		}
		vm.restRequest = value
		return true, nil
	case "RestContext.response":
		if value.Kind != ValueNull && (value.Kind != ValueObject || value.Type != "RestResponse") {
			return true, fmt.Errorf("RestContext.response expects RestResponse")
		}
		vm.restResponse = value
		return true, nil
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if strings.HasPrefix(canonical, root+".") {
				current, _, err := vm.lookupRestContextField(root)
				if err != nil {
					return true, err
				}
				if current.Kind == ValueNull {
					return true, newNullDereferenceError("while assigning " + name)
				}
				if err := vm.assignPath(current, strings.Split(strings.TrimPrefix(canonical, root+"."), "."), value); err != nil {
					return true, err
				}
				if root == "RestContext.request" {
					vm.restRequest = current
				} else {
					vm.restResponse = current
				}
				return true, nil
			}
		}
		return false, nil
	}
}

func canonicalRestContextPath(name string) (string, bool) {
	switch {
	case strings.EqualFold(name, "RestContext.request"):
		return "RestContext.request", true
	case strings.EqualFold(name, "RestContext.response"):
		return "RestContext.response", true
	default:
		for _, root := range []string{"RestContext.request", "RestContext.response"} {
			if len(name) > len(root) && strings.EqualFold(name[:len(root)], root) && name[len(root)] == '.' {
				return root + name[len(root):], true
			}
		}
		return "", false
	}
}

func (vm *VM) constructValue(typeName string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	return vm.constructValueWithLiteral(typeName, args, namedArgs, result, false)
}

func (vm *VM) constructValueLiteral(typeName string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	return vm.constructValueWithLiteral(typeName, args, namedArgs, result, true)
}

func (vm *VM) resolveUniqueNestedTypeName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") || vm.currentClass == "" {
		return "", false
	}
	commonSObjectType := isCommonSObjectTypeName(typeName)
	currentTops := vm.currentLexicalTopCandidates()
	if len(currentTops) == 0 {
		return "", false
	}
	currentTopKey := strings.ToLower(strings.Join(currentTops, "\x01"))
	typeKey := strings.ToLower(typeName)
	cacheKey := currentTopKey + "\x00" + typeKey
	if vm.uniqueNestedTypeCache != nil {
		if cached, ok := vm.uniqueNestedTypeCache[cacheKey]; ok {
			return cached.Name, cached.OK
		}
	} else {
		vm.uniqueNestedTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	suffix := "." + typeKey
	for _, entry := range vm.classNameSearchEntries() {
		if !strings.HasSuffix(entry.Lower, suffix) {
			continue
		}
		for _, currentTop := range currentTops {
			if nestedTypeBelongsToTop(entry.Name, currentTop) {
				vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: entry.Name, OK: true}
				return entry.Name, true
			}
		}
	}
	if commonSObjectType {
		vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	var unique string
	for _, entry := range vm.classNameSearchEntries() {
		if !strings.HasSuffix(entry.Lower, suffix) {
			continue
		}
		if _, ok := vm.lookupClass(typeName); ok {
			continue
		}
		candidate := entry.Name
		if class, ok := vm.lookupClass(entry.Name); ok {
			candidate = class.Name
		}
		if unique != "" && !strings.EqualFold(unique, candidate) {
			vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{}
			return "", false
		}
		unique = candidate
	}
	if unique != "" {
		vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: unique, OK: true}
		return unique, true
	}
	vm.uniqueNestedTypeCache[cacheKey] = uniqueNestedTypeLookup{}
	return "", false
}

func (vm *VM) resolveTopLevelClassName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") {
		return "", false
	}
	currentNamespace := strings.TrimSpace(vm.currentExecutionNamespace())
	cacheKey := strings.ToLower(currentNamespace) + "|" + strings.ToLower(typeName)
	if vm.topLevelTypeCache != nil {
		if cached, ok := vm.topLevelTypeCache[cacheKey]; ok {
			return cached.Name, cached.OK
		}
	} else {
		vm.topLevelTypeCache = make(map[string]uniqueNestedTypeLookup)
	}

	entry, ok := vm.topLevelClassLookupIndex()[canonicalClassLookupKey(typeName)]
	if !ok {
		vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	if currentNamespace != "" {
		if candidate := entry.ByNamespace[canonicalClassLookupKey(currentNamespace)]; candidate != "" {
			vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: candidate, OK: true}
			return candidate, true
		}
	}
	if entry.Ambiguous || entry.Unique == "" {
		vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	vm.topLevelTypeCache[cacheKey] = uniqueNestedTypeLookup{Name: entry.Unique, OK: true}
	return entry.Unique, true
}

func (vm *VM) classNameSearchEntries() []classNameSearchEntry {
	if vm.classNameSearchCache != nil {
		return vm.classNameSearchCache
	}
	classNames := make([]string, 0, len(vm.Classes))
	for name := range vm.Classes {
		classNames = append(classNames, name)
	}
	sort.Strings(classNames)
	vm.classNameSearchCache = make([]classNameSearchEntry, 0, len(classNames))
	for _, name := range classNames {
		vm.classNameSearchCache = append(vm.classNameSearchCache, classNameSearchEntry{
			Name:  name,
			Lower: strings.ToLower(name),
		})
	}
	return vm.classNameSearchCache
}

func (vm *VM) currentLexicalTopCandidates() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if dot := strings.IndexByte(name, '.'); dot > 0 {
			name = name[:dot]
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, name)
	}
	add(vm.currentClass)
	if short := shortTypeName(vm.currentClass); short != "" && !strings.EqualFold(short, vm.currentClass) {
		add(short)
	}
	if class, ok := vm.lookupClass(vm.currentClass); ok {
		add(class.Name)
		if class.Namespace != "" {
			add(runtimeClassName(class))
		}
	}
	return out
}

func nestedTypeBelongsToTop(name, top string) bool {
	name = strings.TrimSpace(name)
	top = strings.TrimSpace(top)
	if name == "" || top == "" {
		return false
	}
	return hasPrefixFold(name, strings.ToLower(top)+".")
}

func (vm *VM) resolveOnlyNestedTypeName(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" || strings.Contains(typeName, ".") {
		return "", false
	}
	typeKey := strings.ToLower(typeName)
	if vm.onlyNestedTypeCache != nil {
		if cached, ok := vm.onlyNestedTypeCache[typeKey]; ok {
			return cached.Name, cached.OK
		}
	} else {
		vm.onlyNestedTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	suffix := "." + typeKey
	unique := ""
	for _, entry := range vm.classNameSearchEntries() {
		if !strings.HasSuffix(entry.Lower, suffix) {
			continue
		}
		candidate := entry.Name
		if class, ok := vm.lookupClass(entry.Name); ok {
			candidate = class.Name
		}
		if unique != "" && !strings.EqualFold(unique, candidate) {
			vm.onlyNestedTypeCache[typeKey] = uniqueNestedTypeLookup{}
			return "", false
		}
		unique = candidate
	}
	if unique == "" {
		vm.onlyNestedTypeCache[typeKey] = uniqueNestedTypeLookup{}
		return "", false
	}
	vm.onlyNestedTypeCache[typeKey] = uniqueNestedTypeLookup{Name: unique, OK: true}
	return unique, true
}

func (vm *VM) queryLocatorIterable(typeName string, value Value) (Value, error) {
	records, ok := value.Fields["Records"]
	if !ok || records.Kind != ValueList {
		return Null, fmt.Errorf("Database.QueryLocator missing records")
	}
	iterable := List(append([]Value(nil), records.List...)...)
	iterable.Type = typeName
	iterable.Fields = map[string]Value{"__queryLocator": value}
	elementType, ok := collectionElementType(typeName)
	if !ok {
		return iterable, nil
	}
	for i, item := range iterable.List {
		coerced, err := vm.coerceAssignable(elementType, item)
		if err != nil {
			return Null, err
		}
		iterable.List[i] = coerced
	}
	return iterable, nil
}

func (vm *VM) resolveEnumClass(typeName string) (Class, bool) {
	cacheKey := vm.currentClass + "|" + typeName
	if vm.enumLookup != nil {
		if cached, ok := vm.enumLookup[cacheKey]; ok {
			return cached.Class, cached.OK
		}
	} else {
		vm.enumLookup = make(map[string]enumClassLookup)
	}
	if enumType, ok := vm.resolveClassName(typeName); ok {
		if class, ok := vm.lookupClass(enumType); ok && len(class.EnumValues) > 0 {
			vm.enumLookup[cacheKey] = enumClassLookup{Class: class, OK: true}
			return class, true
		}
	}
	if !strings.Contains(typeName, ".") {
		if vm.enumSuffixLookup == nil {
			vm.rebuildEnumSuffixLookup()
		}
		if cached, ok := vm.enumSuffixLookup[canonicalClassLookupKey(typeName)]; ok {
			vm.enumLookup[cacheKey] = cached
			return cached.Class, cached.OK
		}
	}
	vm.enumLookup[cacheKey] = enumClassLookup{}
	return Class{}, false
}

func (vm *VM) rebuildEnumSuffixLookup() {
	vm.enumSuffixLookup = make(map[string]enumClassLookup)
	for _, class := range vm.Classes {
		if len(class.EnumValues) == 0 {
			continue
		}
		name := class.Name
		if dot := strings.LastIndexByte(name, '.'); dot >= 0 && dot+1 < len(name) {
			key := canonicalClassLookupKey(name[dot+1:])
			if _, exists := vm.enumSuffixLookup[key]; !exists {
				vm.enumSuffixLookup[key] = enumClassLookup{Class: class, OK: true}
			}
		}
	}
}

func isLikelyEnumValueText(text string) bool {
	if text == "" {
		return false
	}
	if dot := strings.LastIndexByte(text, '.'); dot >= 0 {
		text = text[dot+1:]
	}
	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' {
			continue
		}
		return false
	}
	return true
}

func (vm *VM) ensureAssignable(typeName string, value Value) error {
	_, err := vm.coerceAssignable(typeName, value)
	return err
}

func isDescribeSObjectResultType(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.DescribeSObjectResult") || strings.EqualFold(typeName, "DescribeSObjectResult")
}

func isDescribeFieldResultType(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.DescribeFieldResult") || strings.EqualFold(typeName, "DescribeFieldResult")
}

func isSObjectTypeToken(value Value) bool {
	return value.Kind == ValueObject && (strings.EqualFold(value.Type, "Schema.SObjectType") || strings.EqualFold(value.Type, "SObjectType"))
}

func (vm *VM) describeFromSObjectTypeToken(value Value) (Value, error) {
	objectValue, ok := value.Fields["object"]
	if !ok || objectValue.Kind != ValueString {
		return Null, fmt.Errorf("Schema.SObjectType token missing object")
	}
	objectName, definition, ok := vm.describeObjectDefinition(objectValue.Text)
	if !ok {
		return Null, fmt.Errorf("Schema.SObjectType.getDescribe unknown object %s", objectValue.Text)
	}
	return vm.describeSObjectValue(objectName, definition), nil
}

func (vm *VM) describeFromSObjectFieldToken(value Value) (Value, error) {
	objectValue, objectOK := value.Fields["object"]
	fieldValue, fieldOK := value.Fields["field"]
	if !objectOK || objectValue.Kind != ValueString || !fieldOK || fieldValue.Kind != ValueString {
		return Null, fmt.Errorf("Schema.SObjectField token missing object or field")
	}
	return vm.describeFieldValue(objectValue.Text, fieldValue.Text)
}

func (vm *VM) coerceCast(typeName string, value Value) (Value, error) {
	coerced, err := vm.coerceAssignable(typeName, value)
	if err == nil {
		coerced.Static = typeName
		return coerced, nil
	}
	targetType := typeExceptionTargetName(typeName)
	if value.Kind == ValueDecimal && strings.EqualFold(typeName, "Integer") {
		if value.Decimal < math.MinInt32 || value.Decimal > math.MaxInt32 {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", runtimeValueTypeName(value), targetType))
		}
		return Int(int64(value.Decimal)), nil
	}
	if value.Kind == ValueDecimal && strings.EqualFold(typeName, "Long") {
		if value.Decimal < float64(math.MinInt64) || value.Decimal > float64(math.MaxInt64) {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", runtimeValueTypeName(value), targetType))
		}
		return Int(int64(value.Decimal)), nil
	}
	var thrown *apexThrowError
	if errors.As(err, &thrown) {
		return Null, err
	}
	return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", runtimeValueTypeName(value), targetType))
}

func untypedIntegralDecimalLiteral(value Value) bool {
	if value.Kind != ValueDecimal || value.Type != "" || value.Static != "" {
		return false
	}
	return math.Trunc(value.Decimal) == value.Decimal
}

func typeExceptionTargetName(typeName string) string {
	if strings.EqualFold(typeName, "DateTime") {
		return "Datetime"
	}
	if typed := typeExceptionCollectionName(typeName); typed != "" {
		return typed
	}
	return typeName
}

func typeExceptionCollectionName(typeName string) string {
	base := collectionBase(typeName)
	if base == "" && !isMapType(typeName) {
		return ""
	}
	if isMapType(typeName) {
		keyType, valueType, ok := mapTypeArgs(typeName)
		if !ok {
			return ""
		}
		return "Map<" + typeExceptionAnyName(keyType) + "," + typeExceptionAnyName(valueType) + ">"
	}
	elementType, ok := collectionElementType(typeName)
	if !ok {
		return ""
	}
	return base + "<" + typeExceptionAnyName(elementType) + ">"
}

func typeExceptionAnyName(typeName string) string {
	if strings.EqualFold(typeName, "Object") || strings.EqualFold(typeName, "System.Object") {
		return "ANY"
	}
	return typeName
}

func stringEnumCoercionTarget(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Schema.SObjectType"), strings.EqualFold(typeName, "SObjectType"),
		strings.EqualFold(typeName, "Schema.SObjectField"), strings.EqualFold(typeName, "SObjectField"):
		return false
	}
	if strings.HasSuffix(typeName, "Type") {
		return true
	}
	switch typeName {
	case "TriggerOperation", "RoundingMode", "System.RoundingMode", "LoggingLevel", "AccessType", "System.AccessType", "ApexPages.Severity",
		"Schema.DisplayType", "DisplayType", "Schema.SOAPType", "SOAPType",
		"Metadata.DeployStatus", "Metadata.MetadataType":
		return true
	default:
		return false
	}
}

func (vm *VM) coerceCollectionElement(collectionType string, value Value) (Value, error) {
	elementType, ok := collectionElementType(collectionType)
	if !ok {
		return value, nil
	}
	return vm.coerceAssignable(elementType, value)
}

func (vm *VM) coerceMapEntry(mapType string, key, value Value) (Value, Value, error) {
	keyType, valueType, ok := mapTypeArgs(mapType)
	if !ok {
		return key, value, nil
	}
	coercedKey, err := vm.coerceAssignable(keyType, key)
	if err != nil {
		return Null, Null, fmt.Errorf("key: %w", err)
	}
	if strings.EqualFold(valueType, "String") && strings.EqualFold(value.Type, "Id") {
		if idText, ok := typedIDValueText(value); ok {
			return coercedKey, String(displayIDText(idText)), nil
		}
	}
	coercedValue, err := vm.coerceAssignable(valueType, value)
	if err != nil {
		return Null, Null, fmt.Errorf("value: %w", err)
	}
	return coercedKey, coercedValue, nil
}

func mapStoredKey(value Value, rawKey string) Value {
	if value.MapKeys != nil {
		if key, ok := value.MapKeys[rawKey]; ok {
			return key
		}
	}
	return valueFromMapKey(rawKey)
}

func (vm *VM) runtimeError(thrown Value) error {
	return runtimeError(thrown, vm.callStack)
}

func runtimeError(thrown Value, stack []callFrame) error {
	message := "unhandled exception"
	errorType := "Exception"
	thrown = annotateException(thrown, stack)
	if thrown.Kind != ValueNull {
		message = thrown.String()
		if thrown.Kind == ValueObject && thrown.Type != "" {
			errorType = thrown.Type
			if context, ok := thrown.Fields["__diagnosticContext"]; ok && context.Kind == ValueString && strings.TrimSpace(context.Text) != "" {
				message += " (context: " + context.Text + ")"
			}
		}
	}
	if len(stack) == 0 {
		return &RuntimeError{Type: errorType, Message: message}
	}
	return &RuntimeError{Type: errorType, Message: message, Stack: stackFrames(stack)}
}

func classNameFromMethod(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}
	return ""
}

func apexMethodMemberName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func defaultValue(typeName string, explicit Value) Value {
	if explicit.Kind != "" {
		if explicit.Kind == ValueNull && explicit.Type == "" {
			explicit.Type = typeName
		}
		if (typeName == "Decimal" || typeName == "Double") && explicit.Kind == ValueInt {
			return Decimal(float64(explicit.Int))
		}
		if collectionBase(typeName) != "" || isMapType(typeName) {
			if coerced, err := coerceCollectionValue(typeName, explicit); err == nil {
				return cloneValue(coerced)
			}
		}
		return cloneValue(explicit)
	}
	switch typeName {
	case "String":
		return Value{Kind: ValueNull, Type: typeName}
	default:
		return Value{Kind: ValueNull, Type: typeName}
	}
}

func defaultStaticFieldValue(className, fieldName, typeName string, explicit Value) Value {
	if (explicit.Kind == "" || (explicit.Kind == ValueNull && isSObjectFieldTokenType(explicit.Type))) &&
		isSObjectFieldTokenType(typeName) && strings.TrimSpace(fieldName) != "" &&
		(isCommonSObjectTypeName(className) || isCustomObjectLikeName(className)) {
		return sObjectFieldToken(className, fieldName)
	}
	if emptySObjectFieldTokenValue(explicit) && isSObjectFieldTokenType(typeName) && strings.TrimSpace(fieldName) != "" &&
		(isCommonSObjectTypeName(className) || isCustomObjectLikeName(className)) {
		return sObjectFieldToken(className, fieldName)
	}
	return defaultValue(typeName, explicit)
}

func emptySObjectFieldTokenValue(value Value) bool {
	if value.Kind != ValueObject || !isSObjectFieldTokenType(value.Type) {
		return false
	}
	if field, ok := value.Fields["field"]; ok && field.Kind == ValueString && strings.TrimSpace(field.Text) != "" {
		return false
	}
	if name, ok := value.Fields["Name"]; ok && name.Kind == ValueString && strings.TrimSpace(name.Text) != "" {
		return false
	}
	if name, ok := value.Fields["name"]; ok && name.Kind == ValueString && strings.TrimSpace(name.Text) != "" {
		return false
	}
	return true
}

func (vm *VM) stackFrames() []StackFrame {
	return stackFrames(vm.rawStackFrames())
}

func (vm *VM) rawStackFrames() []callFrame {
	frames := append([]callFrame(nil), vm.callStack...)
	if vm.hasStatement && vm.currentStatement.Line > 0 {
		frames = append(frames, vm.currentStatement)
	}
	for i := range frames {
		frames[i].Symbol = vm.qualifyStackFrameSymbol(frames[i].Symbol)
	}
	return frames
}

func (vm *VM) qualifyStackFrameSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" || hasPrefixFold(symbol, "class.") || hasPrefixFold(symbol, "trigger.") {
		return symbol
	}
	dot := strings.LastIndex(symbol, ".")
	if dot <= 0 {
		return symbol
	}
	className := symbol[:dot]
	class, ok := vm.lookupClass(className)
	if !ok {
		return symbol
	}
	token := vm.classTypeToken(class)
	if token == "" || strings.EqualFold(token, className) {
		return symbol
	}
	return token + symbol[len(className):]
}

func stackFrames(frames []callFrame) []StackFrame {
	out := make([]StackFrame, 0, len(frames))
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		out = append(out, StackFrame{
			Symbol: frame.Symbol,
			File:   frame.File,
			Line:   frame.Line,
			Column: frame.Column,
		})
	}
	return out
}

func (vm *VM) callMethod(method Method, args []Value, result *Result) (Value, error) {
	return vm.callMethodWithReceiver(method, Null, args, result)
}

func activeInstanceSetterKey(owner, field string, receiver Value) string {
	key := owner + "." + field
	if receiver.Kind == ValueObject && receiver.Ref != 0 {
		key += "#" + strconv.FormatUint(receiver.Ref, 10)
	}
	return key
}

func (vm *VM) synchronizeFabricatedSObjectRelationships(value Value) Value {
	childrenName, childrenByRelation, ok := objectFieldValue(value, "childrenByRelation")
	if !ok || childrenByRelation.Kind != ValueMap {
		return value
	}
	fabricatedName, fabricated, ok := objectFieldValue(value, "fabricatedSObject")
	if !ok || fabricated.Kind != ValueObject {
		return value
	}
	nodesName, nodes, ok := objectFieldValue(fabricated, "nodes")
	if !ok || nodes.Kind != ValueList {
		return value
	}

	var childNodeType string
	for _, node := range nodes.List {
		if node.Kind == ValueObject && objectHasFields(node, "fieldName", "children") {
			childNodeType = node.Type
			break
		}
	}
	if childNodeType == "" {
		if _, ok := vm.lookupClass("sfab_ChildRelationshipNode"); !ok {
			return value
		}
		childNodeType = "sfab_ChildRelationshipNode"
	}

	relationships := make(map[string]Value, len(childrenByRelation.Map))
	relationshipOrder := make([]string, 0, len(childrenByRelation.Map))
	for rawKey, childList := range childrenByRelation.Map {
		relation := mapStoredKey(childrenByRelation, rawKey).String()
		if relation == "" || childList.Kind != ValueList {
			continue
		}
		fabricatedChildren := List()
		for i, child := range childList.List {
			child = vm.synchronizeFabricatedSObjectRelationships(child)
			childList.List[i] = child
			if _, childFabricated, ok := objectFieldValue(child, "fabricatedSObject"); ok && childFabricated.Kind == ValueObject {
				fabricatedChildren.List = append(fabricatedChildren.List, childFabricated)
			}
		}
		childrenByRelation.Map[rawKey] = childList
		relationships[relation] = fabricatedChildren
		relationshipOrder = append(relationshipOrder, relation)
	}
	if len(relationships) == 0 {
		return value
	}

	filtered := make([]Value, 0, len(nodes.List)+len(relationships))
	for _, node := range nodes.List {
		if node.Kind == ValueObject {
			if _, fieldName, ok := objectFieldValue(node, "fieldName"); ok && fieldName.Kind == ValueString {
				if _, replace := relationships[fieldName.Text]; replace && objectHasFields(node, "children") {
					continue
				}
			}
		}
		filtered = append(filtered, node)
	}
	for _, relation := range relationshipOrder {
		node := Object(childNodeType)
		node.Fields["fieldName"] = String(relation)
		node.Fields["children"] = relationships[relation]
		filtered = append(filtered, node)
	}
	nodes.List = filtered
	fabricated.Fields[nodesName] = nodes
	value.Fields[fabricatedName] = fabricated
	value.Fields[childrenName] = childrenByRelation
	return value
}

func objectHasFields(value Value, names ...string) bool {
	if value.Kind != ValueObject {
		return false
	}
	for _, name := range names {
		if _, _, ok := objectFieldValue(value, name); !ok {
			return false
		}
	}
	return true
}

func (vm *VM) inferEmptySObjectListRuntimeType(returnType string, value Value, args []Value) string {
	if value.Kind != ValueList || len(value.List) != 0 || value.Runtime != "" {
		return ""
	}
	elementType, ok := collectionElementType(returnType)
	if !ok || !strings.EqualFold(elementType, "SObject") {
		return ""
	}
	if queryText := inlineSOQLQueryText(value); queryText != "" {
		if objectName := vm.soqlResultObjectNameWithExpander(queryText, vm.expandSOQLBinds); objectName != "" {
			return "List<" + objectName + ">"
		}
	}
	return ""
}

func (vm *VM) sObjectNameFromIDSetValue(value Value) string {
	if value.Kind != ValueSet {
		return ""
	}
	elementType, ok := collectionElementType(value.Type)
	if ok && !strings.EqualFold(elementType, "Id") {
		return ""
	}
	objectName := ""
	for _, item := range value.Set {
		idText, ok := idValueText(item)
		if !ok || idText == "" {
			return ""
		}
		name, ok := vm.sObjectNameForID(idText)
		if !ok {
			return ""
		}
		if objectName == "" {
			objectName = name
			continue
		}
		if !strings.EqualFold(objectName, name) {
			return ""
		}
	}
	return objectName
}

func methodHasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(baseModifierName(modifier), expected) {
			return true
		}
	}
	return false
}

func baseModifierName(modifier string) string {
	modifier = strings.TrimSpace(strings.TrimPrefix(modifier, "@"))
	if idx := strings.IndexByte(modifier, '('); idx >= 0 {
		modifier = strings.TrimSpace(modifier[:idx])
	}
	return modifier
}

func describeFieldPermissionFlagName(method string) string {
	switch method {
	case "isCreateable":
		return "createable"
	case "isUpdateable":
		return "updateable"
	default:
		return "accessible"
	}
}

func describeFieldBooleanFlagName(method string) string {
	name := strings.TrimPrefix(method, "is")
	if name == "" {
		return method
	}
	return strings.ToLower(name[:1]) + name[1:]
}

func standardControllerPage(record Value) Value {
	if _, id, ok := objectFieldValue(record, "Id"); ok {
		if idText, ok := idValueText(id); ok && idText != "" {
			return newPageReference("/" + idText)
		}
	}
	return newPageReference("")
}

func standardSetCurrentPage(controller, records Value) Value {
	if records.Kind != ValueList {
		return List()
	}
	pageSize := int(controller.Fields["pageSize"].Int)
	pageNumber := int(controller.Fields["pageNumber"].Int)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageNumber <= 0 {
		pageNumber = 1
	}
	start := (pageNumber - 1) * pageSize
	if start >= len(records.List) {
		return List()
	}
	end := start + pageSize
	if end > len(records.List) {
		end = len(records.List)
	}
	return List(records.List[start:end]...)
}

func standardSetPageCount(controller, records Value) int {
	if records.Kind != ValueList || len(records.List) == 0 {
		return 1
	}
	pageSize := int(controller.Fields["pageSize"].Int)
	if pageSize <= 0 {
		pageSize = 20
	}
	pages := (len(records.List) + pageSize - 1) / pageSize
	if pages < 1 {
		return 1
	}
	return pages
}

func (vm *VM) standardSetDML(receiver Value, op string, result *Result) (Value, Value, bool, bool, error) {
	records := receiver.Fields["records"]
	if records.Kind != ValueList {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.%s requires records", op)
	}
	if _, err := vm.applyDML(op, records, true, "", dml.Options{}, result); err != nil {
		return Null, receiver, false, true, err
	}
	return newPageReference(""), receiver, false, true, nil
}

func (vm *VM) callCustomNotificationMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setBody", "setNotificationTypeId", "setSenderId", "setTargetId", "setTargetPageRef", "setTitle":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.CustomNotification.%s expects String", method)
		}
		value := args[0]
		if idText, ok := typedIDValueText(value); ok {
			value = String(idText)
		}
		receiver.Fields[customNotificationFieldName(method)] = value
		return Null, receiver, true, true, nil
	case "send":
		if len(args) != 1 || args[0].Kind != ValueSet {
			return Null, receiver, false, true, fmt.Errorf("Messaging.CustomNotification.send expects Set<String>")
		}
		appendTrace(result, "apex.notification.custom.send", "apex.notification", map[string]any{"recipients": len(args[0].Set)})
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func customNotificationFieldName(method string) string {
	switch method {
	case "setNotificationTypeId":
		return "notificationTypeId"
	case "setSenderId":
		return "senderId"
	case "setTargetId":
		return "targetId"
	case "setTargetPageRef":
		return "targetPageRef"
	default:
		return emailMessageFieldName(method)
	}
}

func (vm *VM) callPushNotificationMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setPayload":
		if len(args) != 1 || args[0].Kind != ValueMap {
			return Null, receiver, false, true, fmt.Errorf("Messaging.PushNotification.setPayload expects Map<String,Object>")
		}
		receiver.Fields["payload"] = args[0]
		return Null, receiver, true, true, nil
	case "setTtl":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Messaging.PushNotification.setTtl expects Integer")
		}
		receiver.Fields["ttl"] = args[0]
		return Null, receiver, true, true, nil
	case "send":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueSet {
			return Null, receiver, false, true, fmt.Errorf("Messaging.PushNotification.send expects application String and Set<String>")
		}
		appendTrace(result, "apex.notification.push.send", "apex.notification", map[string]any{"application": args[0].Text, "recipients": len(args[1].Set)})
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func messagingPushPayloadApple(args []Value) (Value, error) {
	if len(args) != 4 && len(args) != 8 {
		return Null, fmt.Errorf("Messaging.PushNotificationPayload.apple expects 4 or 8 arguments")
	}
	payload := typedMap("Map<String,Object>")
	aps := typedMap("Map<String,Object>")
	switch len(args) {
	case 4:
		if args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueInt || args[3].Kind != ValueMap {
			return Null, fmt.Errorf("Messaging.PushNotificationPayload.apple expects alert, sound, badgeCount, userData")
		}
		aps.Map[mapKey(String("alert"))] = args[0]
		aps.MapKeys[mapKey(String("alert"))] = String("alert")
		aps.Map[mapKey(String("sound"))] = args[1]
		aps.MapKeys[mapKey(String("sound"))] = String("sound")
		aps.Map[mapKey(String("badge"))] = args[2]
		aps.MapKeys[mapKey(String("badge"))] = String("badge")
	case 8:
		if args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString || args[3].Kind != ValueList ||
			args[4].Kind != ValueString || args[5].Kind != ValueString || args[6].Kind != ValueInt || args[7].Kind != ValueMap {
			return Null, fmt.Errorf("Messaging.PushNotificationPayload.apple expects alertBody, actionLocKey, locKey, locArgs, launchImage, sound, badgeCount, userData")
		}
		alert := typedMap("Map<String,Object>")
		for key, value := range map[string]Value{
			"body":           args[0],
			"action-loc-key": args[1],
			"loc-key":        args[2],
			"loc-args":       args[3],
			"launch-image":   args[4],
		} {
			alert.Map[mapKey(String(key))] = value
			alert.MapKeys[mapKey(String(key))] = String(key)
		}
		aps.Map[mapKey(String("alert"))] = alert
		aps.MapKeys[mapKey(String("alert"))] = String("alert")
		aps.Map[mapKey(String("sound"))] = args[5]
		aps.MapKeys[mapKey(String("sound"))] = String("sound")
		aps.Map[mapKey(String("badge"))] = args[6]
		aps.MapKeys[mapKey(String("badge"))] = String("badge")
	}
	payload.Map[mapKey(String("aps"))] = aps
	payload.MapKeys[mapKey(String("aps"))] = String("aps")
	userData := args[len(args)-1]
	for key, value := range userData.Map {
		if decoded, ok := userData.MapKeys[key]; ok {
			payload.MapKeys[key] = decoded
		}
		payload.Map[key] = value
	}
	return payload, nil
}

func emailMessageFieldName(method string) string {
	if strings.HasPrefix(method, "get") && len(method) > len("get") {
		field := strings.TrimPrefix(method, "get")
		return strings.ToLower(field[:1]) + field[1:]
	}
	if strings.HasPrefix(method, "is") && len(method) > len("is") {
		field := strings.TrimPrefix(method, "is")
		return strings.ToLower(field[:1]) + field[1:]
	}
	if !strings.HasPrefix(method, "set") || len(method) <= len("set") {
		return method
	}
	field := strings.TrimPrefix(method, "set")
	return strings.ToLower(field[:1]) + field[1:]
}

func restMapPut(receiver *Value, field, key string, value Value, caseInsensitive bool) {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		current = typedMap("Map<String,String>")
	}
	if caseInsensitive {
		for rawKey := range current.Map {
			decoded := valueFromMapKey(rawKey)
			if decoded.Kind == ValueString && strings.EqualFold(decoded.Text, key) {
				delete(current.Map, rawKey)
				break
			}
		}
	}
	current.Map[mapKey(String(key))] = value
	receiver.Fields[field] = current
}

func restMapGet(receiver Value, field, key string) Value {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		return Null
	}
	if value, ok := current.Map[mapKey(String(key))]; ok {
		return value
	}
	for rawKey, value := range current.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString && strings.EqualFold(decoded.Text, key) {
			return value
		}
	}
	return Null
}

func restMapKeys(receiver Value, field string) Value {
	current := receiver.Fields[field]
	if current.Kind != ValueMap {
		return List()
	}
	keys := make([]string, 0, len(current.Map))
	for rawKey := range current.Map {
		decoded := valueFromMapKey(rawKey)
		if decoded.Kind == ValueString {
			keys = append(keys, decoded.Text)
		}
	}
	sort.Strings(keys)
	out := make([]Value, 0, len(keys))
	for _, key := range keys {
		out = append(out, String(key))
	}
	return List(out...)
}
func vmFormulaDefaultShouldEvaluate(field storage.Field, rawDefault string) bool {
	if rawDefault == "" {
		return false
	}
	switch field.Type {
	case storage.FieldDate, storage.FieldDateTime:
		return strings.ContainsAny(rawDefault, "()")
	default:
		return false
	}
}
