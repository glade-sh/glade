package vm

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/glade-sh/glade/internal/dml"
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

func (vm *VM) callFrameworkIDGeneratorGenerate(method Method, args []Value) (Value, bool) {
	if !strings.EqualFold(frameworkMockSupportType(method.ClassName), "IDGenerator") ||
		!strings.EqualFold(apexMethodMemberName(method.Name), "generate") ||
		len(args) != 1 || !isSObjectTypeToken(args[0]) {
		return Null, false
	}
	objectName, ok := sObjectTypeTokenObjectName(args[0])
	if !ok {
		return Null, false
	}
	if strings.EqualFold(objectName, "Organization") {
		return platformScalar("Id", "00D000000000002"), true
	}
	if strings.EqualFold(objectName, "User") {
		return platformScalar("Id", "005000000000999"), true
	}
	return Null, false
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
	resolutionClass := vm.methodTypeResolutionClass(method)
	for i, param := range method.Params {
		paramType := vm.resolveTypeNameInClass(resolutionClass, param.Type)
		paramOriginals[param.Name] = args[i]
		coerced, err := vm.coerceAssignable(paramType, vm.valueWithTypesResolvedInClass(resolutionClass, args[i]))
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
	if methodHasModifier(method.Modifiers, "passive-generated") {
		className := method.ClassName
		if className == "" {
			className = receiver.Type
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
		value := vm.passiveGeneratedMethodReturn(method, frame, receiver)
		for _, param := range method.Params {
			updated, ok := frame[param.Name]
			if !ok {
				continue
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
				if vm.propagateAliasSnapshotMutationToScope(vm.Globals, receiverSnapshot, receiverOriginal, updated, vm.collectionMutationSeq != collectionMutationSeqBefore) {
					vm.propagateAliasSnapshotToStatics(receiverSnapshot, updated)
				}
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
	appendTrace(result, "apex.method."+method.Name, "apex.method", map[string]any{
		"method": method.Name,
		"class":  method.ClassName,
		"file":   method.File,
		"line":   method.Line,
		"column": method.Column,
	})
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
			if vm.propagateAliasSnapshotMutationToScope(caller, receiverSnapshot, receiverOriginal, updated, vm.collectionMutationSeq != collectionMutationSeqBefore) {
				vm.propagateAliasSnapshotToStatics(receiverSnapshot, updated)
			}
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

func (vm *VM) callValueMember(receiverName string, receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if dataWeaveStaticScriptReceiver(receiverName) && strings.EqualFold(method, "createScript") {
		if len(args) == 1 && args[0].Kind == ValueString {
			return newDataWeaveScript(args[0].Text), true, nil
		}
		if len(args) == 2 && args[1].Kind == ValueString {
			return newDataWeaveScript(args[1].Text), true, nil
		}
		return Null, true, fmt.Errorf("DataWeave.Script.createScript expects script name String")
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
				vm.currentPage = newPageReference("/apex/current")
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
		if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, method, args, result); handled || err != nil {
			if mutated {
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
			}
			return value, true, err
		}
		if _, classExists := vm.lookupClass(receiver.Type); classExists {
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

	switch receiver.Kind {
	case ValueList:
		method = canonicalCollectionMemberName("List", method)
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
				if item.Kind == ValueObject && vm.isSObjectLikeType(item.Type) {
					if token, ok := vm.sObjectTypeTokenForName(item.Type); ok {
						return token, true, nil
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
			previous := receiver
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
			vm.propagateCollectionMutation(previous, receiver)
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
			previous := receiver
			mutationType := vm.collectionMutationType(receiver)
			for _, value := range values {
				item, err := vm.coerceCollectionElement(mutationType, value)
				if err != nil {
					return Null, true, collectionStoreException(mutationType, value)
				}
				receiver.List = append(receiver.List, item)
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutation(previous, receiver)
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
			previous := receiver
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
			vm.propagateCollectionMutation(previous, receiver)
			return Null, true, nil
		case "remove":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.remove expects integer index")
			}
			previous := receiver
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, listIndexException(i)
			}
			removed := receiver.List[i]
			receiver.List = append(receiver.List[:i], receiver.List[i+1:]...)
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutation(previous, receiver)
			return removed, true, nil
		case "clear":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("List.clear expects 0 arguments")
			}
			previous := receiver
			receiver.List = nil
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutation(previous, receiver)
			return Null, true, nil
		case "set":
			if len(args) != 2 || args[0].Kind != ValueInt {
				return Null, true, fmt.Errorf("List.set expects integer index and value")
			}
			previous := receiver
			i := int(args[0].Int)
			if i < 0 || i >= len(receiver.List) {
				return Null, true, listIndexException(i)
			}
			item, err := vm.coerceCollectionElement(receiver.Type, args[1])
			if err != nil {
				return Null, true, fmt.Errorf("List.set: %w", err)
			}
			receiver.List[i] = item
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
			vm.propagateCollectionMutation(previous, receiver)
			return Null, true, nil
		}
	case ValueSet:
		method = canonicalCollectionMemberName("Set", method)
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
			previous := receiver
			item, err := vm.coerceCollectionElement(receiver.Type, args[0])
			if err != nil {
				return Null, true, fmt.Errorf("Set.add: %w", err)
			}
			contains, err := vm.collectionContainsValue(receiver.Set, item, result)
			if err != nil {
				return Null, true, err
			}
			if !contains {
				receiver.Set = append(receiver.Set, item)
				if err := vm.storeReceiver(receiverName, receiver); err != nil {
					return Null, true, err
				}
				vm.propagateCollectionMutation(previous, receiver)
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
			previous := receiver
			changed := false
			for _, value := range values {
				item, err := vm.coerceCollectionElement(receiver.Type, value)
				if err != nil {
					return Null, true, fmt.Errorf("Set.addAll: %w", err)
				}
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
				vm.propagateCollectionMutation(previous, receiver)
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
	case ValueMap:
		method = canonicalCollectionMemberName("Map", method)
		switch method {
		case "put":
			if len(args) != 2 {
				return Null, true, fmt.Errorf("Map.put expects 2 arguments")
			}
			key, item, err := vm.coerceMapEntry(receiver.Type, args[0], args[1])
			if err != nil {
				return Null, true, fmt.Errorf("Map.put: %w", err)
			}
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
			return previous, true, nil
		case "putAll":
			if len(args) != 1 || (args[0].Kind != ValueMap && args[0].Kind != ValueList) {
				return Null, true, fmt.Errorf("Map.putAll expects Map or List")
			}
			if args[0].Kind == ValueList {
				updated, err := vm.putAllSObjectList(receiver, args[0])
				if err != nil {
					return Null, true, err
				}
				if err := vm.storeReceiver(receiverName, updated); err != nil {
					return Null, true, err
				}
				return Null, true, nil
			}
			for rawKey, value := range args[0].Map {
				keyValue := mapStoredKey(args[0], rawKey)
				key, item, err := vm.coerceMapEntry(receiver.Type, keyValue, value)
				if err != nil {
					return Null, true, fmt.Errorf("Map.putAll: %w", err)
				}
				encodedKey := vm.mapKey(key)
				receiver.Map[encodedKey] = item
				if receiver.MapKeys == nil {
					receiver.MapKeys = make(map[string]Value)
				}
				receiver.MapKeys[encodedKey] = key
			}
			if err := vm.storeReceiver(receiverName, receiver); err != nil {
				return Null, true, err
			}
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

func listRelationshipSObjectValue(value Value) bool {
	return strings.EqualFold(value.Type, "SObject") ||
		isCommonSObjectTypeName(value.Type) ||
		strings.HasSuffix(value.Type, "__c") ||
		strings.HasSuffix(value.Type, "__e") ||
		strings.HasSuffix(value.Type, "__mdt")
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
		"isPlaceholder", "isRequired", "isSuccess", "setContextId", "setIgnoreTemplateSubject",
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

func (vm *VM) userClassShadowsSObjectType(typeName string) bool {
	class, ok := vm.lookupClass(typeName)
	if !ok || class.SuperClass == "" {
		return false
	}
	return !strings.EqualFold(class.SuperClass, "SObject")
}

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
					if lookupField, ok := customRelationshipLookupFieldName(alias); ok {
						if vm.canSynthesizeCustomSObjectFieldMapField(objectName, alias) {
							return vm.sObjectFieldToken(objectName, lookupField), true
						}
						continue
					}
					if isLikelyReferenceIDField(alias) || vm.canSynthesizeCustomSObjectFieldMapField(objectName, alias) {
						return vm.sObjectFieldToken(objectName, alias), true
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
		developerName, ok := value.Fields["developerName"]
		if ok && developerName.Kind == ValueString && strings.EqualFold(developerName.Text, key.Text) {
			return value, true
		}
	}
	return Null, false
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
		developerName, ok := value.Fields["developerName"]
		if ok && developerName.Kind == ValueString && strings.EqualFold(developerName.Text, key.Text) {
			return true
		}
	}
	return false
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

func (vm *VM) inferredCustomFieldReferenceTarget(objectName, fieldName string) string {
	if vm == nil || vm.Org == nil || !hasSuffixFold(fieldName, "__c") {
		return ""
	}
	candidates := []string{fieldName, stripAnyNamespaceToken(fieldName)}
	if strippedNumbered := stripTrailingDigitsFromCustomField(stripAnyNamespaceToken(fieldName)); strippedNumbered != "" {
		candidates = append(candidates, strippedNumbered)
		if namespace := namespaceFromSchemaToken(fieldName); namespace != "" {
			candidates = append(candidates, storage.NamespaceTokenName(namespace, strippedNumbered))
		}
		if namespace := namespaceFromSchemaToken(objectName); namespace != "" {
			candidates = append(candidates, storage.NamespaceTokenName(namespace, strippedNumbered))
		}
	}
	if namespace := namespaceFromSchemaToken(objectName); namespace != "" {
		candidates = append(candidates, storage.NamespaceTokenName(namespace, fieldName))
	}
	if vm.Org.Namespace != "" {
		candidates = append(candidates, storage.NamespaceTokenName(vm.Org.Namespace, fieldName))
	}
	for _, candidate := range candidates {
		if resolved, ok := vm.resolveObjectName(candidate); ok {
			return resolved
		}
	}
	return ""
}

func customRelationshipLookupFieldName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if dot := strings.LastIndex(name, "."); dot >= 0 && dot+1 < len(name) {
		name = name[dot+1:]
	}
	if !hasSuffixFold(name, "__r") {
		return "", false
	}
	return name[:len(name)-3] + "__c", true
}

func stripTrailingDigitsFromCustomField(fieldName string) string {
	if !hasSuffixFold(fieldName, "__c") {
		return ""
	}
	base := fieldName[:len(fieldName)-3]
	for len(base) > 0 {
		last := base[len(base)-1]
		if last < '0' || last > '9' {
			break
		}
		base = base[:len(base)-1]
	}
	if base == "" || base == fieldName[:len(fieldName)-3] {
		return ""
	}
	return base + "__c"
}

func isLikelyNumericCustomField(fieldName string) bool {
	if !hasSuffixFold(fieldName, "__c") {
		return false
	}
	base := strings.TrimSuffix(stripAnyNamespaceToken(fieldName), "__c")
	base = strings.ToLower(base)
	if base == "" {
		return false
	}
	numericTerms := []string{
		"amount", "balance", "cost", "count", "fee", "mrr", "payment", "price",
		"quantity", "shipping", "subtotal", "tax", "total",
	}
	for _, term := range numericTerms {
		if strings.Contains(base, term) {
			return true
		}
	}
	return false
}

func namespaceFromSchemaToken(name string) string {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
}

func (vm *VM) populatedFieldsMapAliasLookup(receiver, key Value) (Value, bool) {
	if receiver.Kind != ValueMap || key.Kind != ValueString || !strings.HasPrefix(receiver.Runtime, "sobject-populated-fields:") {
		return Null, false
	}
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if namespace == "" {
		return Null, false
	}
	for rawKey, value := range receiver.Map {
		candidate := valueFromMapKey(rawKey)
		if candidate.Kind == ValueString && schemaDescribeMapKeyMatches(namespace, candidate.Text, key.Text) {
			return value, true
		}
	}
	return Null, false
}

func populatedFieldsMapAllowsAliasContains(receiver Value) bool {
	if receiver.Kind != ValueMap || receiver.Fields == nil {
		return false
	}
	marker, ok := receiver.Fields[sobjectPopulatedFieldsAliasContainsField]
	return ok && marker.Kind == ValueBool && marker.Bool
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

func (vm *VM) sObjectFieldMapLookupAliases(fieldName string) []string {
	seen := map[string]bool{}
	var aliases []string
	add := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[alias] {
			return
		}
		seen[alias] = true
		aliases = append(aliases, alias)
		if lowered := strings.ToLower(alias); lowered != alias && !seen[lowered] {
			seen[lowered] = true
			aliases = append(aliases, lowered)
		}
	}
	add(fieldName)
	if dot := strings.LastIndex(fieldName, "."); dot >= 0 && dot+1 < len(fieldName) {
		add(fieldName[dot+1:])
	}
	if vm.Org != nil && vm.Org.Namespace != "" {
		for _, alias := range append([]string(nil), aliases...) {
			add(storage.NamespaceTokenName(vm.Org.Namespace, alias))
			add(storage.StripNamespaceToken(vm.Org.Namespace, alias))
		}
	}
	for _, alias := range append([]string(nil), aliases...) {
		add(stripAnyNamespaceToken(alias))
	}
	return aliases
}

func isLikelyReferenceIDField(fieldName string) bool {
	fieldName = strings.TrimSpace(fieldName)
	return !strings.Contains(fieldName, ".") && len(fieldName) > len("Id") && strings.HasSuffix(fieldName, "Id")
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
	return storage.ResolveKnownStandardObjectName(name)
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

func (vm *VM) propagateCollectionMutation(previous, updated Value) {
	if !sameCollectionType(previous, updated) {
		return
	}
	vm.collectionMutationSeq++
	vm.propagateValueMutationToScope(vm.Globals, previous, updated)
	vm.propagateValueMutationToStatics(previous, updated)
}

func (vm *VM) propagateCollectionMutationToScope(scope map[string]Value, previous, updated Value) {
	vm.propagateValueMutationToScope(scope, previous, updated)
}

func (vm *VM) propagateValueMutationToScope(scope map[string]Value, previous, updated Value) {
	if sameAliasValue(previous, updated) {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for name, value := range scope {
		clearRefSeen(seen)
		replaced, changed := replaceValueAlias(value, previous, updated, seen)
		if changed {
			scope[name] = replaced
		}
	}
}

var (
	aliasRefSliceMapPool = sync.Pool{New: func() any { m := make(map[uint64][]string, 8); return &m }}
	aliasRefSetPool      = sync.Pool{New: func() any { m := make(map[uint64]bool, 16); return &m }}
)

func (vm *VM) propagateUpdatedValueAliases(scope map[string]Value, updated Value) {
	if len(scope) == 0 {
		return
	}
	topLevelAliasesPtr := aliasRefSliceMapPool.Get().(*map[uint64][]string)
	topLevelAliases := *topLevelAliasesPtr
	clear(topLevelAliases)
	defer func() {
		// Drop large slices so the pool entry stays compact.
		for k, v := range topLevelAliases {
			if cap(v) > 16 {
				delete(topLevelAliases, k)
			}
		}
		aliasRefSliceMapPool.Put(topLevelAliasesPtr)
	}()
	for name, value := range scope {
		if value.Ref != 0 && value.Ref != updated.Ref {
			topLevelAliases[value.Ref] = append(topLevelAliases[value.Ref], name)
		}
	}
	if len(topLevelAliases) == 0 {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	var walk func(Value, bool)
	walk = func(value Value, root bool) {
		if value.Ref != 0 {
			if seen[value.Ref] {
				return
			}
			seen[value.Ref] = true
			if !root {
				for _, name := range topLevelAliases[value.Ref] {
					if merged, ok := mergeRicherApexMocksAlias(scope[name], value); ok {
						scope[name] = merged
						continue
					}
					scope[name] = value
				}
			}
		}
		switch value.Kind {
		case ValueObject:
			for _, child := range value.Fields {
				walk(child, false)
			}
		case ValueMap:
			for _, child := range value.Map {
				walk(child, false)
			}
			for _, child := range value.MapKeys {
				walk(child, false)
			}
		case ValueList:
			for _, child := range value.List {
				walk(child, false)
			}
		case ValueSet:
			for _, child := range value.Set {
				walk(child, false)
			}
		}
	}
	walk(updated, true)
}

func mergeRicherApexMocksAlias(existing, replacement Value) (Value, bool) {
	if existing.Kind != ValueObject || replacement.Kind != ValueObject || existing.Ref == 0 || existing.Ref != replacement.Ref {
		return Null, false
	}
	if !strings.EqualFold(frameworkMockSupportType(existing.Type), "ApexMocks") ||
		!strings.EqualFold(frameworkMockSupportType(replacement.Type), "ApexMocks") {
		return Null, false
	}
	if len(existing.Fields) <= len(replacement.Fields) {
		return Null, false
	}
	merged := existing
	if replacement.Type != "" {
		merged.Type = replacement.Type
	}
	if replacement.Static != "" {
		merged.Static = replacement.Static
	}
	if replacement.Runtime != "" {
		merged.Runtime = replacement.Runtime
	}
	if merged.Fields == nil {
		merged.Fields = make(map[string]Value, len(replacement.Fields))
	}
	for key, value := range replacement.Fields {
		merged.Fields[key] = value
	}
	return merged, true
}

func cloneValuePreserveRefs(value Value) Value {
	return cloneValuePreserveRefsSeen(value, make(map[uint64]bool))
}

type aliasSnapshot struct {
	ref  uint64
	kind ValueKind
}

func snapshotAlias(value Value) aliasSnapshot {
	return aliasSnapshot{ref: value.Ref, kind: value.Kind}
}

func (s aliasSnapshot) valid() bool {
	return s.ref != 0
}

func cloneValuePreserveRefsSeen(value Value, seen map[uint64]bool) Value {
	out := value
	if value.Ref != 0 {
		if seen[value.Ref] {
			return out
		}
		seen[value.Ref] = true
		defer delete(seen, value.Ref)
	}
	if value.Fields != nil {
		out.Fields = make(map[string]Value, len(value.Fields))
		for key, child := range value.Fields {
			out.Fields[key] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.Map != nil {
		out.Map = make(map[string]Value, len(value.Map))
		for key, child := range value.Map {
			out.Map[key] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.MapKeys != nil {
		out.MapKeys = make(map[string]Value, len(value.MapKeys))
		for key, child := range value.MapKeys {
			out.MapKeys[key] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.MapOrder != nil {
		out.MapOrder = append([]string(nil), value.MapOrder...)
	}
	if value.List != nil {
		out.List = make([]Value, len(value.List))
		for i, child := range value.List {
			out.List[i] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	if value.Set != nil {
		out.Set = make([]Value, len(value.Set))
		for i, child := range value.Set {
			out.Set[i] = cloneValuePreserveRefsSeen(child, seen)
		}
	}
	return out
}

func dataWeaveStaticScriptReceiver(receiverName string) bool {
	return strings.EqualFold(strings.TrimSpace(receiverName), "DataWeave.Script") ||
		strings.EqualFold(strings.TrimSpace(receiverName), "dataweave.Script")
}

func (vm *VM) propagateCollectionMutationToStatics(previous, updated Value) {
	vm.propagateValueMutationToStatics(previous, updated)
}

func (vm *VM) propagateValueMutationToStatics(previous, updated Value) {
	if previous.Ref == 0 {
		return
	}
	if sameAliasValue(previous, updated) {
		return
	}
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	}
	if !vm.staticValueRefs[previous.Ref] {
		return
	}
	locations := vm.staticValueRefFields[previous.Ref]
	if len(locations) == 0 {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for _, location := range locations {
		class, ok := vm.Classes[location.ClassName]
		if !ok || class.StaticFields == nil {
			continue
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			continue
		}
		clearRefSeen(seen)
		replaced, changed := replaceValueAlias(field.Value, previous, updated, seen)
		if !changed {
			continue
		}
		field.Value = replaced
		class.StaticFields[location.FieldName] = field
		vm.Classes[location.ClassName] = class
		vm.rememberStaticValueRefsInField(updated, location)
	}
}

func (vm *VM) propagateAliasSnapshotToScope(scope map[string]Value, previous aliasSnapshot, updated Value) {
	if !previous.valid() {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for name, value := range scope {
		clearRefSeen(seen)
		replaced, changed := replaceAliasSnapshot(value, previous, updated, seen)
		if changed {
			scope[name] = replaced
		}
	}
}

func (vm *VM) propagateAliasSnapshotToStatics(previous aliasSnapshot, updated Value) {
	if !previous.valid() {
		return
	}
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	}
	if !vm.staticValueRefs[previous.ref] {
		return
	}
	locations := vm.staticValueRefFields[previous.ref]
	if len(locations) == 0 {
		return
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	for _, location := range locations {
		class, ok := vm.Classes[location.ClassName]
		if !ok || class.StaticFields == nil {
			continue
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			continue
		}
		clearRefSeen(seen)
		replaced, changed := replaceAliasSnapshot(field.Value, previous, updated, seen)
		if !changed {
			continue
		}
		field.Value = replaced
		class.StaticFields[location.FieldName] = field
		vm.Classes[location.ClassName] = class
		vm.rememberStaticValueRefsInField(updated, location)
	}
}

func (vm *VM) propagateAliasSnapshotMutationToScope(scope map[string]Value, previous aliasSnapshot, original, updated Value, refreshNestedCollections bool) bool {
	if !scopeHasAnyRef(scope) {
		return false
	}
	if sameAliasListCollectionViewOnly(original, updated) {
		return false
	}
	if refreshNestedCollections && sameBackingAliasRefreshKind(updated.Kind) && vm.propagateTopLevelCollectionAliases(scope, updated) {
		return true
	}
	if refreshNestedCollections && sameBackingAliasRefreshKind(updated.Kind) && vm.propagateCollectionValueAliasToScope(scope, original, updated) {
		return true
	}
	if sameAliasRuntimeBacking(original, updated) {
		if refreshNestedCollections && scopeHasNestedCollectionAliasNeedingRefresh(scope, updated) {
			vm.propagateUpdatedValueAliases(scope, updated)
		}
		return false
	}
	if sameAliasRuntimeData(original, updated) || sameAliasRuntimeDataWithCallerCollectionView(original, updated) {
		if valueHasNestedAliasRef(original, previous.ref, make(map[uint64]bool)) {
			vm.propagateUpdatedValueAliases(scope, updated)
		}
		return false
	}
	vm.propagateAliasSnapshotToScope(scope, previous, updated)
	return true
}

func (vm *VM) propagateTopLevelCollectionAliases(scope map[string]Value, updated Value) bool {
	if updated.Ref == 0 {
		return false
	}
	changed := false
	for name, value := range scope {
		if value.Ref == 0 || value.Ref != updated.Ref || value.Kind != updated.Kind {
			continue
		}
		replacement := updated
		replacement.Type = value.Type
		replacement.Static = value.Static
		replacement.Runtime = value.Runtime
		scope[name] = replacement
		changed = true
	}
	return changed
}

func (vm *VM) propagateCollectionValueAliasToScope(scope map[string]Value, original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != updated.Kind {
		return false
	}
	if sameAliasValue(original, updated) {
		return false
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	changed := false
	for name, value := range scope {
		clearRefSeen(seen)
		replaced, replacedValue := replaceValueAlias(value, original, updated, seen)
		if replacedValue {
			scope[name] = replaced
			changed = true
		}
	}
	return changed
}

func sameAliasRuntimeBacking(original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != updated.Kind ||
		original.Type != updated.Type || original.Text != updated.Text ||
		original.Static != updated.Static || original.Runtime != updated.Runtime {
		return false
	}
	switch original.Kind {
	case ValueObject:
		return sameMapBacking(original.Fields, updated.Fields)
	case ValueMap:
		return sameMapBacking(original.Map, updated.Map) &&
			sameMapBacking(original.MapKeys, updated.MapKeys) &&
			sameSliceBacking(original.MapOrder, updated.MapOrder)
	case ValueList:
		return sameSliceBacking(original.List, updated.List)
	case ValueSet:
		return sameSliceBacking(original.Set, updated.Set)
	default:
		return false
	}
}

func sameMapBacking[K comparable, V any](left, right map[K]V) bool {
	if len(left) != len(right) {
		return false
	}
	if len(left) == 0 {
		return (left == nil) == (right == nil)
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}

func sameSliceBacking[T any](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	if len(left) == 0 {
		return (left == nil) == (right == nil)
	}
	return reflect.ValueOf(left).Pointer() == reflect.ValueOf(right).Pointer()
}

func scopeHasNestedCollectionAliasNeedingRefresh(scope map[string]Value, updated Value) bool {
	if updated.Ref == 0 || len(scope) == 0 {
		return false
	}
	refs := make(map[uint64]bool)
	for _, value := range scope {
		if value.Ref == 0 || value.Ref == updated.Ref || !sameBackingAliasRefreshKind(value.Kind) {
			continue
		}
		refs[value.Ref] = true
	}
	if len(refs) == 0 {
		return false
	}
	return valueContainsNestedRefInSet(updated, updated.Ref, refs, make(map[uint64]bool))
}

func sameBackingAliasRefreshKind(kind ValueKind) bool {
	switch kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
}

func valueContainsNestedRefInSet(value Value, rootRef uint64, refs map[uint64]bool, seen map[uint64]bool) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		if value.Ref != rootRef && refs[value.Ref] {
			return true
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueContainsNestedRefInSet(child, rootRef, refs, seen) {
				return true
			}
		}
	}
	return false
}

// scopeHasAnyRef returns true if any binding in scope carries a non-zero
// reference id. Most scopes during expression evaluation contain only
// primitives or null; this lets us bail before invoking the deep
// sameAliasRuntimeData comparison.
func scopeHasAnyRef(scope map[string]Value) bool {
	for _, value := range scope {
		if value.Ref != 0 {
			return true
		}
	}
	return false
}

func sameAliasListCollectionViewOnly(original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != ValueList || updated.Kind != ValueList {
		return false
	}
	if strings.EqualFold(original.Type, updated.Type) || collectionBase(original.Type) != "List" || collectionBase(updated.Type) != "List" {
		return false
	}
	if len(original.List) != len(updated.List) {
		return false
	}
	return true
}

func sameAliasRuntimeDataWithCallerCollectionView(original, updated Value) bool {
	if original.Ref == 0 || original.Ref != updated.Ref || original.Kind != updated.Kind {
		return false
	}
	switch original.Kind {
	case ValueList, ValueSet, ValueMap:
	default:
		return false
	}
	callerView := updated
	callerView.Type = original.Type
	return sameAliasRuntimeData(original, callerView)
}

func (vm *VM) rememberStaticValueRefs(value Value) {
	if vm.staticValueRefs == nil {
		return
	}
	collectValueRefs(value, vm.staticValueRefs, make(map[uint64]bool))
}

func (vm *VM) rememberStaticValueRefsInField(value Value, location staticFieldRef) {
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		return
	}
	collectStaticFieldValueRefs(value, vm.staticValueRefs, vm.staticValueRefFields, location, make(map[uint64]bool))
}

func (vm *VM) invalidateStaticValueRefs() {
	vm.staticValueRefs = nil
	vm.staticValueRefFields = nil
}

func (vm *VM) invalidateStaticValueRefsForChange(previous, updated Value) {
	if vm.staticValueRefs == nil {
		return
	}
	vm.invalidateStaticValueRefs()
}

func valueHasRef(value Value, seen map[uint64]bool) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		return true
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if valueHasRef(child, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueHasRef(child, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueHasRef(child, seen) {
				return true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if valueHasRef(child, seen) {
				return true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if valueHasRef(child, seen) {
				return true
			}
		}
	}
	return false
}

func (vm *VM) collectStaticValueRefs() (map[uint64]bool, map[uint64][]staticFieldRef) {
	refs := make(map[uint64]bool)
	fields := make(map[uint64][]staticFieldRef)
	seen := make(map[uint64]bool)
	for className, class := range vm.Classes {
		for fieldName, field := range class.StaticFields {
			clearRefSeen(seen)
			collectStaticFieldValueRefs(field.Value, refs, fields, staticFieldRef{ClassName: className, FieldName: fieldName}, seen)
		}
	}
	return refs, fields
}

func collectValueRefs(value Value, refs, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		refs[value.Ref] = true
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			collectValueRefs(child, refs, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			collectValueRefs(child, refs, seen)
		}
		for _, child := range value.MapKeys {
			collectValueRefs(child, refs, seen)
		}
	case ValueList:
		for _, child := range value.List {
			collectValueRefs(child, refs, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			collectValueRefs(child, refs, seen)
		}
	}
}

func collectStaticFieldValueRefs(value Value, refs map[uint64]bool, fields map[uint64][]staticFieldRef, location staticFieldRef, seen map[uint64]bool) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		refs[value.Ref] = true
		fields[value.Ref] = appendStaticFieldRef(fields[value.Ref], location)
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	case ValueMap:
		for _, child := range value.Map {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
		for _, child := range value.MapKeys {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	case ValueList:
		for _, child := range value.List {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	case ValueSet:
		for _, child := range value.Set {
			collectStaticFieldValueRefs(child, refs, fields, location, seen)
		}
	}
}

func appendStaticFieldRef(locations []staticFieldRef, location staticFieldRef) []staticFieldRef {
	for _, existing := range locations {
		if existing == location {
			return locations
		}
	}
	return append(locations, location)
}

func replaceCollectionAlias(value, previous, updated Value, seen map[uint64]bool) (Value, bool) {
	return replaceValueAlias(value, previous, updated, seen)
}

func replaceValueAlias(value, previous, updated Value, seen map[uint64]bool) (Value, bool) {
	if previous.Ref == 0 {
		return value, false
	}
	return replaceValueAliasRef(value, previous.Ref, previous.Kind, updated, seen)
}

func replaceAliasSnapshot(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if !previous.valid() {
		return value, false
	}
	return replaceValueAliasRef(value, previous.ref, previous.kind, updated, seen)
}

func replaceValueAliasRef(value Value, previousRef uint64, previousKind ValueKind, updated Value, seen map[uint64]bool) (Value, bool) {
	if value.Ref != 0 {
		if value.Ref == previousRef && value.Kind == previousKind {
			return updated, true
		}
		if seen[value.Ref] {
			return value, false
		}
		seen[value.Ref] = true
	}
	changed := false
	switch value.Kind {
	case ValueObject:
		if objectFieldsCannotContainAliasRef(value.Fields, previousRef, previousKind) {
			return value, false
		}
		for name, child := range value.Fields {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.Fields[name] = replaced
				changed = true
			}
		}
	case ValueMap:
		if mapCannotContainAliasRef(value, previousRef, previousKind) {
			return value, false
		}
		for key, child := range value.Map {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.Map[key] = replaced
				changed = true
			}
		}
		for key, child := range value.MapKeys {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.MapKeys[key] = replaced
				changed = true
			}
		}
	case ValueList:
		if listCannotContainAliasRef(value.List, previousRef, previousKind) {
			return value, false
		}
		for i, child := range value.List {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.List[i] = replaced
				changed = true
			}
		}
	case ValueSet:
		if listCannotContainAliasRef(value.Set, previousRef, previousKind) {
			return value, false
		}
		for i, child := range value.Set {
			replaced, childChanged := replaceValueAliasRef(child, previousRef, previousKind, updated, seen)
			if childChanged {
				value.Set[i] = replaced
				changed = true
			}
		}
	}
	return value, changed
}

func valueContainsAliasRef(value Value, previousRef uint64, previousKind ValueKind, seen map[uint64]bool) bool {
	if previousRef == 0 {
		return false
	}
	if value.Ref != 0 {
		if value.Ref == previousRef && value.Kind == previousKind {
			return true
		}
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
	}
	switch value.Kind {
	case ValueObject:
		if objectFieldsCannotContainAliasRef(value.Fields, previousRef, previousKind) {
			return false
		}
		for _, child := range value.Fields {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueMap:
		if mapCannotContainAliasRef(value, previousRef, previousKind) {
			return false
		}
		for _, child := range value.Map {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueList:
		if listCannotContainAliasRef(value.List, previousRef, previousKind) {
			return false
		}
		for _, child := range value.List {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	case ValueSet:
		if listCannotContainAliasRef(value.Set, previousRef, previousKind) {
			return false
		}
		for _, child := range value.Set {
			if valueContainsAliasRef(child, previousRef, previousKind, seen) {
				return true
			}
		}
	}
	return false
}

func listCannotContainObjectRef(values []Value, previousRef uint64) bool {
	return listCannotContainAliasRef(values, previousRef, ValueObject)
}

func listCannotContainAliasRef(values []Value, previousRef uint64, previousKind ValueKind) bool {
	if previousRef == 0 || len(values) == 0 {
		return false
	}
	for _, value := range values {
		if valueCannotContainAliasRef(value, previousRef, previousKind) {
			continue
		}
		if value.Kind != previousKind || value.Ref == 0 {
			return false
		}
		if value.Ref == previousRef {
			return false
		}
	}
	return true
}

func objectFieldsCannotContainObjectRef(fields map[string]Value, previousRef uint64) bool {
	return objectFieldsCannotContainAliasRef(fields, previousRef, ValueObject)
}

func objectFieldsCannotContainAliasRef(fields map[string]Value, previousRef uint64, previousKind ValueKind) bool {
	if previousRef == 0 || len(fields) == 0 {
		return false
	}
	for _, value := range fields {
		if !valueCannotContainAliasRef(value, previousRef, previousKind) {
			return false
		}
	}
	return true
}

func mapCannotContainObjectRef(value Value, previousRef uint64) bool {
	return mapCannotContainAliasRef(value, previousRef, ValueObject)
}

func mapCannotContainAliasRef(value Value, previousRef uint64, previousKind ValueKind) bool {
	if previousRef == 0 || len(value.Map)+len(value.MapKeys) == 0 {
		return false
	}
	for _, child := range value.Map {
		if !valueCannotContainAliasRef(child, previousRef, previousKind) {
			return false
		}
	}
	for _, child := range value.MapKeys {
		if !valueCannotContainAliasRef(child, previousRef, previousKind) {
			return false
		}
	}
	return true
}

func valueCannotContainObjectRef(value Value, previousRef uint64) bool {
	return valueCannotContainAliasRef(value, previousRef, ValueObject)
}

func valueCannotContainAliasRef(value Value, previousRef uint64, previousKind ValueKind) bool {
	if value.Ref == previousRef && value.Kind == previousKind {
		return false
	}
	switch value.Kind {
	case ValueNull, ValueInt, ValueDecimal, ValueBool, ValueString:
		return true
	case ValueObject:
		return value.Ref != previousRef && len(value.Fields) == 0
	case ValueList:
		return value.Ref != previousRef && len(value.List) == 0
	case ValueSet:
		return value.Ref != previousRef && len(value.Set) == 0
	case ValueMap:
		return value.Ref != previousRef && len(value.Map)+len(value.MapKeys) == 0
	default:
		return false
	}
}

func collectionAliasMatch(left, right Value) bool {
	return valueAliasMatch(left, right)
}

func sameAliasValue(left, right Value) bool {
	if left.Ref == 0 || left.Ref != right.Ref || left.Kind != right.Kind {
		return false
	}
	return sameAliasContent(left, right, make(map[[2]uint64]bool))
}

func sameAliasContent(left, right Value, seen map[[2]uint64]bool) bool {
	if left.Kind != right.Kind || left.Type != right.Type || left.Text != right.Text || left.Static != right.Static || left.Runtime != right.Runtime {
		return false
	}
	if left.Ref != 0 && right.Ref != 0 {
		key := [2]uint64{left.Ref, right.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	switch left.Kind {
	case ValueObject:
		if len(left.Fields) != len(right.Fields) {
			return false
		}
		for name, leftValue := range left.Fields {
			rightValue, ok := right.Fields[name]
			if !ok || !sameAliasContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if !sameAliasContent(left.List[i], right.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(left.Set) != len(right.Set) {
			return false
		}
		rightValues := append([]Value(nil), right.Set...)
		for _, leftValue := range left.Set {
			match := -1
			for i, rightValue := range rightValues {
				if sameAliasContent(leftValue, rightValue, seen) {
					match = i
					break
				}
			}
			if match < 0 {
				return false
			}
			rightValues = append(rightValues[:match], rightValues[match+1:]...)
		}
		return true
	case ValueMap:
		if len(left.Map) != len(right.Map) || len(left.MapKeys) != len(right.MapKeys) {
			return false
		}
		for key, leftValue := range left.Map {
			rightValue, ok := right.Map[key]
			if !ok || !sameAliasContent(leftValue, rightValue, seen) {
				return false
			}
		}
		for key, leftValue := range left.MapKeys {
			rightValue, ok := right.MapKeys[key]
			if !ok || !sameAliasContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	default:
		return left.Equal(right)
	}
}

func sameAliasRuntimeData(left, right Value) bool {
	return sameAliasRuntimeContent(left, right, make(map[[2]uint64]bool))
}

func sameAliasRuntimeContent(left, right Value, seen map[[2]uint64]bool) bool {
	if left.Kind != right.Kind || left.Type != right.Type || left.Text != right.Text ||
		left.Int != right.Int || left.Decimal != right.Decimal || left.Bool != right.Bool {
		return false
	}
	if left.Ref != 0 && right.Ref != 0 && left.Ref != right.Ref {
		return false
	}
	if left.Ref != 0 && right.Ref != 0 {
		key := [2]uint64{left.Ref, right.Ref}
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	switch left.Kind {
	case ValueObject:
		if len(left.Fields) != len(right.Fields) {
			return false
		}
		for name, leftValue := range left.Fields {
			rightValue, ok := right.Fields[name]
			if !ok || !sameAliasRuntimeContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	case ValueList:
		if len(left.List) != len(right.List) {
			return false
		}
		for i := range left.List {
			if !sameAliasRuntimeContent(left.List[i], right.List[i], seen) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(left.Set) != len(right.Set) {
			return false
		}
		rightValues := append([]Value(nil), right.Set...)
		for _, leftValue := range left.Set {
			match := -1
			for i, rightValue := range rightValues {
				if sameAliasRuntimeContent(leftValue, rightValue, seen) {
					match = i
					break
				}
			}
			if match < 0 {
				return false
			}
			rightValues = append(rightValues[:match], rightValues[match+1:]...)
		}
		return true
	case ValueMap:
		if len(left.Map) != len(right.Map) || len(left.MapKeys) != len(right.MapKeys) {
			return false
		}
		for key, leftValue := range left.Map {
			rightValue, ok := right.Map[key]
			if !ok || !sameAliasRuntimeContent(leftValue, rightValue, seen) {
				return false
			}
		}
		for key, leftValue := range left.MapKeys {
			rightValue, ok := right.MapKeys[key]
			if !ok || !sameAliasRuntimeContent(leftValue, rightValue, seen) {
				return false
			}
		}
		return true
	default:
		return left.Equal(right)
	}
}

func valueHasNestedAliasRef(value Value, rootRef uint64, seen map[uint64]bool) bool {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return false
		}
		seen[value.Ref] = true
		if value.Ref != rootRef {
			return true
		}
	}
	switch value.Kind {
	case ValueObject:
		for _, child := range value.Fields {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	case ValueList:
		for _, child := range value.List {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	case ValueSet:
		for _, child := range value.Set {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	case ValueMap:
		for _, child := range value.Map {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
		for _, child := range value.MapKeys {
			if valueHasNestedAliasRef(child, rootRef, seen) {
				return true
			}
		}
	}
	return false
}

func clearRefSeen(seen map[uint64]bool) {
	for ref := range seen {
		delete(seen, ref)
	}
}

func valueAliasMatch(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ValueObject, ValueList, ValueSet, ValueMap:
		return left.Ref != 0 && left.Ref == right.Ref
	default:
		return false
	}
}

func valueAliasSnapshotMatch(left aliasSnapshot, right Value) bool {
	return left.valid() && left.kind == right.Kind && left.ref == right.Ref
}

func sameCollectionType(left, right Value) bool {
	if left.Kind != right.Kind || !strings.EqualFold(left.Type, right.Type) {
		return false
	}
	switch left.Kind {
	case ValueList, ValueSet, ValueMap:
		return true
	default:
		return false
	}
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

// staticFieldValueByRef consults the per-VM reverse index built by
// collectStaticValueRefs. It avoids scanning every class for every lookup;
// nams profiles showed findValueByRef at 184 s cum / 15 % CPU when this
// walked all classes × all static fields per ref.
func (vm *VM) staticFieldValueByRef(ref uint64) (Value, bool) {
	if ref == 0 {
		return Null, false
	}
	if vm.staticValueRefs == nil || vm.staticValueRefFields == nil {
		vm.staticValueRefs, vm.staticValueRefFields = vm.collectStaticValueRefs()
	}
	if !vm.staticValueRefs[ref] {
		return Null, false
	}
	for _, location := range vm.staticValueRefFields[ref] {
		class, ok := vm.Classes[location.ClassName]
		if !ok {
			continue
		}
		field, ok := class.StaticFields[location.FieldName]
		if !ok {
			continue
		}
		if field.Value.Ref == ref {
			return field.Value, true
		}
		if value, ok := findValueByRef(field.Value, ref, make(map[uint64]bool)); ok {
			return value, true
		}
	}
	return Null, false
}

func (vm *VM) scanStaticFieldValueByRef(ref uint64) (Value, bool) {
	for _, class := range vm.Classes {
		for _, field := range class.StaticFields {
			if field.Value.Ref == ref {
				return field.Value, true
			}
			if value, ok := findValueByRef(field.Value, ref, make(map[uint64]bool)); ok {
				return value, true
			}
		}
	}
	return Null, false
}

func (vm *VM) liveScopes() []map[string]Value {
	scopes := make([]map[string]Value, 0, len(vm.scopeStack)+1)
	scopes = append(scopes, vm.Globals)
	for i := len(vm.scopeStack) - 1; i >= 0; i-- {
		scopes = append(scopes, vm.scopeStack[i])
	}
	return scopes
}

func directValueByRefInScope(scope map[string]Value, ref uint64) (Value, bool) {
	for _, value := range scope {
		if value.Ref == ref {
			return value, true
		}
	}
	return Null, false
}

func findValueByRefInScope(scope map[string]Value, ref uint64, seen map[uint64]bool) (Value, bool) {
	for _, value := range scope {
		if found, ok := findValueByRef(value, ref, seen); ok {
			return found, true
		}
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
			if value, ok := args[0].Fields["object"]; ok && value.Kind == ValueString {
				objectName = value.Text
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

func (vm *VM) callFrameworkSObjectDescribeMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if strings.EqualFold(receiver.Type, "framework_SObjectDescribe") {
		return vm.callFrameworkSObjectDescribe(receiver, method, args, result)
	}
	if strings.EqualFold(receiver.Type, "framework_SObjectDescribe.FieldsMap") ||
		strings.EqualFold(receiver.Type, "framework_SObjectDescribe.GlobalDescribeMap") {
		return vm.callFrameworkNamespacedAttributeMap(receiver, method, args)
	}
	return Null, false, nil
}

func (vm *VM) callFrameworkStaticMember(className, method string, args []Value) (Value, bool, error) {
	switch {
	case strings.EqualFold(className, "framework_SObjectDomain") && strings.EqualFold(method, "triggerHandler"):
		return vm.callFrameworkSObjectDomainTriggerHandler(args)
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
	_, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]
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

func (vm *VM) callFrameworkSObjectDomainTriggerHandler(args []Value) (Value, bool, error) {
	if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Type") {
		return Null, true, fmt.Errorf("framework_SObjectDomain.triggerHandler expects Type")
	}
	domainClassName := typeValueName(args[0])
	if domainClassName == "" {
		return Null, true, fmt.Errorf("framework_SObjectDomain.triggerHandler Type is blank")
	}
	if ctx, ok := vm.frameworkMockDatabaseContext(false); ok {
		afterCtx, hasAfterCtx := vm.frameworkMockDatabaseContext(true)
		saved := vm.triggerGlobals
		vm.triggerGlobals = ctx
		beforeDomain, _, err := vm.callFrameworkSObjectDomainTriggerHandlerForContext(domainClassName)
		if err != nil {
			vm.triggerGlobals = saved
			return Null, true, err
		}
		if hasAfterCtx {
			vm.triggerGlobals = afterCtx
			domainOverride := Null
			if frameworkDomainTriggerStateEnabled(beforeDomain) {
				domainOverride = beforeDomain
			}
			if _, _, err := vm.callFrameworkSObjectDomainTriggerHandlerForContextWithDomain(domainClassName, domainOverride); err != nil {
				vm.triggerGlobals = saved
				return Null, true, err
			}
		}
		vm.triggerGlobals = saved
		return Null, true, nil
	}
	return vm.callFrameworkSObjectDomainTriggerHandlerForContext(domainClassName)
}

func (vm *VM) callFrameworkSObjectDomainTriggerHandlerForContext(domainClassName string) (Value, bool, error) {
	return vm.callFrameworkSObjectDomainTriggerHandlerForContextWithDomain(domainClassName, Null)
}

func (vm *VM) callFrameworkSObjectDomainTriggerHandlerForContextWithDomain(domainClassName string, domainOverride Value) (Value, bool, error) {
	if !vm.frameworkTriggerEventEnabled(domainClassName) {
		return Null, true, nil
	}
	records := vm.triggerGlobals["Trigger.new"]
	if records.Kind == ValueNull || records.Kind == "" {
		records = vm.triggerGlobals["Trigger.old"]
	}
	if records.Kind != ValueList {
		records = List()
		records.Type = "List<SObject>"
	}
	records = withConcreteSObjectListRuntime(records)
	domain := domainOverride
	var err error
	if domain.Kind != ValueObject {
		domain, err = vm.constructValue(domainClassName, []Value{records}, nil, resultForLookup())
		if err != nil && hasSuffixFold(domainClassName, ".constructor") {
			domain, err = vm.constructDomainThroughFrameworkConstructor(domainClassName, records)
		}
		if err != nil {
			constructorName := domainClassName
			if !hasSuffixFold(constructorName, "constructor") {
				constructorName += ".Constructor"
			}
			domain, err = vm.constructDomainThroughFrameworkConstructor(constructorName, records)
		}
		if err != nil {
			return Null, true, err
		}
	}
	handler := ""
	switch {
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isInsert"):
		handler = "handleBeforeInsert"
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isUpdate"):
		handler = "handleBeforeUpdate"
	case triggerBool(vm.triggerGlobals, "Trigger.isBefore") && triggerBool(vm.triggerGlobals, "Trigger.isDelete"):
		handler = "handleBeforeDelete"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isInsert"):
		handler = "handleAfterInsert"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isUpdate"):
		handler = "handleAfterUpdate"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isDelete"):
		handler = "handleAfterDelete"
	case triggerBool(vm.triggerGlobals, "Trigger.isAfter") && triggerBool(vm.triggerGlobals, "Trigger.isUndelete"):
		handler = "handleAfterUndelete"
	}
	if handler == "" {
		return domain, true, nil
	}
	handlerArgs := []Value(nil)
	if strings.EqualFold(handler, "handleBeforeUpdate") || strings.EqualFold(handler, "handleAfterUpdate") {
		oldMap := vm.triggerGlobals["Trigger.oldMap"]
		if oldMap.Kind == ValueNull || oldMap.Kind == "" {
			oldMap = typedMap("Map<Id,SObject>")
		}
		handlerArgs = []Value{oldMap}
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(domain.Type, handler, handlerArgs)
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(domain.Type+"."+handler, handlerArgs)
	}
	if !ok {
		return Null, true, fmt.Errorf("%s.%s not found", domain.Type, handler)
	}
	if _, err := vm.callMethodWithReceiver(target, domain, handlerArgs, resultForLookup()); err != nil {
		return Null, true, err
	}
	if triggerBool(vm.triggerGlobals, "Trigger.isBefore") && !triggerBool(vm.triggerGlobals, "Trigger.isDelete") {
		if records, ok := frameworkDomainRecords(domain); ok {
			vm.triggerGlobals["Trigger.new"] = records
		}
	}
	return domain, true, nil
}

func frameworkDomainTriggerStateEnabled(domain Value) bool {
	if domain.Kind != ValueObject {
		return false
	}
	_, config, ok := objectFieldValue(domain, "Configuration")
	if !ok || config.Kind != ValueObject {
		return false
	}
	_, enabled, ok := objectFieldValue(config, "TriggerStateEnabled")
	return ok && enabled.Kind == ValueBool && enabled.Bool
}

func (vm *VM) frameworkTriggerEventEnabled(domainClassName string) bool {
	field, _, ok := vm.lookupStaticField("framework_SObjectDomain", "TriggerEventByClass")
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

func (vm *VM) frameworkMockDatabaseContext(after bool) (map[string]Value, bool) {
	if vm == nil {
		return nil, false
	}
	testField, _, ok := vm.lookupStaticField("framework_SObjectDomain", "Test")
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

func (vm *VM) constructDomainThroughFrameworkConstructor(constructorName string, records Value) (Value, error) {
	constructor, err := vm.constructValue(constructorName, nil, nil, resultForLookup())
	if err != nil {
		return Null, err
	}
	construct, ok, ambiguous := vm.resolveInstanceMethodForArgs(constructor.Type, "construct", []Value{records})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(constructor.Type+".construct", []Value{records})
	}
	if !ok {
		return Null, fmt.Errorf("%s.construct(List<SObject>) not found", constructor.Type)
	}
	return vm.callMethodWithReceiver(construct, constructor, []Value{records}, resultForLookup())
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

func frameworkDomainRecords(domain Value) (Value, bool) {
	if domain.Kind != ValueObject {
		return Null, false
	}
	for name, value := range domain.Fields {
		if strings.EqualFold(name, "objects") && value.Kind == ValueList {
			return value, true
		}
	}
	return Null, false
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

func isArgumentCaptorAnyObjectType(typeName string) bool {
	name := strings.ToLower(strings.TrimSpace(typeName))
	return strings.HasSuffix(name, "argumentcaptor.anyobject")
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

func (vm *VM) callFrameworkSObjectDescribe(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	token, ok := frameworkSObjectDescribeToken(receiver)
	if !ok {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "getsobjecttype":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getSObjectType expects 0 arguments")
		}
		return token, true, nil
	case "getdescribe":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getDescribe expects 0 arguments")
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		return describe, true, nil
	case "getfieldsmap":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getFieldsMap expects 0 arguments")
		}
		fields, err := vm.frameworkSObjectDescribeFields(token, result)
		return fields, true, err
	case "getfield":
		if len(args) != 1 && len(args) != 2 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getField expects field name")
		}
		if args[0].Kind == ValueNull {
			return Null, true, nil
		}
		if args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getField expects String")
		}
		implyNamespace := true
		if len(args) == 2 {
			if args[1].Kind != ValueBool {
				return Null, true, fmt.Errorf("framework_SObjectDescribe.getField implyNamespace expects Boolean")
			}
			implyNamespace = args[1].Bool
		}
		field, err := vm.frameworkSObjectDescribeField(token, args[0].Text, implyNamespace, result)
		return field, true, err
	case "getnamefield":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectDescribe.getNameField expects 0 arguments")
		}
		field, err := vm.frameworkSObjectDescribeNameField(token, result)
		return field, true, err
	}
	return Null, false, nil
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

func frameworkSObjectDescribeToken(receiver Value) (Value, bool) {
	_, token, ok := objectFieldValue(receiver, "token")
	return token, ok && token.Kind == ValueObject && strings.EqualFold(token.Type, "Schema.SObjectType")
}

func frameworkNamespacedAttributeMapValues(receiver Value) (Value, bool) {
	_, values, ok := objectFieldValue(receiver, "values")
	return values, ok && values.Kind == ValueMap
}

func (vm *VM) frameworkSObjectDescribeFields(token Value, result *Result) (Value, error) {
	describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
	if err != nil || !handled {
		return describe, err
	}
	fields, ok := describe.Fields["fields"]
	if !ok {
		return Null, fmt.Errorf("framework_SObjectDescribe fields are not available")
	}
	value, _, _, _, err := vm.callPlatformObjectMember(fields, "getMap", nil, result)
	return value, err
}

func (vm *VM) frameworkSObjectDescribeField(token Value, fieldName string, implyNamespace bool, result *Result) (Value, error) {
	fields, err := vm.frameworkSObjectDescribeFields(token, result)
	if err != nil {
		return Null, err
	}
	lookupName := fieldName
	if hasSuffixFold(lookupName, "__r") {
		lookupName = lookupName[:len(lookupName)-len("__r")] + "__c"
	}
	value := vm.frameworkNamespacedAttributeMapGet(fields, lookupName, implyNamespace, "")
	if value.Kind == ValueNull {
		value = vm.frameworkNamespacedAttributeMapGet(fields, fieldName+"Id", implyNamespace, "")
	}
	return value, nil
}

func (vm *VM) frameworkSObjectDescribeNameField(token Value, result *Result) (Value, error) {
	fields, err := vm.frameworkSObjectDescribeFields(token, result)
	if err != nil {
		return Null, err
	}
	for _, key := range sortedMapKeys(fields.Map) {
		field := fields.Map[key]
		describe, _, _, handled, err := vm.callPlatformObjectMember(field, "getDescribe", nil, result)
		if err != nil || !handled {
			return Null, err
		}
		if isNameField, ok := describe.Fields["nameField"]; ok && isNameField.Kind == ValueBool && isNameField.Bool {
			return field, nil
		}
	}
	return Null, nil
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

func (vm *VM) callFrameworkSimpleDMLMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if !strings.EqualFold(receiver.Type, "framework_SObjectUnitOfWork.SimpleDML") {
		return Null, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.SimpleDML.%s expects List<SObject>", method)
	}
	switch strings.ToLower(method) {
	case "dmlinsert":
		_, err := vm.executeDatabaseDML("insert", []Value{args[0]}, result)
		return Null, true, err
	case "dmlupdate":
		_, err := vm.executeDatabaseDML("update", []Value{args[0]}, result)
		return Null, true, err
	case "dmldelete":
		_, err := vm.executeDatabaseDML("delete", []Value{args[0]}, result)
		return Null, true, err
	case "eventpublish":
		_, err := vm.eventBusPublish([]Value{args[0]}, result)
		return Null, true, err
	case "emptyrecyclebin":
		if len(args[0].List) == 0 {
			return Null, true, nil
		}
		_, err := vm.executeDatabaseRecordAction("emptyRecycleBin", []Value{args[0]}, result, "Database.EmptyRecycleBinResult")
		return Null, true, err
	default:
		return Null, false, nil
	}
}

func (vm *VM) callFrameworkSObjectUnitOfWorkMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if strings.EqualFold(frameworkMockSupportType(receiver.Type), "SObjectUnitOfWork.SendEmailWork") {
		switch strings.ToLower(method) {
		case "registeremail":
			if len(args) != 1 || !messagingEmailAssignable(args[0].Type, "Messaging.Email") {
				return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.SendEmailWork.registerEmail expects Messaging.Email")
			}
			emails, ok := receiver.Fields["emails"]
			if !ok || emails.Kind != ValueList {
				emails = typedList("List<Messaging.Email>")
			}
			emails.List = append(emails.List, args[0])
			receiver.Fields["emails"] = emails
			return Null, true, nil
		case "dowork":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.SendEmailWork.doWork expects 0 arguments")
			}
			emails, ok := receiver.Fields["emails"]
			if !ok || emails.Kind != ValueList || len(emails.List) == 0 {
				return Null, true, nil
			}
			_, err := vm.sendEmail([]Value{emails}, result)
			return Null, true, err
		}
	}
	if frameworkSObjectUnitOfWorkRelationshipsType(receiver.Type) {
		switch strings.ToLower(method) {
		case "add":
			var relationship Value
			if len(args) == 1 && args[0].Kind == ValueObject {
				relationship = args[0]
			} else if len(args) == 2 && args[0].Kind == ValueObject && args[1].Kind == ValueObject {
				// fflib email relationship: add(Messaging.SingleEmailMessage, SObject)
				if !strings.Contains(strings.ToLower(args[0].Type), "singleemailmessage") {
					return Null, true, fmt.Errorf("%s.add expects relationship object or record, field, relatedTo", receiver.Type)
				}
				relationship = Object("framework_SObjectUnitOfWork.EmailRelationship")
				relationship.Fields["email"] = args[0]
				relationship.Fields["relatedTo"] = args[1]
			} else if len(args) == 3 && args[0].Kind == ValueObject && args[1].Kind == ValueObject && args[2].Kind == ValueObject {
				fieldName, err := vm.sObjectFieldArg(args[0].Type, args[1])
				if err != nil {
					return Null, true, err
				}
				relationship = Object("framework_SObjectUnitOfWork.Relationship")
				relationship.Fields["Record"] = args[0]
				relationship.Fields["RelatedToField"] = sObjectFieldToken(vm.canonicalSObjectValueType(args[0]), fieldName)
				relationship.Fields["RelatedTo"] = args[2]
			} else if len(args) == 4 && args[0].Kind == ValueObject && args[1].Kind == ValueObject && args[2].Kind == ValueObject {
				// fflib external-id relationship: add(SObject, SObjectField, SObjectField, Object)
				fieldName, err := vm.sObjectFieldArg(args[0].Type, args[1])
				if err != nil {
					return Null, true, err
				}
				relationshipName := fieldName
				if hasSuffixFold(relationshipName, "__c") {
					relationshipName = relationshipName[:len(relationshipName)-3] + "__r"
				} else if hasSuffixFold(relationshipName, "id") {
					relationshipName = relationshipName[:len(relationshipName)-2]
				}
				relationship = Object("framework_SObjectUnitOfWork.RelationshipByExternalId")
				relationship.Fields["Record"] = args[0]
				relationship.Fields["RelatedToField"] = sObjectFieldToken(vm.canonicalSObjectValueType(args[0]), fieldName)
				relationship.Fields["ExternalIdField"] = args[2]
				relationship.Fields["ExternalId"] = args[3]
				relationship.Fields["RelationshipName"] = String(relationshipName)
			} else {
				return Null, true, fmt.Errorf("%s.add expects relationship object or record, field, relatedTo", receiver.Type)
			}
			relationships, ok := receiver.Fields["m_relationships"]
			if !ok || relationships.Kind != ValueList {
				relationships = typedList("List<framework_SObjectUnitOfWork.IRelationship>")
			}
			relationships.List = append(relationships.List, relationship)
			receiver.Fields["m_relationships"] = relationships
			return Null, true, nil
		}
	}
	if !vm.isSObjectUnitOfWorkRuntimeType(receiver.Type) {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "commitwork":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.commitWork expects 0 arguments")
		}
		return Null, true, vm.commitFrameworkSObjectUnitOfWork(receiver, result)
	case "handleregistertype":
		return vm.callFrameworkSObjectUnitOfWorkHandleRegisterType(receiver, args)
	case "registernew":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkNew(receiver, args)
	case "registerdirty":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkDirty(receiver, args)
	case "registerdeleted":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_deletedMapByType", args, true)
	case "registerupsert":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkUpsert(receiver, args)
	case "registeremptyrecyclebin":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_emptyRecycleBinMapByType", args, true)
	case "registerpermanentlydeleted":
		if err := vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_emptyRecycleBinMapByType", args, true); err != nil {
			return Null, true, err
		}
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_deletedMapByType", args, true)
	case "registerpublishbeforetransaction":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_publishBeforeListByType", args, false)
	case "registerpublishaftersuccesstransaction":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_publishAfterSuccessListByType", args, false)
	case "registerpublishafterfailuretransaction":
		return Null, true, vm.registerFrameworkSObjectUnitOfWorkRecords(receiver, "m_publishAfterFailureListByType", args, false)
	default:
		return Null, false, nil
	}
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

func (vm *VM) callFrameworkSObjectUnitOfWorkHandleRegisterType(receiver Value, args []Value) (Value, bool, error) {
	if len(args) != 1 || !isSObjectTypeToken(args[0]) {
		return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.handleRegisterType expects Schema.SObjectType")
	}
	objectName, ok := sObjectTypeTokenObjectName(args[0])
	if !ok || objectName == "" {
		return Null, true, fmt.Errorf("framework_SObjectUnitOfWork.handleRegisterType requires SObjectType object name")
	}
	if vm.Org != nil {
		if canonical, resolved := vm.resolveObjectName(objectName); resolved && canonical != "" {
			objectName = canonical
		}
	}
	receiver.Fields["m_newListByType"] = frameworkMapPut(receiver.Fields["m_newListByType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_dirtyMapByType"] = frameworkMapPut(receiver.Fields["m_dirtyMapByType"], String(objectName), typedMap("Map<Id,SObject>"))
	receiver.Fields["m_deletedMapByType"] = frameworkMapPut(receiver.Fields["m_deletedMapByType"], String(objectName), typedMap("Map<Id,SObject>"))
	receiver.Fields["m_emptyRecycleBinMapByType"] = frameworkMapPut(receiver.Fields["m_emptyRecycleBinMapByType"], String(objectName), typedMap("Map<Id,SObject>"))
	receiver.Fields["m_relationships"] = frameworkMapPut(receiver.Fields["m_relationships"], String(objectName), frameworkUnitOfWorkRelationships())
	receiver.Fields["m_publishBeforeListByType"] = frameworkMapPut(receiver.Fields["m_publishBeforeListByType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_publishAfterSuccessListByType"] = frameworkMapPut(receiver.Fields["m_publishAfterSuccessListByType"], String(objectName), typedList("List<SObject>"))
	receiver.Fields["m_publishAfterFailureListByType"] = frameworkMapPut(receiver.Fields["m_publishAfterFailureListByType"], String(objectName), typedList("List<SObject>"))
	// Derived UoW implementations rely on this callback during registration.
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onRegisterType", []Value{args[0]}, &Result{}); err != nil {
		return Null, true, err
	}
	return Null, true, nil
}

func (vm *VM) registerFrameworkSObjectUnitOfWorkUpsert(receiver Value, args []Value) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	for _, record := range records {
		if sObjectIDValue(record).Kind == ValueNull {
			if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_newListByType", record, false); err != nil {
				return err
			}
		} else if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", record, true); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) registerFrameworkSObjectUnitOfWorkNew(receiver Value, args []Value) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	var relationshipField Value
	var relatedTo Value
	hasRelationship := false
	if len(args) >= 3 && args[1].Kind == ValueObject && isSObjectFieldTokenType(args[1].Type) && args[2].Kind == ValueObject && sObjectValueType(args[2].Type) {
		relationshipField = args[1]
		relatedTo = args[2]
		hasRelationship = true
	}
	for _, record := range records {
		if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_newListByType", record, false); err != nil {
			return err
		}
		if hasRelationship {
			objectName := vm.canonicalSObjectValueType(record)
			if err := vm.addFrameworkSObjectUnitOfWorkRelationship(receiver, objectName, record, relationshipField, relatedTo); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vm *VM) registerFrameworkSObjectUnitOfWorkDirty(receiver Value, args []Value) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	dirtyFields := frameworkSObjectUnitOfWorkDirtyFields(args)
	for _, record := range records {
		if err := vm.addFrameworkSObjectUnitOfWorkDirtyRecord(receiver, record, dirtyFields); err != nil {
			return err
		}
	}
	return nil
}

func frameworkSObjectUnitOfWorkDirtyFields(args []Value) []Value {
	if len(args) < 2 || args[1].Kind != ValueList || len(args[1].List) == 0 {
		return nil
	}
	fields := make([]Value, 0, len(args[1].List))
	for _, field := range args[1].List {
		if field.Kind == ValueObject && isSObjectFieldTokenType(field.Type) {
			fields = append(fields, field)
		}
	}
	return fields
}

func (vm *VM) registerFrameworkSObjectUnitOfWorkRecords(receiver Value, fieldName string, args []Value, requireID bool) error {
	records, err := vm.frameworkSObjectUnitOfWorkRegisterRecords(args)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := vm.addFrameworkSObjectUnitOfWorkRecord(receiver, fieldName, record, requireID); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) addFrameworkSObjectUnitOfWorkDirtyRecord(receiver Value, record Value, dirtyFields []Value) error {
	if len(dirtyFields) == 0 {
		return vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", record, true)
	}
	existing, ok, err := vm.frameworkSObjectUnitOfWorkRegisteredRecord(receiver, "m_dirtyMapByType", record)
	if err != nil {
		return err
	}
	if !ok {
		return vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", record, true)
	}
	for _, fieldToken := range dirtyFields {
		field, err := vm.sObjectFieldArg(record.Type, fieldToken)
		if err != nil {
			return err
		}
		actual, value, ok := objectFieldValue(record, field)
		if !ok {
			value = Null
			actual = field
		}
		setExplicitSObjectField(&existing, actual, value)
	}
	return vm.addFrameworkSObjectUnitOfWorkRecord(receiver, "m_dirtyMapByType", existing, true)
}

func (vm *VM) frameworkSObjectUnitOfWorkRegisteredRecord(receiver Value, fieldName string, record Value) (Value, bool, error) {
	id := sObjectIDValue(record)
	if id.Kind == ValueNull {
		return Null, false, newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", frameworkSObjectUnitOfWorkMissingIDMessage(fieldName))
	}
	objectName := vm.canonicalSObjectValueType(record)
	bucket, ok := receiver.Fields[fieldName]
	if !ok || bucket.Kind != ValueMap {
		return Null, false, fmt.Errorf("framework_SObjectUnitOfWork missing %s", fieldName)
	}
	value, ok := bucket.Map[vm.resolveObjectBucketKey(bucket, objectName)]
	if !ok || value.Kind != ValueMap {
		return Null, false, nil
	}
	registered, ok := value.Map[vm.mapKey(id)]
	return registered, ok, nil
}

func (vm *VM) frameworkSObjectUnitOfWorkRegisterRecords(args []Value) ([]Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("framework_SObjectUnitOfWork register method expects SObject or List<SObject>")
	}
	value := args[0]
	if value.Kind == ValueList {
		return append([]Value(nil), value.List...), nil
	}
	if value.Kind == ValueObject && sObjectValueType(value.Type) {
		return []Value{value}, nil
	}
	return nil, fmt.Errorf("framework_SObjectUnitOfWork register method expects SObject or List<SObject>")
}

func (vm *VM) addFrameworkSObjectUnitOfWorkRecord(receiver Value, fieldName string, record Value, requireID bool) error {
	if record.Kind != ValueObject || !sObjectValueType(record.Type) {
		return fmt.Errorf("framework_SObjectUnitOfWork register method expects SObject")
	}
	id := sObjectIDValue(record)
	if requireID && id.Kind == ValueNull {
		return newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", frameworkSObjectUnitOfWorkMissingIDMessage(fieldName))
	}
	if !requireID && id.Kind != ValueNull && strings.EqualFold(fieldName, "m_newListByType") {
		return newExceptionError("framework_SObjectUnitOfWork.UnitOfWorkException", "Only new records can be registered as new")
	}
	objectName := vm.canonicalSObjectValueType(record)
	bucket, ok := receiver.Fields[fieldName]
	if !ok || bucket.Kind != ValueMap {
		return fmt.Errorf("framework_SObjectUnitOfWork missing %s", fieldName)
	}
	key := vm.resolveObjectBucketKey(bucket, objectName)
	value, ok := bucket.Map[key]
	if !ok {
		if strings.Contains(strings.ToLower(bucket.Type), "list") || strings.Contains(strings.ToLower(fieldName), "publish") || strings.Contains(strings.ToLower(fieldName), "newlist") {
			value = typedList("List<SObject>")
		} else {
			value = typedMap("Map<Id,SObject>")
		}
	}
	if value.Kind == ValueList {
		value.List = append(value.List, record)
	} else if value.Kind == ValueMap {
		recordKey := vm.mapKey(id)
		if id.Kind == ValueNull {
			recordKey = vm.mapKey(record)
		}
		if _, exists := value.Map[recordKey]; !exists {
			value.MapOrder = append(value.MapOrder, recordKey)
		}
		value.Map[recordKey] = record
		if value.MapKeys == nil {
			value.MapKeys = make(map[string]Value)
		}
		value.MapKeys[recordKey] = id
	} else {
		return fmt.Errorf("framework_SObjectUnitOfWork %s bucket must be list or map", fieldName)
	}
	bucket.Map[key] = value
	if bucket.MapKeys == nil {
		bucket.MapKeys = make(map[string]Value)
	}
	bucket.MapKeys[key] = String(objectName)
	if !containsString(bucket.MapOrder, key) {
		bucket.MapOrder = append(bucket.MapOrder, key)
	}
	receiver.Fields[fieldName] = bucket
	return nil
}

func frameworkSObjectUnitOfWorkMissingIDMessage(fieldName string) string {
	switch strings.ToLower(fieldName) {
	case "m_deletedmapbytype", "m_emptyrecyclebinmapbytype":
		return "New records cannot be registered for deletion"
	case "m_dirtymapbytype":
		return "New records cannot be registered as dirty"
	default:
		return "New records cannot be registered for this operation"
	}
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

func (vm *VM) addFrameworkSObjectUnitOfWorkRelationship(receiver Value, objectName string, record Value, relatedToField Value, relatedTo Value) error {
	fieldName, err := vm.sObjectFieldArg(record.Type, relatedToField)
	if err != nil {
		return err
	}
	bucket, ok := receiver.Fields["m_relationships"]
	if !ok || bucket.Kind != ValueMap {
		return fmt.Errorf("framework_SObjectUnitOfWork missing m_relationships")
	}
	relationshipBucket, ok := bucket.Map[mapKey(String(objectName))]
	if !ok || relationshipBucket.Kind != ValueObject {
		relationshipBucket = frameworkUnitOfWorkRelationships()
	}
	relationships, ok := relationshipBucket.Fields["m_relationships"]
	if !ok || relationships.Kind != ValueList {
		relationships = typedList("List<framework_SObjectUnitOfWork.IRelationship>")
	}
	relationship := Object("framework_SObjectUnitOfWork.Relationship")
	relationship.Fields["Record"] = record
	relationship.Fields["RelatedToField"] = sObjectFieldToken(vm.canonicalSObjectValueType(record), fieldName)
	relationship.Fields["RelatedTo"] = relatedTo
	relationships.List = append(relationships.List, relationship)
	relationshipBucket.Fields["m_relationships"] = relationships
	receiver.Fields["m_relationships"] = frameworkMapPut(bucket, String(objectName), relationshipBucket)
	return nil
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

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (vm *VM) constructFrameworkSObjectUnitOfWork(args []Value, namedArgs map[string]Value) (Value, error) {
	if len(namedArgs) != 0 || len(args) < 1 || len(args) > 3 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("framework_SObjectUnitOfWork constructor expects List<Schema.SObjectType>[, IDML][, UnresolvedRelationshipBehavior]")
	}
	uow := Object("framework_SObjectUnitOfWork")
	sObjectTypes := List(append([]Value(nil), args[0].List...)...)
	sObjectTypes.Type = args[0].Type
	uow.Fields["m_sObjectTypes"] = sObjectTypes
	uow.Fields["m_newListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_dirtyMapByType"] = typedMap("Map<String,Map<Id,SObject>>")
	uow.Fields["m_deletedMapByType"] = typedMap("Map<String,Map<Id,SObject>>")
	uow.Fields["m_emptyRecycleBinMapByType"] = typedMap("Map<String,Map<Id,SObject>>")
	uow.Fields["m_relationships"] = typedMap("Map<String,framework_SObjectUnitOfWork.Relationships>")
	uow.Fields["m_publishBeforeListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_publishAfterSuccessListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_publishAfterFailureListByType"] = typedMap("Map<String,List<SObject>>")
	uow.Fields["m_workList"] = typedList("List<framework_SObjectUnitOfWork.IDoWork>")
	emailWork := Object("framework_SObjectUnitOfWork.SendEmailWork")
	emailWork.Fields["emails"] = typedList("List<Messaging.Email>")
	uow.Fields["m_emailWork"] = emailWork
	uow.Fields["m_dml"] = Object("framework_SObjectUnitOfWork.SimpleDML")
	uow.Fields["m_unresolvedRelationshipBehaviour"] = frameworkSObjectUnitOfWorkRelationshipBehaviorValue("IgnoreOutOfOrder")
	switch len(args) {
	case 2:
		if behavior, ok := frameworkSObjectUnitOfWorkRelationshipBehaviorName(args[1]); ok {
			uow.Fields["m_unresolvedRelationshipBehaviour"] = frameworkSObjectUnitOfWorkRelationshipBehaviorValue(behavior)
		} else {
			uow.Fields["m_dml"] = args[1]
		}
	case 3:
		uow.Fields["m_dml"] = args[1]
		behavior, ok := frameworkSObjectUnitOfWorkRelationshipBehaviorName(args[2])
		if !ok {
			return Null, fmt.Errorf("framework_SObjectUnitOfWork constructor expects UnresolvedRelationshipBehavior as third argument")
		}
		uow.Fields["m_unresolvedRelationshipBehaviour"] = frameworkSObjectUnitOfWorkRelationshipBehaviorValue(behavior)
	}
	for _, sObjectType := range args[0].List {
		if _, handled, err := vm.callFrameworkSObjectUnitOfWorkHandleRegisterType(uow, []Value{sObjectType}); err != nil || !handled {
			if err != nil {
				return Null, err
			}
			return Null, fmt.Errorf("framework_SObjectUnitOfWork constructor expects Schema.SObjectType entries")
		}
	}
	uow.Fields["m_relationships"] = frameworkMapPut(uow.Fields["m_relationships"], String("Messaging.SingleEmailMessage"), frameworkUnitOfWorkRelationships())
	return uow, nil
}

func frameworkSObjectUnitOfWorkRelationshipBehaviorName(value Value) (string, bool) {
	text := strings.TrimSpace(value.Text)
	if text == "" && value.Kind == ValueString {
		text = strings.TrimSpace(value.Text)
	}
	switch strings.ToLower(text) {
	case "attemptresolveoutoforder":
		return "AttemptResolveOutOfOrder", true
	case "ignoreoutoforder":
		return "IgnoreOutOfOrder", true
	default:
		return "", false
	}
}

func frameworkSObjectUnitOfWorkRelationshipBehaviorValue(name string) Value {
	return Value{Kind: ValueObject, Type: "framework_SObjectUnitOfWork.UnresolvedRelationshipBehavior", Text: name}
}

func frameworkSObjectUnitOfWorkAttemptsOutOfOrder(receiver Value) bool {
	if behavior, ok := receiver.Fields["m_unresolvedRelationshipBehaviour"]; ok {
		name, ok := frameworkSObjectUnitOfWorkRelationshipBehaviorName(behavior)
		return ok && strings.EqualFold(name, "AttemptResolveOutOfOrder")
	}
	return false
}

func (vm *VM) commitFrameworkSObjectUnitOfWork(receiver Value, result *Result) error {
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onCommitWorkStarting", nil, result); err != nil {
		return err
	}
	wasSuccessful := false
	defer func() {
		_ = vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onCommitWorkFinished", []Value{Bool(wasSuccessful)}, result)
	}()
	fail := func(cause error) error {
		if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterFailureEventsStarting", nil, result); err != nil {
			return err
		}
		if err := vm.publishFrameworkSObjectUnitOfWorkEvents(receiver, "m_publishAfterFailureListByType", result); err != nil {
			return err
		}
		if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterFailureEventsFinished", nil, result); err != nil {
			return err
		}
		return cause
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishBeforeEventsStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.publishFrameworkSObjectUnitOfWorkEvents(receiver, "m_publishBeforeListByType", result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishBeforeEventsFinished", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDMLStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_newListByType", "insert", result); err != nil {
		return fail(err)
	}
	if frameworkSObjectUnitOfWorkAttemptsOutOfOrder(receiver) {
		if err := vm.updateFrameworkSObjectUnitOfWorkOutOfOrderRelationships(receiver, result); err != nil {
			return fail(err)
		}
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_dirtyMapByType", "update", result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkDML(receiver, "m_deletedMapByType", "delete", result); err != nil {
		return fail(err)
	}
	if err := vm.applyFrameworkSObjectUnitOfWorkRecordAction(receiver, "m_emptyRecycleBinMapByType", result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDMLFinished", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDoWorkStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.runFrameworkSObjectUnitOfWorkWorkItems(receiver, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onDoWorkFinished", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onCommitWorkFinishing", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterSuccessEventsStarting", nil, result); err != nil {
		return fail(err)
	}
	if err := vm.publishFrameworkSObjectUnitOfWorkEvents(receiver, "m_publishAfterSuccessListByType", result); err != nil {
		return fail(err)
	}
	if err := vm.callFrameworkSObjectUnitOfWorkLifecycle(receiver, "onPublishAfterSuccessEventsFinished", nil, result); err != nil {
		return fail(err)
	}
	wasSuccessful = true
	return nil
}

func (vm *VM) callFrameworkSObjectUnitOfWorkLifecycle(receiver Value, methodName string, args []Value, result *Result) error {
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, methodName, args)
	if ambiguous {
		return vm.ambiguousOverloadError(receiver.Type+"."+methodName, args)
	}
	if !ok {
		return nil
	}
	_, err := vm.callMethodWithReceiver(method, receiver, args, result)
	return err
}

func (vm *VM) replayFrameworkSObjectUnitOfWorkTypeRegistrations(receiver Value) error {
	typesField, ok := receiver.Fields["m_newListByType"]
	if !ok || typesField.Kind != ValueMap {
		return nil
	}
	onRegisterArgs := []Value{sObjectTypeToken("")}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(receiver.Type, "onRegisterType", onRegisterArgs)
	if ambiguous || !ok || vm.isSObjectUnitOfWorkBaseType(method.ClassName) {
		return nil
	}
	keys := typesField.MapOrder
	if len(keys) == 0 {
		for key := range typesField.Map {
			keys = append(keys, key)
		}
	}
	result := &Result{}
	for _, key := range keys {
		name, exists := typesField.MapKeys[key]
		if !exists || name.Kind != ValueString {
			continue
		}
		if _, err := vm.callMethodWithReceiver(method, receiver, []Value{sObjectTypeToken(name.Text)}, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) applyFrameworkSObjectUnitOfWorkDML(receiver Value, fieldName, op string, result *Result) error {
	buckets := frameworkSObjectUnitOfWorkRecordBuckets(receiver, fieldName)
	if op == "delete" {
		reverseFrameworkSObjectUnitOfWorkRecordBuckets(buckets)
	}
	for _, bucket := range buckets {
		records := bucket.records
		if len(records.List) == 0 {
			continue
		}
		if op == "insert" || op == "update" {
			if err := vm.resolveFrameworkSObjectUnitOfWorkRelationships(receiver, bucket.objectName, result); err != nil {
				return err
			}
		}
		if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, op, records, result); err != nil {
			return err
		} else if handled {
			// Custom IDML implementations own persistence behavior for this operation.
			continue
		}
		if _, err := vm.executeDatabaseDML(op, []Value{records}, result); err != nil {
			return err
		}
	}
	return nil
}

func reverseFrameworkSObjectUnitOfWorkRecordBuckets(buckets []frameworkSObjectUnitOfWorkRecordBucket) {
	for i, j := 0, len(buckets)-1; i < j; i, j = i+1, j-1 {
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}
}

func frameworkCustomDMLHandlesPersistence(receiver Value) bool {
	dmlValue, ok := receiver.Fields["m_dml"]
	if !ok || dmlValue.Kind != ValueObject {
		return false
	}
	_, hasOptions := dmlValue.Fields["dmlOptions"]
	_, hasParsedErrors := dmlValue.Fields["parsedErrors"]
	return hasOptions && hasParsedErrors
}

func frameworkSObjectUnitOfWorkRecordsHaveIDs(records Value) bool {
	if records.Kind != ValueList || len(records.List) == 0 {
		return false
	}
	for _, record := range records.List {
		if sObjectIDValue(record).Kind == ValueNull {
			return false
		}
	}
	return true
}

type frameworkSObjectUnitOfWorkRecordBucket struct {
	objectName string
	records    Value
}

func frameworkSObjectUnitOfWorkRecordBuckets(receiver Value, fieldName string) []frameworkSObjectUnitOfWorkRecordBucket {
	field, ok := receiver.Fields[fieldName]
	if !ok || field.Kind != ValueMap {
		return nil
	}
	out := make([]frameworkSObjectUnitOfWorkRecordBucket, 0, len(field.MapOrder)+len(field.Map))
	seen := make(map[string]bool, len(field.Map))
	appendBucket := func(key string, value Value) {
		list := frameworkSObjectUnitOfWorkRecordList(value)
		if list.Kind != ValueList {
			return
		}
		objectName := ""
		if field.MapKeys != nil {
			if keyValue, ok := field.MapKeys[key]; ok && keyValue.Kind == ValueString {
				objectName = keyValue.Text
			}
		}
		if objectName == "" {
			objectName = strings.TrimPrefix(key, "s:")
		}
		out = append(out, frameworkSObjectUnitOfWorkRecordBucket{objectName: objectName, records: list})
	}
	for _, key := range field.MapOrder {
		if value, ok := field.Map[key]; ok {
			appendBucket(key, value)
			seen[key] = true
		}
	}
	for key, value := range field.Map {
		if seen[key] {
			continue
		}
		appendBucket(key, value)
	}
	return out
}

func (vm *VM) applyFrameworkSObjectUnitOfWorkRecordAction(receiver Value, fieldName string, result *Result) error {
	for _, records := range frameworkSObjectUnitOfWorkRecordLists(receiver, fieldName) {
		if len(records.List) == 0 {
			continue
		}
		if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, "emptyRecycleBin", records, result); handled || err != nil {
			return err
		}
		if _, err := vm.executeDatabaseRecordAction("emptyRecycleBin", []Value{records}, result, "Database.EmptyRecycleBinResult"); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) publishFrameworkSObjectUnitOfWorkEvents(receiver Value, fieldName string, result *Result) error {
	for _, records := range frameworkSObjectUnitOfWorkRecordLists(receiver, fieldName) {
		if len(records.List) == 0 {
			continue
		}
		if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, "eventPublish", records, result); handled || err != nil {
			return err
		}
		if _, err := vm.eventBusPublish([]Value{records}, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) callFrameworkSObjectUnitOfWorkCustomDML(receiver Value, op string, records Value, result *Result) (bool, error) {
	dmlValue, ok := receiver.Fields["m_dml"]
	if !ok || dmlValue.Kind != ValueObject || strings.EqualFold(dmlValue.Type, "framework_SObjectUnitOfWork.SimpleDML") {
		return false, nil
	}
	methodName := ""
	switch op {
	case "insert":
		methodName = "dmlInsert"
	case "update":
		methodName = "dmlUpdate"
	case "delete":
		methodName = "dmlDelete"
	case "eventPublish", "emptyRecycleBin":
		methodName = op
	default:
		return false, nil
	}
	methodArgs := []Value{records}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(dmlValue.Type, methodName, methodArgs)
	if !ok && !ambiguous {
		if widened, err := coerceCollectionValue("List<SObject>", records); err == nil {
			methodArgs = []Value{widened}
			method, ok, ambiguous = vm.resolveInstanceMethodForArgs(dmlValue.Type, methodName, methodArgs)
		}
	}
	if ambiguous {
		return true, vm.ambiguousOverloadError(dmlValue.Type+"."+methodName, methodArgs)
	}
	if !ok {
		return false, nil
	}
	_, err := vm.callMethodWithReceiver(method, dmlValue, methodArgs, result)
	return true, err
}

func (vm *VM) resolveFrameworkSObjectUnitOfWorkRelationships(receiver Value, objectName string, result *Result) error {
	bucket, ok := receiver.Fields["m_relationships"]
	if !ok || bucket.Kind != ValueMap {
		return nil
	}
	relationshipsValue, ok := bucket.Map[vm.resolveObjectBucketKey(bucket, objectName)]
	if !ok || relationshipsValue.Kind != ValueObject {
		return nil
	}
	_, relationships, ok := objectFieldValue(relationshipsValue, "m_relationships")
	if !ok || relationships.Kind != ValueList {
		return nil
	}
	for _, relationship := range relationships.List {
		if relationship.Kind != ValueObject {
			continue
		}
		if strings.EqualFold(relationship.Type, "framework_SObjectUnitOfWork.Relationship") {
			if err := vm.resolveFrameworkSObjectUnitOfWorkRelationship(receiver, relationship); err != nil {
				return err
			}
			continue
		}
		method, ok, ambiguous := vm.resolveInstanceMethodForArgs(relationship.Type, "resolve", nil)
		if ambiguous {
			return vm.ambiguousOverloadError(relationship.Type+".resolve", nil)
		}
		if !ok {
			continue
		}
		if _, err := vm.callMethodWithReceiver(method, relationship, nil, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) resolveFrameworkSObjectUnitOfWorkRelationship(receiver Value, relationship Value) error {
	_, record, hasRecord := objectFieldValue(relationship, "Record")
	_, relatedToField, hasField := objectFieldValue(relationship, "RelatedToField")
	_, relatedTo, hasRelated := objectFieldValue(relationship, "RelatedTo")
	if !hasRecord || !hasField || !hasRelated || record.Kind != ValueObject || relatedTo.Kind != ValueObject {
		return nil
	}
	relatedID := sObjectIDValue(relatedTo)
	if relatedID.Kind == ValueNull {
		return nil
	}
	fieldName, err := vm.sObjectFieldArg(record.Type, relatedToField)
	if err != nil {
		return err
	}
	previous := snapshotAlias(record)
	setExplicitSObjectField(&record, fieldName, relatedID)
	relationship.Fields["Record"] = record
	replaced, changed := replaceAliasSnapshot(receiver, previous, record, make(map[uint64]bool))
	if changed && replaced.Kind == ValueObject {
		receiver.Fields = replaced.Fields
	}
	vm.propagateAliasSnapshotToScope(vm.Globals, previous, record)
	vm.propagateAliasSnapshotToStatics(previous, record)
	return nil
}

func (vm *VM) updateFrameworkSObjectUnitOfWorkOutOfOrderRelationships(receiver Value, result *Result) error {
	bucket, ok := receiver.Fields["m_relationships"]
	if !ok || bucket.Kind != ValueMap {
		return nil
	}
	updates := typedList("List<SObject>")
	seen := make(map[string]int)
	for _, bucketKey := range bucket.MapOrder {
		relationshipsValue, ok := bucket.Map[bucketKey]
		if !ok || relationshipsValue.Kind != ValueObject {
			continue
		}
		_, relationships, ok := objectFieldValue(relationshipsValue, "m_relationships")
		if !ok || relationships.Kind != ValueList {
			continue
		}
		for _, relationship := range relationships.List {
			updateRecord, ok, err := vm.frameworkSObjectUnitOfWorkOutOfOrderUpdate(relationship)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			updateKey := vm.mapKey(sObjectIDValue(updateRecord))
			if existingIndex, exists := seen[updateKey]; exists {
				updates.List[existingIndex] = mergeSparseSObjectUpdate(updates.List[existingIndex], updateRecord)
				continue
			}
			seen[updateKey] = len(updates.List)
			updates.List = append(updates.List, updateRecord)
		}
	}
	if len(updates.List) == 0 {
		return nil
	}
	if handled, err := vm.callFrameworkSObjectUnitOfWorkCustomDML(receiver, "update", updates, result); handled || err != nil {
		return err
	}
	_, err := vm.executeDatabaseDML("update", []Value{updates}, result)
	return err
}

func (vm *VM) frameworkSObjectUnitOfWorkOutOfOrderUpdate(relationship Value) (Value, bool, error) {
	if relationship.Kind != ValueObject || !strings.EqualFold(relationship.Type, "framework_SObjectUnitOfWork.Relationship") {
		return Null, false, nil
	}
	_, record, hasRecord := objectFieldValue(relationship, "Record")
	_, relatedToField, hasField := objectFieldValue(relationship, "RelatedToField")
	_, relatedTo, hasRelated := objectFieldValue(relationship, "RelatedTo")
	if !hasRecord || !hasField || !hasRelated || record.Kind != ValueObject || relatedTo.Kind != ValueObject {
		return Null, false, nil
	}
	recordID := sObjectIDValue(record)
	relatedID := sObjectIDValue(relatedTo)
	if recordID.Kind == ValueNull || relatedID.Kind == ValueNull {
		return Null, false, nil
	}
	fieldName, err := vm.sObjectFieldArg(record.Type, relatedToField)
	if err != nil {
		return Null, false, err
	}
	current, _, err := vm.callSObjectMember(record, "get", []Value{relatedToField})
	if err != nil {
		return Null, false, err
	}
	if current.Equal(relatedID) {
		return Null, false, nil
	}
	updateRecord := Object(record.Type)
	updateRecord.Fields["Id"] = recordID
	setExplicitSObjectField(&updateRecord, fieldName, relatedID)
	return updateRecord, true, nil
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

func frameworkSObjectUnitOfWorkRecordLists(receiver Value, fieldName string) []Value {
	field, ok := receiver.Fields[fieldName]
	if !ok || field.Kind != ValueMap {
		return nil
	}
	out := make([]Value, 0, len(field.MapOrder)+len(field.Map))
	seen := make(map[string]bool, len(field.Map))
	for _, key := range field.MapOrder {
		if value, ok := field.Map[key]; ok {
			if list := frameworkSObjectUnitOfWorkRecordList(value); list.Kind == ValueList {
				out = append(out, list)
			}
			seen[key] = true
		}
	}
	for key, value := range field.Map {
		if seen[key] {
			continue
		}
		if list := frameworkSObjectUnitOfWorkRecordList(value); list.Kind == ValueList {
			out = append(out, list)
		}
	}
	return out
}

func frameworkSObjectUnitOfWorkRecordList(value Value) Value {
	switch value.Kind {
	case ValueList:
		return value
	case ValueMap:
		list := typedList("List<SObject>")
		if len(value.MapOrder) != 0 {
			seen := make(map[string]bool, len(value.Map))
			for _, key := range value.MapOrder {
				if item, ok := value.Map[key]; ok {
					list.List = append(list.List, item)
					seen[key] = true
				}
			}
			for key, item := range value.Map {
				if !seen[key] {
					list.List = append(list.List, item)
				}
			}
			return list
		}
		for _, item := range value.Map {
			list.List = append(list.List, item)
		}
		return list
	default:
		return Null
	}
}

func (vm *VM) runFrameworkSObjectUnitOfWorkWorkItems(receiver Value, result *Result) error {
	workList, ok := receiver.Fields["m_workList"]
	if !ok || workList.Kind != ValueList {
		workList = typedList("List<framework_SObjectUnitOfWork.IDoWork>")
	}
	if emailWork, ok := receiver.Fields["m_emailWork"]; ok && emailWork.Kind == ValueObject {
		workList.List = append(workList.List, emailWork)
		receiver.Fields["m_workList"] = workList
	}
	for _, work := range workList.List {
		if work.Kind != ValueObject {
			continue
		}
		if _, handled, err := vm.callFrameworkSObjectUnitOfWorkMember(work, "doWork", nil, result); handled || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		method, ok, ambiguous := vm.resolveInstanceMethodForArgs(work.Type, "doWork", nil)
		if ambiguous {
			return vm.ambiguousOverloadError(work.Type+".doWork", nil)
		}
		if !ok {
			continue
		}
		if _, err := vm.callMethodWithReceiver(method, work, nil, result); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) callFrameworkQueryFactoryMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if !strings.EqualFold(receiver.Type, "framework_QueryFactory.Ordering") {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "getfield":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_QueryFactory.Ordering.getField expects 0 arguments")
		}
		_, field, _ := objectFieldValue(receiver, "field")
		return field, true, nil
	case "getdirection":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_QueryFactory.Ordering.getDirection expects 0 arguments")
		}
		_, direction, _ := objectFieldValue(receiver, "direction")
		return direction, true, nil
	case "tosoql":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("framework_QueryFactory.Ordering.toSOQL expects 0 arguments")
		}
		field := ""
		if _, value, ok := objectFieldValue(receiver, "field"); ok {
			field = scalarText(value)
		}
		direction := "DESC"
		if _, value, ok := objectFieldValue(receiver, "direction"); ok && frameworkQueryFactorySortAscending(value) {
			direction = "ASC"
		}
		nulls := " NULLS FIRST "
		if _, value, ok := objectFieldValue(receiver, "nullsLast"); ok && value.Kind == ValueBool && value.Bool {
			nulls = " NULLS LAST "
		}
		return String(field + " " + direction + nulls), true, nil
	default:
		return Null, false, nil
	}
}

func frameworkQueryFactorySortAscending(value Value) bool {
	text := value.Text
	if text == "" {
		text = value.String()
	}
	return strings.EqualFold(text, "ASCENDING") || strings.EqualFold(text, "framework_QueryFactory.SortOrder.ASCENDING")
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

func frameworkUnitOfWorkRelationships() Value {
	relationships := Object("framework_SObjectUnitOfWork.Relationships")
	relationships.Fields["m_relationships"] = typedList("List<framework_SObjectUnitOfWork.IRelationship>")
	return relationships
}

func frameworkSObjectUnitOfWorkRelationshipsType(typeName string) bool {
	return strings.EqualFold(typeName, "framework_SObjectUnitOfWork.Relationships") ||
		strings.EqualFold(frameworkMockSupportType(typeName), "SObjectUnitOfWork.Relationships")
}

func sObjectTypeTokenObjectName(value Value) (string, bool) {
	_, objectName, ok := objectFieldValue(value, "object")
	return objectName.Text, ok && objectName.Kind == ValueString
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

func (vm *VM) constructFrameworkFastDTO(typeName string, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	if len(namedArgs) != 0 {
		return Null, false, nil
	}
	switch strings.ToLower(typeName) {
	case "framework_methodargvalues", "fflib_methodargvalues":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, false, nil
		}
		value := Object(typeName)
		value.Fields["argValues"] = args[0]
		return value, true, nil
	case "framework_invocationonmock", "fflib_invocationonmock":
		if len(args) != 3 {
			return Null, false, nil
		}
		value := Object(typeName)
		value.Fields["qm"] = args[0]
		value.Fields["methodArg"] = args[1]
		value.Fields["mockInstance"] = args[2]
		return value, true, nil
	case "framework_qualifiedmethod", "fflib_qualifiedmethod":
		if len(args) != 3 && len(args) != 4 {
			return Null, false, nil
		}
		value := Object(typeName)
		value.Fields["typeName"] = args[0]
		value.Fields["methodName"] = args[1]
		value.Fields["methodArgTypes"] = args[2]
		if len(args) == 4 {
			value.Fields["mockInstance"] = args[3]
		} else {
			value.Fields["mockInstance"] = Null
		}
		return value, true, nil
	default:
		return Null, false, nil
	}
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
		class, ok := vm.Classes[className]
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
		class, ok := vm.Classes[className]
		if !ok {
			continue
		}
		found := false
		for fieldName, field := range class.StaticFields {
			if strings.EqualFold(fieldName, name) {
				vm.captureFrameworkMethodCountRecorderRollback(fieldName, field.Value)
				field.Value = value
				class.StaticFields[fieldName] = field
				vm.rememberStaticValueRefsInField(value, staticFieldRef{ClassName: class.Name, FieldName: fieldName})
				found = true
				break
			}
		}
		if !found {
			if class.StaticFields == nil {
				class.StaticFields = make(map[string]Field)
			}
			class.StaticFields[name] = Field{Name: name, Type: value.Type, Static: true, Value: value, InitialValue: value}
			vm.rememberStaticValueRefsInField(value, staticFieldRef{ClassName: class.Name, FieldName: name})
		}
		vm.Classes[class.Name] = class
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

func (vm *VM) callSObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method,
		"addError", "hasErrors", "getErrors", "get", "put", "putSObject", "isSet", "clear",
		"getPopulatedFieldsAsMap", "getSObjectType", "getSObjects", "getQuickActionName",
		"getAll", "getInstance", "getOrgDefaults", "getValues", "recalculateFormulas",
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

func namespaceTokenInSchemaName(name string) string {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
}
