package vm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) collectionMutationType(receiver Value) string {
	if collectionBase(receiver.Runtime) != "List" {
		return receiver.Type
	}
	if receiver.Type == "" || strings.EqualFold(receiver.Runtime, receiver.Type) {
		return receiver.Runtime
	}
	runtimeElement, runtimeOK := collectionElementType(receiver.Runtime)
	declaredElement, declaredOK := collectionElementType(receiver.Type)
	if !runtimeOK || !declaredOK {
		return receiver.Type
	}
	if vm.typeAssignableTo(runtimeElement, declaredElement) {
		return receiver.Runtime
	}
	return receiver.Type
}

func collectionStoreException(collectionType string, value Value) error {
	return newExceptionError("System.TypeException", fmt.Sprintf("Collection store exception adding %s to %s", runtimeValueTypeName(value), typeExceptionAnyName(collectionType)))
}

func (vm *VM) callMethodWithReceiver(method Method, receiver Value, args []Value, result *Result) (Value, error) {
	if len(vm.callStack) >= maxApexCallDepth {
		return Null, &RuntimeError{Type: "RuntimeError", Message: "maximum Apex call stack depth exceeded", Stack: vm.stackFrames()}
	}
	if len(args) != len(method.Params) {
		return Null, fmt.Errorf("%s expects %d arguments", method.Name, len(method.Params))
	}
	if value, handled := vm.callFrameworkIDGeneratorGenerate(method, args); handled {
		return value, nil
	}
	if isStubProxy(receiver) && stubProxyCanInterceptMethod(method) {
		if value, handled, err := vm.callStubProxyMember(receiver, apexMethodMemberName(method.Name), args, result); handled || err != nil {
			return value, err
		}
	}
	if cartExtensionMockBackedRuntimeType(method.ClassName) || cartExtensionMockBackedRuntimeType(receiver.Type) {
		var value Value
		var handled bool
		var err error
		switch {
		case strings.EqualFold(method.ClassName, "CartExtension.SplitShipmentService"), strings.EqualFold(receiver.Type, "CartExtension.SplitShipmentService"):
			value, _, _, handled, err = vm.callCartExtensionMockBackedSplitShipment(receiver, apexMethodMemberName(method.Name), args)
		default:
			value, _, _, handled, err = vm.callCartExtensionMockBackedCalculator(receiver, apexMethodMemberName(method.Name), args)
		}
		if handled || err != nil {
			return value, err
		}
	}
	if method.Unsupported != "" {
		return Null, fmt.Errorf("%s is not supported by the local VM: %s", method.Name, method.Unsupported)
	}
	if methodHasModifier(method.Modifiers, "abstract") {
		if receiver.Kind == ValueObject {
			baseType := method.ClassName
			if baseType == "" {
				baseType, _, _ = strings.Cut(method.Name, ".")
			}
			methodName := apexMethodMemberName(method.Name)
			if override, found := vm.uniqueConcreteOverride(receiver, baseType, methodName, args); found {
				method = override
			} else if override, found := vm.concreteOverrideByReceiverFields(receiver, baseType, methodName, args); found {
				method = override
			} else {
				return Null, fmt.Errorf("cannot execute abstract method %s", method.Name)
			}
		} else {
			return Null, fmt.Errorf("cannot execute abstract method %s", method.Name)
		}
	}
	if method.Unsupported != "" {
		return Null, fmt.Errorf("%s is not supported by the local VM: %s", method.Name, method.Unsupported)
	}
	if methodHasModifier(method.Modifiers, "abstract") {
		return Null, fmt.Errorf("cannot execute abstract method %s", method.Name)
	}
	if method.ClassName != "" && !strings.Contains(method.Name, ".<static_") {
		if err := vm.ensureClassInitialized(method.ClassName); err != nil {
			return Null, err
		}
	}
	collectionMutationSeqBefore := vm.collectionMutationSeq
	if receiver.Kind == ValueObject && apexMethodMemberName(method.Name) == "toSObject" {
		receiver = vm.synchronizeFabricatedSObjectRelationships(receiver)
	}
	constructorKey := ""
	if method.IsConstructor {
		constructorKey = constructorCallKey(method)
		if vm.activeConstructors == nil {
			vm.activeConstructors = make(map[string]int)
		}
		vm.activeConstructors[constructorKey]++
		defer func() {
			vm.activeConstructors[constructorKey]--
			if vm.activeConstructors[constructorKey] <= 0 {
				delete(vm.activeConstructors, constructorKey)
			}
		}()
	}
	frame := make(map[string]Value, len(method.Params))
	frameTypes := make(map[string]string, len(method.Params))
	paramSnapshots := make(map[string]aliasSnapshot, len(method.Params))
	paramOriginals := make(map[string]Value, len(method.Params))
	materializedCurrentPageParams := make(map[string]bool)
	resolutionClass := vm.methodTypeResolutionClass(method)
	for i, param := range method.Params {
		paramType := vm.resolveTypeNameInClass(resolutionClass, param.Type)
		paramOriginals[param.Name] = args[i]
		arg := args[i]
		if isImplicitCurrentPageNull(arg) && strings.EqualFold(paramType, "PageReference") {
			if vm.currentPage.Kind == "" {
				vm.currentPage = newPageReference("")
			}
			arg = vm.currentPage
			materializedCurrentPageParams[param.Name] = true
		}
		coerced, err := vm.coerceAssignable(paramType, vm.valueWithTypesResolvedInClass(resolutionClass, arg))
		if err != nil {
			if alternate, ok, ambiguous := vm.resolveInstanceMethodForArgs(method.ClassName, apexMethodMemberName(method.Name), args); ok && !ambiguous && methodSignature(alternate) != methodSignature(method) {
				return vm.callAccessibleAlternateMethodWithReceiver(alternate, receiver, args, result)
			}
			for superClass := vm.superClassName(method.ClassName); superClass != ""; superClass = vm.superClassName(superClass) {
				if alternate, ok, ambiguous := vm.resolveInstanceMethodForArgs(superClass, apexMethodMemberName(method.Name), args); ok && !ambiguous {
					return vm.callAccessibleAlternateMethodWithReceiver(alternate, receiver, args, result)
				}
			}
			if alternate, ok := vm.alternateMethodWithCoercibleArgs(method, args); ok {
				return vm.callAccessibleAlternateMethodWithReceiver(alternate, receiver, args, result)
			}
			return Null, fmt.Errorf("%s parameter %s: %w", method.Name, param.Name, err)
		}
		coerced.Static = paramType
		frame[param.Name] = coerced
		frameTypes[param.Name] = paramType
		paramSnapshots[param.Name] = snapshotAlias(coerced)
	}
	receiverOriginal := receiver
	if receiver.Kind != ValueNull {
		frame["this"] = receiver
	}
	receiverSnapshot := aliasSnapshot{}
	if receiver.Kind != ValueNull {
		receiverSnapshot = snapshotAlias(receiver)
	}
	commerceClassName := method.ClassName
	if commerceClassName == "" && receiver.Kind == ValueObject {
		commerceClassName = receiver.Type
	}
	if strings.EqualFold(commerceClassName, "commercepayments.ClientSidePaymentAdapter") &&
		strings.EqualFold(apexMethodMemberName(method.Name), "processClientRequest") {
		return Null, newExceptionError("UnsupportedOperationException", method.Name+" local stub surface")
	}
	if strings.EqualFold(commerceClassName, "CartExtension.CheckoutPlaceOrder") &&
		commerceLocalHarnessRuntimeMethod(commerceClassName, apexMethodMemberName(method.Name)) {
		if method.ClassName == "" {
			method.ClassName = commerceClassName
		}
		return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), nil
	}
	if value, handled := vm.generatedUnsupportedFamilyExplicitMethodDefault(method, receiver, args); handled {
		return value, nil
	}
	if err, handled := vm.generatedUnsupportedFamilyExplicitMethodError(method, receiver, args); handled {
		return Null, err
	}
	if receiver.Kind == ValueObject && passiveGeneratedMethod(method) && generatedPlatformObjectMemberReceiver(receiver.Type) {
		callArgs := make([]Value, 0, len(method.Params))
		for _, param := range method.Params {
			callArgs = append(callArgs, frame[param.Name])
		}
		value, _, _, handled, err := vm.callPlatformObjectMember(receiver, apexMethodMemberName(method.Name), callArgs, result)
		if handled || err != nil {
			return value, err
		}
	}
	if methodHasModifier(method.Modifiers, "passive-generated") {
		className := method.ClassName
		if className == "" {
			className = receiver.Type
		}
		if strings.EqualFold(className, "Invocable.Action") &&
			(strings.EqualFold(apexMethodMemberName(method.Name), "createCustomAction") || strings.EqualFold(apexMethodMemberName(method.Name), "createStandardAction")) {
			callArgs := make([]Value, 0, len(method.Params))
			for _, param := range method.Params {
				callArgs = append(callArgs, frame[param.Name])
			}
			if value, handled := newInvocableAction(apexMethodMemberName(method.Name), callArgs); handled {
				return value, nil
			}
		}
		if receiver.Kind == ValueObject && strings.EqualFold(className, "Invocable.Action") {
			callArgs := make([]Value, 0, len(method.Params))
			for _, param := range method.Params {
				callArgs = append(callArgs, frame[param.Name])
			}
			if value, handled := vm.callInvocableActionMember(receiver, apexMethodMemberName(method.Name), callArgs); handled {
				return value, nil
			}
		}
		if connectAPIPrimaryUsageClass(className) && !connectAPIPrimaryUsageAllowedMethod(className, apexMethodMemberName(method.Name)) {
			return Null, newExceptionError("ConnectApi.ConnectApiException", method.Name+" is not supported in local tests")
		}
		if vm.generatedOptionalWrapperType(className) {
			if value, handled := vm.generatedOptionalWrapperStaticDefault(className, apexMethodMemberName(method.Name), args); handled {
				return value, nil
			}
		}
		if generatedFamilyUnsupportedTypePrefix(className) && !vm.generatedPassiveDTOAccessorMethod(className, method) {
			if value, handled := vm.generatedUnsupportedFamilyExplicitMethodDefault(method, receiver, args); handled {
				return value, nil
			}
			if err, handled := vm.generatedUnsupportedFamilyExplicitMethodError(method, receiver, args); handled {
				return Null, err
			}
			return Null, newExceptionError("UnsupportedOperationException", method.Name+" local stub surface")
		}
	}
	if passiveGeneratedMethod(method) {
		if receiver.Kind == ValueObject && userProvisioningBatchableType(receiver.Type) {
			callArgs := make([]Value, 0, len(method.Params))
			for _, param := range method.Params {
				callArgs = append(callArgs, frame[param.Name])
			}
			value, _, _, handled, err := callUserProvisioningBatchableMember(receiver, apexMethodMemberName(method.Name), callArgs)
			if handled || err != nil {
				return value, err
			}
		}
		if receiver.Kind == ValueObject &&
			(strings.EqualFold(receiver.Type, "DataWeave.Script") || strings.HasPrefix(receiver.Type, "DataWeaveScriptResource.")) &&
			strings.EqualFold(apexMethodMemberName(method.Name), "execute") {
			dataWeaveArgs := make([]Value, 0, len(method.Params))
			for _, param := range method.Params {
				if value, ok := frame[param.Name]; ok {
					dataWeaveArgs = append(dataWeaveArgs, value)
				}
			}
			value, _, _, handled, err := callDataWeaveScriptMember(receiver, "execute", dataWeaveArgs)
			if handled || err != nil {
				return value, err
			}
		}
		value := vm.passiveGeneratedMethodReturn(method, frame, receiver)
		for _, param := range method.Params {
			updated, ok := frame[param.Name]
			if !ok {
				continue
			}
			if materializedCurrentPageParams[param.Name] {
				vm.currentPage = updated
			}
			previous := paramSnapshots[param.Name]
			if valueAliasSnapshotMatch(previous, updated) {
				if vm.propagateAliasSnapshotMutationToScope(vm.Globals, previous, paramOriginals[param.Name], updated, vm.collectionMutationSeq != collectionMutationSeqBefore) {
					vm.propagateAliasSnapshotToStatics(previous, updated)
					vm.propagateUpdatedValueAliases(vm.Globals, updated)
				}
				continue
			}
			for _, arg := range args {
				if !valueAliasMatch(arg, updated) {
					continue
				}
				vm.propagateValueMutationToScope(vm.Globals, arg, updated)
				vm.propagateValueMutationToStatics(arg, updated)
				vm.propagateUpdatedValueAliases(vm.Globals, updated)
				break
			}
		}
		if receiver.Kind != ValueNull {
			if updated, ok := frame["this"]; ok && valueAliasSnapshotMatch(receiverSnapshot, updated) {
				vm.propagateAliasSnapshotMutationToScope(vm.Globals, receiverSnapshot, receiverOriginal, updated, vm.collectionMutationSeq != collectionMutationSeqBefore)
				vm.propagateAliasSnapshotToStatics(receiverSnapshot, updated)
				vm.propagateUpdatedValueAliases(vm.Globals, updated)
			}
		}
		return value, nil
	}
	caller := vm.Globals
	callerTypes := vm.VarTypes
	callerClass := vm.currentClass
	callerMethod := vm.currentMethod
	callerStatement := vm.currentStatement
	callerHasStatement := vm.hasStatement
	vm.scopeStack = append(vm.scopeStack, caller)
	vm.Globals = frame
	vm.VarTypes = frameTypes
	vm.currentClass = method.ClassName
	vm.currentMethod = method
	if vm.currentClass == "" {
		vm.currentClass = classNameFromMethod(method.Name)
	}
	vm.callStack = append(vm.callStack, callFrame{
		Symbol: vm.apexMethodFrameSymbol(method),
		File:   method.File,
		Line:   method.Line,
		Column: method.Column,
	})
	traceStart, traceStartedAt := traceSpanStart(result)
	appendTrace(result, "apex.method."+method.Name, "apex.method", vm.traceMethodArgs(method))
	defer func() {
		appendDurationTrace(
			result,
			"apex.method."+method.Name,
			"apex.method",
			traceStart,
			traceDurationSince(traceStartedAt),
			vm.traceMethodArgs(method),
		)
	}()
	defer func() {
		vm.callStack = vm.callStack[:len(vm.callStack)-1]
		vm.Globals = caller
		vm.VarTypes = callerTypes
		vm.currentClass = callerClass
		vm.currentMethod = callerMethod
		vm.currentStatement = callerStatement
		vm.hasStatement = callerHasStatement
		vm.scopeStack = vm.scopeStack[:len(vm.scopeStack)-1]
	}()

	out, err := vm.executeProgram(method.Program, result)
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) {
			if len(thrown.stack) == 0 {
				thrown.stack = vm.rawStackFrames()
			}
			return Null, thrown
		}
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) {
			if len(runtimeErr.Stack) == 0 {
				runtimeErr.Stack = vm.stackFrames()
			}
			return Null, runtimeErr
		}
		return Null, &RuntimeError{Type: "RuntimeError", Message: err.Error(), Stack: vm.stackFrames()}
	}
	if out.signal == signalThrow {
		stack := out.thrownStack
		if len(stack) == 0 {
			stack = vm.rawStackFrames()
		}
		return Null, &apexThrowError{value: out.thrown, stack: stack}
	}
	if out.signal == signalBreak || out.signal == signalContinue {
		return Null, fmt.Errorf("%s outside loop", out.signal)
	}
	value := out.value
	if out.signal != signalReturn {
		if method.ReturnType != "" && !strings.EqualFold(method.ReturnType, "void") {
			return Null, fmt.Errorf("%s must return %s", method.Name, method.ReturnType)
		}
		value = Null
	}
	if method.ReturnType != "" && !strings.EqualFold(method.ReturnType, "void") {
		returnType := vm.resolveTypeNameInClass(method.ClassName, method.ReturnType)
		coerced, err := vm.coerceAssignable(returnType, value)
		if err != nil {
			return Null, fmt.Errorf("%s return: %w", method.Name, err)
		}
		coerced.Static = returnType
		if inferred := vm.inferEmptySObjectListRuntimeType(returnType, coerced, args); inferred != "" {
			coerced.Runtime = inferred
		}
		value = coerced
	}
	for _, param := range method.Params {
		updated, ok := frame[param.Name]
		if !ok {
			continue
		}
		if materializedCurrentPageParams[param.Name] {
			vm.currentPage = updated
		}
		previous := paramSnapshots[param.Name]
		if valueAliasSnapshotMatch(previous, updated) {
			if vm.propagateAliasSnapshotMutationToScope(caller, previous, paramOriginals[param.Name], updated, vm.collectionMutationSeq != collectionMutationSeqBefore) {
				vm.propagateAliasSnapshotToStatics(previous, updated)
				vm.propagateUpdatedValueAliases(caller, updated)
			}
			continue
		}
		for _, arg := range args {
			if !valueAliasMatch(arg, updated) {
				continue
			}
			vm.propagateValueMutationToScope(caller, arg, updated)
			vm.propagateValueMutationToStatics(arg, updated)
			vm.propagateUpdatedValueAliases(caller, updated)
			break
		}
	}
	if receiver.Kind != ValueNull {
		if updated, ok := frame["this"]; ok && valueAliasSnapshotMatch(receiverSnapshot, updated) {
			vm.propagateAliasSnapshotMutationToScope(caller, receiverSnapshot, receiverOriginal, updated, vm.collectionMutationSeq != collectionMutationSeqBefore)
			vm.propagateAliasSnapshotToStatics(receiverSnapshot, updated)
			vm.propagateUpdatedValueAliases(caller, updated)
		}
	}
	if method.IsConstructor {
		if updated, ok := frame["this"]; ok && updated.Kind == ValueObject {
			return updated, nil
		}
	}
	return value, nil
}

func (vm *VM) callAccessibleAlternateMethodWithReceiver(method Method, receiver Value, args []Value, result *Result) (Value, error) {
	if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
		return Null, err
	}
	return vm.callMethodWithReceiver(method, receiver, args, result)
}

func (vm *VM) alternateMethodWithCoercibleArgs(method Method, args []Value) (Method, bool) {
	candidates := vm.registeredMethodCandidates(method.Name)
	if len(candidates) == 0 {
		return Method{}, false
	}
	coercible := make([]Method, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.IsStatic != method.IsStatic || methodSignature(candidate) == methodSignature(method) || len(candidate.Params) != len(args) {
			continue
		}
		fits := true
		for i, param := range candidate.Params {
			paramType := vm.resolveTypeNameInClass(candidate.ClassName, param.Type)
			if _, err := vm.coerceAssignable(paramType, args[i]); err != nil {
				fits = false
				break
			}
		}
		if fits {
			coercible = append(coercible, candidate)
		}
	}
	target, ok, ambiguous := vm.matchMethodByArgs(coercible, args)
	if !ok || ambiguous {
		return Method{}, false
	}
	return target, true
}

func (vm *VM) callMember(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 2 {
		return Null, false, nil
	}
	receiverName, method := parts[0], parts[1]
	if strings.EqualFold(receiverName, "super") {
		if len(parts) != 2 {
			return Null, false, nil
		}
		receiver, ok := vm.Globals["this"]
		if !ok || receiver.Kind != ValueObject {
			return Null, true, fmt.Errorf("super call requires instance receiver")
		}
		dispatchClass := vm.currentClass
		if dispatchClass == "" {
			dispatchClass = receiver.Type
		}
		class, _ := vm.lookupClass(dispatchClass)
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(vm.resolvedSuperClassName(class), method, args)
		if ambiguous {
			return Null, true, vm.ambiguousOverloadError(callee, args)
		}
		if !ok {
			if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
				if mutated {
					vm.Globals["this"] = updated
				}
				return value, true, err
			}
			return Null, true, unsupportedCallError(callee)
		}
		if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name, target.Modifiers); err != nil {
			return Null, true, err
		}
		value, err := vm.callMethodWithReceiver(target, receiver, args, result)
		return value, true, err
	}
	if len(parts) > 2 {
		receiverName = strings.Join(parts[:len(parts)-1], ".")
		method = parts[len(parts)-1]
		if method == "addError" {
			value, handled, err := vm.callSObjectFieldAddError(parts[:len(parts)-1], args)
			if handled || err != nil {
				return value, true, err
			}
		}
		receiver, err := vm.lookup(receiverName)
		if err != nil {
			if _, ok := vm.Globals[parts[0]]; ok {
				return Null, true, err
			}
			return Null, false, nil
		}
		return vm.callValueMember(receiverName, receiver, method, args, result)
	}
	receiver, ok := vm.Globals[receiverName]
	if !ok {
		if actual, found := vm.lookupGlobalName(receiverName); found {
			receiverName = actual
			receiver = vm.Globals[actual]
			ok = true
		}
	}
	if !ok {
		if vm.staticReceiverName(receiverName, method) && !vm.hasRuntimeReceiver(receiverName) {
			return Null, false, nil
		}
		if thisValue, hasThis := vm.Globals["this"]; hasThis && thisValue.Kind == ValueObject {
			if _, _, hasField := objectFieldValue(thisValue, receiverName); hasField {
				receiver, err := vm.lookup(receiverName)
				if err != nil {
					if !strings.Contains(err.Error(), "unknown variable") {
						return Null, true, err
					}
					return Null, false, nil
				}
				return vm.callValueMember(receiverName, receiver, method, args, result)
			}
		}
		if declared := vm.declaredReceiverType(receiverName); isMapType(declared) || collectionBase(declared) != "" {
			receiver, err := vm.lookup(receiverName)
			if err == nil {
				return vm.callValueMember(receiverName, receiver, method, args, result)
			}
			if !strings.Contains(err.Error(), "unknown variable") {
				return Null, true, err
			}
		}
		staticContext := vm.currentClass
		if staticContext == "" {
			staticContext = vm.currentMethod.ClassName
		}
		if staticContext != "" {
			if field, owner, ok := vm.lookupStaticField(staticContext, receiverName); ok {
				if mapLikeMemberName(method) && field.Value.Kind != ValueMap {
					if strictField, strictOwner, strictOK := vm.lookupStaticFieldStrict(staticContext, receiverName); strictOK {
						field = strictField
						owner = strictOwner
					}
				}
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+receiverName, field.Modifiers); err != nil {
					return Null, true, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, true, err
				}
				field, _, _ = vm.lookupStaticField(owner, receiverName)
				receiver := field.Value
				if field.Getter != nil {
					var err error
					receiver, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, true, err
					}
				}
				return vm.callValueMember(receiverName, receiver, method, args, result)
			}
		}
		if receiverName == "" {
			return Null, false, nil
		}
		if !unicode.IsLower([]rune(receiverName)[0]) {
			return Null, false, nil
		}
		if vm.currentNamespace != "" {
			if field, owner, ok := vm.lookupNamespaceStaticField(vm.currentNamespace, receiverName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+receiverName, field.Modifiers); err != nil {
					return Null, true, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, true, err
				}
				field, _, _ = vm.lookupStaticField(owner, receiverName)
				receiver = field.Value
				if field.Getter != nil {
					var err error
					receiver, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, true, err
					}
				}
				return vm.callValueMember(receiverName, receiver, method, args, result)
			}
		}
		var err error
		receiver, err = vm.lookup(receiverName)
		if err != nil {
			return Null, false, nil
		}
		return vm.callValueMember(receiverName, receiver, method, args, result)
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func (vm *VM) lookupNamespaceStaticField(namespace, fieldName string) (Field, string, bool) {
	var best Field
	var owner string
	bestScore := -1
	for _, class := range vm.Classes {
		if !strings.EqualFold(class.Namespace, namespace) {
			continue
		}
		field, ok := vm.lookupFieldInMap(class.StaticFields, fieldName)
		if !ok {
			continue
		}
		score := vm.fieldProvenanceScore(field)
		if !class.Dependency {
			score += 16
		}
		if !strings.Contains(class.Name, ".") {
			score += 4
		}
		if score > bestScore {
			bestScore = score
			best = field
			owner = class.Name
		}
	}
	if bestScore < 0 {
		return Field{}, "", false
	}
	return best, owner, true
}

func (vm *VM) lookupStaticFieldStrict(typeName, fieldName string) (Field, string, bool) {
	for search := typeName; search != ""; {
		for current := search; current != ""; {
			class, ok := vm.lookupClass(current)
			if !ok {
				break
			}
			if field, ok := class.StaticFields[fieldName]; ok {
				if field.Name == "" {
					field.Name = fieldName
				}
				field.StorageName = fieldName
				return field, class.Name, true
			}
			for candidate, field := range class.StaticFields {
				if !strings.EqualFold(candidate, fieldName) {
					continue
				}
				if field.Name == "" {
					field.Name = candidate
				}
				field.StorageName = candidate
				return field, class.Name, true
			}
			current = class.SuperClass
		}
		dot := strings.LastIndex(search, ".")
		if dot < 0 {
			break
		}
		search = search[:dot]
	}
	return Field{}, "", false
}

func (vm *VM) declaredReceiverType(receiverName string) string {
	if typ := vm.VarTypes[receiverName]; typ != "" {
		return typ
	}
	if !strings.Contains(receiverName, ".") {
		for _, className := range vm.lookupContextClasses() {
			if typ := vm.fieldPathTargetType(className, []string{receiverName}); typ != "" {
				return typ
			}
		}
	}
	if hasPrefixFold(receiverName, "this.") && vm.currentClass != "" {
		parts := strings.Split(receiverName, ".")
		if len(parts) > 1 {
			if typ := vm.fieldPathTargetType(vm.currentClass, parts[1:]); typ != "" {
				return typ
			}
		}
	}
	className, memberName, ok := vm.splitClassMember(receiverName)
	if !ok {
		return ""
	}
	preferDependency := vm.classMemberReferenceUsesExplicitNamespace(receiverName, className)
	if field, _, ok := vm.lookupStaticFieldForReceiver(className, memberName, preferDependency); ok {
		return field.Type
	}
	return ""
}

func (vm *VM) callStaticPropertyReceiverMember(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 3 {
		return Null, false, nil
	}
	method := parts[len(parts)-1]
	for split := len(parts) - 2; split >= 1; split-- {
		className := strings.Join(parts[:split], ".")
		fieldName := parts[split]
		preferDependency := vm.classMemberReferenceUsesExplicitNamespace(strings.Join(parts[:split+1], "."), className)
		field, owner, ok := vm.lookupStaticFieldForReceiver(className, fieldName, preferDependency)
		if !ok {
			continue
		}
		if err := vm.checkMemberAccess(owner, field.Access, owner+"."+fieldName, field.Modifiers); err != nil {
			return Null, true, err
		}
		if err := vm.ensureClassInitialized(owner); err != nil {
			return Null, true, err
		}
		field, _, _ = vm.lookupStaticFieldForReceiver(owner, fieldName, preferDependency)
		receiver := field.Value
		if field.Getter != nil {
			var err error
			receiver, err = vm.callGetter(owner, field, Null)
			if err != nil {
				return Null, true, err
			}
		}
		if len(parts[split+1:len(parts)-1]) > 0 {
			var err error
			receiver, err = vm.lookupPath(receiver, parts[split+1:len(parts)-1])
			if err != nil {
				return Null, true, err
			}
		}
		receiverName := strings.Join(parts[:len(parts)-1], ".")
		return vm.callValueMember(receiverName, receiver, method, args, result)
	}
	return Null, false, nil
}

func (vm *VM) callClassLiteralReceiverMember(callee string, args []Value, result *Result) (Value, bool, error) {
	lower := strings.ToLower(callee)
	index := strings.LastIndex(lower, ".class.")
	if index < 0 {
		return Null, false, nil
	}
	method := callee[index+len(".class."):]
	if method == "" || strings.Contains(method, ".") {
		return Null, false, nil
	}
	receiverName := callee[:index+len(".class")]
	receiver, err := vm.lookup(receiverName)
	if err != nil {
		return Null, false, nil
	}
	if strings.EqualFold(receiver.Type, "Type") {
		if value, handled, err := vm.callTypeObjectMember(receiver, method, args, result); handled || err != nil {
			return value, true, err
		}
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func (vm *VM) callValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if dataWeaveStaticScriptReceiver(receiverName) && strings.EqualFold(method, "createScript") {
		value, err := dataWeaveCreateScript(args)
		return value, true, err
	}
	if strings.EqualFold(method, "addError") && strings.Contains(receiverName, ".") {
		value, handled, err := vm.callSObjectFieldAddError(strings.Split(receiverName, "."), args)
		if handled || err != nil {
			return value, true, err
		}
	}
	if receiver.Kind == ValueNull {
		if isImplicitCurrentPageNull(receiver) {
			if vm.currentPage.Kind == "" {
				vm.currentPage = newPageReference("")
			}
			receiver = vm.currentPage
		} else if materialized, ok := materializeTypedNullCollection(receiver); ok {
			receiver = materialized
			if receiverName != "" {
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
			}
		} else {
			return Null, true, newNullDereferenceError(nullMemberContext(receiverName, method))
		}
	}
	if receiver.Kind != ValueMap && mapLikeMemberName(method) {
		if declared := vm.declaredReceiverType(receiverName); isMapType(declared) {
			coerced := typedMap(declared)
			if receiver.Kind == ValueObject && receiver.Fields != nil {
				if mapField, ok := receiver.Fields["map"]; ok && mapField.Kind == ValueMap {
					coerced = mapField
				}
			}
			receiver = coerced
			if receiverName != "" {
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
			}
		}
	}
	if receiver.Kind == ValueObject && strings.EqualFold(receiver.Type, "Type") {
		if value, handled, err := vm.callTypeObjectMember(receiver, method, args, result); handled || err != nil {
			return value, true, err
		}
	}
	if value, handled, err := vm.callGeneratedOptionalWrapperMember(receiver, method, args); handled || err != nil {
		return value, true, err
	}
	if _, declared := vm.VarTypes[receiverName]; !declared {
		if value, handled, err := vm.callSObjectTypeStaticMember(receiverName, method, args); handled || err != nil {
			return value, true, err
		}
	}
	if receiverType := vm.VarTypes[receiverName]; strings.EqualFold(receiverType, "Id") || idMemberReceiver(receiver, method) {
		if value, handled, err := vm.callIdMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
	}
	if receiver.Static == "" && (receiver.Kind == ValueList || receiver.Kind == ValueSet || receiver.Kind == ValueMap) {
		if declaredType := vm.VarTypes[receiverName]; declaredType != "" {
			receiver.Static = declaredType
		}
	}
	if value, updated, mutated, ok, err := callStdlibMember(receiver, method, args); ok || err != nil {
		if mutated {
			if err := vm.storeReceiver(receiverName, updated); err != nil {
				return Null, true, err
			}
		}
		return value, true, err
	}
	if receiver.Kind != ValueObject && !(strings.EqualFold(method, "clone") && (receiver.Kind == ValueList || receiver.Kind == ValueSet || receiver.Kind == ValueMap)) {
		if receiver.Kind == ValueString && strings.EqualFold(method, "name") && len(args) == 0 && (receiverName == "" || vm.declaredReceiverIsEnum(receiverName)) {
			return String(receiver.Text), true, nil
		}
		if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
	}
	if receiver.Kind == ValueObject {
		return vm.callObjectValueMember(receiverName, receiver, method, args, result)
	}
	return vm.callNonObjectValueMember(receiverName, receiver, method, args, result)
}

// callObjectValueMember dispatches a member call whose receiver is a class
// instance or platform object: stub proxies, the fflib framework members,
// enums, managed passive members, platform objects, user-class methods, and
// SObject members. It is the ValueObject arm of callValueMember.
func (vm *VM) callObjectValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if isStubProxy(receiver) {
		value, handled, err := vm.callStubProxyMember(receiver, method, args, result)
		if handled || err != nil {
			return value, true, err
		}
	}
	if value, handled, err := vm.callFrameworkSObjectDescribeMember(receiver, method, args, result); handled || err != nil {
		return value, true, err
	}
	if value, handled, err := vm.callFrameworkSObjectUnitOfWorkMember(receiver, method, args, result); handled || err != nil {
		return value, true, err
	}
	if value, handled, err := vm.callFrameworkQueryFactoryMember(receiver, method, args); handled || err != nil {
		return value, true, err
	}
	if value, handled, err := vm.callFrameworkSimpleDMLMember(receiver, method, args, result); handled || err != nil {
		return value, true, err
	}
	if value, handled, err := vm.callFrameworkMockRecorderMember(receiver, method, args); handled || err != nil {
		return value, true, err
	}
	if value, handled, err := vm.callFrameworkMatcherMember(receiver, method, args); handled || err != nil {
		return value, true, err
	}
	if strings.EqualFold(method, "values") || strings.EqualFold(method, "valueOf") {
		if value, handled, err := vm.callEnumStaticMember(runtimeObjectType(receiver), method, args); handled || err != nil {
			return value, true, err
		}
	}
	if value, handled, err := vm.callEnumMember(receiver, method, args); handled || err != nil {
		return value, true, err
	}
	if value, updated, mutated, handled, err := vm.callManagedPassiveMember(receiver, method, args); handled || err != nil {
		if mutated {
			if err := vm.storeReceiver(receiverName, updated); err != nil {
				return Null, true, err
			}
		}
		return value, true, err
	}
	if visualEditorPlatformObjectType(receiver.Type) {
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	if cachePartitionPlatformObjectType(receiver.Type) {
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	if authConfigurationPlatformObjectType(receiver.Type) {
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	if metadataContainerPlatformObjectType(receiver.Type) {
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	if localRuntimeHarnessPlatformObjectType(receiver.Type) {
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	if _, classExists := vm.lookupClass(receiver.Type); classExists && !strings.EqualFold(receiver.Runtime, "System.Cookie") {
		dispatchType := runtimeObjectType(receiver)
		target, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchType, method, args)
		if ambiguous {
			return Null, true, vm.ambiguousOverloadError(memberCallName(receiverName, dispatchType, method), args)
		}
		if ok {
			if methodHasModifier(target.Modifiers, "abstract") {
				if override, found := vm.uniqueConcreteOverride(receiver, dispatchType, method, args); found {
					target = override
				} else if override, found := vm.concreteOverrideByReceiverFields(receiver, dispatchType, method, args); found {
					target = override
				}
			}
			accessMethod := target
			if receiverType := vm.VarTypes[receiverName]; receiverType != "" {
				accessMethod = vm.dispatchAccessMethod(receiverType, target, method, args)
			} else if receiver.Static != "" {
				accessMethod = vm.dispatchAccessMethod(receiver.Static, target, method, args)
			}
			if err := vm.checkMemberAccess(accessMethod.ClassName, accessMethod.Access, accessMethod.Name, accessMethod.Modifiers); err != nil {
				return Null, true, err
			}
			value, err := vm.callMethodWithReceiver(target, receiver, args, result)
			if err == nil && passiveGeneratedSelfReturn(target, receiver, value) {
				if err := vm.storeReceiver(receiverName, value); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
		if mutated {
			if err := vm.storeReceiver(receiverName, updated); err != nil {
				return Null, true, err
			}
		}
		return value, true, err
	}
	if vm.isSObjectLikeType(receiver.Type) {
		if value, handled, err := vm.callSObjectMember(receiver, method, args); handled || err != nil {
			if method == "put" || method == "addError" || method == "clear" || method == "recalculateFormulas" {
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
	}
	dispatchType := runtimeObjectType(receiver)
	if value, handled := vm.commerceLocalHarnessInstanceDefault(receiverName, receiver, dispatchType, method, args); handled {
		return value, true, nil
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchType, method, args)
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(memberCallName(receiverName, dispatchType, method), args)
	}
	if !ok {
		if declaredType := vm.declaredReceiverType(receiverName); declaredType != "" && !strings.EqualFold(declaredType, dispatchType) {
			target, ok, ambiguous = vm.resolveInstanceMethodForArgs(declaredType, method, args)
			if ambiguous {
				return Null, true, vm.ambiguousOverloadError(memberCallName(receiverName, declaredType, method), args)
			}
			if ok {
				if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name, target.Modifiers); err != nil {
					return Null, true, err
				}
				value, err := vm.callMethodWithReceiver(target, receiver, args, result)
				return value, true, err
			}
		}
		if value, updated, mutated, handled, err := vm.callManagedPassiveMember(receiver, method, args); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
		if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
		if value, handled := vm.generatedPlatformInstanceDefault(receiverName, receiver, method, args); handled {
			return value, true, nil
		}
		if value, handled, err := vm.callManagedPassiveMissingMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
		if receiver.Fields != nil {
			if mapField, ok := receiver.Fields["map"]; ok && mapField.Kind == ValueMap {
				proxyName := receiverName
				if proxyName != "" {
					proxyName += ".map"
				}
				return vm.callValueMember(proxyName, mapField, method, args, result)
			}
		}
		if value, handled, err := vm.retrySimpleNameStaticFieldMember(receiverName, receiver, method, args, result); handled || err != nil {
			return value, true, err
		}
		if !vm.currentMethod.Dependency && strings.Contains(dispatchType, ".") {
			short := shortTypeName(dispatchType)
			if vm.typeNameIsLocalClass(short) {
				if target, ok, ambiguous := vm.resolveLocalInstanceMethodForArgs(short, method, args); ambiguous {
					return Null, true, vm.ambiguousOverloadError(memberCallName(receiverName, short, method), args)
				} else if ok {
					if err := vm.checkMemberAccess(target.ClassName, target.Access, target.Name, target.Modifiers); err != nil {
						return Null, true, err
					}
					value, err := vm.callMethodWithReceiver(target, receiver, args, result)
					return value, true, err
				}
			}
		}
		return Null, true, unsupportedCallError(memberCallName(receiverName, dispatchType, method))
	}
	if methodHasModifier(target.Modifiers, "abstract") {
		if override, found := vm.uniqueConcreteOverride(receiver, dispatchType, method, args); found {
			target = override
		} else if override, found := vm.concreteOverrideByReceiverFields(receiver, dispatchType, method, args); found {
			target = override
		}
	}
	accessMethod := target
	if receiverType := vm.VarTypes[receiverName]; receiverType != "" {
		accessMethod = vm.dispatchAccessMethod(receiverType, target, method, args)
	} else if receiver.Static != "" {
		accessMethod = vm.dispatchAccessMethod(receiver.Static, target, method, args)
	}
	if err := vm.checkMemberAccess(accessMethod.ClassName, accessMethod.Access, accessMethod.Name, accessMethod.Modifiers); err != nil {
		return Null, true, err
	}
	value, err := vm.callMethodWithReceiver(target, receiver, args, result)
	if err == nil && passiveGeneratedSelfReturn(target, receiver, value) {
		if err := vm.storeReceiver(receiverName, value); err != nil {
			return Null, true, err
		}
	}
	return value, true, err
}

// callNonObjectValueMember dispatches a member call whose receiver is a
// collection or other non-object value (List, Set, Map, and the shared
// fall-through handling). It is the non-ValueObject arm of callValueMember.
func (vm *VM) callNonObjectValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	switch receiver.Kind {
	case ValueList:
		method = canonicalCollectionMemberName("List", method)
		if value, handled, err := vm.callListValueMember(receiverName, receiver, method, args, result); handled || err != nil {
			return value, handled, err
		}
	case ValueSet:
		method = canonicalCollectionMemberName("Set", method)
		if value, handled, err := vm.callSetValueMember(receiverName, receiver, method, args, result); handled || err != nil {
			return value, handled, err
		}
	case ValueMap:
		method = canonicalCollectionMemberName("Map", method)
		if value, handled, err := vm.callMapValueMember(receiverName, receiver, method, args, result); handled || err != nil {
			return value, handled, err
		}
	}
	if value, handled := vm.generatedPlatformInstanceDefault(receiverName, receiver, method, args); handled {
		return value, true, nil
	}
	if receiver.Kind == ValueObject && receiver.Fields != nil {
		if mapField, ok := receiver.Fields["map"]; ok && mapField.Kind == ValueMap {
			proxyName := receiverName
			if proxyName != "" {
				proxyName += ".map"
			}
			return vm.callValueMember(proxyName, mapField, method, args, result)
		}
	}
	if value, handled, err := vm.retrySimpleNameStaticFieldMember(receiverName, receiver, method, args, result); handled || err != nil {
		return value, true, err
	}
	return Null, true, unsupportedCallError(memberCallName(receiverName, receiver.Type, method))
}

func (vm *VM) retrySimpleNameStaticFieldMember(receiverName string, current Value, method string, args []Value, result *Result) (Value, bool, error) {
	if receiverName == "" || strings.Contains(receiverName, ".") || vm.currentClass == "" {
		return Null, false, nil
	}
	if _, declared := vm.VarTypes[receiverName]; declared {
		return Null, false, nil
	}
	field, owner, ok := vm.lookupStaticField(vm.currentClass, receiverName)
	if !ok {
		return Null, false, nil
	}
	if err := vm.checkMemberAccess(owner, field.Access, owner+"."+receiverName, field.Modifiers); err != nil {
		return Null, true, err
	}
	if err := vm.ensureClassInitialized(owner); err != nil {
		return Null, true, err
	}
	field, _, _ = vm.lookupStaticField(owner, receiverName)
	retry := field.Value
	if field.Getter != nil {
		var err error
		retry, err = vm.callGetter(owner, field, Null)
		if err != nil {
			return Null, true, err
		}
	}
	if current.Kind == retry.Kind && current.Type == retry.Type && current.Ref == retry.Ref {
		return Null, false, nil
	}
	switch retry.Kind {
	case ValueMap, ValueList, ValueSet, ValueObject:
		return vm.callValueMember(receiverName, retry, method, args, result)
	case ValueNull:
		if strings.TrimSpace(retry.Type) == "" {
			if strings.TrimSpace(field.Type) != "" {
				retry.Type = field.Type
			}
		}
		if materialized, ok := materializeTypedNullCollection(retry); ok {
			if err := vm.storeReceiver(receiverName, materialized); err != nil {
				return Null, true, err
			}
			return vm.callValueMember(receiverName, materialized, method, args, result)
		}
		return vm.callValueMember(receiverName, retry, method, args, result)
	}
	return Null, false, nil
}

func materializeTypedNullCollection(value Value) (Value, bool) {
	if value.Kind != ValueNull || strings.TrimSpace(value.Type) == "" {
		return Value{}, false
	}
	switch collectionBase(value.Type) {
	case "List":
		return typedList(value.Type), true
	case "Set":
		return typedSet(value.Type), true
	}
	if isMapType(value.Type) {
		return typedMap(value.Type), true
	}
	return Value{}, false
}

func mapLikeMemberName(method string) bool {
	switch canonicalCollectionMemberName("Map", method) {
	case "put", "putAll", "get", "containsKey", "containsValue", "keySet", "values", "remove", "clear", "size", "isEmpty", "clone", "deepClone":
		return true
	default:
		return false
	}
}

func (vm *VM) commerceLocalHarnessInstanceDefault(receiverName string, receiver Value, dispatchType, method string, args []Value) (Value, bool) {
	candidates := []string{dispatchType, receiver.Type, receiver.Static, receiver.Runtime}
	if declaredType := vm.declaredReceiverType(receiverName); declaredType != "" {
		candidates = append(candidates, declaredType)
	}
	for _, candidate := range candidates {
		if !commerceLocalHarnessRuntimeMethod(candidate, method) {
			continue
		}
		generated, ok := vm.generatedPlatformMethodForArgs(candidate, method, args, false)
		if !ok {
			continue
		}
		return vm.generatedPlatformMethodDefaultReturn(generated, receiver, args), true
	}
	return Null, false
}

func memberCallName(receiverName, receiverType, method string) string {
	if receiverName != "" {
		return receiverName + "." + method
	}
	if receiverType != "" {
		return receiverType + "." + method
	}
	return "." + method
}

func runtimeObjectType(value Value) string {
	if value.Runtime != "" {
		return value.Runtime
	}
	return value.Type
}

func canonicalCollectionMemberName(collection, method string) string {
	var known []string
	switch collection {
	case "List":
		known = []string{"add", "addAll", "addToRelationship", "getAddedToRelationship", "getMarkedForDeletion", "markForDelete", "size", "isEmpty", "get", "contains", "indexOf", "clone", "deepClone", "iterator", "sort", "remove", "clear", "set", "getSObjectType"}
	case "Set":
		known = []string{"add", "addAll", "size", "isEmpty", "contains", "containsAll", "remove", "clear", "removeAll", "retainAll", "clone", "deepClone", "iterator"}
	case "Map":
		known = []string{"put", "putAll", "get", "containsKey", "containsValue", "keySet", "values", "remove", "clear", "size", "isEmpty", "clone", "deepClone"}
	}
	for _, candidate := range known {
		if strings.EqualFold(method, candidate) {
			return candidate
		}
	}
	return method
}

func listRelationshipValues(receiver Value, field string) Value {
	if receiver.Fields != nil {
		if list, ok := receiver.Fields[field]; ok && list.Kind == ValueList {
			return list
		}
	}
	out := List()
	out.Type = "List<SObject>"
	return out
}

func canonicalPlatformObjectMemberName(typeName, method string) string {
	if strings.EqualFold(typeName, "TimeZone") && strings.EqualFold(method, "getID") {
		return "getID"
	}
	known := []string{
		"get", "iterator",
		"getQuery",
		"getJobId", "getTriggerId",
		"getAsyncApexJobId", "getRequestId", "getResult", "getException",
		"getDuplicateSignature", "setDuplicateSignature",
		"getMaximumQueueableStackDepth", "setMaximumQueueableStackDepth",
		"getMinimumQueueableDelayInMinutes", "setMinimumQueueableDelayInMinutes",
		"newSObject", "getDescribe", "getRecordTypeInfosByName", "getRecordTypeInfosById",
		"getRecordTypeId",
		"getMap",
		"getName", "getLabel", "getType", "getSoapType",
		"isNillable", "isExternalId", "isUnique", "isEncrypted", "isNameField",
		"getLength", "getPrecision", "getScale", "isHtmlFormatted",
		"getReferenceTo", "getRelationshipName", "getPicklistValues", "getSObjectField",
		"getFields", "getFieldPath", "getRequired", "getDbRequired",
		"getController", "getControllerValues", "isAccessible", "isCreateable", "isUpdateable",
		"getTabs", "isSelected", "getSObjectName", "isCustom", "getIconUrl", "getIcons",
		"getContentType", "getHeight", "getTheme", "getWidth",
		"to15", "to18", "getSObjectType",
		"toStartOfMonth", "toEndOfMonth", "format", "formatGmt", "toString",
		"date", "dateGmt", "time", "timeGmt", "year", "month", "day", "getTime",
		"addDays", "addMonths", "addYears", "addHours", "addMinutes", "addSeconds", "addMilliseconds",
		"daysBetween", "monthsBetween", "isSameDay", "dayOfYear", "daysInMonth", "isLeapYear",
		"equals", "hashCode", "newInstance", "isAssignableFrom", "getNamespace", "getPackageName",
		"send", "toExternalForm", "getProtocol", "getHost", "getAuthority",
		"getPath", "getQuery", "getRef", "getFile", "getPort", "getDefaultPort",
		"addHeader", "getHeader", "getHeaderKeys", "addParameter", "addParam",
		"getParameter", "getParam", "getParameterKeys", "getParamKeys",
		"setStaticResource", "setStatusCode", "setHeader",
		"getId", "getRecord", "save", "quickSave", "delete", "view", "edit",
		"cancel", "reset", "addFields",
		"getRecords", "getSelected", "setSelected", "getPageSize", "setPageSize",
		"getPageNumber", "first", "last", "next", "previous", "getHasNext",
		"getHasPrevious", "getCompleteResult", "getListViewOptions", "setPageNumber", "setFilterId", "getFilterId",
		"setToAddresses", "setCcAddresses", "setBccAddresses", "setFileAttachments",
		"setEntityAttachments", "setDocumentAttachments", "setTargetObjectIds",
		"setBody", "setContentType", "setFileName", "setInline",
		"getBody", "getContentType", "getFileName", "getId", "getInline",
		"setSubject", "setPlainTextBody", "setHtmlBody", "setReplyTo",
		"setSenderDisplayName", "setSaveAsActivity", "setTreatBodiesAsTemplate",
		"setTreatTargetObjectAsRecipient", "setUseSignature", "setBccSender",
		"setOneClickPost", "setUnsubscribeComment", "setUnsubscribeUrls",
		"getToAddresses", "getCcAddresses", "getBccAddresses", "getFileAttachments",
		"getEntityAttachments", "getDocumentAttachments", "getTargetObjectIds",
		"setWhatIds", "setTemplateId", "setDescription", "setOptOutPolicy",
		"setEmailPriority", "setOrgWideEmailAddressId", "setWhatId",
		"getWhatIds", "getTemplateId", "getDescription", "getOptOutPolicy",
		"getSaveAsActivity", "getEmailPriority", "getOrgWideEmailAddressId",
		"getTemplateName", "getUnsubscribeComment", "getUnsubscribeUrls",
		"getOneClickPost", "isTreatBodiesAsTemplate", "isTreatTargetObjectAsRecipient",
		"isUserMail",
		"getActionEnumOrId", "getActionName", "getActionType", "getContextId", "getDefaultValue",
		"getDefaultValueFormulas", "getDefaultValues", "getErrors", "getField", "getFromAddressList",
		"getInReplyToId", "getLayout", "getParameters", "getQuickActionName", "getRecord", "getSuccessMessage",
		"getTargetParentField", "getTargetRecordTypeId", "getTargetSObject", "getTargetSobjectType",
		"getContextSobjectType", "getIds", "isCreated", "isEditableForNew", "isEditableForUpdate",
		"isPlaceholder", "isRequired", "isStandard", "isSuccess", "setContextId", "setIgnoreTemplateSubject",
		"setInsertTemplateBody", "setQuickActionName", "setRecord", "setTemplateId",
	}
	if isExceptionType(typeName) {
		known = append(known,
			"getMessage",
			"setMessage",
			"getNumDml",
			"getDmlType",
			"getDmlMessage",
			"getDmlStatusCode",
			"getDmlFields",
			"getDmlId",
			"getDmlIndex",
			"getInaccessibleFields",
			"getCause",
			"initCause",
			"getDescription",
			"getIndex",
			"getPattern",
			"getTypeName",
			"getLineNumber",
			"getStackTraceString",
			"toString",
		)
	}
	return canonicalStdlibMemberName(method, known...)
}

func (vm *VM) storeReceiver(receiverName string, value Value) error {
	if receiverName == "" {
		return nil
	}
	if strings.Contains(receiverName, ".") {
		return vm.assign(receiverName, value)
	}
	if _, declared := vm.VarTypes[receiverName]; !declared {
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if _, _, _, ok := vm.lookupThisFieldRoot(this, receiverName); ok {
				return vm.assign(receiverName, value)
			}
		}
	}
	if _, ok := vm.lookupGlobalName(receiverName); ok {
		return vm.assign(receiverName, value)
	}
	if vm.currentClass != "" {
		if _, _, ok := vm.lookupStaticField(vm.currentClass, receiverName); ok {
			return vm.assign(receiverName, value)
		}
	}
	vm.Globals[receiverName] = value
	return nil
}

func (vm *VM) mapKey(value Value) string {
	if key, ok := vm.apexObjectHashMapKey(value); ok {
		return key
	}
	return mapKey(value)
}

func (vm *VM) apexObjectHashMapKey(value Value) (string, bool) {
	if isStubProxy(value) {
		return "", false
	}
	if value.Kind != ValueObject || value.Type == "" ||
		(sObjectValueType(value.Type) && !vm.userClassShadowsSObjectType(value.Type)) ||
		strings.HasPrefix(value.Type, "Schema.") || platformScalarObject(value.Type) ||
		strings.EqualFold(value.Type, "Type") {
		return "", false
	}
	if key, ok := vm.frameworkQualifiedMethodMapKey(value); ok {
		return key, true
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(value.Type, "hashCode", nil)
	if !ok || ambiguous || target.Name == "" || strings.EqualFold(target.ClassName, "Object") {
		if value.Ref != 0 {
			return string(ValueObject) + ":" + value.Type + ":ref:" + strconv.FormatUint(value.Ref, 10), true
		}
		return "", false
	}
	result, err := vm.callMethodWithReceiver(target, value, nil, &Result{})
	if err != nil || result.Kind != ValueInt {
		if value.Ref != 0 {
			return string(ValueObject) + ":" + value.Type + ":ref:" + strconv.FormatUint(value.Ref, 10), true
		}
		return "", false
	}
	return string(ValueObject) + ":" + value.Type + ":hash:" + strconv.FormatInt(result.Int, 10), true
}

func objectStringField(value Value, field string) string {
	_, raw, ok := objectFieldValue(value, field)
	if !ok {
		return ""
	}
	switch raw.Kind {
	case ValueString:
		return raw.Text
	case ValueObject:
		if raw.Type == "String" {
			if text, ok := raw.Fields["value"]; ok && text.Kind == ValueString {
				return text.Text
			}
		}
	}
	return ""
}

func (vm *VM) specialMapLookup(receiver, key Value) (Value, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString {
		return Null, false
	}
	if objectName, ok := sObjectFieldMapObjectName(receiver); ok || receiver.Type == "Schema.SObjectFieldMap" {
		if objectName != "" && vm.sObjectFieldMapKeyIsChildRelationship(objectName, key.Text) {
			return Null, true
		}
		for _, alias := range vm.sObjectFieldMapLookupAliases(key.Text) {
			if value, ok := receiver.Map[mapKey(String(alias))]; ok {
				return value, true
			}
		}
		if objectName != "" {
			if vm.canSynthesizeSObjectFieldMapField(objectName) {
				for _, alias := range vm.sObjectFieldMapLookupAliases(key.Text) {
					if systemField, ok := syntheticSObjectSystemField(alias); ok {
						return vm.sObjectFieldToken(objectName, systemField.APIName), true
					}
				}
			}
		}
		return Null, false
	}
	if receiver.Type == "Schema.GlobalDescribeMap" {
		if vm != nil {
			if resolved, ok := vm.resolveGlobalDescribeObjectName(key.Text); ok {
				return sObjectTypeToken(resolved), true
			}
		}
		for _, alias := range vm.schemaDescribeMapAliases(key.Text) {
			if value, ok := receiver.Map[mapKey(String(alias))]; ok {
				return value, true
			}
		}
		return Null, false
	}
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if mapContainsOnlySObjectFieldTokens(receiver) {
		for rawKey, value := range receiver.Map {
			candidate := valueFromMapKey(rawKey)
			if candidate.Kind == ValueString && schemaDescribeMapKeyMatches(namespace, candidate.Text, key.Text) {
				return value, true
			}
		}
	}
	for _, value := range receiver.Map {
		if value.Kind != ValueObject || value.Type != "Schema.RecordTypeInfo" {
			continue
		}
		if recordTypeInfoKeyMatches(value, key.Text) {
			return value, true
		}
	}
	return Null, false
}

func (vm *VM) specialMapContainsKey(receiver, key Value) bool {
	if receiver.Kind != ValueMap || key.Kind != ValueString {
		return false
	}
	if objectName, ok := sObjectFieldMapObjectName(receiver); ok || receiver.Type == "Schema.SObjectFieldMap" {
		_ = objectName
		for _, alias := range vm.sObjectFieldMapLookupAliases(key.Text) {
			if _, ok := receiver.Map[mapKey(String(alias))]; ok {
				return true
			}
		}
		return false
	}
	if receiver.Type == "Schema.GlobalDescribeMap" {
		if vm != nil {
			if _, ok := vm.resolveGlobalDescribeObjectName(key.Text); ok {
				return true
			}
		}
		for _, alias := range vm.schemaDescribeMapAliases(key.Text) {
			if _, ok := receiver.Map[mapKey(String(alias))]; ok {
				return true
			}
		}
		return false
	}
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if mapContainsOnlySObjectFieldTokens(receiver) {
		for rawKey := range receiver.Map {
			candidate := valueFromMapKey(rawKey)
			if candidate.Kind == ValueString && schemaDescribeMapKeyMatches(namespace, candidate.Text, key.Text) {
				return true
			}
		}
	}
	for _, value := range receiver.Map {
		if value.Kind != ValueObject || value.Type != "Schema.RecordTypeInfo" {
			continue
		}
		if recordTypeInfoKeyMatches(value, key.Text) {
			return true
		}
	}
	return false
}

func recordTypeInfoKeyMatches(value Value, key string) bool {
	for _, field := range []string{"name", "developerName"} {
		candidate, ok := value.Fields[field]
		if !ok || candidate.Kind != ValueString {
			continue
		}
		if strings.EqualFold(candidate.Text, key) {
			return true
		}
		if compactRecordTypeKey(candidate.Text) != "" && compactRecordTypeKey(candidate.Text) == compactRecordTypeKey(key) {
			return true
		}
	}
	return false
}

func compactRecordTypeKey(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func metadataContainerPlatformObjectType(typeName string) bool {
	return strings.EqualFold(typeName, "Metadata.DeployContainer")
}

func authConfigurationPlatformObjectType(typeName string) bool {
	return strings.EqualFold(typeName, "Auth.AuthConfiguration") || strings.EqualFold(typeName, "auth.AuthConfiguration")
}

func localRuntimeHarnessPlatformObjectType(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "eventbus.testbroker"),
		strings.EqualFold(typeName, "externalservicetest"),
		strings.EqualFold(typeName, "invocable.action"),
		strings.EqualFold(typeName, "invocable.action.result"),
		strings.EqualFold(typeName, "testasynchttp"),
		strings.EqualFold(typeName, "functions.functioninvokemock"):
		return true
	default:
		return false
	}
}

func (vm *VM) namespaceStringMapLookup(receiver Value, key Value) (Value, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString {
		return Null, false
	}
	if _, valueType, ok := mapTypeArgs(receiver.Type); ok && strings.EqualFold(valueType, "String") {
		return Null, false
	}
	if !isCustomObjectLikeName(key.Text) && !hasSuffixFold(key.Text, "__mdt") && !hasManagedStringNamespaceToken(key.Text) && strings.TrimSpace(vm.currentCallerNamespace()) == "" {
		return Null, false
	}
	aliases := []string{key.Text, localSchemaName(key.Text)}
	if base, ok := managedStringNamespaceBase(key.Text); ok {
		aliases = append(aliases, base)
		if namespace := strings.TrimSpace(vm.currentCallerNamespace()); namespace != "" {
			aliases = append(aliases, namespace+"__"+base)
		}
	} else if namespace := strings.TrimSpace(vm.currentCallerNamespace()); namespace != "" && !strings.Contains(key.Text, "__") {
		aliases = append(aliases, namespace+"__"+key.Text)
	}
	if vm.Org != nil && vm.Org.Namespace != "" {
		aliases = append(aliases,
			storage.NamespaceTokenName(vm.Org.Namespace, key.Text),
			storage.StripNamespaceToken(vm.Org.Namespace, key.Text),
		)
	}
	seen := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		normalized := strings.ToLower(alias)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		if value, ok := receiver.Map[mapKey(String(alias))]; ok {
			return value, true
		}
	}
	return Null, false
}

func hasManagedStringNamespaceToken(value string) bool {
	_, ok := managedStringNamespaceBase(value)
	return ok
}

func managedStringNamespaceBase(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Count(value, "__") != 1 {
		return "", false
	}
	namespace, base, ok := strings.Cut(value, "__")
	if !ok || namespace == "" || base == "" {
		return "", false
	}
	switch strings.ToLower(base) {
	case "c", "e", "r", "mdt":
		return "", false
	}
	return base, true
}

func (vm *VM) populatedFieldsKeySetContains(receiver, key Value) (bool, bool) {
	if receiver.Kind != ValueSet || key.Kind != ValueString || !strings.HasPrefix(receiver.Runtime, "sobject-populated-fields:") {
		return false, false
	}
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if namespace == "" {
		return false, false
	}
	for _, candidate := range receiver.Set {
		if candidate.Kind == ValueString && schemaDescribeMapKeyMatches(namespace, candidate.Text, key.Text) {
			return true, true
		}
	}
	return false, true
}

func schemaDescribeMapKeyMatches(namespace, canonical, candidate string) bool {
	if strings.EqualFold(canonical, candidate) {
		return true
	}
	if namespace != "" {
		if strings.EqualFold(canonical, storage.StripNamespaceToken(namespace, candidate)) ||
			strings.EqualFold(storage.StripNamespaceToken(namespace, canonical), candidate) {
			return true
		}
	}
	return strings.EqualFold(stripAnyNamespaceToken(canonical), stripAnyNamespaceToken(candidate))
}

func stripAnyNamespaceToken(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	rest := name[first+2:]
	if strings.Contains(rest, "__") {
		return rest
	}
	return name
}

func (vm *VM) resolveGlobalDescribeObjectName(name string) (string, bool) {
	if vm == nil {
		return "", false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	orgNamespace := ""
	if vm.Org != nil {
		orgNamespace = strings.TrimSpace(vm.Org.Namespace)
	}
	namespaces := []string{
		strings.TrimSpace(vm.currentCallerNamespace()),
		orgNamespace,
	}
	if vm.Org != nil && isCustomObjectLikeName(name) && storage.StripAnyNamespaceToken(name) == name {
		for _, namespace := range namespaces {
			if namespace == "" {
				continue
			}
			prefixed := storage.NamespaceTokenName(namespace, name)
			if prefixed == name {
				continue
			}
			if _, ok := vm.Org.Objects[prefixed]; ok {
				return prefixed, true
			}
			for candidate := range vm.Org.Objects {
				if strings.EqualFold(candidate, prefixed) {
					return candidate, true
				}
			}
		}
	}
	if vm.Org != nil {
		if resolved, ok := vm.resolveObjectName(name); ok {
			return resolved, true
		}
	}
	return "", false
}

func (vm *VM) objectKeyMapLookup(receiver Value, key Value) (Value, bool, error) {
	if receiver.Kind != ValueMap || len(receiver.Map) == 0 || len(receiver.MapKeys) == 0 {
		return Null, false, nil
	}
	for rawKey, storedKey := range receiver.MapKeys {
		equal, err := vm.mapKeysEqual(storedKey, key)
		if err != nil {
			return Null, false, err
		}
		if equal {
			value, ok := receiver.Map[rawKey]
			return value, ok, nil
		}
	}
	return Null, false, nil
}

func (vm *VM) declaredReceiverIsEnum(receiverName string) bool {
	declaredType := vm.declaredReceiverType(receiverName)
	if declaredType == "" {
		return false
	}
	_, ok := vm.resolveEnumClass(declaredType)
	return ok
}

func (vm *VM) mapKeysEqual(storedKey, lookupKey Value) (bool, error) {
	if equal, ok := vm.frameworkQualifiedMethodKeysEqual(storedKey, lookupKey); ok {
		return equal, nil
	}
	if equal, handled := vm.resolvedEnumValuesEqual(storedKey, lookupKey); handled {
		return equal, nil
	}
	if storedKey.Kind == ValueObject {
		if target, ok, ambiguous := vm.resolveInstanceMethodForArgs(storedKey.Type, "equals", []Value{lookupKey}); ok && !ambiguous {
			value, err := vm.callMethodWithReceiver(target, storedKey, []Value{lookupKey}, &Result{})
			if err != nil {
				return false, err
			}
			if value.Kind == ValueBool {
				return value.Bool, nil
			}
		}
	}
	return storedKey.Equal(lookupKey), nil
}

var (
	aliasRefSliceMapPool = sync.Pool{New: func() any { m := make(map[uint64][]string, 8); return &m }}
	aliasRefSetPool      = sync.Pool{New: func() any { m := make(map[uint64]bool, 16); return &m }}
	aliasPairSetPool     = sync.Pool{New: func() any { m := make(map[[2]uint64]bool, 16); return &m }}
)

type aliasSnapshot struct {
	ref      uint64
	kind     ValueKind
	typeName string
}

func (s aliasSnapshot) valid() bool {
	return s.ref != 0
}

func dataWeaveStaticScriptReceiver(receiverName string) bool {
	return strings.EqualFold(strings.TrimSpace(receiverName), "DataWeave.Script") ||
		strings.EqualFold(strings.TrimSpace(receiverName), "dataweave.Script")
}

func isStubProxy(receiver Value) bool {
	if receiver.Kind != ValueObject {
		return false
	}
	provider, ok := receiver.Fields["__gladeStubProvider"]
	return ok && provider.Kind == ValueObject
}

func (vm *VM) currentStubProvider(receiver Value) Value {
	provider := receiver.Fields["__gladeStubProvider"]
	if provider.Kind != ValueObject || provider.Ref == 0 {
		return provider
	}
	if current, ok := vm.findValueByRef(provider.Ref); ok && current.Kind == ValueObject {
		if frameworkApexMocksProviderActive(current) {
			return current
		}
		if frameworkApexMocksProviderHasRecorder(current) {
			return current
		}
		if frameworkApexMocksProviderHasRecorder(provider) {
			return provider
		}
		if !stubProviderStateFlagSet(current) {
			if live, liveOK := vm.findLiveStubProvider(provider); liveOK {
				return live
			}
		}
		return current
	}
	if live, ok := vm.findLiveStubProvider(provider); ok && frameworkApexMocksProviderActive(live) {
		return live
	}
	if frameworkApexMocksProviderHasRecorder(provider) {
		if current, ok := vm.findLiveStubProvider(provider); ok {
			return current
		}
		return provider
	}
	if current, ok := vm.findLiveStubProvider(provider); ok {
		return current
	}
	return provider
}

func (vm *VM) findLiveStubProvider(provider Value) (Value, bool) {
	var candidate Value
	found := false
	for _, scope := range vm.liveScopes() {
		for _, value := range scope {
			if value.Kind != ValueObject || !strings.EqualFold(value.Type, provider.Type) {
				continue
			}
			if stubProviderStateFlagSet(value) {
				return value, true
			}
			if !found {
				candidate = value
				found = true
			} else {
				candidate = Null
			}
		}
	}
	return candidate, found && candidate.Kind == ValueObject
}

func stubProviderStateFlagSet(provider Value) bool {
	for _, name := range []string{"Stubbing", "Verifying", "verifying", "Recording"} {
		if _, value, ok := objectFieldValue(provider, name); ok && value.Kind == ValueBool && value.Bool {
			return true
		}
	}
	return false
}

func (vm *VM) findValueByRef(ref uint64) (Value, bool) {
	if ref == 0 {
		return Null, false
	}
	for _, scope := range vm.liveScopes() {
		if value, ok := directValueByRefInScope(scope, ref); ok {
			return value, true
		}
	}
	if value, ok := vm.staticFieldValueByRef(ref); ok {
		return value, true
	}
	for _, scope := range vm.liveScopes() {
		if value, ok := findValueByRefInScope(scope, ref, make(map[uint64]bool)); ok {
			return value, true
		}
	}
	if value, ok := vm.scanStaticFieldValueByRef(ref); ok {
		return value, true
	}
	return Null, false
}

func findValueByRef(value Value, ref uint64, seen map[uint64]bool) (Value, bool) {
	if value.Ref != 0 {
		if value.Ref == ref {
			return value, true
		}
		if seen[value.Ref] {
			return Null, false
		}
		seen[value.Ref] = true
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if found, ok := findValueByRef(child, ref, seen); ok {
				return found, true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if found, ok := findValueByRef(child, ref, seen); ok {
				return found, true
			}
		}
		for _, child := range value.MapKeys {
			if found, ok := findValueByRef(child, ref, seen); ok {
				return found, true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if found, ok := findValueByRef(child, ref, seen); ok {
				return found, true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if found, ok := findValueByRef(child, ref, seen); ok {
				return found, true
			}
		}
	}
	return Null, false
}

func (vm *VM) callStubProxyMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, method, args)
	if ambiguous {
		target, ok = vm.firstRegisteredMethodByArity(receiver.Type, method, args)
		if !ok {
			return Null, true, vm.ambiguousOverloadError(receiver.Type+"."+method, args)
		}
	}
	if !ok {
		target, ok, ambiguous = vm.resolveInstanceMethodByArity(receiver.Type, method, len(args))
		if ambiguous {
			ok = target.Name != ""
			if !ok {
				return Null, true, vm.ambiguousOverloadError(receiver.Type+"."+method, args)
			}
		}
	}
	if !ok {
		if vm.isSObjectLikeType(receiver.Type) && sObjectMemberCallShapeSupported(method, args) {
			if value, handled, err := vm.callSObjectMember(receiver, method, args); handled || err != nil {
				return value, true, err
			}
		}
		if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
			return value, true, err
		}
		return vm.callStubProxyDynamicMember(receiver, method, args, result)
	}
	if !stubProxyCanInterceptMethod(target) {
		return Null, false, nil
	}
	provider := vm.currentStubProvider(receiver)
	paramTypes := make([]Value, 0, len(target.Params))
	paramNames := make([]Value, 0, len(target.Params))
	for _, param := range target.Params {
		paramTypes = append(paramTypes, platformScalar("Type", vm.resolveTypeNameInClass(target.ClassName, param.Type)))
		paramNames = append(paramNames, String(param.Name))
	}
	returnType := vm.resolveTypeNameInClass(target.ClassName, target.ReturnType)
	if returnType == "" {
		returnType = "Object"
	}
	metadataArgs := []Value{
		receiver,
		String(apexMethodMemberName(target.Name)),
		platformScalar("Type", returnType),
		{Kind: ValueList, Type: "List<Type>", List: paramTypes},
		{Kind: ValueList, Type: "List<String>", List: paramNames},
		{Kind: ValueList, Type: "List<Object>", List: append([]Value(nil), args...)},
	}
	recordedBefore := vm.frameworkStubInvocationCount(receiver, target)
	handler, ok, ambiguous := vm.resolveInstanceMethodForArgs(provider.Type, "handleMethodCall", metadataArgs)
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(provider.Type+".handleMethodCall", metadataArgs)
	}
	if !ok {
		return Null, true, fmt.Errorf("StubProvider %s must implement handleMethodCall", provider.Type)
	}
	recordingBefore := vm.stubProviderRecordingMode(provider)
	value, err := vm.callMethodWithReceiver(handler, provider, metadataArgs, result)
	if err != nil {
		return Null, true, err
	}
	provider = vm.currentStubProvider(receiver)
	if !recordingBefore && !vm.stubProviderRecordingMode(provider) && vm.frameworkStubInvocationCount(receiver, target) <= recordedBefore {
		if err := vm.frameworkRecordStubInvocation(receiver, target, args); err != nil {
			return Null, true, err
		}
	}
	if value.Kind == ValueNull && !vm.stubProviderRecordingMode(provider) {
		if fallback, ok, err := vm.unstubbedFrameworkMockReturnFallback(receiver, method, target, args, result); ok || err != nil {
			return fallback, true, err
		}
	}
	if value.Kind == ValueNull && stubReturnCanUseReceiver(target.ReturnType, receiver) && !vm.stubProviderRecordingMode(provider) {
		return receiver, true, nil
	}
	if target.ReturnType == "" || strings.EqualFold(target.ReturnType, "void") {
		return Null, true, nil
	}
	coerced, err := vm.coerceAssignable(target.ReturnType, value)
	if err != nil {
		if fallback, ok := frameworkMismatchedStubReturnFallback(target.ReturnType, value, provider); ok {
			return fallback, true, nil
		}
		return Null, true, fmt.Errorf("stubbed %s.%s return: %w", receiver.Type, method, err)
	}
	coerced = normalizeFrameworkStubReturnValue(target.ReturnType, coerced, provider)
	return coerced, true, nil
}

func (vm *VM) callStubProxyDynamicMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	provider := vm.currentStubProvider(receiver)
	paramTypes := make([]Value, 0, len(args))
	paramNames := make([]Value, 0, len(args))
	for i, arg := range args {
		paramTypes = append(paramTypes, platformScalar("Type", valueTypeName(arg)))
		paramNames = append(paramNames, String(fmt.Sprintf("arg%d", i)))
	}
	metadataArgs := []Value{
		receiver,
		String(apexMethodMemberName(method)),
		platformScalar("Type", "Object"),
		{Kind: ValueList, Type: "List<Type>", List: paramTypes},
		{Kind: ValueList, Type: "List<String>", List: paramNames},
		{Kind: ValueList, Type: "List<Object>", List: append([]Value(nil), args...)},
	}
	handler, ok, ambiguous := vm.resolveInstanceMethodForArgs(provider.Type, "handleMethodCall", metadataArgs)
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(provider.Type+".handleMethodCall", metadataArgs)
	}
	if !ok {
		return Null, true, fmt.Errorf("StubProvider %s must implement handleMethodCall", provider.Type)
	}
	value, err := vm.callMethodWithReceiver(handler, provider, metadataArgs, result)
	if err != nil {
		return Null, true, err
	}
	return value, true, nil
}

func (vm *VM) unstubbedDatasetServiceListFallback(receiverType, method string, target Method) (Value, bool) {
	if !hasSuffixFold(shortTypeName(receiverType), "DatasetService") {
		return Null, false
	}
	if !strings.EqualFold(method, "getDataSetMetadataByTag") && !strings.EqualFold(method, "getDataSetMetadataByTags") {
		return Null, false
	}
	returnType := vm.resolveTypeNameInClass(target.ClassName, target.ReturnType)
	if collectionBase(returnType) != "List" {
		return Null, false
	}
	return typedList(returnType), true
}

func (vm *VM) unstubbedSchemaServiceFallback(receiverType, method string, args []Value) (Value, bool) {
	if !hasSuffixFold(shortTypeName(receiverType), "SchemaService") {
		return Null, false
	}
	switch {
	case strings.EqualFold(method, "getDescribeResult") && len(args) == 1:
		objectName := ""
		switch {
		case args[0].Kind == ValueObject && isSObjectTypeToken(args[0]):
			if name, ok := sObjectTypeTokenObjectName(args[0]); ok {
				objectName = name
			}
		case args[0].Kind == ValueString:
			objectName = args[0].Text
		}
		if objectName == "" {
			return Null, false
		}
		canonical, definition, ok := vm.describeObjectDefinition(objectName)
		if !ok {
			return Null, false
		}
		return vm.describeSObjectValue(canonical, definition), true
	case strings.EqualFold(method, "getSObjectTypeByName") && len(args) == 1 && args[0].Kind == ValueString:
		if token, ok := vm.sObjectTypeTokenForName(args[0].Text); ok {
			return token, true
		}
	case strings.EqualFold(method, "getSObjectField") && len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueString:
		return sObjectFieldToken(args[0].Text, args[1].Text), true
	}
	return Null, false
}

func (vm *VM) unstubbedUnitOfWorkFallbackType(receiverType, method string, target Method) (string, bool) {
	if !strings.EqualFold(method, "getInstance") {
		return "", false
	}
	if !hasSuffixFold(shortTypeName(receiverType), "unitofworkservice") {
		return "", false
	}
	returnType := vm.resolveTypeNameInClass(target.ClassName, target.ReturnType)
	returnShort := shortTypeName(returnType)
	if len(returnShort) < 2 || returnShort[0] != 'I' || !hasSuffixFold(returnShort, "unitofwork") {
		return "", false
	}
	concreteShort := returnShort[1:]
	qualifier := typeQualifier(returnType)
	if qualifier == "" {
		qualifier = typeQualifier(receiverType)
	}
	concreteType := qualifier + concreteShort
	if _, ok := vm.resolveClassName(concreteType); ok {
		return concreteType, true
	}
	if qualifier != "" {
		if _, ok := vm.resolveClassName(concreteShort); ok {
			return concreteShort, true
		}
	}
	return "", false
}

func typeQualifier(typeName string) string {
	if i := strings.LastIndex(typeName, "."); i >= 0 {
		return typeName[:i+1]
	}
	return ""
}

func (vm *VM) callManagedAPIChain(callee string, args []Value) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 4 {
		return Null, false, nil
	}
	method := parts[len(parts)-1]
	version := parts[len(parts)-2]
	apiClass := parts[len(parts)-3]
	namespace := strings.Join(parts[:len(parts)-3], ".")
	if namespace == "" || !managedAPIVersionToken(version) || !strings.HasSuffix(apiClass, "Api") {
		return Null, false, nil
	}
	if _, ok := vm.resolveClassName(namespace + "." + apiClass); ok {
		return Null, false, nil
	}
	if generatedPlatformTypeName(namespace + "." + apiClass) {
		return Null, false, nil
	}
	if !looksManagedQualifiedType(namespace + "." + apiClass) {
		return Null, false, nil
	}
	name := strings.ToLower(apexMethodMemberName(method))
	switch {
	case strings.HasPrefix(name, "setmock"):
		return Null, true, nil
	case strings.HasPrefix(name, "is") || strings.HasPrefix(name, "has") || strings.HasPrefix(name, "can") || strings.HasPrefix(name, "should"):
		return Bool(false), true, nil
	}
	if len(args) != 0 {
		return Null, false, nil
	}
	return Object(namespace + "." + apiClass + managedAPIVersionName(version) + exportedMemberName(method)), true, nil
}

func (vm *VM) callManagedSingletonChain(callee string, args []Value) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) < 4 {
		return Null, false, nil
	}
	method := parts[len(parts)-1]
	property := parts[len(parts)-2]
	className := strings.Join(parts[:len(parts)-2], ".")
	if !strings.EqualFold(property, "Instance") || !looksManagedQualifiedType(className) {
		return Null, false, nil
	}
	if _, ok := vm.resolveClassName(className); ok {
		return Null, false, nil
	}
	if generatedPlatformTypeName(className) {
		return Null, false, nil
	}
	return managedPassiveReturnForMethod(className, method, args)
}

func (vm *VM) callManagedStaticFactory(typeName, method string, args []Value, result *Result) (Value, bool, error) {
	if !looksManagedQualifiedType(typeName) {
		return Null, false, nil
	}
	if _, ok := vm.resolveClassName(typeName); ok {
		return Null, false, nil
	}
	if generatedPlatformTypeName(typeName) {
		return Null, false, nil
	}
	name := strings.ToLower(apexMethodMemberName(method))
	switch {
	case strings.EqualFold(name, "newinstance"), strings.HasPrefix(name, "create"), strings.HasPrefix(name, "from"):
		return Object(typeName), true, nil
	default:
		return managedPassiveReturnForMethod(typeName, method, args)
	}
}

func managedQualifiedTypeParts(typeName string) (string, string, bool) {
	namespace, className, ok := strings.Cut(typeName, ".")
	namespace = strings.TrimSpace(namespace)
	className = strings.TrimSpace(className)
	return namespace, className, ok && namespace != "" && className != ""
}

func (vm *VM) callManagedPassiveMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if receiver.Kind != ValueObject || !looksManagedQualifiedType(receiver.Type) {
		return Null, receiver, false, false, nil
	}
	if generatedPlatformTypeName(receiver.Type) {
		return Null, receiver, false, false, nil
	}
	if _, ok := vm.resolveClassName(receiver.Type); ok {
		return Null, receiver, false, false, nil
	}
	value, handled, err := managedPassiveReturnForMethod(receiver.Type, method, args)
	if handled || err != nil {
		if value.Kind == ValueObject && strings.EqualFold(value.Type, receiver.Type) {
			value = receiver
		}
		return value, receiver, false, true, err
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) callManagedPassiveMissingMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Kind != ValueObject || !looksManagedQualifiedType(receiver.Type) || generatedPlatformTypeName(receiver.Type) {
		return Null, false, nil
	}
	return managedPassiveReturnForMethod(receiver.Type, method, args)
}

func generatedPlatformTypeName(typeName string) bool {
	_, ok := generatedPlatformTypes()[strings.ToLower(typeName)]
	return ok
}

func managedPassiveReturnForMethod(typeName, method string, args []Value) (Value, bool, error) {
	name := strings.ToLower(apexMethodMemberName(method))
	switch {
	case strings.EqualFold(name, "enter"):
		return Null, true, nil
	case strings.HasPrefix(name, "with"):
		return Object(typeName), true, nil
	case strings.HasPrefix(name, "setmock"):
		return Null, true, nil
	case (strings.EqualFold(name, "getbyid") || strings.EqualFold(name, "getbyids")) && len(args) != 0:
		return List(), true, nil
	case strings.EqualFold(name, "getisocode"):
		return String(""), true, nil
	case strings.HasPrefix(name, "get") && strings.HasSuffix(name, "id"):
		return platformScalar("Id", "001000000000001AAA"), true, nil
	case strings.HasPrefix(name, "get") && strings.HasSuffix(name, "quantity"):
		return Int(0), true, nil
	case strings.HasPrefix(name, "is") || strings.HasPrefix(name, "has") || strings.HasPrefix(name, "can") || strings.HasPrefix(name, "should"):
		return Bool(false), true, nil
	default:
		return Null, false, nil
	}
}

func managedPassiveFieldDefault(field string) Value {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "requests", "responses", "items", "records", "results", "errors":
		return List()
	default:
		return Null
	}
}

func managedAPIVersionToken(version string) bool {
	if len(version) < 2 || version[0] != 'v' && version[0] != 'V' {
		return false
	}
	for _, r := range version[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func managedAPIVersionName(version string) string {
	if version == "" {
		return ""
	}
	return "V" + version[1:]
}

func managedAPIVersionedTypeAssignable(source, target string) bool {
	sourceNamespace, sourceLocal, ok := strings.Cut(source, ".")
	if !ok {
		return false
	}
	targetNamespace, targetLocal, ok := strings.Cut(target, ".")
	if !ok || !strings.EqualFold(sourceNamespace, targetNamespace) {
		return false
	}
	return strings.EqualFold(stripManagedAPIVersionToken(sourceLocal), stripManagedAPIVersionToken(targetLocal))
}

func stripManagedAPIVersionToken(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] != 'V' && name[i] != 'v' {
			continue
		}
		j := i + 1
		for j < len(name) && name[j] >= '0' && name[j] <= '9' {
			j++
		}
		if j > i+1 {
			return name[:i] + name[j:]
		}
	}
	return name
}

func exportedMemberName(name string) string {
	name = apexMethodMemberName(name)
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func looksManagedQualifiedType(typeName string) bool {
	namespace, local, ok := strings.Cut(typeName, ".")
	if !ok || strings.TrimSpace(namespace) == "" || strings.TrimSpace(local) == "" {
		return false
	}
	switch strings.ToLower(namespace) {
	case "apex", "apexpages", "auth", "cache", "canvas", "component", "connectapi",
		"database", "dom", "eventbus", "metadata", "messaging", "process", "quickaction",
		"schema", "search", "site", "support", "system", "test", "userprovisioning",
		"visualeditor", "wave":
		return false
	default:
		return true
	}
}

func objectBoolField(object Value, name string) bool {
	_, value, ok := objectFieldValue(object, name)
	return ok && value.Kind == ValueBool && value.Bool
}

func valuesFromMap(value Value) []Value {
	if value.Kind != ValueMap {
		return nil
	}
	out := make([]Value, 0, len(value.Map))
	for _, key := range orderedValueMapKeys(value) {
		out = append(out, value.Map[key])
	}
	return out
}

func triggerBool(values map[string]Value, name string) bool {
	if values == nil {
		return false
	}
	if value, ok := values[name]; ok && value.Kind == ValueBool {
		return value.Bool
	}
	for candidate, value := range values {
		if strings.EqualFold(candidate, name) && value.Kind == ValueBool {
			return value.Bool
		}
	}
	return false
}

func isArgumentCaptorAnyObjectType(typeName string) bool {
	name := strings.ToLower(strings.TrimSpace(typeName))
	return strings.HasSuffix(name, "argumentcaptor.anyobject")
}

func (vm *VM) isSObjectUnitOfWorkRuntimeType(typeName string) bool {
	if vm.isSObjectUnitOfWorkBaseType(typeName) {
		return true
	}
	for current := typeName; current != ""; {
		class, ok := vm.Classes[current]
		if !ok {
			return false
		}
		if vm.isSObjectUnitOfWorkBaseType(class.SuperClass) {
			return true
		}
		current = class.SuperClass
	}
	return false
}

func (vm *VM) isSObjectUnitOfWorkBaseType(typeName string) bool {
	if strings.EqualFold(typeName, "framework_SObjectUnitOfWork") {
		return true
	}
	return strings.EqualFold(typeName, "SObjectUnitOfWork") || hasSuffixFold(typeName, "_sobjectunitofwork")
}

func (vm *VM) resolveObjectBucketKey(bucket Value, objectName string) string {
	key := mapKey(String(objectName))
	if _, ok := bucket.Map[key]; ok {
		return key
	}
	normalizedWanted := strings.ToLower(stripAnyNamespaceToken(objectName))
	for existingKey, existingValue := range bucket.MapKeys {
		if existingValue.Kind != ValueString {
			continue
		}
		if strings.EqualFold(existingValue.Text, objectName) {
			return existingKey
		}
		if strings.EqualFold(stripAnyNamespaceToken(existingValue.Text), normalizedWanted) {
			return existingKey
		}
	}
	return key
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

type frameworkSObjectUnitOfWorkRecordBucket struct {
	objectName string
	records    Value
}

func (vm *VM) callStackHasFieldInitializerForType(typeName string) bool {
	typeName = strings.ToLower(typeName)
	for _, frame := range vm.callStack {
		symbol := strings.ToLower(frame.Symbol)
		if strings.HasPrefix(symbol, typeName+".<field_init>.") {
			return true
		}
	}
	return false
}

func (vm *VM) callStackHasFieldInitializer() bool {
	for _, frame := range vm.callStack {
		if strings.Contains(strings.ToLower(frame.Symbol), ".<field_init>.") {
			return true
		}
	}
	return false
}

func isSelectorMockType(typeName string) bool {
	typeName = strings.ToLower(typeName)
	if strings.Contains(typeName, "selector") {
		return true
	}
	return strings.Contains(typeName, "__sfdc_apexstub") && strings.Contains(typeName, "selector")
}

func stubReturnCanUseReceiver(returnType string, receiver Value) bool {
	if receiver.Kind != ValueObject || strings.TrimSpace(returnType) == "" || strings.EqualFold(returnType, "void") {
		return false
	}
	return strings.EqualFold(returnType, receiver.Type)
}

func stubCollectionDefaultValue(returnType string) (Value, bool) {
	base := collectionBase(returnType)
	switch base {
	case "List":
		value := List()
		value.Type = returnType
		return value, true
	case "Set":
		value := Set()
		value.Type = returnType
		return value, true
	case "Map":
		value := typedMap(returnType)
		return value, true
	default:
		return Null, false
	}
}

func namespaceTokenInSchemaName(name string) string {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
}

// callListValueMember dispatches a method call on a List receiver. It returns
// handled=false when the method is not a List member so callNonObjectValueMember
// continues with the shared fall-through handling.
func (vm *VM) callListValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	switch method {
	case "getSObjectType":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.getSObjectType expects 0 arguments")
		}
		if declaredType := vm.VarTypes[receiverName]; declaredType != "" {
			if elementType, ok := collectionElementType(declaredType); ok && !strings.EqualFold(elementType, "sObject") {
				if token, ok := vm.sObjectTypeTokenForName(elementType); ok {
					return token, true, nil
				}
			}
		}
		if elementType, ok := collectionElementType(receiver.Static); ok && !strings.EqualFold(elementType, "sObject") {
			if token, ok := vm.sObjectTypeTokenForName(elementType); ok {
				return token, true, nil
			}
		}
		if elementType, ok := collectionElementType(receiver.Runtime); ok && !strings.EqualFold(elementType, "sObject") {
			if token, ok := vm.sObjectTypeTokenForName(elementType); ok {
				return token, true, nil
			}
		}
		if elementType, ok := collectionElementType(receiver.Type); ok && !strings.EqualFold(elementType, "sObject") {
			if token, ok := vm.sObjectTypeTokenForName(elementType); ok {
				return token, true, nil
			}
		}
		for _, item := range receiver.List {
			if item.Kind == ValueObject {
				for _, typeName := range []string{item.Type, item.Runtime, item.Static} {
					if strings.EqualFold(typeName, "SObject") || strings.EqualFold(typeName, "Object") {
						continue
					}
					if vm.isSObjectLikeType(typeName) {
						if token, ok := vm.sObjectTypeTokenForName(typeName); ok {
							return token, true, nil
						}
					}
				}
			}
		}
		if elementType, ok := collectionElementType(receiver.Type); ok && strings.EqualFold(elementType, "sObject") {
			return Null, true, nil
		}
		if len(receiver.List) == 0 {
			return Null, true, nil
		}
		if token, ok := vm.sObjectTypeTokenForName("SObject"); ok {
			return token, true, nil
		}
		return Null, true, fmt.Errorf("List.getSObjectType requires SObject list")
	case "add":
		if len(args) != 1 && len(args) != 2 {
			return Null, true, fmt.Errorf("List.add expects 1 or 2 arguments")
		}
		previous := snapshotAlias(receiver)
		valueArg := args[0]
		insertAt := -1
		if len(args) == 2 {
			if args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.add index expects Integer")
			}
			insertAt = int(args[0].Int)
			if insertAt < 0 || insertAt > len(receiver.List) {
				return Null, true, listIndexException(insertAt)
			}
			valueArg = args[1]
		}
		mutationType := vm.collectionMutationType(receiver)
		item, err := vm.coerceCollectionElement(mutationType, valueArg)
		if err != nil {
			return Null, true, collectionStoreException(mutationType, valueArg)
		}
		vm.markCollectionRefsEscaped(item)
		if insertAt >= 0 {
			receiver.List = append(receiver.List, Null)
			copy(receiver.List[insertAt+1:], receiver.List[insertAt:])
			receiver.List[insertAt] = item
		} else {
			receiver.List = append(receiver.List, item)
		}
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		if insertAt >= 0 {
			return Null, true, nil
		}
		return Bool(true), true, nil
	case "addAll":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("List.addAll expects List, Set, or Iterable")
		}
		values, err := vm.iterableCollectionMembers(args[0], result, "List.addAll")
		if err != nil {
			return Null, true, err
		}
		previous := snapshotAlias(receiver)
		mutationType := vm.collectionMutationType(receiver)
		for _, value := range values {
			item, err := vm.coerceCollectionElement(mutationType, value)
			if err != nil {
				return Null, true, collectionStoreException(mutationType, value)
			}
			vm.markCollectionRefsEscaped(item)
			receiver.List = append(receiver.List, item)
		}
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		return Null, true, nil
	case "addToRelationship":
		updated, err := listAppendSObjects(receiver, "__glade_added_to_relationship", args, "List.addToRelationship")
		if err != nil {
			return Null, true, err
		}
		if err := vm.storeReceiver(receiverName, updated); err != nil {
			return Null, true, err
		}
		return Null, true, nil
	case "markForDelete":
		updated, err := listAppendSObjects(receiver, "__glade_marked_for_delete", args, "List.markForDelete")
		if err != nil {
			return Null, true, err
		}
		if err := vm.storeReceiver(receiverName, updated); err != nil {
			return Null, true, err
		}
		return Null, true, nil
	case "getAddedToRelationship":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.getAddedToRelationship expects 0 arguments")
		}
		return listRelationshipValues(receiver, "__glade_added_to_relationship"), true, nil
	case "getMarkedForDeletion":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.getMarkedForDeletion expects 0 arguments")
		}
		return listRelationshipValues(receiver, "__glade_marked_for_delete"), true, nil
	case "size":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.size expects 0 arguments")
		}
		return Int(int64(len(receiver.List))), true, nil
	case "isEmpty":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.isEmpty expects 0 arguments")
		}
		return Bool(len(receiver.List) == 0), true, nil
	case "get":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, true, fmt.Errorf("List.get expects integer index")
		}
		i := int(args[0].Int)
		if i < 0 || i >= len(receiver.List) {
			return Null, true, listIndexException(i)
		}
		return receiver.List[i], true, nil
	case "contains":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("List.contains expects 1 argument")
		}
		contains, err := vm.collectionContainsValue(receiver.List, args[0], result)
		if err != nil {
			return Null, true, err
		}
		return Bool(contains), true, nil
	case "indexOf":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("List.indexOf expects 1 argument")
		}
		i, err := vm.collectionIndexOfValue(receiver.List, args[0], result)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(i)), true, nil
	case "clone":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.clone expects 0 arguments")
		}
		cloned := receiver
		cloned.Ref = newValueRef()
		cloned.List = append([]Value(nil), receiver.List...)
		return cloned, true, nil
	case "deepClone":
		if len(args) > 3 {
			return Null, true, fmt.Errorf("List.deepClone expects at most 3 Boolean arguments")
		}
		for _, arg := range args {
			if arg.Kind != ValueBool {
				return Null, true, fmt.Errorf("List.deepClone preserve options expect Boolean arguments")
			}
		}
		return cloneValue(receiver), true, nil
	case "iterator":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.iterator expects 0 arguments")
		}
		return collectionIterator(receiver), true, nil
	case "sort":
		if len(args) > 1 {
			return Null, true, fmt.Errorf("List.sort expects 0 or 1 arguments")
		}
		previous := snapshotAlias(receiver)
		sorted := append([]Value(nil), receiver.List...)
		if len(args) == 0 {
			if err := vm.sortComparableValues(sorted, result); err != nil {
				return Null, true, err
			}
		} else {
			if err := vm.sortValuesWithComparator(sorted, args[0], result); err != nil {
				return Null, true, err
			}
		}
		receiver.List = sorted
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		return Null, true, nil
	case "remove":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, true, fmt.Errorf("List.remove expects integer index")
		}
		previous := snapshotAlias(receiver)
		i := int(args[0].Int)
		if i < 0 || i >= len(receiver.List) {
			return Null, true, listIndexException(i)
		}
		removed := receiver.List[i]
		receiver.List = append(receiver.List[:i], receiver.List[i+1:]...)
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		return removed, true, nil
	case "clear":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("List.clear expects 0 arguments")
		}
		previous := snapshotAlias(receiver)
		receiver.List = nil
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		return Null, true, nil
	case "set":
		if len(args) != 2 || args[0].Kind != ValueInt {
			return Null, true, fmt.Errorf("List.set expects integer index and value")
		}
		previous := snapshotAlias(receiver)
		i := int(args[0].Int)
		if i < 0 || i >= len(receiver.List) {
			return Null, true, listIndexException(i)
		}
		item, err := vm.coerceCollectionElement(receiver.Type, args[1])
		if err != nil {
			return Null, true, fmt.Errorf("List.set: %w", err)
		}
		vm.markCollectionRefsEscaped(item)
		receiver.List[i] = item
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		return Null, true, nil
	}
	return Null, false, nil
}

// callSetValueMember dispatches a method call on a Set receiver. It returns
// handled=false when the method is not a Set member so callNonObjectValueMember
// continues with the shared fall-through handling.
func (vm *VM) callSetValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	switch method {
	case "get":
		if receiverName == "disabledTriggers" {
			if len(args) != 1 {
				return Null, true, fmt.Errorf("Set-backed disabledTriggers.get expects 1 argument")
			}
			contains, err := vm.collectionContainsValue(receiver.Set, args[0], result)
			if err != nil {
				return Null, true, err
			}
			if contains {
				return Int(1), true, nil
			}
			return Int(0), true, nil
		}
	case "add":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Set.add expects 1 argument")
		}
		previous := snapshotAlias(receiver)
		item, err := vm.coerceCollectionElement(receiver.Type, args[0])
		if err != nil {
			return Null, true, fmt.Errorf("Set.add: %w", err)
		}
		vm.markCollectionRefsEscaped(item)
		contains, err := vm.collectionContainsValue(receiver.Set, item, result)
		if err != nil {
			return Null, true, err
		}
		if !contains {
			receiver.Set = append(receiver.Set, item)
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutationFromSnapshot(previous, receiver)
			return Bool(true), true, nil
		}
		return Bool(false), true, nil
	case "addAll":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Set.addAll expects List, Set, or Iterable")
		}
		values, err := vm.iterableCollectionMembers(args[0], result, "Set.addAll")
		if err != nil {
			return Null, true, err
		}
		previous := snapshotAlias(receiver)
		changed := false
		for _, value := range values {
			item, err := vm.coerceCollectionElement(receiver.Type, value)
			if err != nil {
				return Null, true, fmt.Errorf("Set.addAll: %w", err)
			}
			vm.markCollectionRefsEscaped(item)
			contains, err := vm.collectionContainsValue(receiver.Set, item, result)
			if err != nil {
				return Null, true, err
			}
			if !contains {
				receiver.Set = append(receiver.Set, item)
				changed = true
			}
		}
		if changed {
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutationFromSnapshot(previous, receiver)
		}
		return Bool(changed), true, nil
	case "size":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Set.size expects 0 arguments")
		}
		return Int(int64(len(receiver.Set))), true, nil
	case "isEmpty":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Set.isEmpty expects 0 arguments")
		}
		return Bool(len(receiver.Set) == 0), true, nil
	case "contains":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Set.contains expects 1 argument")
		}
		if contains, ok := vm.populatedFieldsKeySetContains(receiver, args[0]); ok {
			return Bool(contains), true, nil
		}
		contains, err := vm.collectionContainsValue(receiver.Set, args[0], result)
		if err != nil {
			return Null, true, err
		}
		return Bool(contains), true, nil
	case "containsAll":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
			return Null, true, fmt.Errorf("Set.containsAll expects List or Set")
		}
		for _, value := range collectionMembers(args[0]) {
			contains, err := vm.collectionContainsValue(receiver.Set, value, result)
			if err != nil {
				return Null, true, err
			}
			if !contains {
				return Bool(false), true, nil
			}
		}
		return Bool(true), true, nil
	case "remove":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Set.remove expects 1 argument")
		}
		i, err := vm.collectionIndexOfValue(receiver.Set, args[0], result)
		if err != nil {
			return Null, true, err
		}
		if i >= 0 {
			receiver.Set = append(receiver.Set[:i], receiver.Set[i+1:]...)
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			return Bool(true), true, nil
		}
		return Bool(false), true, nil
	case "clear":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Set.clear expects 0 arguments")
		}
		receiver.Set = nil
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		return Null, true, nil
	case "removeAll":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
			return Null, true, fmt.Errorf("Set.removeAll expects List or Set")
		}
		changed := false
		out := receiver.Set[:0]
		remove := collectionMembers(args[0])
		for _, value := range receiver.Set {
			contains, err := vm.collectionContainsValue(remove, value, result)
			if err != nil {
				return Null, true, err
			}
			if contains {
				changed = true
				continue
			}
			out = append(out, value)
		}
		receiver.Set = out
		if changed {
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
		}
		return Bool(changed), true, nil
	case "retainAll":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueSet) {
			return Null, true, fmt.Errorf("Set.retainAll expects List or Set")
		}
		changed := false
		keep := collectionMembers(args[0])
		out := receiver.Set[:0]
		for _, value := range receiver.Set {
			contains, err := vm.collectionContainsValue(keep, value, result)
			if err != nil {
				return Null, true, err
			}
			if contains {
				out = append(out, value)
				continue
			}
			changed = true
		}
		receiver.Set = out
		if changed {
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
		}
		return Bool(changed), true, nil
	case "clone":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Set.clone expects 0 arguments")
		}
		cloned := receiver
		cloned.Ref = newValueRef()
		cloned.Set = append([]Value(nil), receiver.Set...)
		return cloned, true, nil
	case "deepClone":
		if len(args) != 0 {
			return Null, true, unsupportedCallError("Set.deepClone with preserve options")
		}
		return cloneValue(receiver), true, nil
	case "iterator":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Set.iterator expects 0 arguments")
		}
		return collectionIterator(receiver), true, nil
	case "put":
		if receiverName == "disabledTriggers" {
			if len(args) != 2 {
				return Null, true, fmt.Errorf("Set-backed disabledTriggers.put expects 2 arguments")
			}
			if args[1].Kind != ValueInt {
				return Null, true, fmt.Errorf("Set-backed disabledTriggers.put expects Integer counter")
			}
			contains, err := vm.collectionContainsValue(receiver.Set, args[0], result)
			if err != nil {
				return Null, true, err
			}
			old := Int(0)
			if contains {
				old = Int(1)
			}
			if args[1].Int > 0 {
				if !contains {
					receiver.Set = append(receiver.Set, args[0])
					if err := vm.storeReceiver(receiverName, receiver); err != nil {
						return Null, true, err
					}
				}
			} else if contains {
				i, err := vm.collectionIndexOfValue(receiver.Set, args[0], result)
				if err != nil {
					return Null, true, err
				}
				if i >= 0 {
					receiver.Set = append(receiver.Set[:i], receiver.Set[i+1:]...)
					if err := vm.storeReceiver(receiverName, receiver); err != nil {
						return Null, true, err
					}
				}
			}
			return old, true, nil
		}
	}
	return Null, false, nil
}

// callMapValueMember dispatches a method call on a Map receiver. It returns
// handled=false when the method is not a Map member so callNonObjectValueMember
// continues with the shared fall-through handling.
func (vm *VM) callMapValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	switch method {
	case "put":
		if len(args) != 2 {
			return Null, true, fmt.Errorf("Map.put expects 2 arguments")
		}
		previousReceiver := snapshotAlias(receiver)
		key, item, err := vm.coerceMapEntry(receiver.Type, args[0], args[1])
		if err != nil {
			return Null, true, fmt.Errorf("Map.put: %w", err)
		}
		vm.markCollectionRefsEscaped(key, item)
		previous := Null
		encodedKey := vm.mapKey(key)
		if existing, ok := receiver.Map[encodedKey]; ok {
			previous = existing
		} else {
			receiver.MapOrder = append(receiver.MapOrder, encodedKey)
		}
		receiver.Map[encodedKey] = item
		if receiver.MapKeys == nil {
			receiver.MapKeys = make(map[string]Value)
		}
		receiver.MapKeys[encodedKey] = key
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previousReceiver, receiver)
		return previous, true, nil
	case "putAll":
		if len(args) != 1 || (args[0].Kind != ValueMap && args[0].Kind != ValueList) {
			return Null, true, fmt.Errorf("Map.putAll expects Map or List")
		}
		previousReceiver := snapshotAlias(receiver)
		if args[0].Kind == ValueList {
			updated, err := vm.putAllSObjectList(receiver, args[0])
			if err != nil {
				return Null, true, err
			}
			if err := vm.storeReceiver(receiverName, updated); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutationFromSnapshot(previousReceiver, updated)
			return Null, true, nil
		}
		for _, rawKey := range orderedValueMapKeys(args[0]) {
			value := args[0].Map[rawKey]
			keyValue := mapStoredKey(args[0], rawKey)
			key, item, err := vm.coerceMapEntry(receiver.Type, keyValue, value)
			if err != nil {
				return Null, true, fmt.Errorf("Map.putAll: %w", err)
			}
			vm.markCollectionRefsEscaped(key, item)
			encodedKey := vm.mapKey(key)
			if _, exists := receiver.Map[encodedKey]; !exists {
				receiver.MapOrder = append(receiver.MapOrder, encodedKey)
			}
			receiver.Map[encodedKey] = item
			if receiver.MapKeys == nil {
				receiver.MapKeys = make(map[string]Value)
			}
			receiver.MapKeys[encodedKey] = key
		}
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		vm.propagateCollectionMutationFromSnapshot(previousReceiver, receiver)
		return Null, true, nil
	case "get":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Map.get expects 1 argument")
		}
		key := vm.mapLookupKey(receiver, args[0])
		if objectName, ok := sObjectFieldMapObjectName(receiver); ok && args[0].Kind == ValueString && vm.sObjectFieldMapKeyIsChildRelationship(objectName, args[0].Text) && !vm.sObjectFieldMapDirectValueMatchesKey(receiver, key, args[0].Text) {
			return missingMapValue(receiver), true, nil
		}
		value, ok := receiver.Map[key]
		if !ok {
			value, ok = vm.namespaceStringMapLookup(receiver, args[0])
		}
		if !ok {
			value, ok = vm.populatedFieldsMapAliasLookup(receiver, args[0])
		}
		if !ok {
			value, ok = vm.specialMapLookup(receiver, args[0])
		}
		if !ok {
			var err error
			value, ok, err = vm.objectKeyMapLookup(receiver, args[0])
			if err != nil {
				return Null, true, err
			}
		}
		if !ok {
			return missingMapValue(receiver), true, nil
		}
		if value.Kind == ValueNull && value.Type == "" {
			value = missingMapValue(receiver)
		}
		return value, true, nil
	case "contains", "containsKey":
		if len(args) != 1 {
			if method == "contains" {
				return Null, true, fmt.Errorf("Map.contains expects 1 argument")
			}
			return Null, true, fmt.Errorf("Map.containsKey expects 1 argument")
		}
		key := vm.mapLookupKey(receiver, args[0])
		if objectName, ok := sObjectFieldMapObjectName(receiver); ok && args[0].Kind == ValueString && vm.sObjectFieldMapKeyIsChildRelationship(objectName, args[0].Text) && !vm.sObjectFieldMapDirectValueMatchesKey(receiver, key, args[0].Text) {
			return Bool(false), true, nil
		}
		_, ok := receiver.Map[key]
		if !ok {
			_, ok = vm.namespaceStringMapLookup(receiver, args[0])
		}
		if !ok {
			ok = vm.specialMapContainsKey(receiver, args[0])
		}
		if !ok {
			var err error
			_, ok, err = vm.objectKeyMapLookup(receiver, args[0])
			if err != nil {
				return Null, true, err
			}
		}
		return Bool(ok), true, nil
	case "containsValue":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Map.containsValue expects 1 argument")
		}
		for _, value := range receiver.Map {
			if value.Equal(args[0]) {
				return Bool(true), true, nil
			}
		}
		return Bool(false), true, nil
	case "remove":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Map.remove expects 1 argument")
		}
		previousReceiver := snapshotAlias(receiver)
		key := vm.mapLookupKey(receiver, args[0])
		removed := Null
		if value, ok := receiver.Map[key]; ok {
			removed = value
			delete(receiver.Map, key)
			delete(receiver.MapKeys, key)
			if len(receiver.MapOrder) > 0 {
				filtered := receiver.MapOrder[:0]
				for _, orderedKey := range receiver.MapOrder {
					if orderedKey != key {
						filtered = append(filtered, orderedKey)
					}
				}
				receiver.MapOrder = filtered
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutationFromSnapshot(previousReceiver, receiver)
		}
		return removed, true, nil
	case "keySet":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Map.keySet expects 0 arguments")
		}
		if out, ok := vm.sObjectFieldMapCanonicalKeySet(receiver); ok {
			return out, true, nil
		}
		out := Set()
		for _, rawKey := range orderedValueMapKeys(receiver) {
			out.Set = append(out.Set, mapStoredKey(receiver, rawKey))
		}
		if strings.HasPrefix(receiver.Runtime, "sobject-populated-fields:") {
			out.Runtime = receiver.Runtime + ":keyset"
		}
		if keyType, _, ok := mapTypeArgs(receiver.Type); ok {
			out.Type = "Set<" + keyType + ">"
		}
		return out, true, nil
	case "values":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Map.values expects 0 arguments")
		}
		if out, ok := sObjectFieldMapCanonicalValues(receiver); ok {
			return out, true, nil
		}
		out := List()
		for _, key := range orderedValueMapKeys(receiver) {
			out.List = append(out.List, receiver.Map[key])
		}
		if _, valueType, ok := mapTypeArgs(receiver.Type); ok {
			out.Type = "List<" + valueType + ">"
		}
		return out, true, nil
	case "size":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Map.size expects 0 arguments")
		}
		if size, ok := sObjectFieldMapCanonicalSize(receiver); ok {
			return Int(int64(size)), true, nil
		}
		return Int(int64(len(receiver.Map))), true, nil
	case "isEmpty":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Map.isEmpty expects 0 arguments")
		}
		if size, ok := sObjectFieldMapCanonicalSize(receiver); ok {
			return Bool(size == 0), true, nil
		}
		return Bool(len(receiver.Map) == 0), true, nil
	case "clear":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Map.clear expects 0 arguments")
		}
		receiver.Map = map[string]Value{}
		receiver.MapKeys = map[string]Value{}
		receiver.MapOrder = nil
		if err := vm.storeReceiver(receiverName, receiver); err != nil {
			return Null, true, err
		}
		return Null, true, nil
	case "clone":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Map.clone expects 0 arguments")
		}
		cloned := receiver
		cloned.Ref = newValueRef()
		cloned.Map = make(map[string]Value, len(receiver.Map))
		for key, value := range receiver.Map {
			cloned.Map[key] = value
		}
		if receiver.MapKeys != nil {
			cloned.MapKeys = make(map[string]Value, len(receiver.MapKeys))
			for key, value := range receiver.MapKeys {
				cloned.MapKeys[key] = value
			}
		}
		if receiver.MapOrder != nil {
			cloned.MapOrder = append([]string(nil), receiver.MapOrder...)
		}
		return cloned, true, nil
	case "deepClone":
		if len(args) != 0 {
			return Null, true, unsupportedCallError("Map.deepClone with preserve options")
		}
		return cloneValue(receiver), true, nil
	}
	return Null, false, nil
}
