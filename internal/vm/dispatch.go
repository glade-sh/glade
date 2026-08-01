package vm

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) call(callee string, args []Value, namedArgs map[string]Value, result *Result) (Value, error) {
	vm.markRootCollectionRefsEscaped(args...)
	for _, value := range namedArgs {
		vm.markCollectionRefsEscaped(value)
	}
	if strings.HasPrefix(callee, "new:") {
		return vm.constructValue(strings.TrimPrefix(callee, "new:"), args, namedArgs, result)
	}
	if strings.HasPrefix(callee, "newlit:") {
		return vm.constructValueLiteral(strings.TrimPrefix(callee, "newlit:"), args, namedArgs, result)
	}
	if (callee == "this" || callee == "super") && vm.currentMethod.IsConstructor {
		return vm.callChainedConstructor(callee, args, result)
	}
	originalCallee := callee
	callee = normalizeStaticCallCasing(callee)
	callContextClass := vm.currentClass
	if callContextClass == "" {
		callContextClass = vm.currentMethod.ClassName
	}
	if strings.HasPrefix(strings.ToLower(callee), "schema.sobjecttype.") {
		if value, handled, err := vm.callSchemaSObjectTypePath(callee, args, result); handled || err != nil {
			return value, err
		}
	}
	if vm.shouldUseBuiltinStaticPrecedence(originalCallee, callee) {
		goto platformStaticCall
	}
	if value, handled, err := vm.callBuiltinStaticFieldMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callClassLiteralReceiverMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if !strings.Contains(callee, ".") {
		for _, contextClass := range vm.lookupContextClasses() {
			if method, ok, ambiguous := vm.matchRegisteredStaticMethod(contextClass+"."+callee, args); ok {
				if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(method.ClassName); err != nil {
					return Null, err
				}
				if vm.shouldEnqueueFuture(method) {
					return vm.enqueueFuture(method, args, result)
				}
				return vm.callMethod(method, args, result)
			} else if ambiguous {
				return Null, vm.ambiguousOverloadError(callee, args)
			}
		}
	}
	if vm.calleeStartsWithRuntimeReceiver(callee, args) {
		if value, ok, err := vm.callMember(callee, args, result); ok || err != nil {
			return value, err
		}
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if value, handled, err := vm.callEnumStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if dataWeaveStaticScriptReceiver(className) && strings.EqualFold(methodName, "createScript") {
			return dataWeaveCreateScript(args)
		}
		if value, handled, err := vm.callFrameworkStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if packagedControllerDefaultMethod(className, methodName) || packagedControllerUnsupportedMethod(className, methodName) {
			if value, handled, err := vm.callPackagedControllerStatic(callee, args); handled || err != nil {
				return value, err
			}
		}
		if method, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
		if value, handled, err := vm.callGenericCollectionStaticMember(methodName, args); handled || err != nil {
			return value, err
		}
	}
	if value, ok, err := vm.callMember(callee, args, result); ok || err != nil {
		return value, err
	}
	if callContextClass != "" && !strings.Contains(callee, ".") {
		staticContexts := []string{callContextClass}
		if ns := strings.TrimSpace(vm.currentNamespace); ns != "" && !strings.Contains(callContextClass, ".") {
			staticContexts = append(staticContexts, ns+"."+callContextClass)
		}
		dispatchClass := callContextClass
		receiver := Null
		if this, ok := vm.Globals["this"]; ok {
			receiver = this
			if this.Kind == ValueObject && this.Type != "" {
				dispatchClass = this.Type
			}
		}
		if isStubProxy(receiver) {
			if value, handled, err := vm.callStubProxyMember(receiver, callee, args, result); handled || err != nil {
				return value, err
			}
		}
		if receiver.Kind == ValueNull {
			for _, staticContext := range staticContexts {
				if method, ok, ambiguous := vm.resolveStaticMethodForArgs(staticContext, callee, args); ok {
					if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
						return Null, err
					}
					if err := vm.ensureClassInitialized(method.ClassName); err != nil {
						return Null, err
					}
					if vm.shouldEnqueueFuture(method) {
						return vm.enqueueFuture(method, args, result)
					}
					return vm.callMethod(method, args, result)
				} else if ambiguous {
					return Null, vm.ambiguousOverloadError(callee, args)
				}
			}
			for _, outerClass := range lexicalOuterClasses(callContextClass) {
				if method, ok, ambiguous := vm.resolveStaticMethodForArgs(outerClass, callee, args); ok {
					if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
						return Null, err
					}
					if err := vm.ensureClassInitialized(method.ClassName); err != nil {
						return Null, err
					}
					if vm.shouldEnqueueFuture(method) {
						return vm.enqueueFuture(method, args, result)
					}
					return vm.callMethod(method, args, result)
				} else if ambiguous {
					return Null, vm.ambiguousOverloadError(callee, args)
				}
			}
		}
		if method, ok, ambiguous := vm.resolveInstanceMethodForArgs(dispatchClass, callee, args); ok {
			accessMethod := vm.dispatchAccessMethod(vm.currentClass, method, callee, args)
			if err := vm.checkMemberAccess(accessMethod.ClassName, accessMethod.Access, accessMethod.Name, accessMethod.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethodWithReceiver(method, receiver, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
		if method, ok := vm.firstRegisteredMethodByArity(dispatchClass, callee, args); ok {
			accessMethod := vm.dispatchAccessMethod(vm.currentClass, method, callee, args)
			if err := vm.checkMemberAccess(accessMethod.ClassName, accessMethod.Access, accessMethod.Name, accessMethod.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethodWithReceiver(method, receiver, args, result)
		}
		if receiver.Kind == ValueObject && isExceptionType(receiver.Type) {
			if value, updated, mutated, handled, err := vm.callPlatformObjectMember(receiver, callee, args, result); handled || err != nil {
				if mutated {
					vm.Globals["this"] = updated
				}
				return value, err
			}
		}
		for _, staticContext := range staticContexts {
			if method, ok, ambiguous := vm.resolveStaticMethodForArgs(staticContext, callee, args); ok {
				if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(method.ClassName); err != nil {
					return Null, err
				}
				if vm.shouldEnqueueFuture(method) {
					return vm.enqueueFuture(method, args, result)
				}
				return vm.callMethod(method, args, result)
			} else if ambiguous {
				return Null, vm.ambiguousOverloadError(callee, args)
			}
		}
		for _, outerClass := range lexicalOuterClasses(callContextClass) {
			if method, ok, ambiguous := vm.resolveStaticMethodForArgs(outerClass, callee, args); ok {
				if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(method.ClassName); err != nil {
					return Null, err
				}
				if vm.shouldEnqueueFuture(method) {
					return vm.enqueueFuture(method, args, result)
				}
				return vm.callMethod(method, args, result)
			} else if ambiguous {
				return Null, vm.ambiguousOverloadError(callee, args)
			}
		}
	}
	if method, ok, ambiguous := vm.matchRegisteredMethod(callee, args); ok {
		if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
			return Null, err
		}
		if err := vm.ensureClassInitialized(method.ClassName); err != nil {
			return Null, err
		}
		if vm.shouldEnqueueFuture(method) {
			return vm.enqueueFuture(method, args, result)
		}
		return vm.callMethod(method, args, result)
	} else if ambiguous {
		return Null, vm.ambiguousOverloadError(callee, args)
	}
	if value, handled, err := vm.callSchemaSObjectTypePath(callee, args, result); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callStaticPropertyReceiverMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callDottedReceiverMember(callee, args, result); handled || err != nil {
		return value, err
	}
	if dot := strings.LastIndex(callee, "."); dot > 0 && dot < len(callee)-1 {
		typeName, methodName := callee[:dot], callee[dot+1:]
		if value, handled, err := vm.callFrameworkStaticMember(typeName, methodName, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callManagedAPIChain(callee, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callManagedSingletonChain(callee, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callManagedStaticFactory(typeName, methodName, args, result); handled || err != nil {
			return value, err
		}
		if _, classExists := vm.resolveClassName(typeName); !classExists {
			if value, handled, err := vm.callSObjectTypeStaticMember(typeName, methodName, args); handled || err != nil {
				return value, err
			}
		}
		if value, handled, err := vm.callCustomDataStaticMember(callee[:dot], callee[dot+1:], args); handled || err != nil {
			return value, err
		}
	}
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if value, handled, err := vm.callConnectAPICommunitiesStatic(className, methodName, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callCustomDataStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callEnumStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callFrameworkStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
		if method, ok, ambiguous := vm.resolveStaticMethodForArgs(className, methodName, args); ok {
			if err := vm.checkMemberAccess(method.ClassName, method.Access, method.Name, method.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(method.ClassName); err != nil {
				return Null, err
			}
			if vm.shouldEnqueueFuture(method) {
				return vm.enqueueFuture(method, args, result)
			}
			return vm.callMethod(method, args, result)
		} else if ambiguous {
			return Null, vm.ambiguousOverloadError(callee, args)
		}
		if value, handled, err := vm.callGenericCollectionStaticMember(methodName, args); handled || err != nil {
			return value, err
		}
		if value, handled, err := vm.callSObjectTypeStaticMember(className, methodName, args); handled || err != nil {
			return value, err
		}
	}
	if typeName, memberName, ok := splitDottedTypeMember(callee); ok && strings.EqualFold(typeName, "Search") {
		switch {
		case strings.EqualFold(memberName, "query"):
			return vm.searchQuery(args)
		case strings.EqualFold(memberName, "find"):
			return vm.searchFind(args)
		case strings.EqualFold(memberName, "suggest"):
			return vm.searchSuggest(args)
		}
		return Null, unsupportedCallError(callee + " local search/SOSL surface")
	}
platformStaticCall:
	callee = normalizeStaticCallCasing(callee)
	if className, methodName, ok := vm.splitClassMember(callee); ok {
		if value, handled, err := vm.callConnectAPICommunitiesStatic(className, methodName, args); handled || err != nil {
			return value, err
		}
	}
	if value, handled, err := vm.callConnectAPIPrimaryUsageStaticOutcome(callee, args); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callConnectAPITestFixtureStatic(callee, args); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callConnectAPILocalStatic(callee, args); handled || err != nil {
		return value, err
	}
	if value, handled := vm.callConnectAPIReadOnlyStaticDefault(callee, args); handled {
		return value, nil
	}
	if value, handled, err := vm.callPushUpgradeCustomizationRepository(callee, args); handled || err != nil {
		return value, err
	}
	if strings.EqualFold(callee, "WebStoreContext.getCommerceContext") {
		return Null, unsupportedCallError("WebStoreContext.getCommerceContext local commerce context service")
	}
	if value, handled, err := vm.callAppLauncherControllerStatic(callee, args); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callIndustryControllerStatic(callee, args); handled || err != nil {
		return value, err
	}
	if value, handled, err := vm.callPackagedControllerStatic(callee, args); handled || err != nil {
		return value, err
	}
	if strings.EqualFold(callee, "Approval.process") {
		return vm.executeApprovalProcess(args)
	}
	if reason, ok := unsupportedIntegrationSurface(callee); ok {
		return Null, unsupportedCallError(callee + " " + reason)
	}
	if reason, ok := unsupportedCoreStaticSurface(callee); ok {
		return Null, unsupportedCallError(callee + " " + reason)
	}
	if strings.HasPrefix(callee, "Limits.") && unsupportedLimitGetter(strings.TrimPrefix(callee, "Limits.")) {
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Null, unsupportedCallError(callee)
	}
	if strings.HasPrefix(callee, "Limits.") {
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if value, ok := vm.limitValue(strings.TrimPrefix(callee, "Limits.")); ok {
			return value, nil
		}
	}
	if value, handled, err := vm.callSystemLabelStatic(callee, args); handled || err != nil {
		return value, err
	}
	if strings.EqualFold(callee, "eventbus.TriggerContext.currentContext") {
		if len(args) != 0 {
			return Null, fmt.Errorf("eventbus.TriggerContext.currentContext expects 0 arguments")
		}
		return Object("eventbus.TriggerContext"), nil
	}

	switch callee {
	case "Datacloud.FindDuplicates.findDuplicates":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("Datacloud.FindDuplicates.findDuplicates expects List<SObject>")
		}
		results := make([]Value, 0, len(args[0].List))
		for _, record := range args[0].List {
			if record.Kind != ValueObject || !vm.isSObjectLikeType(record.Type) {
				return Null, fmt.Errorf("Datacloud.FindDuplicates.findDuplicates expects List<SObject>")
			}
			results = append(results, newDatacloudFindDuplicatesResult())
		}
		return List(results...), nil
	case "Datacloud.FindDuplicatesByIds.findDuplicatesByIds":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("Datacloud.FindDuplicatesByIds.findDuplicatesByIds expects List<Id>")
		}
		results := make([]Value, 0, len(args[0].List))
		for _, id := range args[0].List {
			if _, ok := idValueText(id); !ok {
				return Null, fmt.Errorf("Datacloud.FindDuplicatesByIds.findDuplicatesByIds expects List<Id>")
			}
			results = append(results, newDatacloudFindDuplicatesResult())
		}
		return List(results...), nil
	case "System.assert":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("System.assert expects 1 or 2 arguments")
		}
		if args[0].Kind != ValueBool {
			return Null, fmt.Errorf("System.assert expects Boolean, got %s", args[0].Kind)
		}
		if !args[0].Bool {
			message, err := vm.assertMessage("assertion failed", args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isFalse":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Assert.isFalse expects 1 or 2 arguments")
		}
		if args[0].Kind != ValueBool {
			return Null, fmt.Errorf("Assert.isFalse expects Boolean, got %s", args[0].Kind)
		}
		if args[0].Bool {
			message, err := vm.assertMessage("assertion failed", args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isNull":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Assert.isNull expects 1 or 2 arguments")
		}
		if args[0].Kind != ValueNull {
			value, err := vm.displayString(args[0], result)
			if err != nil {
				return Null, err
			}
			message, err := vm.assertMessage(fmt.Sprintf("expected null, actual <%s>", value), args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isNotNull":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Assert.isNotNull expects 1 or 2 arguments")
		}
		if args[0].Kind == ValueNull {
			message, err := vm.assertMessage("value should not be null", args[1:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isInstanceOfType":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("Assert.isInstanceOfType expects value, Type[, message]")
		}
		if args[1].Kind != ValueObject || args[1].Type != "Type" {
			return Null, fmt.Errorf("Assert.isInstanceOfType expects Type as second argument")
		}
		expectedType := typeValueName(args[1])
		actualType := valueTypeName(args[0])
		if args[0].Kind == ValueObject {
			actualType = runtimeObjectType(args[0])
		}
		matches := args[0].Kind != ValueNull && vm.typeMatches(actualType, expectedType, make(map[string]bool))
		if !matches {
			message, err := vm.assertMessage(fmt.Sprintf("expected instance of <%s>, actual <%s>", expectedType, actualType), args[2:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.isNotInstanceOfType":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("Assert.isNotInstanceOfType expects value, Type[, message]")
		}
		if args[1].Kind != ValueObject || args[1].Type != "Type" {
			return Null, fmt.Errorf("Assert.isNotInstanceOfType expects Type as second argument")
		}
		expectedType := typeValueName(args[1])
		actualType := valueTypeName(args[0])
		if args[0].Kind == ValueObject {
			actualType = runtimeObjectType(args[0])
		}
		matches := args[0].Kind != ValueNull && vm.typeMatches(actualType, expectedType, make(map[string]bool))
		if matches {
			message, err := vm.assertMessage(fmt.Sprintf("expected not instance of <%s>, actual <%s>", expectedType, actualType), args[2:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "Assert.fail":
		if len(args) > 1 {
			return Null, fmt.Errorf("Assert.fail expects 0 or 1 arguments")
		}
		message, err := vm.assertMessage("assertion failed", args, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError(message)
	case "System.assertEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertEquals expects 2 or 3 arguments")
		}
		equal, err := vm.apexEquals(args[0], args[1], result)
		if err != nil {
			return Null, err
		}
		if !equal && args[0].Kind == ValueString && args[1].Kind == ValueString {
			equal = vm.equalCurrentNamespaceApexStubText(args[0].Text, args[1].Text)
		}
		if !equal {
			expected, err := vm.displayString(args[0], result)
			if err != nil {
				return Null, err
			}
			actual, err := vm.displayString(args[1], result)
			if err != nil {
				return Null, err
			}
			message, err := vm.assertMessage(fmt.Sprintf("expected <%s>, actual <%s>", expected, actual), args[2:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "System.assertNotEquals":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("System.assertNotEquals expects 2 or 3 arguments")
		}
		equal, err := vm.apexEquals(args[0], args[1], result)
		if err != nil {
			return Null, err
		}
		if equal {
			value, err := vm.displayString(args[0], result)
			if err != nil {
				return Null, err
			}
			message, err := vm.assertMessage(fmt.Sprintf("values should not be equal: <%s>", value), args[2:], result)
			if err != nil {
				return Null, err
			}
			return Null, vm.assertError(message)
		}
		return Null, nil
	case "System.equals":
		if len(args) != 2 {
			return Null, fmt.Errorf("System.equals expects 2 arguments")
		}
		equal, err := vm.apexEquals(args[0], args[1], result)
		if err != nil {
			return Null, err
		}
		return Bool(equal), nil
	case "System.hashCode":
		if len(args) != 1 {
			return Null, fmt.Errorf("System.hashCode expects 1 argument")
		}
		return Int(int64(valueHashCode(args[0]))), nil
	case "System.debug":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("System.debug expects message or logging level and message")
		}
		messageArg := args[0]
		level := "DEBUG"
		if len(args) == 2 {
			if !isLoggingLevelValue(args[0]) {
				return Null, fmt.Errorf("System.debug expects LoggingLevel as first argument")
			}
			level = args[0].Text
			messageArg = args[1]
		}
		line, err := vm.displayString(messageArg, result)
		if err != nil {
			return Null, err
		}
		result.Debug = append(result.Debug, line)
		event := DebugEvent{
			Level:    level,
			Message:  line,
			TracePos: len(result.Trace),
		}
		if result.traceEnabled {
			result.DebugEvents = append(result.DebugEvents, event)
		}
		if vm.debugOutputSink != nil {
			vm.debugOutputSink(event)
		}
		if vm.Stdout != nil {
			fmt.Fprintln(vm.Stdout, line)
		}
		return Null, nil
	case "Database.query":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Database.query expects query String and optional AccessLevel")
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.query expects query String")
		}
		if len(args) == 2 && !isDatabaseAccessLevelValue(args[1]) {
			return Null, fmt.Errorf("Database.query expects AccessLevel")
		}
		if len(args) == 2 {
			return vm.executeSOQLWithAccessLevel(args[0].Text, args[1], result)
		}
		return vm.executeSOQL(args[0].Text, result)
	case "Database.queryWithBinds":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("Database.queryWithBinds expects query String, bind Map, and optional AccessLevel")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("Database.queryWithBinds expects query String and bind Map")
		}
		if len(args) == 3 && (args[2].Kind != ValueObject || args[2].Type != "AccessLevel") {
			return Null, fmt.Errorf("Database.queryWithBinds expects AccessLevel")
		}
		if len(args) == 3 {
			return vm.executeSOQLWithBindMapAccessLevel(args[0].Text, args[1], args[2], result)
		}
		return vm.executeSOQLWithBindMap(args[0].Text, args[1], result)
	case "Database.countQuery":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Database.countQuery expects query String and optional AccessLevel")
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.countQuery expects query String")
		}
		if len(args) == 2 && !isDatabaseAccessLevelValue(args[1]) {
			return Null, fmt.Errorf("Database.countQuery expects AccessLevel")
		}
		var value Value
		var err error
		if len(args) == 2 {
			value, err = vm.executeSOQLWithAccessLevel(args[0].Text, args[1], result)
		} else {
			value, err = vm.executeSOQL(args[0].Text, result)
		}
		if err != nil {
			return Null, err
		}
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
		return Int(int64(len(value.List))), nil
	case "Database.countQueryWithBinds":
		if len(args) != 3 {
			return Null, fmt.Errorf("Database.countQueryWithBinds expects query String, bind Map, and AccessLevel")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("Database.countQueryWithBinds expects query String and bind Map")
		}
		if !isDatabaseAccessLevelValue(args[2]) {
			return Null, fmt.Errorf("Database.countQueryWithBinds expects AccessLevel")
		}
		value, err := vm.executeSOQLWithBindMapAccessLevel(args[0].Text, args[1], args[2], result)
		if err != nil {
			return Null, err
		}
		if count, ok := aggregateCount(value); ok {
			return count, nil
		}
		return Int(int64(len(value.List))), nil
	case "Database.getQueryLocator":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Database.getQueryLocator expects query String or inline SOQL and optional AccessLevel")
		}
		if args[0].Kind != ValueString && args[0].Kind != ValueList {
			return Null, fmt.Errorf("Database.getQueryLocator expects query String or inline SOQL")
		}
		if len(args) == 2 && !isDatabaseAccessLevelValue(args[1]) {
			return Null, fmt.Errorf("Database.getQueryLocator expects AccessLevel")
		}
		query := ""
		value := args[0]
		if args[0].Kind == ValueString {
			var err error
			query = args[0].Text
			if len(args) == 2 {
				value, err = vm.executeSOQLWithAccessLevel(args[0].Text, args[1], result)
			} else {
				value, err = vm.executeSOQL(args[0].Text, result)
			}
			if err != nil {
				return Null, err
			}
		} else {
			query = inlineSOQLQueryText(args[0])
		}
		locator := Object("Database.QueryLocator")
		locator.Fields["Records"] = value
		locator.Fields["Query"] = String(query)
		if err := vm.incrementQueryLocatorRows(value); err != nil {
			return Null, err
		}
		return locator, nil
	case "Database.getQueryLocatorWithBinds":
		if len(args) != 3 {
			return Null, fmt.Errorf("Database.getQueryLocatorWithBinds expects query String, bind Map, and AccessLevel")
		}
		if args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("Database.getQueryLocatorWithBinds expects query String and bind Map")
		}
		if !isDatabaseAccessLevelValue(args[2]) {
			return Null, fmt.Errorf("Database.getQueryLocatorWithBinds expects AccessLevel")
		}
		value, err := vm.executeSOQLWithBindMapAccessLevel(args[0].Text, args[1], args[2], result)
		if err != nil {
			return Null, err
		}
		locator := Object("Database.QueryLocator")
		locator.Fields["Records"] = value
		locator.Fields["Query"] = String(args[0].Text)
		if err := vm.incrementQueryLocatorRows(value); err != nil {
			return Null, err
		}
		return locator, nil
	case "Database.getAsyncLocator":
		if len(args) != 1 {
			return Null, fmt.Errorf("Database.getAsyncLocator expects local result or locator")
		}
		return databaseAsyncLocatorValue(args[0]), nil
	case "Database.getCursor", "Database.getPaginationCursor":
		if len(args) < 1 || len(args) > 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects query String and optional cursor options", callee)
		}
		value, err := vm.executeSOQL(args[0].Text, result)
		if err != nil {
			return Null, err
		}
		cursorType := "Database.Cursor"
		if callee == "Database.getPaginationCursor" {
			cursorType = "Database.PaginationCursor"
		}
		cursor := Object(cursorType)
		cursor.Fields["Records"] = value
		cursor.Fields["Query"] = args[0]
		return cursor, nil
	case "Database.getCursorWithBinds", "Database.getPaginationCursorWithBinds":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("%s expects query String, bind Map, and cursor options", callee)
		}
		value, err := vm.executeSOQLWithBindMap(args[0].Text, args[1], result)
		if err != nil {
			return Null, err
		}
		cursorType := "Database.Cursor"
		if callee == "Database.getPaginationCursorWithBinds" {
			cursorType = "Database.PaginationCursor"
		}
		cursor := Object(cursorType)
		cursor.Fields["Records"] = value
		cursor.Fields["Query"] = args[0]
		return cursor, nil
	case "Security.stripInaccessible":
		if len(args) < 2 || len(args) > 4 {
			return Null, fmt.Errorf("Security.stripInaccessible expects AccessType, records, and optional enforceRootObjectCRUD")
		}
		if args[0].Kind != ValueObject || args[0].Type != "AccessType" {
			return Null, fmt.Errorf("Security.stripInaccessible expects AccessType")
		}
		if args[1].Kind != ValueList {
			return Null, fmt.Errorf("Security.stripInaccessible expects List<SObject>")
		}
		if len(args) >= 3 && args[2].Kind != ValueBool {
			return Null, fmt.Errorf("Security.stripInaccessible expects Boolean enforceRootObjectCRUD")
		}
		scopedPermissionSetID := ""
		if len(args) == 4 && args[3].Kind != ValueNull {
			var ok bool
			scopedPermissionSetID, ok = typedIDValueText(args[3])
			if !ok {
				return Null, fmt.Errorf("Security.stripInaccessible expects Id permissionSetId")
			}
		}
		enforceRootObjectCRUD := true
		if len(args) >= 3 {
			enforceRootObjectCRUD = args[2].Bool
		}
		records, removedFields, modifiedIndexes, err := vm.stripInaccessibleRecords(args[0].Text, args[1], enforceRootObjectCRUD, scopedPermissionSetID)
		if err != nil {
			return Null, err
		}
		decision := Object("SObjectAccessDecision")
		decision.Fields["records"] = records
		decision.Fields["removedFields"] = removedFields
		decision.Fields["modifiedIndexes"] = modifiedIndexes
		return decision, nil
	case "Database.setSavepoint":
		if len(args) != 0 {
			return Null, fmt.Errorf("Database.setSavepoint expects 0 arguments")
		}
		if vm.Org == nil {
			return Null, fmt.Errorf("Database.setSavepoint requires org storage")
		}
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return Null, err
		}
		if err := vm.incrementLimit("savepoints", 1); err != nil {
			return Null, err
		}
		vm.nextSavepoint++
		id := fmt.Sprintf("sp-%d", vm.nextSavepoint)
		if vm.isolationJournal != nil {
			vm.savepointMarks[id] = vm.isolationJournal.Mark()
		} else {
			vm.savepoints[id] = snapshotRuntimeOrgState(vm.Org)
		}
		vm.emailSavepoints[id] = append([]CapturedEmail(nil), vm.capturedEmails...)
		vm.savepointOrder[id] = vm.nextSavepoint
		savepoint := Object("System.Savepoint")
		savepoint.Fields["Id"] = String(id)
		return savepoint, nil
	case "Database.rollback":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "System.Savepoint" {
			return Null, fmt.Errorf("Database.rollback expects Savepoint")
		}
		if vm.Org == nil {
			return Null, fmt.Errorf("Database.rollback requires org storage")
		}
		idValue, ok := args[0].Fields["Id"]
		if !ok || idValue.Kind != ValueString {
			return Null, fmt.Errorf("Database.rollback received invalid Savepoint")
		}
		targetOrder, ok := vm.savepointOrder[idValue.Text]
		if !ok {
			return Null, fmt.Errorf("Database.rollback received invalid Savepoint")
		}
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return Null, err
		}
		currentSequences := copyOrgIDSequences(vm.Org.IDSequences)
		if mark, ok := vm.savepointMarks[idValue.Text]; ok {
			if vm.isolationJournal == nil {
				return Null, fmt.Errorf("Database.rollback received invalid Savepoint")
			}
			if err := vm.isolationJournal.Rollback(mark); err != nil {
				return Null, err
			}
			vm.applyMaxIDSequencesForJournalRollback(currentSequences)
		} else {
			snapshot, ok := vm.savepoints[idValue.Text]
			if !ok {
				return Null, fmt.Errorf("Database.rollback received invalid Savepoint")
			}
			restored := cloneRuntimeOrgState(snapshot)
			restored.IDSequences = maxOrgIDSequences(restored.IDSequences, currentSequences)
			*vm.Org = restored
		}
		vm.capturedEmails = append([]CapturedEmail(nil), vm.emailSavepoints[idValue.Text]...)
		if err := vm.incrementLimit("savepointRollbacks", 1); err != nil {
			return Null, err
		}
		vm.clearCustomDataCache()
		for id, order := range vm.savepointOrder {
			if order > targetOrder {
				delete(vm.savepoints, id)
				delete(vm.savepointMarks, id)
				delete(vm.emailSavepoints, id)
				delete(vm.savepointOrder, id)
			}
		}
		return Null, nil
	case "Database.releaseSavepoint":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "System.Savepoint" {
			return Null, fmt.Errorf("Database.releaseSavepoint expects Savepoint")
		}
		idValue, ok := args[0].Fields["Id"]
		if !ok || idValue.Kind != ValueString {
			return Null, fmt.Errorf("Database.releaseSavepoint received invalid Savepoint")
		}
		targetOrder, ok := vm.savepointOrder[idValue.Text]
		if !ok {
			return Null, fmt.Errorf("Database.releaseSavepoint received invalid Savepoint")
		}
		for id, order := range vm.savepointOrder {
			if order >= targetOrder {
				delete(vm.savepoints, id)
				delete(vm.savepointMarks, id)
				delete(vm.emailSavepoints, id)
				delete(vm.savepointOrder, id)
			}
		}
		return Null, nil
	case "Database.upsert", "Database.undelete":
		return vm.executeDatabaseDML(strings.TrimPrefix(callee, "Database."), args, result)
	case "Database.insert", "Database.update", "Database.delete":
		return vm.executeDatabaseDML(strings.TrimPrefix(callee, "Database."), args, result)
	case "Database.insertAsync", "Database.updateAsync", "Database.deleteAsync":
		op := strings.TrimPrefix(callee, "Database.")
		op = strings.TrimSuffix(op, "Async")
		return vm.executeDatabaseAsyncDML(op, args, result)
	case "Database.insertImmediate", "Database.updateImmediate", "Database.deleteImmediate":
		op := strings.TrimPrefix(callee, "Database.")
		op = strings.TrimSuffix(op, "Immediate")
		return vm.executeDatabaseDML(op, args, result)
	case "Database.getAsyncSaveResult", "Database.getAsyncDeleteResult":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s expects async operation id or local result", callee)
		}
		if args[0].Kind == ValueObject || args[0].Kind == ValueList {
			return args[0], nil
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects async operation id or local result", callee)
		}
		return Null, newExceptionError("AsyncException", fmt.Sprintf("%s unknown async DML result locator %q", callee, args[0].Text))
	case "Database.getDeleted":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.getDeleted expects object name String, start Datetime, and end Datetime")
		}
		return vm.databaseGetDeleted(args)
	case "Database.getUpdated":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Database.getUpdated expects object name String, start Datetime, and end Datetime")
		}
		return vm.databaseGetUpdated(args)
	case "Database.emptyRecycleBin":
		return vm.executeDatabaseRecordAction("emptyRecycleBin", args, result, "Database.EmptyRecycleBinResult")
	case "Database.lock", "Database.unlock":
		op := strings.TrimPrefix(callee, "Database.")
		return vm.executeDatabaseRecordAction(op, args, result, databaseRecordActionResultType(op))
	case "Database.treeSave":
		return vm.executeDatabaseTreeSave(args, result)
	case "Database.convertLead":
		return vm.executeDatabaseConvertLead(args, result)
	case "Approval.lock":
		return vm.executeDatabaseRecordAction("lock", args, result, "Approval.LockResult")
	case "Approval.unlock":
		return vm.executeDatabaseRecordAction("unlock", args, result, "Approval.UnlockResult")
	case "Approval.isLocked":
		return vm.executeApprovalIsLocked(args)
	case "Approval.process":
		return vm.executeApprovalProcess(args)
	case "Answers.findSimilar":
		if len(args) != 1 {
			return Null, fmt.Errorf("Answers.findSimilar expects Question")
		}
		return typedList("List<Id>"), nil
	case "Database.merge":
		return vm.executeDatabaseMerge(args, result)
	case "Limits.getQueries", "Limits.getLimitQueries", "Limits.getQueryRows", "Limits.getLimitQueryRows",
		"Limits.getDmlStatements", "Limits.getLimitDmlStatements", "Limits.getDmlRows", "Limits.getLimitDmlRows",
		"Limits.getDMLStatements", "Limits.getLimitDMLStatements", "Limits.getDMLRows", "Limits.getLimitDMLRows",
		"Limits.getHeapSize", "Limits.getLimitHeapSize", "Limits.getCpuTime", "Limits.getLimitCpuTime",
		"Limits.getCallouts", "Limits.getLimitCallouts", "Limits.getQueueableJobs", "Limits.getLimitQueueableJobs",
		"Limits.getFutureCalls", "Limits.getLimitFutureCalls", "Limits.getAsyncJobs", "Limits.getLimitAsyncJobs",
		"Limits.getAsyncCalls", "Limits.getLimitAsyncCalls",
		"Limits.getBatchJobs", "Limits.getLimitBatchJobs", "Limits.getScheduledJobs", "Limits.getLimitScheduledJobs",
		"Limits.getEmailInvocations", "Limits.getLimitEmailInvocations",
		"Limits.getAggregateQueries", "Limits.getLimitAggregateQueries",
		"Limits.getFindSimilarCalls", "Limits.getLimitFindSimilarCalls",
		"Limits.getMobilePushApexCalls", "Limits.getLimitMobilePushApexCalls",
		"Limits.getQueryLocatorRows", "Limits.getLimitQueryLocatorRows",
		"Limits.getSavepointRollbacks", "Limits.getLimitSavepointRollbacks",
		"Limits.getSoslQueries", "Limits.getLimitSoslQueries":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if value, ok := vm.limitValue(strings.TrimPrefix(callee, "Limits.")); ok {
			return value, nil
		}
		return Null, unsupportedCallError(callee)
	case "OrgLimits.getAll":
		if len(args) != 0 {
			return Null, fmt.Errorf("OrgLimits.getAll expects 0 arguments")
		}
		return List(vm.orgLimitValues()...), nil
	case "OrgLimits.getMap":
		if len(args) != 0 {
			return Null, fmt.Errorf("OrgLimits.getMap expects 0 arguments")
		}
		out := typedMap("Map<String,OrgLimit>")
		for _, limit := range vm.orgLimitValues() {
			name := limit.Fields["name"]
			out.Map[mapKey(name)] = limit
			out.MapKeys[mapKey(name)] = name
		}
		return out, nil
	case "String.valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("String.valueOf expects 1 argument")
		}
		if args[0].Kind == ValueNull {
			return Value{Kind: ValueNull, Type: "String"}, nil
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Date") {
			text, err := stringValueOfDate(args[0])
			if err != nil {
				return Null, err
			}
			return String(text), nil
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Blob") {
			raw := ""
			if value, ok := args[0].Fields["value"]; ok && value.Kind == ValueString {
				raw = value.Text
			}
			return String(fmt.Sprintf("Blob[%d]", len(raw))), nil
		}
		if args[0].Kind == ValueObject && isPackageVersionValue(args[0]) {
			return String(versionValueString(args[0])), nil
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Datetime") {
			datetimeValue, err := parsePlatformDatetime(args[0])
			if err != nil {
				return Null, err
			}
			_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), datetimeValue)
			if !ok {
				return Null, unsupportedCallError("String.valueOf timezone " + vm.currentUserTimeZoneID())
			}
			return String(local.Format("2006-01-02 15:04:05")), nil
		}
		text, err := vm.displayString(args[0], result)
		if err != nil {
			return Null, err
		}
		text = vm.resolveLabelMergeExpressions(text)
		return String(text), nil
	case "String.format":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueList {
			return Null, fmt.Errorf("String.format expects format String and List arguments")
		}
		pattern := vm.resolveLabelMergeExpressions(args[0].Text)
		formatted, err := formatString(pattern, args[1].List, func(value Value) (string, error) {
			return vm.displayString(value, result)
		})
		if err != nil {
			return Null, err
		}
		return String(formatted), nil
	case "String.isBlank", "String.isNotBlank", "String.isEmpty", "String.isNotEmpty", "String.join", "String.getCommonPrefix", "String.getLevenshteinDistance", "String.stripAll", "String.fromCharArray", "String.escapeSingleQuotes", "String.toLowerCase", "String.toUpperCase":
		return stringStatic(callee, args)
	case "Integer.valueOf", "Long.valueOf", "Decimal.valueOf", "Double.valueOf":
		return numericStatic(callee, args)
	case "Boolean.valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("Boolean.valueOf expects 1 argument")
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		if args[0].Kind == ValueBool {
			return args[0], nil
		}
		if args[0].Kind != ValueString {
			return Null, newExceptionError("System.TypeException", "Boolean.valueOf expects String or Boolean")
		}
		return Bool(strings.EqualFold(strings.TrimSpace(args[0].Text), "true")), nil
	case "AccessLevel.withPermissionSetId":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("AccessLevel.withPermissionSetId expects String")
		}
		value := Value{Kind: ValueObject, Type: "AccessLevel", Text: "USER_MODE", Fields: map[string]Value{}}
		value.Fields["permissionSetId"] = args[0]
		return value, nil
	case "RoundingMode.valueOf":
		return roundingModeStatic(args)
	case "Id.valueOf":
		return idStatic(callee, args)
	case "Pattern.compile":
		return patternCompile(args)
	case "Pattern.matches":
		return patternMatches(args)
	case "Pattern.quote":
		return patternQuote(args)
	case "PageReference.forResource":
		return vm.pageReferenceForResource(args)
	case "Matcher.quoteReplacement":
		return matcherQuoteReplacement(args)
	case "Math.abs", "Math.floor", "Math.ceil", "Math.round", "Math.rint", "Math.roundToLong", "Math.signum", "Math.sqrt", "Math.cbrt",
		"Math.acos", "Math.asin", "Math.atan", "Math.cos", "Math.sin", "Math.tan", "Math.cosh", "Math.sinh", "Math.tanh",
		"Math.exp", "Math.log", "Math.log10":
		return mathUnary(callee, args)
	case "Math.max", "Math.min", "Math.mod", "Math.pow", "Math.atan2":
		return mathBinary(callee, args)
	case "Math.random":
		if len(args) != 0 {
			return Null, fmt.Errorf("Math.random expects 0 arguments")
		}
		return Decimal(0.5), nil
	case "UUID.randomUUID":
		if len(args) != 0 {
			return Null, fmt.Errorf("UUID.randomUUID expects 0 arguments")
		}
		return uuidValue(vm.nextDeterministicUUID()), nil
	case "UUID.fromString":
		if len(args) != 1 {
			return Null, fmt.Errorf("UUID.fromString expects String")
		}
		text := ""
		if args[0].Kind == ValueString {
			text = args[0].Text
		} else if objectText, ok := platformScalarObjectText(args[0]); ok {
			text = objectText
		} else {
			return Null, fmt.Errorf("UUID.fromString expects String")
		}
		if _, err := parseUUIDText(text); err != nil {
			return Null, err
		}
		return uuidValue(strings.ToLower(text)), nil
	case "Date.today", "System.today":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("Date", vm.fakeNow.Format("2006-01-02")), nil
	case "Date.newInstance":
		if len(args) != 3 || args[0].Kind != ValueInt || args[1].Kind != ValueInt || args[2].Kind != ValueInt {
			return Null, fmt.Errorf("Date.newInstance expects year, month, day integers")
		}
		year, month, day := normalizeDateNewInstanceParts(int(args[0].Int), int(args[1].Int), int(args[2].Int))
		value, err := dateFromNewInstanceParts(year, month, day)
		if err != nil {
			return Null, err
		}
		return platformScalar("Date", value.Format("2006-01-02")), nil
	case "Date.daysInMonth":
		if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
			return Null, fmt.Errorf("Date.daysInMonth expects year and month integers")
		}
		month := time.Month(args[1].Int)
		if month < time.January || month > time.December {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("Invalid month: %d", args[1].Int))
		}
		year := int(args[0].Int)
		if year == 0 {
			year = 1
		}
		return Int(int64(daysInMonth(year, month))), nil
	case "Date.isLeapYear":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, fmt.Errorf("Date.isLeapYear expects year Integer")
		}
		year := int(args[0].Int)
		if year == 0 {
			year = 1
		}
		leap := year%4 == 0 && (year%100 != 0 || year%400 == 0)
		return Bool(leap), nil
	case "Date.valueOf":
		if len(args) != 1 {
			return Null, newExceptionError("System.NullPointerException", "Date.valueOf expects String")
		}
		if args[0].Kind == ValueNull {
			return Null, nil
		}
		var text string
		if args[0].Kind == ValueString {
			text = args[0].Text
		} else if objectText, ok := platformScalarObjectText(args[0]); ok {
			text = objectText
		} else {
			return Null, newExceptionError("System.TypeException", "Date.valueOf expects String")
		}
		date, err := parseDateText(text)
		if err != nil {
			return Null, newExceptionError("System.TypeException", "Invalid date: "+text)
		}
		return platformScalar("Date", date.Format("2006-01-02")), nil
	case "Date.parse":
		if len(args) != 1 {
			return Null, newExceptionError("System.NullPointerException", "Date.parse expects String")
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Date.parse expects String")
		}
		if args[0].Kind != ValueString {
			return Null, newExceptionError("System.TypeException", "Date.parse expects String")
		}
		date, err := parseDateParseText(args[0].Text)
		if err != nil {
			return Null, newExceptionError("System.TypeException", "Invalid date: "+args[0].Text)
		}
		return platformScalar("Date", date.Format("2006-01-02")), nil
	case "Datetime.now", "System.now":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if originalType, originalMember, ok := splitDottedTypeMember(originalCallee); ok &&
			originalType == "DateTime" && originalMember == "Now" && vm.hasLastNow {
			return platformScalar("Datetime", vm.lastNow.Format(time.RFC3339)), nil
		}
		now := vm.fakeNow
		vm.lastNow = now
		vm.hasLastNow = true
		vm.fakeNow = vm.fakeNow.Add(time.Second)
		return platformScalar("Datetime", now.Format(time.RFC3339)), nil
	case "System.currentTimeMillis":
		if len(args) != 0 {
			return Null, fmt.Errorf("System.currentTimeMillis expects 0 arguments")
		}
		if vm.hasLastNow {
			return Int(vm.lastNow.UnixMilli()), nil
		}
		return Int(vm.fakeNow.UnixMilli()), nil
	case "System.isBatch", "System.isFuture", "System.isQueueable", "System.isScheduled":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Bool(vm.isAsyncKind(callee)), nil
	case "System.isFunctionCallback", "System.isRunningElasticCompute":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Bool(false), nil
	case "System.getApplicationReadWriteMode":
		if len(args) != 0 {
			return Null, fmt.Errorf("System.getApplicationReadWriteMode expects 0 arguments")
		}
		mode := Object("ApplicationReadWriteMode")
		mode.Fields["value"] = String("READ_WRITE")
		return mode, nil
	case "System.getQuiddityShortCode":
		if len(args) != 1 {
			return Null, fmt.Errorf("System.getQuiddityShortCode expects Quiddity")
		}
		name := strings.ToUpper(strings.TrimSpace(typeValueText(args[0])))
		if name == "" && args[0].Kind == ValueString {
			name = strings.ToUpper(strings.TrimSpace(args[0].Text))
		}
		return String(quiddityShortCode(name)), nil
	case "System.requestVersion":
		if len(args) != 0 {
			return Null, fmt.Errorf("System.requestVersion expects 0 arguments")
		}
		return vm.requestVersionValue(), nil
	case "System.abortJob":
		return vm.abortJob(args)
	case "Database.scheduleBatch", "System.scheduleBatch":
		return vm.scheduleBatch(args, result)
	case "System.attachFinalizer":
		if len(args) != 1 || args[0].Kind != ValueObject {
			return Null, fmt.Errorf("System.attachFinalizer expects Finalizer object")
		}
		if vm.currentAsyncKind == "Queueable" {
			vm.currentFinalizer = args[0]
		}
		return Null, nil
	case "AsyncInfo.hasMaxStackDepth", "System.AsyncInfo.hasMaxStackDepth":
		if len(args) != 0 {
			return Null, fmt.Errorf("AsyncInfo.hasMaxStackDepth expects 0 arguments")
		}
		if vm.currentAsyncKind != "Queueable" {
			return Null, newExceptionError("System.AsyncException", "hasMaxStackDepth is not allowed outside a Queueable of Finalizer execution")
		}
		return Bool(vm.currentQueueableMaxDepth > 0), nil
	case "AsyncInfo.getCurrentQueueableStackDepth", "System.AsyncInfo.getCurrentQueueableStackDepth":
		if len(args) != 0 {
			return Null, fmt.Errorf("AsyncInfo.getCurrentQueueableStackDepth expects 0 arguments")
		}
		if vm.currentAsyncKind != "Queueable" {
			return Null, newExceptionError("System.AsyncException", "getCurrentQueueableStackDepth is not allowed outside a Queueable or Finalizer execution")
		}
		if vm.currentQueueableDepth > 0 {
			return Int(int64(vm.currentQueueableDepth)), nil
		}
		return Int(1), nil
	case "AsyncInfo.getMaximumQueueableStackDepth", "System.AsyncInfo.getMaximumQueueableStackDepth":
		if len(args) != 0 {
			return Null, fmt.Errorf("AsyncInfo.getMaximumQueueableStackDepth expects 0 arguments")
		}
		if vm.currentAsyncKind != "Queueable" {
			return Null, newExceptionError("System.AsyncException", "getMaximumQueueableStackDepth is not allowed outside a Queueable or Finalizer execution")
		}
		if vm.currentQueueableMaxDepth > 0 {
			return Int(int64(vm.currentQueueableMaxDepth)), nil
		}
		return Int(0), nil
	case "AsyncInfo.getMinimumQueueableDelayInMinutes", "System.AsyncInfo.getMinimumQueueableDelayInMinutes":
		if len(args) != 0 {
			return Null, fmt.Errorf("AsyncInfo.getMinimumQueueableDelayInMinutes expects 0 arguments")
		}
		if vm.currentAsyncKind != "Queueable" {
			return Null, newExceptionError("System.AsyncException", "getMinimumQueueableDelayInMinutes is not allowed outside a Queueable or Finalizer execution")
		}
		return Int(int64(vm.currentQueueableDelay)), nil
	case "Datetime.newInstance", "Datetime.newInstanceGmt":
		if len(args) == 1 {
			if args[0].Kind != ValueInt {
				return Null, fmt.Errorf("%s expects integer milliseconds", callee)
			}
			return platformScalar("Datetime", formatPlatformDatetime(time.UnixMilli(args[0].Int).UTC())), nil
		}
		if len(args) == 2 {
			if args[0].Kind != ValueObject || args[0].Type != "Date" || args[1].Kind != ValueObject || args[1].Type != "Time" {
				return Null, fmt.Errorf("%s expects Date and Time", callee)
			}
			date, err := parsePlatformDate(args[0])
			if err != nil {
				return Null, err
			}
			clock, err := parsePlatformTime(args[1])
			if err != nil {
				return Null, err
			}
			hour := int(clock / time.Hour)
			clock %= time.Hour
			minute := int(clock / time.Minute)
			clock %= time.Minute
			second := int(clock / time.Second)
			clock %= time.Second
			millisecond := int(clock / time.Millisecond)
			zoneID := "UTC"
			if callee == "Datetime.newInstance" {
				zoneID = vm.currentUserTimeZoneID()
			}
			value, err := datetimeFromLocalParts(date.Year(), int(date.Month()), date.Day(), hour, minute, second, millisecond, zoneID)
			if err != nil {
				return Null, err
			}
			return platformScalar("Datetime", formatPlatformDatetime(value)), nil
		}
		if len(args) != 3 && len(args) != 6 {
			return Null, fmt.Errorf("%s expects year, month, day[, hour, minute, second] integers", callee)
		}
		for i := 0; i < len(args); i++ {
			if args[i].Kind != ValueInt {
				return Null, fmt.Errorf("%s expects integer parts", callee)
			}
		}
		year, month, day := normalizeDateNewInstanceParts(int(args[0].Int), int(args[1].Int), int(args[2].Int))
		if err := validateDateParts(year, month, day); err != nil {
			if year == 0 || month < 1 || month > 12 || day < 1 {
				value, valueErr := dateFromNewInstanceParts(year, month, day)
				if valueErr != nil {
					return Null, valueErr
				}
				year, month, day = value.Year(), int(value.Month()), value.Day()
			} else {
				return Null, err
			}
		}
		hour, minute, second := 0, 0, 0
		if len(args) == 6 {
			hour, minute, second = int(args[3].Int), int(args[4].Int), int(args[5].Int)
		}
		if err := validateTimeParts(hour, minute, second); err != nil {
			return Null, err
		}
		zoneID := "UTC"
		if callee == "Datetime.newInstance" {
			zoneID = vm.currentUserTimeZoneID()
		}
		value, err := datetimeFromLocalParts(year, month, day, hour, minute, second, 0, zoneID)
		if err != nil {
			return Null, err
		}
		return platformScalar("Datetime", formatPlatformDatetime(value)), nil
	case "Datetime.valueOf", "Datetime.valueOfGmt":
		if len(args) != 1 {
			return Null, newExceptionError("System.NullPointerException", fmt.Sprintf("%s expects String", callee))
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", fmt.Sprintf("%s expects String", callee))
		}
		text := ""
		if args[0].Kind == ValueString {
			text = args[0].Text
		} else if objectText, ok := platformScalarObjectText(args[0]); ok {
			text = objectText
		} else {
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s expects String", callee))
		}
		value, err := parseDatetimeText(text)
		if err != nil {
			return Null, newExceptionError("System.TypeException", "Invalid date/time: "+text)
		}
		if strings.Contains(text, ".") {
			value = value.Truncate(time.Second)
		}
		out := platformScalar("Datetime", formatPlatformDatetime(value))
		if callee == "Datetime.valueOfGmt" && strings.Contains(text, "T") && strings.Contains(text, ".") {
			out.Fields["legacyIsoFractionalTruncate"] = Bool(true)
		}
		return out, nil
	case "Datetime.parse":
		if len(args) != 1 {
			return Null, newExceptionError("System.NullPointerException", "Datetime.parse expects String")
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Datetime.parse expects String")
		}
		if args[0].Kind != ValueString {
			return Null, newExceptionError("System.TypeException", "Datetime.parse expects String")
		}
		value, err := parseDatetimeParseText(args[0].Text, vm.currentUserTimeZoneID())
		if err != nil {
			return Null, newExceptionError("System.TypeException", "Invalid date/time: "+args[0].Text)
		}
		return platformScalar("Datetime", formatPlatformDatetime(value)), nil
	case "String.valueOfGmt":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Datetime" {
			return Null, fmt.Errorf("String.valueOfGmt expects Datetime")
		}
		datetimeValue, err := parsePlatformDatetime(args[0])
		if err != nil {
			return Null, err
		}
		return String(datetimeValue.UTC().Format("2006-01-02 15:04:05")), nil
	case "LoggingLevel.values":
		return loggingLevelValues(args)
	case "ApexPages.Severity.values":
		return apexPagesSeverityValues(args)
	case "Metadata.DeployStatus.values":
		return metadataDeployStatusValues(args)
	case "Metadata.MetadataType.values":
		return metadataMetadataTypeValues(args)
	case "RoundingMode.values":
		return roundingModeValues(args)
	case "System.isRunningTest", "System.Test.isRunningTest", "Test.isRunningTest":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Bool(vm.testContext != nil), nil
	case "Test.Database.hasRecords":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.Database.hasRecords expects 0 arguments")
		}
		if vm.testContext != nil {
			return Null, unsupportedCallError("Test.Database.hasRecords")
		}
		return Bool(false), nil
	case "Type.forName", "System.Type.forName":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Type.forName expects type name or namespace and type name")
		}
		if len(args) == 1 {
			if args[0].Kind == ValueNull {
				return Null, nil
			}
			typeName, ok := stringLikeValueText(args[0])
			if !ok {
				return Null, fmt.Errorf("Type.forName expects String")
			}
			return vm.typeForName("", typeName, false), nil
		}
		namespace := ""
		if args[0].Kind != ValueNull {
			var ok bool
			namespace, ok = stringLikeValueText(args[0])
			if !ok {
				return Null, fmt.Errorf("Type.forName expects namespace String or null")
			}
		}
		if args[1].Kind == ValueNull {
			return Null, nil
		}
		typeName, ok := stringLikeValueText(args[1])
		if !ok {
			return Null, fmt.Errorf("Type.forName expects type name String")
		}
		return vm.typeForName(namespace, typeName, true), nil
	case "Test.getStandardPricebookId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.getStandardPricebookId expects 0 arguments")
		}
		if vm.testContext == nil {
			return Null, fmt.Errorf("Test.getStandardPricebookId is only available in test context")
		}
		return String("01s000000000001"), nil
	case "Time.newInstance":
		if len(args) < 3 || len(args) > 4 {
			return Null, fmt.Errorf("Time.newInstance expects hour, minute, second[, millisecond]")
		}
		for i := 0; i < len(args); i++ {
			if args[i].Kind != ValueInt {
				return Null, fmt.Errorf("Time.newInstance expects integer parts")
			}
		}
		if err := validateTimeParts(int(args[0].Int), int(args[1].Int), int(args[2].Int)); err != nil {
			return Null, err
		}
		millisecond := 0
		if len(args) == 4 {
			if args[3].Int < 0 || args[3].Int > 999 {
				return Null, fmt.Errorf("invalid Time millisecond: %d", args[3].Int)
			}
			millisecond = int(args[3].Int)
		}
		return platformScalar("Time", formatPlatformTimeWithMillis(int(args[0].Int), int(args[1].Int), int(args[2].Int), millisecond)), nil
	case "Time.valueOf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Time.valueOf expects String")
		}
		parsed, err := parseTimeText(args[0].Text)
		if err != nil {
			return Null, err
		}
		return platformScalar("Time", parsed), nil
	case "TimeZone.getTimeZone":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("TimeZone.getTimeZone expects String")
		}
		return fixedTimeZone(args[0].Text)
	case "Blob.valueOf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Blob.valueOf expects String")
		}
		return platformScalar("Blob", args[0].Text), nil
	case "Blob.toPdf":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Blob.toPdf expects String")
		}
		pdf := "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n"
		return platformScalar("Blob", pdf), nil
	case "EncodingUtil.base64Encode":
		blob, err := blobStringArg("EncodingUtil.base64Encode", args)
		if err != nil {
			return Null, err
		}
		return String(base64.StdEncoding.EncodeToString([]byte(blob))), nil
	case "EncodingUtil.base64Decode":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("EncodingUtil.base64Decode expects String")
		}
		decoded, err := base64.StdEncoding.DecodeString(args[0].Text)
		if err != nil {
			return Null, newExceptionError("System.StringException", "EncodingUtil.base64Decode invalid base64 string: "+err.Error())
		}
		return platformScalar("Blob", string(decoded)), nil
	case "EncodingUtil.convertFromHex":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("EncodingUtil.convertFromHex expects String")
		}
		decoded, err := hex.DecodeString(args[0].Text)
		if err != nil {
			return Null, newExceptionError("System.InvalidParameterValueException", "invalid hexadecimal string")
		}
		return platformScalar("Blob", string(decoded)), nil
	case "EncodingUtil.convertToHex":
		blob, err := blobStringArg("EncodingUtil.convertToHex", args)
		if err != nil {
			return Null, err
		}
		return String(hex.EncodeToString([]byte(blob))), nil
	case "EncodingUtil.urlEncode":
		if len(args) != 2 {
			return Null, fmt.Errorf("EncodingUtil.urlEncode expects String and charset")
		}
		if args[0].Kind == ValueNull || args[1].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		text, err := stringArg("EncodingUtil.urlEncode", args[:1])
		if err != nil {
			return Null, err
		}
		charset, err := stringArg("EncodingUtil.urlEncode", args[1:])
		if err != nil {
			return Null, err
		}
		encoded, err := urlEncodeWithCharset("EncodingUtil.urlEncode", text, charset)
		if err != nil {
			return Null, newExceptionError("System.StringException", err.Error())
		}
		return String(encoded), nil
	case "EncodingUtil.urlDecode":
		if len(args) != 2 {
			return Null, fmt.Errorf("EncodingUtil.urlDecode expects String and charset")
		}
		if args[0].Kind == ValueNull || args[1].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		text, err := stringArg("EncodingUtil.urlDecode", args[:1])
		if err != nil {
			return Null, err
		}
		charset, err := stringArg("EncodingUtil.urlDecode", args[1:])
		if err != nil {
			return Null, err
		}
		decoded, err := urlDecodeWithCharset("EncodingUtil.urlDecode", text, charset)
		if err != nil {
			return Null, newExceptionError("System.StringException", err.Error())
		}
		return String(decoded), nil
	case "Crypto.generateDigest":
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.generateDigest expects algorithm and Blob")
		}
		blob, err := blobStringArg("Crypto.generateDigest", args[1:])
		if err != nil {
			return Null, err
		}
		digest, err := generateDigest(args[0].Text, []byte(blob))
		if err != nil {
			return Null, newExceptionError("System.SecurityException", err.Error())
		}
		return platformScalar("Blob", string(digest)), nil
	case "Crypto.generateMac":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.generateMac expects algorithm, input Blob, and privateKey Blob")
		}
		input, err := blobStringArg("Crypto.generateMac input", args[1:2])
		if err != nil {
			return Null, err
		}
		key, err := blobStringArg("Crypto.generateMac privateKey", args[2:])
		if err != nil {
			return Null, err
		}
		mac, err := generateMac(args[0].Text, []byte(input), []byte(key))
		if err != nil {
			return Null, newExceptionError("System.SecurityException", err.Error())
		}
		return platformScalar("Blob", string(mac)), nil
	case "Crypto.verifyHmac":
		if len(args) != 4 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.verifyHmac expects algorithm, input Blob, privateKey Blob, and mac Blob")
		}
		input, err := blobStringArg("Crypto.verifyHmac input", args[1:2])
		if err != nil {
			return Null, err
		}
		key, err := blobStringArg("Crypto.verifyHmac privateKey", args[2:3])
		if err != nil {
			return Null, err
		}
		expected, err := blobStringArg("Crypto.verifyHmac mac", args[3:])
		if err != nil {
			return Null, err
		}
		actual, err := generateMac(args[0].Text, []byte(input), []byte(key))
		if err != nil {
			return Null, newExceptionError("System.SecurityException", err.Error())
		}
		return Bool(hmac.Equal(actual, []byte(expected))), nil
	case "Crypto.areEqualConstantTime":
		if len(args) != 2 {
			return Null, fmt.Errorf("Crypto.areEqualConstantTime expects left Blob and right Blob")
		}
		left, err := blobStringArg("Crypto.areEqualConstantTime left", args[:1])
		if err != nil {
			return Null, err
		}
		right, err := blobStringArg("Crypto.areEqualConstantTime right", args[1:])
		if err != nil {
			return Null, err
		}
		return Bool(subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1), nil
	case "Crypto.encrypt":
		if len(args) != 4 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.encrypt expects algorithm, privateKey Blob, initializationVector Blob, and clearText Blob")
		}
		key, err := blobStringArg("Crypto.encrypt privateKey", args[1:2])
		if err != nil {
			return Null, err
		}
		iv, err := blobStringArg("Crypto.encrypt initializationVector", args[2:3])
		if err != nil {
			return Null, err
		}
		clearText, err := blobStringArg("Crypto.encrypt clearText", args[3:])
		if err != nil {
			return Null, err
		}
		cipherText, err := encryptAESCBC(args[0].Text, []byte(key), []byte(iv), []byte(clearText))
		if err != nil {
			return Null, newExceptionError("System.InvalidParameterValueException", err.Error())
		}
		return platformScalar("Blob", string(cipherText)), nil
	case "Crypto.decrypt":
		if len(args) != 4 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.decrypt expects algorithm, privateKey Blob, initializationVector Blob, and cipherText Blob")
		}
		key, err := blobStringArg("Crypto.decrypt privateKey", args[1:2])
		if err != nil {
			return Null, err
		}
		iv, err := blobStringArg("Crypto.decrypt initializationVector", args[2:3])
		if err != nil {
			return Null, err
		}
		cipherText, err := blobStringArg("Crypto.decrypt cipherText", args[3:])
		if err != nil {
			return Null, err
		}
		clearText, err := decryptAESCBC(args[0].Text, []byte(key), []byte(iv), []byte(cipherText))
		if err != nil {
			return Null, newExceptionError("System.InvalidParameterValueException", err.Error())
		}
		return platformScalar("Blob", string(clearText)), nil
	case "Crypto.encryptWithManagedIV":
		if len(args) == 4 {
			return Null, unsupportedCallError("Crypto.encryptWithManagedIV local authenticated-data managed-IV AES surface")
		}
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.encryptWithManagedIV expects algorithm, privateKey Blob, and clearText Blob")
		}
		key, err := blobStringArg("Crypto.encryptWithManagedIV privateKey", args[1:2])
		if err != nil {
			return Null, err
		}
		clearText, err := blobStringArg("Crypto.encryptWithManagedIV clearText", args[2:])
		if err != nil {
			return Null, err
		}
		iv := managedIV([]byte(key), []byte(clearText))
		cipherText, err := encryptAESCBC(args[0].Text, []byte(key), iv, []byte(clearText))
		if err != nil {
			return Null, newExceptionError("System.InvalidParameterValueException", err.Error())
		}
		return platformScalar("Blob", string(append(append([]byte{}, iv...), cipherText...))), nil
	case "Crypto.decryptWithManagedIV":
		if len(args) == 4 {
			return Null, unsupportedCallError("Crypto.decryptWithManagedIV local authenticated-data managed-IV AES surface")
		}
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Crypto.decryptWithManagedIV expects algorithm, privateKey Blob, and cipherText Blob")
		}
		key, err := blobStringArg("Crypto.decryptWithManagedIV privateKey", args[1:2])
		if err != nil {
			return Null, err
		}
		cipherText, err := blobStringArg("Crypto.decryptWithManagedIV cipherText", args[2:])
		if err != nil {
			return Null, err
		}
		if len(cipherText) < aes.BlockSize {
			return Null, newExceptionError("System.InvalidParameterValueException", "cipherText must include managed IV")
		}
		clearText, err := decryptAESCBC(args[0].Text, []byte(key), []byte(cipherText[:aes.BlockSize]), []byte(cipherText[aes.BlockSize:]))
		if err != nil {
			return Null, newExceptionError("System.InvalidParameterValueException", err.Error())
		}
		return platformScalar("Blob", string(clearText)), nil
	case "Crypto.sign", "Crypto.signWithCertificate":
		if len(args) != 3 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects algorithm, input Blob, and key", callee)
		}
		input, err := blobStringArg(callee+" input", args[1:2])
		if err != nil {
			return Null, err
		}
		signature, err := localCryptoSignature(args[0].Text, []byte(input))
		if err != nil {
			if callee == "Crypto.signWithCertificate" {
				return Null, newExceptionError("System.NoDataFoundException", err.Error())
			}
			return Null, newExceptionError("System.SecurityException", err.Error())
		}
		return platformScalar("Blob", string(signature)), nil
	case "Crypto.verify", "Crypto.verifyWithCertificate":
		if len(args) != 4 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects algorithm, input Blob, signature Blob, and key", callee)
		}
		input, err := blobStringArg(callee+" input", args[1:2])
		if err != nil {
			return Null, err
		}
		signature, err := blobStringArg(callee+" signature", args[2:3])
		if err != nil {
			return Null, err
		}
		expected, err := localCryptoSignature(args[0].Text, []byte(input))
		if err != nil {
			if callee == "Crypto.verifyWithCertificate" {
				return Null, newExceptionError("System.NoDataFoundException", err.Error())
			}
			return Null, newExceptionError("ApexExecutionError", err.Error())
		}
		return Bool(hmac.Equal([]byte(signature), expected)), nil
	case "Crypto.generateAESKey":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, fmt.Errorf("Crypto.generateAESKey expects Integer key size")
		}
		switch args[0].Int {
		case 128, 192, 256:
			key := make([]byte, int(args[0].Int/8))
			for i := range key {
				key[i] = byte(i + 1)
			}
			return platformScalar("Blob", string(key)), nil
		default:
			return Null, newExceptionError("System.InvalidParameterValueException", "Crypto.generateAESKey expects 128, 192, or 256")
		}
	case "Crypto.getRandomInteger":
		if len(args) != 0 {
			return Null, fmt.Errorf("Crypto.getRandomInteger expects 0 arguments")
		}
		return Int(int64(int32(vm.nextDeterministicCryptoLong()))), nil
	case "Crypto.getRandomLong":
		if len(args) != 0 {
			return Null, fmt.Errorf("Crypto.getRandomLong expects 0 arguments")
		}
		return Int(vm.nextDeterministicCryptoLong()), nil
	case "JSON.createGenerator":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, fmt.Errorf("JSON.createGenerator expects Boolean")
		}
		return newJSONGenerator(args[0].Bool), nil
	case "JSON.createParser":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("JSON.createParser expects String")
		}
		return newJSONParser(args[0].Text)
	case "JSON.serialize":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("JSON.serialize expects 1 or 2 arguments")
		}
		suppressNulls, err := jsonSuppressNulls("JSON.serialize", args[1:])
		if err != nil {
			return Null, err
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Schema.SObjectField") {
			return Null, jsonDeserializeException("Type cannot be serialized")
		}
		data, err := jsonMarshalNoEscape(vm.jsonFromValueForSerialize(args[0], suppressNulls))
		if err != nil {
			return Null, jsonDeserializeException("%s", err.Error())
		}
		return String(string(data)), nil
	case "JSON.serializePretty":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("JSON.serializePretty expects 1 or 2 arguments")
		}
		suppressNulls, err := jsonSuppressNulls("JSON.serializePretty", args[1:])
		if err != nil {
			return Null, err
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Schema.SObjectField") {
			return Null, jsonDeserializeException("Type cannot be serialized")
		}
		data, err := jsonMarshalNoEscapeIndent(vm.jsonFromValueForSerialize(args[0], suppressNulls), "", "  ")
		if err != nil {
			return Null, jsonDeserializeException("%s", err.Error())
		}
		return String(formatSalesforcePrettyJSON(data)), nil
	case "JSON.deserializeUntyped":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, jsonDeserializeException("Argument cannot be null.")
		}
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("JSON.deserializeUntyped expects String")
		}
		decoded, err := decodeJSONUntypedValue(args[0].Text)
		if err != nil {
			return Null, jsonDeserializeException("JSON.deserializeUntyped invalid JSON input: %v", err)
		}
		return decoded, nil
	case "JSON.deserialize", "JSON.deserializeStrict":
		if len(args) == 2 && args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
		}
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String and Type", callee)
		}
		strict := callee == "JSON.deserializeStrict"
		decoded, err := decodeJSONValueForDeserialize(args[0].Text, strict)
		if err != nil {
			return Null, jsonDeserializeException("%s", err.Error())
		}
		if args[1].Kind == ValueObject && args[1].Type == "Type" {
			return vm.typedValueFromJSON(typeValueName(args[1]), decoded, strict)
		}
		return valueFromJSON(decoded), nil
	case "Schema.getGlobalDescribe":
		if len(args) != 0 {
			return Null, fmt.Errorf("Schema.getGlobalDescribe expects 0 arguments")
		}
		appendTraceLazy(result, "apex.describe.global", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("getGlobalDescribe", nil)
		})
		return vm.schemaGlobalDescribe(), nil
	case "Schema.describeSObjects":
		if (len(args) != 1 && len(args) != 2) || args[0].Kind != ValueList {
			return Null, fmt.Errorf("Schema.describeSObjects expects List")
		}
		if vm.Org == nil {
			return Null, fmt.Errorf("Schema.describeSObjects requires org state")
		}
		describes := make([]Value, 0, len(args[0].List))
		for _, item := range args[0].List {
			objectName, err := vm.schemaDescribeObjectName(item)
			if err != nil {
				return Null, err
			}
			resolved := ""
			definition := storage.ObjectDefinition{}
			ok := false
			if canonical, found := vm.resolveObjectName(objectName); found {
				resolved = canonical
				definition = vm.Org.Objects[canonical].Definition
				ok = true
			}
			if !ok {
				return Null, newExceptionError("System.SObjectException", fmt.Sprintf("Schema.describeSObjects unknown object %s", objectName))
			}
			describes = append(describes, vm.describeSObjectValue(resolved, definition))
		}
		appendTraceLazy(result, "apex.describe.sobjects", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("describeSObjects", map[string]any{
				"count": len(describes),
			})
		})
		return List(describes...), nil
	case "Schema.describeTabs":
		if len(args) != 0 {
			return Null, fmt.Errorf("Schema.describeTabs expects 0 arguments")
		}
		appendTraceLazy(result, "apex.describe.tabs", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("describeTabs", map[string]any{
				"count": len(vm.schemaDescribeTabValues()),
			})
		})
		return vm.schemaDescribeTabs(), nil
	case "Schema.getAppDescribe":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Schema.getAppDescribe expects app name String")
		}
		appendTraceLazy(result, "apex.describe.app", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("getAppDescribe", map[string]any{
				"app": args[0].Text,
			})
		})
		return vm.schemaGlobalDescribe(), nil
	case "Schema.getModuleDescribe":
		if len(args) > 1 || (len(args) == 1 && args[0].Kind != ValueString) {
			return Null, fmt.Errorf("Schema.getModuleDescribe expects optional module name String")
		}
		module := ""
		if len(args) == 1 {
			module = args[0].Text
		}
		appendTraceLazy(result, "apex.describe.module", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("getModuleDescribe", map[string]any{
				"module": module,
			})
		})
		return vm.schemaGlobalDescribe(), nil
	case "Schema.describeDataCategoryGroups":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("Schema.describeDataCategoryGroups expects List<String>")
		}
		describes := vm.schemaDescribeDataCategoryGroups(args[0])
		appendTraceLazy(result, "apex.describe.dataCategoryGroups", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("describeDataCategoryGroups", map[string]any{
				"count": len(describes.List),
			})
		})
		return describes, nil
	case "Schema.describeDataCategoryGroupStructures":
		if len(args) != 2 || args[0].Kind != ValueList || args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Schema.describeDataCategoryGroupStructures expects List<Schema.DataCategoryGroupSobjectTypePair> and Boolean")
		}
		describes := vm.schemaDescribeDataCategoryGroupStructures(args[0], args[1].Bool)
		appendTraceLazy(result, "apex.describe.dataCategoryGroupStructures", "apex.describe", func() map[string]any {
			return vm.traceDescribeArgs("describeDataCategoryGroupStructures", map[string]any{
				"count": len(describes.List),
			})
		})
		return describes, nil
	case "FeatureManagement.checkPermission":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.checkPermission expects String")
		}
		if vm.currentUserHasPermission(args[0].Text) {
			return Bool(true), nil
		}
		return Bool(false), nil
	case "FeatureManagement.changeProtection":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.changeProtection expects namespace, feature, and protection String values")
		}
		return Null, nil
	case "FeatureManagement.checkPackageBooleanValue":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.checkPackageBooleanValue expects String")
		}
		if value, ok := vm.managedFeatureValues[managedFeatureValueKey("Boolean", args[0].Text)]; ok && value.Kind == ValueBool {
			return value, nil
		}
		return Bool(false), nil
	case "FeatureManagement.setPackageBooleanValue":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueBool {
			return Null, fmt.Errorf("FeatureManagement.setPackageBooleanValue expects String and Boolean")
		}
		if vm.managedFeatureValues == nil {
			vm.managedFeatureValues = make(map[string]Value)
		}
		vm.managedFeatureValues[managedFeatureValueKey("Boolean", args[0].Text)] = args[1]
		return Null, nil
	case "FeatureManagement.checkPackageIntegerValue":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.checkPackageIntegerValue expects String")
		}
		if value, ok := vm.managedFeatureValues[managedFeatureValueKey("Integer", args[0].Text)]; ok && value.Kind == ValueInt {
			return value, nil
		}
		return Null, nil
	case "FeatureManagement.setPackageIntegerValue":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueInt {
			return Null, fmt.Errorf("FeatureManagement.setPackageIntegerValue expects String and Integer")
		}
		if vm.managedFeatureValues == nil {
			vm.managedFeatureValues = make(map[string]Value)
		}
		vm.managedFeatureValues[managedFeatureValueKey("Integer", args[0].Text)] = args[1]
		return Null, nil
	case "FeatureManagement.checkPackageDateValue":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("FeatureManagement.checkPackageDateValue expects String")
		}
		if value, ok := vm.managedFeatureValues[managedFeatureValueKey("Date", args[0].Text)]; ok && value.Kind == ValueObject && strings.EqualFold(value.Type, "Date") {
			return value, nil
		}
		return Null, nil
	case "FeatureManagement.setPackageDateValue":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueObject || !strings.EqualFold(args[1].Type, "Date") {
			return Null, fmt.Errorf("FeatureManagement.setPackageDateValue expects String and Date")
		}
		if vm.managedFeatureValues == nil {
			vm.managedFeatureValues = make(map[string]Value)
		}
		vm.managedFeatureValues[managedFeatureValueKey("Date", args[0].Text)] = cloneValue(args[1])
		return Null, nil
	case "BusinessHours.add", "BusinessHours.addGmt":
		return vm.businessHoursAdd(callee, args)
	case "BusinessHours.diff":
		return vm.businessHoursDiff(args)
	case "BusinessHours.isWithin":
		return vm.businessHoursIsWithin(args)
	case "BusinessHours.nextStartDate":
		return vm.businessHoursNextStartDate(args)
	case "EventBus.publish":
		return vm.eventBusPublish(args, result)
	case "EventBus.publishWithAccessLevel":
		return vm.eventBusPublishWithAccessLevel(args, result)
	case "EventBus.publishAfterCommit":
		return Null, unsupportedCallError(callee + " local platform event after-commit delivery surface")
	case "IntegrationTest.commitTestOnly":
		return Null, unsupportedCallError(callee + " local IntegrationTest developer preview service surface")
	case "Request.getCurrent", "System.Request.getCurrent", "RequestImpl.getCurrent":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return vm.currentRequestValue(), nil
	case "UIRequest.getCurrent":
		if len(args) != 0 {
			return Null, fmt.Errorf("UIRequest.getCurrent expects 0 arguments")
		}
		return vm.currentUIRequestValue(), nil
	case "ConnectApi.Organization.getSettings":
		return vm.connectAPIOrganizationSettings(args)
	case "ConnectApi.ChatterUsers.getFollowings", "System.ConnectApi.ChatterUsers.getFollowings":
		return vm.connectAPIChatterUsersGetFollowings(args)
	case "ConnectApi.Communities.getCommunity", "System.ConnectApi.Communities.getCommunity":
		return vm.connectAPICommunity(args)
	case "ConnectApi.Communities.getCommunities", "System.ConnectApi.Communities.getCommunities":
		return vm.connectAPICommunities(args)
	case "ConnectApi.NamedCredentials.getNamedCredentials":
		return vm.connectAPINamedCredentialsGetNamedCredentials(args)
	case "ConnectApi.NamedCredentials.createExternalCredential":
		return vm.connectAPINamedCredentialsCreateExternalCredential(args)
	case "ConnectApi.NamedCredentials.createNamedCredential":
		return vm.connectAPINamedCredentialsCreateNamedCredential(args)
	case "ConnectApi.NamedCredentials.getExternalCredential":
		return vm.connectAPINamedCredentialsGetExternalCredential(args)
	case "ConnectApi.UserProfiles.getUserProfile":
		return vm.connectAPIUserProfile(args)
	case "ConnectApi.UserProfiles.getPhoto":
		return vm.connectAPIUserPhoto(args)
	case "ConnectApi.UserProfiles.setPhoto":
		return vm.connectAPIUserSetPhoto(args)
	case "ConnectApi.UserProfiles.deletePhoto":
		return vm.connectAPIUserDeletePhoto(args)
	case "ConnectApi.NextBestAction.executeStrategy":
		return vm.connectApiNBAExecuteStrategy(args)
	case "ConnectApi.NextBestAction.setRecommendationReaction":
		return vm.connectApiNBASetRecommendationReaction(args)
	case "ConnectApi.Orchestration.getOrchestrationInstanceCollection",
		"ConnectApi.Orchestrator.getOrchestrationInstanceCollection":
		return vm.connectApiOrchGetInstanceCollection(args)
	case "ConnectApi.Orchestration.publishOrchestrationEvent",
		"ConnectApi.Orchestrator.publishOrchestrationEvent":
		return vm.connectApiOrchPublishEvent(args)
	case "ConnectApi.ChatterFeeds.postFeedElement", "System.ConnectApi.ChatterFeeds.postFeedElement":
		return vm.connectAPIChatterPostFeedElement(args)
	case "ConnectApi.ChatterFeeds.postFeedElementBatch", "System.ConnectApi.ChatterFeeds.postFeedElementBatch":
		return vm.connectAPIChatterPostFeedElementBatch(args)
	case "ConnectApi.ChatterFeeds.updateComment", "System.ConnectApi.ChatterFeeds.updateComment":
		return vm.connectAPIChatterUpdateComment(args)
	case "ConnectApi.ChatterFeeds.getComment", "System.ConnectApi.ChatterFeeds.getComment":
		return vm.connectAPIChatterGetComment(args)
	case "ConnectApi.ChatterUsers.setPhoto", "System.ConnectApi.ChatterUsers.setPhoto":
		return vm.connectAPIChatterUsersSetPhoto(args)
	case "ConnectApi.ChatterUsers.getReputation", "System.ConnectApi.ChatterUsers.getReputation":
		return vm.connectAPIChatterUsersGetReputation(args)
	case "ConnectApi.CommerceCart.getCartSummary", "System.ConnectApi.CommerceCart.getCartSummary":
		return vm.connectAPICommerceCartGetCartSummary(args)
	case "ConnectApi.CommerceCart.addItemToCart", "System.ConnectApi.CommerceCart.addItemToCart":
		return vm.connectAPICommerceCartAddItemToCart(args)
	case "ConnectApi.CommerceCart.addItemsToCart", "System.ConnectApi.CommerceCart.addItemsToCart":
		return vm.connectAPICommerceCartAddItemsToCart(args)
	case "ConnectApi.CommerceCart.getCartItems", "System.ConnectApi.CommerceCart.getCartItems":
		return vm.connectAPICommerceCartGetCartItems(args)
	case "ConnectApi.CommerceCatalog.getProduct", "System.ConnectApi.CommerceCatalog.getProduct":
		return vm.connectAPICommerceCatalogGetProduct(args)
	case "ConnectApi.CommerceStorePricing.getProductPrice", "System.ConnectApi.CommerceStorePricing.getProductPrice":
		return vm.connectAPICommerceStorePricingGetProductPrice(args)
	case "ConnectApi.CommerceStorePricing.getProductPrices", "System.ConnectApi.CommerceStorePricing.getProductPrices":
		return vm.connectAPICommerceStorePricingGetProductPrices(args)
	case "ConnectApi.Topics.getTopicSuggestions", "System.ConnectApi.Topics.getTopicSuggestions":
		return vm.connectAPITopicsGetTopicSuggestions(args)
	case "ConnectApi.Wave.executeQuery", "System.ConnectApi.Wave.executeQuery":
		return vm.connectAPIWaveExecuteQuery(args)
	case "Metadata.Operations.enqueueDeployment":
		return vm.metadataEnqueueDeployment(args, result)
	case "Metadata.Operations.checkDeployStatus":
		return vm.metadataCheckDeployStatus(args, result)
	case "Metadata.Operations.retrieve":
		return vm.metadataRetrieve(args)
	case "reports.ReportManager.describeReport":
		return vm.reportsDescribeReport(args)
	case "reports.ReportManager.getDatatypeFilterOperatorMap":
		return vm.reportsDatatypeFilterOperatorMap(args)
	case "reports.ReportManager.getReportInstance":
		return vm.reportsGetReportInstance(args)
	case "reports.ReportManager.getReportInstances":
		return vm.reportsGetReportInstances(args)
	case "reports.ReportManager.runAsyncReport":
		return vm.reportsRunAsyncReport(args)
	case "reports.ReportManager.runReport":
		return vm.reportsRunReport(args)
	case "IsvPartners.AppAnalytics.logCustomInteraction":
		if len(args) < 1 || len(args) > 2 {
			return Null, fmt.Errorf("IsvPartners.AppAnalytics.logCustomInteraction expects interaction label[, id]")
		}
		return Null, nil
	case "UserProvisioning.UserProvisioningLog.log":
		if len(args) != 2 && len(args) != 3 && len(args) != 5 {
			return Null, fmt.Errorf("UserProvisioning.UserProvisioningLog.log expects 2, 3, or 5 arguments")
		}
		return Null, nil
	case "pref_center.TokenUtility.generateToken":
		return prefCenterGenerateToken(args)
	case "pref_center.TokenUtility.generateTokens":
		return prefCenterGenerateTokens(args)
	case "UserManagement.initSelfRegistration":
		if len(args) != 2 {
			return Null, fmt.Errorf("UserManagement.initSelfRegistration expects 2 arguments")
		}
		return String("local-self-registration"), nil
	case "UserManagement.formatPhoneNumber":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("UserManagement.formatPhoneNumber expects country code and phone number Strings")
		}
		return String(formatLocalPhoneNumber(args[0].Text, args[1].Text)), nil
	case "UserManagement.verifySelfRegistration":
		if len(args) != 4 {
			return Null, fmt.Errorf("UserManagement.verifySelfRegistration expects 4 arguments")
		}
		redirect := newPageReference("/")
		if args[3].Kind == ValueString && strings.TrimSpace(args[3].Text) != "" {
			redirect = newPageReference(args[3].Text)
		}
		return newAuthVerificationResult(redirect, Bool(true), Null), nil
	case "UserManagement.deregisterVerificationMethod", "UserManagement.obfuscateUser":
		if len(args) < 1 || len(args) > 2 {
			return Null, fmt.Errorf("%s expects user Id and optional extra argument", callee)
		}
		return Null, nil
	case "UserManagement.initPasswordlessLogin":
		if len(args) != 2 {
			return Null, fmt.Errorf("UserManagement.initPasswordlessLogin expects user Id and verification method")
		}
		return String("local-passwordless-login"), nil
	case "UserManagement.initRegisterVerificationMethod", "UserManagement.initVerificationMethod":
		if len(args) != 1 && len(args) != 3 {
			return Null, fmt.Errorf("%s expects verification method or verification method, action name, and extras", callee)
		}
		return String("local-verification"), nil
	case "UserManagement.registerVerificationMethod":
		if len(args) != 2 {
			return Null, fmt.Errorf("UserManagement.registerVerificationMethod expects verification method and start URL")
		}
		startURL := "/"
		if args[1].Kind == ValueString && strings.TrimSpace(args[1].Text) != "" {
			startURL = args[1].Text
		}
		return newPageReference(startURL), nil
	case "UserManagement.sendAsyncEmailConfirmation":
		if len(args) != 4 {
			return Null, fmt.Errorf("UserManagement.sendAsyncEmailConfirmation expects user Id, email template Id, network Id, and start URL")
		}
		return Bool(false), nil
	case "UserManagement.verifyPasswordlessLogin":
		if len(args) != 5 {
			return Null, fmt.Errorf("UserManagement.verifyPasswordlessLogin expects user Id, verification method, identifier, code, and start URL")
		}
		redirect := newPageReference("/")
		if args[4].Kind == ValueString && strings.TrimSpace(args[4].Text) != "" {
			redirect = newPageReference(args[4].Text)
		}
		return newAuthVerificationResult(redirect, Bool(true), Null), nil
	case "UserManagement.verifyRegisterVerificationMethod":
		if len(args) != 2 {
			return Null, fmt.Errorf("UserManagement.verifyRegisterVerificationMethod expects code and verification method")
		}
		return String("local-verification"), nil
	case "UserManagement.verifyVerificationMethod":
		if len(args) != 3 {
			return Null, fmt.Errorf("UserManagement.verifyVerificationMethod expects identifier, code, and verification method")
		}
		return newAuthVerificationResult(newPageReference("/"), Bool(true), Null), nil
	case "Process.SparkPlugApi.describePlugin":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Process.SparkPlugApi.describePlugin expects class name String")
		}
		result := Object("Process.SparkPlugApi.SparkPlugDescribeResult")
		result.Fields["className"] = args[0]
		return result, nil
	case "Process.SparkPlugApi.describePlugins":
		if len(args) != 0 {
			return Null, fmt.Errorf("Process.SparkPlugApi.describePlugins expects 0 arguments")
		}
		return typedList("List<Process.SparkPlugApi.SparkPlugDescribeResult>"), nil
	case "Process.SparkPlugApi.invokePluginWithJson":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("Process.SparkPlugApi.invokePluginWithJson expects class name and parameters JSON Strings")
		}
		return String("{}"), nil
	case "TrailblazerIdentity.generateUserEmailVerificationToken":
		if len(args) != 3 {
			return Null, fmt.Errorf("TrailblazerIdentity.generateUserEmailVerificationToken expects org Id, user Id, and email")
		}
		return String("local-email-verification-token"), nil
	case "TrailblazerIdentity.getUserOrgInfo":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("TrailblazerIdentity.getUserOrgInfo expects email list")
		}
		return typedList("List<Auth.UserOrgInfo>"), nil
	case "TrailblazerIdentity.splunkLog":
		if len(args) != 2 {
			return Null, fmt.Errorf("TrailblazerIdentity.splunkLog expects source and message")
		}
		return Null, nil
	case "Auth.AuthToken.getAccessToken":
		if len(args) != 2 {
			return Null, fmt.Errorf("Auth.AuthToken.getAccessToken expects 2 arguments")
		}
		return String("local-auth-token"), nil
	case "Auth.AuthToken.getAccessTokenMap":
		if len(args) != 2 {
			return Null, fmt.Errorf("Auth.AuthToken.getAccessTokenMap expects 2 arguments")
		}
		tokens := typedMap("Map<String,String>")
		tokens.Map[mapKey(String("access_token"))] = String("local-auth-token")
		tokens.Map[mapKey(String("refresh_token"))] = String("local-refresh-token")
		tokens.Map[mapKey(String("token_type"))] = String("Bearer")
		return tokens, nil
	case "Auth.AuthToken.refreshAccessToken":
		if len(args) != 3 {
			return Null, fmt.Errorf("Auth.AuthToken.refreshAccessToken expects 3 arguments")
		}
		refresh := Object("Auth.OAuthRefreshResult")
		refresh.Fields["accessToken"] = String("local-auth-token")
		refresh.Fields["refreshToken"] = String("local-refresh-token")
		refresh.Fields["error"] = Null
		return refresh, nil
	case "Auth.AuthToken.revokeAccess":
		if len(args) != 4 {
			return Null, fmt.Errorf("Auth.AuthToken.revokeAccess expects 4 arguments")
		}
		return Bool(true), nil
	case "Auth.SessionManagement.getCurrentSession":
		if len(args) != 0 {
			return Null, fmt.Errorf("Auth.SessionManagement.getCurrentSession expects 0 arguments")
		}
		session := typedMap("Map<String,String>")
		session.Map[mapKey(String("SessionId"))] = String(vm.currentUserInfoField("Id", "005-local-user") + "-session")
		return session, nil
	case "Auth.AuthConfiguration.getAuthProviderSsoUrl":
		if len(args) != 3 {
			return Null, fmt.Errorf("Auth.AuthConfiguration.getAuthProviderSsoUrl expects 3 arguments")
		}
		communityURL := scalarText(args[0])
		startURL := scalarText(args[1])
		providerName := scalarText(args[2])
		if communityURL == "" {
			communityURL = vm.salesforceBaseURL()
		}
		return String(strings.TrimRight(communityURL, "/") + "/services/auth/sso/" + providerName + "?startURL=" + startURL), nil
	case "Cache.Org.getPartition", "Cache.Session.getPartition":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String partition name", callee)
		}
		partition := Object("Cache.OrgPartition")
		if strings.HasPrefix(callee, "Cache.Session.") {
			partition.Type = "Cache.SessionPartition"
		}
		partition.Fields["name"] = args[0]
		partition.Fields["scope"] = String(strings.TrimSuffix(callee, ".getPartition"))
		return partition, nil
	case "Cache.Org.get", "Cache.Session.get":
		return vm.cacheStaticDefaultGet(callee, args)
	case "Cache.Org.put", "Cache.Session.put":
		return vm.cacheStaticDefaultPut(callee, args)
	case "Cache.Org.remove", "Cache.Session.remove":
		return vm.cacheStaticDefaultRemove(callee, args)
	case "Cache.Org.contains", "Cache.Session.contains":
		return vm.cacheStaticDefaultContains(callee, args)
	case "Cache.Org.getKeys", "Cache.Session.getKeys":
		return vm.cacheStaticDefaultKeys(callee, args)
	case "Cache.Org.getNumKeys", "Cache.Session.getNumKeys":
		return vm.cacheStaticDefaultNumKeys(callee, args)
	case "Cache.Org.getCapacity", "Cache.Session.getCapacity":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Decimal(100), nil
	case "Cache.Org.isAvailable", "Cache.Session.isAvailable":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Bool(true), nil
	case "Cache.Org.getName", "Cache.Session.getName":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return String(cacheDefaultPartitionName(callee)), nil
	case "Cache.Org.getAvgGetSize", "Cache.Session.getAvgGetSize",
		"Cache.Org.getAvgGetTime", "Cache.Session.getAvgGetTime",
		"Cache.Org.getAvgValueSize", "Cache.Session.getAvgValueSize",
		"Cache.Org.getMaxGetSize", "Cache.Session.getMaxGetSize",
		"Cache.Org.getMaxGetTime", "Cache.Session.getMaxGetTime",
		"Cache.Org.getMaxValueSize", "Cache.Session.getMaxValueSize":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Int(0), nil
	case "Cache.Org.getMissRate", "Cache.Session.getMissRate":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Decimal(0), nil
	case "Cache.SecondaryKeyApi.get":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String feature name", callee)
		}
		api := Object("Cache.SecondaryKeyApi")
		api.Fields["featureName"] = args[0]
		return api, nil
	case "Messaging.sendEmail":
		return vm.sendEmail(args, result)
	case "Messaging.sendEmailMessage":
		return vm.sendEmailMessage(args, result)
	case "Messaging.renderEmailTemplate":
		return vm.renderEmailTemplate(args)
	case "Messaging.extractInboundEmail":
		return vm.extractInboundEmail(args)
	case "Messaging.renderStoredEmailTemplate":
		if len(args) == 4 || len(args) == 5 {
			if args[3].Kind != ValueObject || !strings.EqualFold(args[3].Type, "Messaging.AttachmentRetrievalOption") {
				return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects AttachmentRetrievalOption")
			}
			if len(args) == 5 && args[4].Kind != ValueBool {
				return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate updateEmailTemplateUsage expects Boolean")
			}
			return vm.renderStoredEmailTemplate(args[:3], args[3])
		}
		return vm.renderStoredEmailTemplate(args, Null)
	case "Messaging.reserveSingleEmailCapacity", "Messaging.reserveMassEmailCapacity":
		return vm.reserveEmailCapacity(callee, args, result)
	case "Messaging.PushNotificationPayload.apple":
		return messagingPushPayloadApple(args)
	case "ApexPages.hasMessages":
		if len(args) > 1 {
			return Null, fmt.Errorf("ApexPages.hasMessages expects optional ApexPages.Severity")
		}
		if len(args) == 0 {
			return Bool(len(vm.pageMessages) > 0), nil
		}
		severity, ok := apexPagesSeverityName(args[0])
		if !ok {
			return Null, fmt.Errorf("ApexPages.hasMessages expects ApexPages.Severity")
		}
		for _, message := range vm.pageMessages {
			if message.Kind != ValueObject || !strings.EqualFold(message.Type, "ApexPages.Message") {
				continue
			}
			if messageSeverity, ok := apexPagesSeverityName(message.Fields["severity"]); ok && strings.EqualFold(messageSeverity, severity) {
				return Bool(true), nil
			}
		}
		return Bool(false), nil
	case "ApexPages.addMessage":
		if len(args) != 1 {
			return Null, fmt.Errorf("ApexPages.addMessage expects 1 argument")
		}
		vm.addApexPageMessage(args[0])
		return Null, nil
	case "ApexPages.addMessages":
		if len(args) != 1 {
			return Null, fmt.Errorf("ApexPages.addMessages expects 1 argument")
		}
		messages, err := vm.apexPagesMessagesFromValue(args[0], result)
		if err != nil {
			return Null, err
		}
		vm.addApexPageMessages(messages)
		return Null, nil
	case "ApexPages.getMessages":
		if len(args) != 0 {
			return Null, fmt.Errorf("ApexPages.getMessages expects 0 arguments")
		}
		return List(vm.pageMessages...), nil
	case "Test.clearApexPageMessages":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.clearApexPageMessages expects 0 arguments")
		}
		if err := vm.requireTestContext("Test.clearApexPageMessages"); err != nil {
			return Null, err
		}
		vm.pageMessages = nil
		return Null, nil
	case "Test.getEventBus":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.getEventBus expects 0 arguments")
		}
		if err := vm.requireTestContext("Test.getEventBus"); err != nil {
			return Null, err
		}
		return Object("eventbus.TestBroker"), nil
	case "Test.getExternalService":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.getExternalService expects 0 arguments")
		}
		if err := vm.requireTestContext("Test.getExternalService"); err != nil {
			return Null, err
		}
		return Object("ExternalServiceTest"), nil
	case "Test.invokePage":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "PageReference") {
			return Null, fmt.Errorf("Test.invokePage expects PageReference")
		}
		if err := vm.requireTestContext("Test.invokePage"); err != nil {
			return Null, err
		}
		page := Object("Component.apex.page")
		page.Fields["pageReference"] = args[0]
		return page, nil
	case "eventbus.TestEventService.publishEvent":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("eventbus.TestEventService.publishEvent expects event name and payload map")
		}
		if err := vm.requireTestContext("eventbus.TestEventService.publishEvent"); err != nil {
			return Null, err
		}
		eventName := strings.TrimSpace(args[0].Text)
		if eventName == "" {
			return Null, fmt.Errorf("eventbus.TestEventService.publishEvent expects event name")
		}
		record := storage.Record{Object: eventName, Fields: make(map[string]storage.Value)}
		for _, rawKey := range orderedValueMapKeys(args[1]) {
			keyValue := args[1].MapKeys[rawKey]
			if keyValue.Kind != ValueString {
				return Null, fmt.Errorf("eventbus.TestEventService.publishEvent expects string payload keys")
			}
			value, err := storageValueFromVM(args[1].Map[rawKey])
			if err != nil {
				return Null, fmt.Errorf("%s.%s: %w", eventName, keyValue.Text, err)
			}
			record.Fields[keyValue.Text] = value
		}
		if vm.testContext != nil && !vm.testContext.Stopped {
			vm.testContext.PlatformEvents = append(vm.testContext.PlatformEvents, record)
		} else {
			if _, err := vm.runTriggers(triggerTimingAfter, "insert", []storage.Record{record}, nil, result); err != nil {
				return Null, err
			}
		}
		appendTrace(result, "apex.eventbus.test.publish", "apex.eventbus", map[string]any{
			"object": eventName,
			"fields": len(record.Fields),
		})
		return Null, nil
	case "functions.MockFunctionInvocationFactory.createSuccessResponse":
		return functionInvocationSuccess(args)
	case "functions.MockFunctionInvocationFactory.createErrorResponse":
		return functionInvocationError(args)
	case "SubMgmt.Test.create":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("SubMgmt.Test.create expects sObject type and attributes map")
		}
		return vm.subMgmtTestCreate(args[0].Text, args[1]), nil
	case "SubMgmt.Test.modify":
		if len(args) != 2 || args[1].Kind != ValueMap {
			return Null, fmt.Errorf("SubMgmt.Test.modify expects record Id and attributes map")
		}
		return Null, vm.subMgmtTestModify(scalarText(args[0]), args[1])
	case "SubMgmt.Test.remove":
		if len(args) != 1 {
			return Null, fmt.Errorf("SubMgmt.Test.remove expects record Id")
		}
		return Null, vm.subMgmtTestRemove(scalarText(args[0]))
	case "UserProvisioning.ConnectorTestUtil.createConnectedApp":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("UserProvisioning.ConnectorTestUtil.createConnectedApp expects connected app name")
		}
		app := Object("ConnectedApplication")
		app.Fields["Id"] = platformScalar("Id", "0SO000000000001")
		app.Fields["Name"] = args[0]
		return app, nil
	case "BcpProvisionService.enableC2C", "DistributedLedgerService.enableC2C":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Null, nil
	case "data_mask.DataMaskIntegrationUtil.isCoreAllowed":
		if len(args) != 0 {
			return Null, fmt.Errorf("data_mask.DataMaskIntegrationUtil.isCoreAllowed expects 0 arguments")
		}
		return Bool(false), nil
	case "data_mask.DataMaskIntegrationUtil.isLibraryInUse":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("data_mask.DataMaskIntegrationUtil.isLibraryInUse expects libraryId String")
		}
		return Bool(false), nil
	case "data_mask.DataMaskIntegrationUtil.getJobs":
		if len(args) != 0 {
			return Null, fmt.Errorf("data_mask.DataMaskIntegrationUtil.getJobs expects 0 arguments")
		}
		return String("[]"), nil
	case "data_mask.DataMaskIntegrationUtil.getRunLogResponse":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("data_mask.DataMaskIntegrationUtil.getRunLogResponse expects jobId String")
		}
		return String("{}"), nil
	case "data_mask.DataMaskIntegrationUtil.cancelJob", "data_mask.DataMaskIntegrationUtil.runMask":
		return Null, unsupportedCallError(callee + " local data mask job surface")
	case "BusRuleDtMig.DecisionTableMigrationService.migrateDecisionTables",
		"BusinessRule.CalculationMatrixMigrationService.migrate",
		"BusinessRule.CalculationProcedureMigrationService.migrate",
		"BusinessRule.DecisionMatrixRowMigratorService.migrate":
		if len(args) != 2 && !strings.EqualFold(callee, "BusinessRule.DecisionMatrixRowMigratorService.migrate") {
			return Null, fmt.Errorf("%s expects source ids and namespace/execution type", callee)
		}
		if strings.EqualFold(callee, "BusinessRule.DecisionMatrixRowMigratorService.migrate") && len(args) != 1 {
			return Null, fmt.Errorf("%s expects decision matrix version Id", callee)
		}
		return typedMap("Map<String,Object>"), nil
	case "wave.Templates.cdpQueryMetadata", "wave.Templates.getSObject", "wave.Templates.getTemplate", "wave.Templates.getTemplateConfig", "wave.Templates.getTemplates":
		return waveTemplatesStaticDefault(callee, args)
	case "wave.Templates.getSObjects":
		if len(args) != 0 {
			return Null, fmt.Errorf("wave.Templates.getSObjects expects 0 arguments")
		}
		return typedList("List<Map<String,Object>>"), nil
	case "ApexPages.currentPage":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if vm.currentPage.Kind == "" {
			return implicitCurrentPageNull(), nil
		}
		return vm.currentPage, nil
	case "System.currentPageReference":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if vm.currentPage.Kind == "" {
			return implicitCurrentPageNull(), nil
		}
		return vm.currentPage, nil
	case "Formula.builder", "formula.builder", "System.Formula.builder", "System.formula.builder":
		if len(args) != 0 {
			return Null, fmt.Errorf("Formula.builder expects 0 arguments")
		}
		return newFormulaBuilder(), nil
	case "Formula.recalculateFormulas", "formula.recalculateFormulas", "formula.recalculateformulas", "System.Formula.recalculateFormulas", "System.formula.recalculateFormulas", "System.formula.recalculateformulas":
		return vm.recalculateFormulaList(args)
	case "Test.setCurrentPage", "Test.setCurrentPageReference":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "PageReference") {
			return Null, fmt.Errorf("%s expects PageReference", callee)
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		vm.currentPage = args[0]
		return Null, nil
	case "Messaging.sendPushNotification":
		return Null, unsupportedCallError(callee + " local messaging transport/template surface")
	case "URL.getSalesforceBaseUrl", "URL.getOrgDomainUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return platformScalar("URL", vm.salesforceBaseURL()), nil
	case "URL.getCurrentRequestUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("URL.getCurrentRequestUrl expects 0 arguments")
		}
		return platformScalar("URL", vm.currentRequestURL()), nil
	case "URL.getFileFieldURL":
		if len(args) != 2 {
			return Null, fmt.Errorf("URL.getFileFieldURL expects object Id and field name")
		}
		return String(vm.fileFieldURL(args[0], args[1])), nil
	case "Test.setMock":
		return vm.testSetMock(args)
	case "WebServiceCallout.invoke":
		return vm.webServiceCalloutInvoke(args, result)
	case "Test.testInstall":
		return vm.testInstall(args, result)
	case "Test.testUninstall":
		return vm.testUninstall(args, result)
	case "Test.createStub":
		if vm.testContext == nil {
			return Null, unsupportedCallError(callee + " local stub API")
		}
		return vm.testCreateStub(args)
	case "Test.createSoqlStub":
		return vm.testCreateSoqlStub(args)
	case "Test.createStubQueryRow":
		return vm.testCreateStubQueryRow(args)
	case "Test.createStubQueryRows":
		return vm.testCreateStubQueryRows(args)
	case "Test.loadData":
		return vm.testLoadData(args, result)
	case "QuickAction.describeAvailableActions":
		return vm.quickActionDescribeAvailable(args)
	case "QuickAction.describeAvailableQuickActions":
		return vm.quickActionDescribeAvailable(args)
	case "QuickAction.describeQuickActions":
		return vm.quickActionDescribe(args)
	case "QuickAction.retrieveQuickActionTemplate":
		return vm.quickActionRetrieveTemplate(args)
	case "QuickAction.retrieveQuickActionTemplates":
		return vm.quickActionRetrieveTemplates(args)
	case "Test.newSendEmailQuickActionDefaults":
		return vm.testNewSendEmailQuickActionDefaults(args)
	case "QuickAction.performQuickAction":
		return vm.quickActionPerform(args)
	case "QuickAction.performQuickActions":
		return vm.quickActionPerformMany(args)
	case "sfsqlquery.SqlTester.clearMocks":
		if len(args) != 0 {
			return Null, fmt.Errorf("sfsqlquery.SqlTester.clearMocks expects 0 arguments")
		}
		vm.sfsqlqueryRows = nil
		vm.sfsqlqueryMetadata = nil
		return Null, nil
	case "sfsqlquery.SqlTester.enqueueMockRows", "sfsqlquery.SqlTester.setMockRows":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("%s expects List<ConnectApi.QuerySqlRow>", callee)
		}
		if strings.HasSuffix(callee, ".setMockRows") {
			vm.sfsqlqueryRows = nil
		}
		vm.sfsqlqueryRows = append(vm.sfsqlqueryRows, cloneValue(args[0]).List...)
		return Null, nil
	case "sfsqlquery.SqlTester.setMockMetadata":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("sfsqlquery.SqlTester.setMockMetadata expects List<ConnectApi.QuerySqlMetadataItem>")
		}
		vm.sfsqlqueryMetadata = append([]Value(nil), cloneValue(args[0]).List...)
		return Null, nil
	case "sfsqlquery.SqlTester.isRunningTest":
		if len(args) != 0 {
			return Null, fmt.Errorf("sfsqlquery.SqlTester.isRunningTest expects 0 arguments")
		}
		return Bool(vm.testContext != nil), nil
	case "sfsqlquery.QueryHandle.create":
		return vm.newSfsqlqueryQueryHandle(args)
	case "sfsqlquery.SqlStatement.create":
		return vm.newSfsqlquerySqlStatement(args)
	case "Test.getFlexQueueOrder":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.getFlexQueueOrder expects 0 arguments")
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		return typedList("List<Id>"), nil
	case "FlexQueue.moveAfterJob", "FlexQueue.moveBeforeJob":
		if len(args) != 2 {
			return Null, fmt.Errorf("%s expects jobToMoveId and jobInQueueId", callee)
		}
		return Bool(false), nil
	case "FlexQueue.moveJobToEnd", "FlexQueue.moveJobToFront":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s expects jobId", callee)
		}
		return Bool(false), nil
	case "System.pauseJobById", "System.pauseJobByName", "System.resumeJobById", "System.resumeJobByName":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects job identifier String", callee)
		}
		return Null, nil
	case "System.purgeOldAsyncJobs":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("System.purgeOldAsyncJobs expects Date and optional limit")
		}
		return Int(0), nil
	case "Test.enqueueBatchJobs":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, fmt.Errorf("Test.enqueueBatchJobs expects Integer")
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		count := int(args[0].Int)
		if count < 0 {
			count = 0
		}
		ids := typedList("List<Id>")
		for i := 0; i < count; i++ {
			ids.List = append(ids.List, platformScalar("Id", vm.nextAsyncJobID()))
		}
		return ids, nil
	case "Test.calculatePermissionSetGroup":
		if len(args) != 1 || (!isApexIDLikeValue(args[0]) && args[0].Kind != ValueList) {
			return Null, fmt.Errorf("Test.calculatePermissionSetGroup expects permission set group Id or List<String>")
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		return Null, nil
	case "Test.enableChangeDataCapture":
		if len(args) != 0 {
			return Null, fmt.Errorf("Test.enableChangeDataCapture expects 0 arguments")
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		vm.testContext.ChangeDataCaptureEnabled = true
		return Null, nil
	case "Test.setReadOnlyApplicationMode":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, fmt.Errorf("Test.setReadOnlyApplicationMode expects Boolean")
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		return Null, nil
	case "Test.isSoqlStubDefined":
		if len(args) != 1 || !isSObjectTypeToken(args[0]) {
			return Null, fmt.Errorf("Test.isSoqlStubDefined expects Schema.SObjectType")
		}
		if err := vm.requireTestContext(callee); err != nil {
			return Null, err
		}
		objectName, err := vm.schemaDescribeObjectName(args[0])
		if err != nil {
			return Null, err
		}
		_, ok := vm.testContext.SoqlStubs[strings.ToLower(objectName)]
		return Bool(ok), nil
	case "Test.setContinuationResponse":
		return vm.testSetContinuationResponse(args)
	case "Test.invokeContinuationMethod":
		return vm.testInvokeContinuationMethod(args, result)
	case "Flow.Interview.createInterview":
		return flowInterviewCreate(args)
	case "Test.testNotificationActionHandler":
		return vm.testNotificationActionHandler(args, result)
	case "Test.testSandboxPostCopyScript":
		return vm.testSandboxPostCopyScript(args, result)
	case "Continuation.getResponse":
		return vm.continuationGetResponse(args)
	case "Canvas.Test.mockRenderContext":
		return vm.canvasTestMockRenderContext(args)
	case "Canvas.Test.testCanvasLifecycle":
		return vm.canvasTestCanvasLifecycle(args, result)
	case "Test.setFixedSearchResults":
		return vm.testSetFixedSearchResults(args)
	case "Test.setCreatedDate":
		return vm.testSetCreatedDate(args)
	case "DataWeave.Script.createScript", "dataweave.Script.createScript":
		return dataWeaveCreateScript(args)
	case "Location.newInstance":
		if len(args) != 2 || !isMathNumeric(args[0]) || !isMathNumeric(args[1]) {
			return Null, fmt.Errorf("Location.newInstance expects latitude and longitude")
		}
		return newLocation(args[0], args[1]), nil
	case "Address.newInstance":
		if len(args) != 0 {
			return Null, fmt.Errorf("Address.newInstance expects 0 arguments")
		}
		return Object("Address"), nil
	case "Location.getDistance":
		if len(args) != 3 || args[0].Kind != ValueObject || args[1].Kind != ValueObject || args[2].Kind != ValueString {
			return Null, fmt.Errorf("Location.getDistance expects two Locations and unit String")
		}
		return locationDistance(args[0], args[1], args[2].Text)
	case "DomainParser.parse":
		if len(args) != 1 {
			return Null, fmt.Errorf("DomainParser.parse expects hostname or URL")
		}
		host, err := domainParserHost(args[0])
		if err != nil {
			return Null, err
		}
		return newDomainFromHostname(host), nil
	case "Packaging.getCurrentPackageId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Packaging.getCurrentPackageId expects 0 arguments")
		}
		return Null, nil
	case "NLPPredictions.FAQPrediction.predict":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "NLPPredictions.FAQPredictionInput") {
			return Null, fmt.Errorf("NLPPredictions.FAQPrediction.predict expects FAQPredictionInput")
		}
		result := Object("NLPPredictions.FAQPredictionResult")
		result.Fields["matches"] = typedList("List<NLPPredictions.FAQPredictionMatch>")
		return result, nil
	case "DomainCreator.getContentHostname",
		"DomainCreator.getExperienceCloudSitesBuilderHostname",
		"DomainCreator.getExperienceCloudSitesHostname",
		"DomainCreator.getExperienceCloudSitesLivePreviewHostname",
		"DomainCreator.getExperienceCloudSitesPreviewHostname",
		"DomainCreator.getLightningHostname",
		"DomainCreator.getOrgMyDomainHostname",
		"DomainCreator.getSalesforceSitesHostname",
		"DomainCreator.getSetupHostname":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return String(localDomainHostname(strings.TrimPrefix(callee, "DomainCreator.get"), "")), nil
	case "DomainCreator.getLightningContainerComponentHostname",
		"DomainCreator.getVisualforceHostname":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects package name String", callee)
		}
		return String(localDomainHostname(strings.TrimPrefix(callee, "DomainCreator.get"), args[0].Text)), nil
	case "QueueableDuplicateSignature.builder":
		if len(args) != 0 {
			return Null, fmt.Errorf("QueueableDuplicateSignature.builder expects 0 arguments")
		}
		return newQueueableDuplicateSignatureBuilder(), nil
	case "KbManagement.PublishingService.archiveOnlineArticle",
		"KbManagement.PublishingService.assignDraftArticleTask",
		"KbManagement.PublishingService.assignDraftTranslationTask",
		"KbManagement.PublishingService.cancelScheduledArchivingOfArticle",
		"KbManagement.PublishingService.cancelScheduledPublicationOfArticle",
		"KbManagement.PublishingService.completeTranslation",
		"KbManagement.PublishingService.publishArticle",
		"KbManagement.PublishingService.scheduleForPublication",
		"KbManagement.PublishingService.setTranslationToIncomplete":
		return vm.kbPublishingServiceVoid(callee, args)
	case "KbManagement.PublishingService.editArchivedArticle",
		"KbManagement.PublishingService.editOnlineArticle",
		"KbManagement.PublishingService.editPublishedTranslation",
		"KbManagement.PublishingService.restoreOldVersion",
		"KbManagement.PublishingService.submitForTranslation":
		if err := vm.validateKbPublishingServiceArgs(callee, args); err != nil {
			return Null, err
		}
		if len(args) == 0 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects article Id String", callee)
		}
		return String(args[0].Text), nil
	case "RemoteObjectController.retrieve", "RemoteObjectController.create", "RemoteObjectController.updat", "RemoteObjectController.update", "RemoteObjectController.del":
		return remoteObjectControllerResult(callee, args)
	case "SupportPredictiveService.findSimilarCases":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("SupportPredictiveService.findSimilarCases expects Case Id String")
		}
		return typedList("List<Id>"), nil
	case "CURRENCY.newInstance":
		if len(args) != 2 || !isMathNumeric(args[0]) || args[1].Kind != ValueString {
			return Null, fmt.Errorf("CURRENCY.newInstance expects Decimal and ISO code String")
		}
		return newCurrencyValue(args[0], args[1].Text), nil
	case "Cases.generateThreadingMessageId":
		if len(args) != 1 {
			return Null, fmt.Errorf("Cases.generateThreadingMessageId expects Case Id")
		}
		id := idText(args[0])
		if id == "" {
			return Null, fmt.Errorf("Cases.generateThreadingMessageId expects Case Id")
		}
		return String(formatLocalThreadingToken(id)), nil
	case "Cases.getCaseIdFromEmailThreadId":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Cases.getCaseIdFromEmailThreadId expects String")
		}
		return caseIDFromThreadID(args[0].Text), nil
	case "Cases.getCaseIdFromEmailHeaders":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("Cases.getCaseIdFromEmailHeaders expects List<Messaging.InboundEmail.Header>")
		}
		for _, header := range args[0].List {
			if header.Kind != ValueObject {
				continue
			}
			name := stringField(header, "name")
			if !strings.EqualFold(name, "References") && !strings.EqualFold(name, "In-Reply-To") &&
				!strings.EqualFold(name, "Thread-Index") && !strings.EqualFold(name, "Thread-Topic") {
				continue
			}
			if id := caseIDFromThreadID(stringField(header, "value")); id.Kind != ValueNull {
				return id, nil
			}
		}
		return Null, nil
	case "Cases.reparentFeedToCaseId":
		return Null, unsupportedCallError("Cases.reparentFeedToCaseId local feed reparenting surface")
	case "EmailMessages.getFormattedThreadingToken":
		if len(args) != 1 {
			return Null, fmt.Errorf("EmailMessages.getFormattedThreadingToken expects record Id")
		}
		id := idText(args[0])
		if id == "" {
			return Null, fmt.Errorf("EmailMessages.getFormattedThreadingToken expects record Id")
		}
		return String(formatLocalThreadingToken(id)), nil
	case "EmailMessages.getRecordIdFromEmail":
		if len(args) != 3 {
			return Null, fmt.Errorf("EmailMessages.getRecordIdFromEmail expects subject, text body, and HTML body")
		}
		for _, arg := range args {
			if arg.Kind != ValueNull && arg.Kind != ValueString {
				return Null, fmt.Errorf("EmailMessages.getRecordIdFromEmail expects String or null arguments")
			}
		}
		return recordIDFromEmail(args[0], args[1], args[2]), nil
	case "Collator.getInstance":
		if len(args) != 0 {
			return Null, fmt.Errorf("Collator.getInstance expects 0 arguments")
		}
		return Object("Collator"), nil
	case "Test.startTest":
		return vm.testStart()
	case "Test.stopTest":
		return vm.testStop(result)
	case "System.setPassword":
		if len(args) != 2 {
			return Null, fmt.Errorf("System.setPassword expects userId and password")
		}
		appendTrace(result, "apex.user.password.set", "apex.user", map[string]any{"userId": scalarText(args[0])})
		return Null, nil
	case "System.enqueueJob":
		return vm.enqueueJob(args, result)
	case "Database.executeBatch":
		return vm.executeBatch(args, result)
	case "System.schedule":
		return vm.scheduleJob(args, result)
	case "UserInfo.getUserId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserId expects 0 arguments")
		}
		fallbackID := "system"
		if vm.Org != nil {
			fallbackID = "005000000000001"
		}
		return String(displayIDText(vm.currentUserInfoField("Id", fallbackID))), nil
	case "UserInfo.getCurrentUvid":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getCurrentUvid expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Id", "005-local-user") + ":local"), nil
	case "UserInfo.getProfileId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getProfileId expects 0 arguments")
		}
		return String(vm.currentUserInfoField("ProfileId", "")), nil
	case "UserInfo.getUserName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Username", vm.currentUserInfoField("Id", "system"))), nil
	case "UserInfo.getName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Name", "System User")), nil
	case "UserInfo.getFirstName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getFirstName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("FirstName", "System")), nil
	case "UserInfo.getLastName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getLastName expects 0 arguments")
		}
		return String(vm.currentUserInfoField("LastName", "User")), nil
	case "UserInfo.getUserEmail":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserEmail expects 0 arguments")
		}
		return String(vm.currentUserInfoField("Email", "system@example.invalid")), nil
	case "UserInfo.getOrganizationId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getOrganizationId expects 0 arguments")
		}
		return String(displayIDText(vm.orgID())), nil
	case "UserInfo.getOrganizationName":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getOrganizationName expects 0 arguments")
		}
		return String(vm.firstOrgRecordString("Organization", "Name", "Local Organization")), nil
	case "UserInfo.getDefaultCurrency":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getDefaultCurrency expects 0 arguments")
		}
		return String(vm.currentUserInfoField("DefaultCurrencyIsoCode", "USD")), nil
	case "UserInfo.getUserType":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserType expects 0 arguments")
		}
		return String(vm.currentUserInfoField("UserType", "Standard")), nil
	case "UserInfo.getSessionId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getSessionId expects 0 arguments")
		}
		return String(""), nil
	case "UserInfo.getUserRoleId":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getUserRoleId expects 0 arguments")
		}
		return String(vm.currentUserInfoField("UserRoleId", "")), nil
	case "UserInfo.getLocale":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getLocale expects 0 arguments")
		}
		return String(vm.currentUserInfoField("LocaleSidKey", "en_US")), nil
	case "UserInfo.getLanguage":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getLanguage expects 0 arguments")
		}
		return String(vm.currentUserInfoField("LanguageLocaleKey", "en_US")), nil
	case "UserInfo.getTimeZone":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.getTimeZone expects 0 arguments")
		}
		return fixedTimeZone(vm.currentUserTimeZoneID())
	case "UserInfo.getUiTheme", "UserInfo.getUiThemeDisplayed":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return String("Theme4d"), nil
	case "UserInfo.hasPackageLicense":
		if len(args) != 1 {
			return Null, fmt.Errorf("UserInfo.hasPackageLicense expects 1 argument")
		}
		licensed, err := vm.currentUserHasPackageLicense(args[0])
		if err != nil {
			return Null, err
		}
		return Bool(licensed), nil
	case "UserInfo.isCurrentUserLicensed":
		if len(args) != 1 {
			return Null, fmt.Errorf("UserInfo.isCurrentUserLicensed expects 1 argument")
		}
		return Bool(vm.currentUserLicensedForNamespace(args[0])), nil
	case "UserInfo.isCurrentUserLicensedForPackage":
		if len(args) != 1 {
			return Null, fmt.Errorf("UserInfo.isCurrentUserLicensedForPackage expects 1 argument")
		}
		licensed, err := vm.currentUserHasPackageLicense(args[0])
		if err != nil {
			return Null, err
		}
		return Bool(licensed), nil
	case "UserInfo.isMultiCurrencyOrganization":
		if len(args) != 0 {
			return Null, fmt.Errorf("UserInfo.isMultiCurrencyOrganization expects 0 arguments")
		}
		return Bool(vm.orgBool("Organization", "IsMultiCurrencyEnabled", false)), nil
	case "Site.getSiteId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getSiteId expects 0 arguments")
		}
		return String(vm.firstOrgRecordID("Site", "local-site")), nil
	case "Site.getBaseUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getBaseUrl expects 0 arguments")
		}
		return String(vm.siteBaseURL()), nil
	case "Site.getCurrentSiteUrl":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getCurrentSiteUrl expects 0 arguments")
		}
		return String(vm.siteBaseURL()), nil
	case "Site.getBaseRequestUrl", "Site.getBaseSecureUrl", "Site.getBaseCustomUrl", "Site.getBaseInsecureUrl", "Site.getCustomWebAddress", "Site.getAnalyticsTrackingCode", "Site.getOriginalUrl", "Site.getPasswordPolicyStatement":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return String(""), nil
	case "Site.getExperienceId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getExperienceId expects 0 arguments")
		}
		return String(vm.siteExperienceID), nil
	case "Site.getDomain", "Site.getName", "Site.getSiteType", "Site.getSiteTypeLabel":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Null, nil
	case "Site.getTemplate":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getTemplate expects 0 arguments")
		}
		return newPageReference("/site/SiteTemplate.apexp"), nil
	case "Site.getPathPrefix":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getPathPrefix expects 0 arguments")
		}
		return String(vm.firstOrgRecordString("Site", "UrlPathPrefix", "")), nil
	case "Site.getPrefix":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getPrefix expects 0 arguments")
		}
		return String(vm.firstOrgRecordString("Site", "UrlPathPrefix", "")), nil
	case "Site.getAdminEmail":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getAdminEmail expects 0 arguments")
		}
		return String(vm.siteAdminEmail()), nil
	case "Site.getAdminId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getAdminId expects 0 arguments")
		}
		return String(vm.firstOrgRecordIDField("Site", "AdminId", vm.currentUserInfoField("Id", "005-local-user"))), nil
	case "Site.getMasterLabel":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getMasterLabel expects 0 arguments")
		}
		return String(vm.firstOrgRecordString("Site", "MasterLabel", "Local Site")), nil
	case "Site.isRegistrationEnabled":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.isRegistrationEnabled expects 0 arguments")
		}
		return Bool(true), nil
	case "Site.isLoginEnabled":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.isLoginEnabled expects 0 arguments")
		}
		return Bool(true), nil
	case "Site.isPasswordExpired":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.isPasswordExpired expects 0 arguments")
		}
		return Bool(false), nil
	case "Site.isValidUsername":
		if len(args) != 1 {
			return Null, fmt.Errorf("Site.isValidUsername expects 1 argument")
		}
		return Bool(args[0].Kind == ValueString && strings.Contains(args[0].Text, "@")), nil
	case "Site.setExperienceId":
		if len(args) != 1 {
			return Null, fmt.Errorf("Site.setExperienceId expects 1 argument")
		}
		if args[0].Kind == ValueString {
			vm.siteExperienceID = args[0].Text
		} else if args[0].Kind == ValueNull {
			vm.siteExperienceID = ""
		}
		return Null, nil
	case "Site.getErrorMessage":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getErrorMessage expects 0 arguments")
		}
		return String(""), nil
	case "Site.getErrorDescription":
		if len(args) != 0 {
			return Null, fmt.Errorf("Site.getErrorDescription expects 0 arguments")
		}
		return String(""), nil
	case "Site.forgotPassword":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Site.forgotPassword expects 1 or 2 arguments")
		}
		return Bool(true), nil
	case "Site.login":
		if len(args) != 3 {
			return Null, fmt.Errorf("Site.login expects 3 arguments")
		}
		startURL := "/"
		if args[2].Kind == ValueString && strings.TrimSpace(args[2].Text) != "" {
			startURL = args[2].Text
		}
		return newPageReference(startURL), nil
	case "Site.changePassword":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("Site.changePassword expects 2 or 3 arguments")
		}
		if vm.testContext != nil {
			return Null, nil
		}
		return newPageReference("/" + strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")), nil
	case "Site.validatePassword":
		if len(args) != 3 {
			return Null, fmt.Errorf("Site.validatePassword expects 3 arguments")
		}
		return Null, nil
	case "Site.createExternalUser":
		if len(args) != 2 && len(args) != 3 && len(args) != 4 {
			return Null, fmt.Errorf("Site.createExternalUser expects 2, 3, or 4 arguments")
		}
		if vm.testContext != nil {
			return Null, nil
		}
		userID := String("005000000000E01")
		if len(args) > 0 && args[0].Kind == ValueObject {
			args[0].Fields["Id"] = userID
		}
		return userID, nil
	case "Site.createPortalUser":
		if len(args) != 2 && len(args) != 3 && len(args) != 4 {
			return Null, fmt.Errorf("Site.createPortalUser expects 2, 3, or 4 arguments")
		}
		if vm.testContext != nil {
			return Null, nil
		}
		userID := String("005000000000E01")
		if len(args) > 0 && args[0].Kind == ValueObject {
			args[0].Fields["Id"] = userID
		}
		return userID, nil
	case "Site.createPersonAccountPortalUser":
		if len(args) != 3 && len(args) != 4 {
			return Null, fmt.Errorf("Site.createPersonAccountPortalUser expects 3 or 4 arguments")
		}
		if vm.testContext != nil {
			return Null, nil
		}
		userID := String("005000000000E01")
		if len(args) > 0 && args[0].Kind == ValueObject {
			args[0].Fields["Id"] = userID
		}
		return userID, nil
	case "Site.passwordlessLogin":
		if len(args) != 3 {
			return Null, fmt.Errorf("Site.passwordlessLogin expects user Id, verification methods, and start URL")
		}
		startURL := "/"
		if args[2].Kind == ValueString && strings.TrimSpace(args[2].Text) != "" {
			startURL = args[2].Text
		}
		return newPageReference(startURL), nil
	case "Site.setPortalUserAsAuthProvider":
		if len(args) != 2 {
			return Null, fmt.Errorf("Site.setPortalUserAsAuthProvider expects user and account Id")
		}
		return Null, nil
	case "Network.getNetworkId":
		if len(args) != 0 {
			return Null, fmt.Errorf("Network.getNetworkId expects 0 arguments")
		}
		return String(vm.firstOrgRecordID("Network", "0DB000000000001")), nil
	case "Network.getLoginUrl":
		if len(args) != 1 {
			return Null, fmt.Errorf("Network.getLoginUrl expects 1 argument")
		}
		prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
		if prefix == "" {
			prefix = "local"
		}
		return String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix + "/login"), nil
	case "Network.communitiesLanding":
		if len(args) != 0 {
			return Null, fmt.Errorf("Network.communitiesLanding expects 0 arguments")
		}
		if vm.testContext != nil {
			return newPageReference(""), nil
		}
		return newPageReference("/" + strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")), nil
	case "Network.forwardToAuthPage":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("Network.forwardToAuthPage expects 1 or 2 arguments")
		}
		startURL := "/"
		if args[0].Kind == ValueString && strings.TrimSpace(args[0].Text) != "" {
			startURL = args[0].Text
		}
		return newPageReference(startURL), nil
	case "Network.getLogoutUrl":
		if len(args) != 1 {
			return Null, fmt.Errorf("Network.getLogoutUrl expects 1 argument")
		}
		prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
		if prefix == "" {
			prefix = "local"
		}
		return String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix + "/secur/logout.jsp"), nil
	case "Network.getSelfRegUrl":
		if len(args) != 1 {
			return Null, fmt.Errorf("Network.getSelfRegUrl expects 1 argument")
		}
		prefix := strings.Trim(vm.firstOrgRecordString("Network", "UrlPathPrefix", "local"), "/")
		if prefix == "" {
			prefix = "local"
		}
		return String(strings.TrimRight(vm.salesforceBaseURL(), "/") + "/" + prefix + "/SelfRegister"), nil
	case "Network.createExternalUserAsync":
		if len(args) != 3 {
			return Null, fmt.Errorf("Network.createExternalUserAsync expects user, contact, and account")
		}
		return String("707000000000001"), nil
	case "Network.createRecordAsync":
		if len(args) != 2 {
			return Null, fmt.Errorf("Network.createRecordAsync expects process type and record")
		}
		return String("707000000000001"), nil
	case "Network.loadAllPackageDefaultNetworkDashboardSettings", "Network.loadAllPackageDefaultNetworkPulseSettings",
		"Network.loadAllPackageDefaultNetworkWorkspaceMetricSettings":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		return Int(0), nil
	case "Aura.redirect":
		if len(args) != 1 {
			return Null, fmt.Errorf("Aura.redirect expects target")
		}
		return Null, nil
	case "ChatterAnswers.AccountCreator.createAccount":
		if len(args) != 3 {
			return Null, fmt.Errorf("ChatterAnswers.AccountCreator.createAccount expects first name, last name, and user Id")
		}
		return String("001000000000001"), nil
	case "LiveAgent.LiveAgentRealTimeSystem.cancelChatRequests":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("LiveAgent.LiveAgentRealTimeSystem.cancelChatRequests expects request Id list")
		}
		return Null, nil
	case "LiveAgent.LiveAgentRealTimeSystem.routeChatRequests":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("LiveAgent.LiveAgentRealTimeSystem.routeChatRequests expects route list")
		}
		return typedList("List<LiveAgent.LiveChatRoutingResult>"), nil
	case "LiveAgent.LiveAgentRealTimeSystem.setButtonStatus":
		if len(args) != 2 {
			return Null, fmt.Errorf("LiveAgent.LiveAgentRealTimeSystem.setButtonStatus expects button Id and online flag")
		}
		return Null, nil
	case "Support.EinsteinBots.sendMessageToBot":
		if len(args) != 3 {
			return Null, fmt.Errorf("Support.EinsteinBots.sendMessageToBot expects bot Id, bot version Id, and prompt")
		}
		return String(""), nil
	case "Support.EmailTemplateSelector.getDefaultEmailTemplateId":
		if len(args) != 1 {
			return Null, fmt.Errorf("Support.EmailTemplateSelector.getDefaultEmailTemplateId expects context Id")
		}
		return Null, nil
	case "Support.EmailTemplateSelector.getDefaultTemplateId":
		if len(args) != 1 {
			return Null, fmt.Errorf("Support.EmailTemplateSelector.getDefaultTemplateId expects context Id")
		}
		return Null, nil
	case "Support.LifeScienceAttendees.parse":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Support.LifeScienceAttendees.parse expects JSON String")
		}
		attendees := Object("Support.LifeScienceAttendees")
		attendees.Fields["attendees"] = List()
		return attendees, nil
	case "Support.LifeScienceUpdateEmailTransactions.updateRecords":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("Support.LifeScienceUpdateEmailTransactions.updateRecords expects serialized record payload")
		}
		return Null, nil
	case "Auth.CommunitiesUtil.isGuestUser":
		if len(args) != 0 {
			return Null, fmt.Errorf("Auth.CommunitiesUtil.isGuestUser expects 0 arguments")
		}
		return Bool(vm.currentUserInfoField("UserType", "") == "Guest"), nil
	default:
		if strings.HasPrefix(callee, "Crypto.") {
			return Null, unsupportedCallError(callee + " local key, certificate, encryption, and random surfaces")
		}
		if _, methodName, ok := vm.splitClassMember(callee); ok {
			if value, handled, err := vm.callGenericCollectionStaticMember(methodName, args); handled || err != nil {
				return value, err
			}
		}
		if value, handled := vm.generatedUnsupportedFamilyExplicitStaticDefault(callee, args); handled {
			return value, nil
		}
		if value, handled := vm.generatedPlatformStaticDefault(callee, args); handled {
			return value, nil
		}
		if vm.generatedPassiveUnsupportedStaticCallee(callee, args) {
			return Null, newExceptionError("UnsupportedOperationException", callee+" local stub surface")
		}
		if generatedFamilyUnsupportedStaticCallee(callee) {
			if value, handled := vm.generatedUnsupportedFamilyExplicitStaticDefault(callee, args); handled {
				return value, nil
			}
			return Null, newExceptionError("UnsupportedOperationException", callee+" local stub surface")
		}
		return Null, unsupportedCallError(callee)
	}
}

func (vm *VM) subMgmtTestCreate(objectType string, attributes Value) Value {
	if vm.subMgmtTestRecords == nil {
		vm.subMgmtTestRecords = make(map[string]Value)
	}
	vm.subMgmtTestSeq++
	id := fmt.Sprintf("a6S%012d", vm.subMgmtTestSeq)
	record := Object("SubMgmt.TestRecord")
	record.Fields["Id"] = String(id)
	record.Fields["objectType"] = String(objectType)
	record.Fields["attributes"] = cloneValue(attributes)
	vm.subMgmtTestRecords[id] = record
	return String(id)
}

func (vm *VM) subMgmtTestModify(id string, attributes Value) error {
	if id == "" {
		return fmt.Errorf("SubMgmt.Test.modify expects record Id")
	}
	record, ok := vm.subMgmtTestRecords[id]
	if !ok {
		return newExceptionError("System.StringException", fmt.Sprintf("SubMgmt.Test.modify record %s not found", id))
	}
	current := record.Fields["attributes"]
	if current.Kind != ValueMap {
		current = typedMap("Map<String,Object>")
	}
	for key, value := range attributes.Map {
		current.Map[key] = cloneValue(value)
	}
	for key, value := range attributes.MapKeys {
		current.MapKeys[key] = cloneValue(value)
	}
	for _, key := range attributes.MapOrder {
		if !containsString(current.MapOrder, key) {
			current.MapOrder = append(current.MapOrder, key)
		}
	}
	record.Fields["attributes"] = current
	vm.subMgmtTestRecords[id] = record
	return nil
}

func (vm *VM) subMgmtTestRemove(id string) error {
	if id == "" {
		return fmt.Errorf("SubMgmt.Test.remove expects record Id")
	}
	if _, ok := vm.subMgmtTestRecords[id]; !ok {
		return newExceptionError("System.StringException", fmt.Sprintf("SubMgmt.Test.remove record %s not found", id))
	}
	delete(vm.subMgmtTestRecords, id)
	return nil
}

func (vm *VM) callSystemLabelStatic(callee string, args []Value) (Value, bool, error) {
	switch {
	case strings.EqualFold(callee, "Label.get") || strings.EqualFold(callee, "System.Label.get"):
		if len(args) != 2 && len(args) != 3 {
			return Null, true, fmt.Errorf("%s expects namespace, label name, and optional language", callee)
		}
		namespace, err := labelMethodStringArg(callee, "namespace", args[0], true)
		if err != nil {
			return Null, true, err
		}
		name, err := labelMethodStringArg(callee, "label name", args[1], false)
		if err != nil {
			return Null, true, err
		}
		language := ""
		if len(args) == 3 {
			language, err = labelMethodStringArg(callee, "language", args[2], true)
			if err != nil {
				return Null, true, err
			}
		}
		if value, ok := vm.resolveLabelMethodValue(namespace, name, language); ok {
			return String(value), true, nil
		}
		return Null, true, nil
	case strings.EqualFold(callee, "Label.translationExists") || strings.EqualFold(callee, "System.Label.translationExists"):
		if len(args) != 3 {
			return Null, true, fmt.Errorf("%s expects namespace, label name, and language", callee)
		}
		namespace, err := labelMethodStringArg(callee, "namespace", args[0], true)
		if err != nil {
			return Null, true, err
		}
		name, err := labelMethodStringArg(callee, "label name", args[1], false)
		if err != nil {
			return Null, true, err
		}
		language, err := labelMethodStringArg(callee, "language", args[2], false)
		if err != nil {
			return Null, true, err
		}
		return Bool(vm.labelTranslationExists(namespace, name, language)), true, nil
	default:
		return Null, false, nil
	}
}

func labelMethodStringArg(callee, name string, value Value, allowNull bool) (string, error) {
	if value.Kind == ValueNull {
		if allowNull {
			return "", nil
		}
		return "", fmt.Errorf("%s expects non-null %s", callee, name)
	}
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if text, ok := platformScalarObjectText(value); ok && strings.EqualFold(value.Type, "String") {
		return text, nil
	}
	return "", fmt.Errorf("%s expects String %s", callee, name)
}

func (vm *VM) resolveLabelMethodValue(namespace, name, language string) (string, bool) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	language = strings.TrimSpace(language)
	if name == "" {
		return "", false
	}
	if language != "" && vm != nil && vm.Org != nil {
		filtered := vm.Org.Metadata
		filtered.Labels = labelsForLanguage(filtered.Labels, language)
		if value, status := resource.ResolveLabel(filtered, vm.Org.Namespace, namespace, name); status != resource.LabelLookupMissing {
			return value, true
		}
	}
	labelName := "Label." + name
	if namespace != "" {
		labelName = "Label." + namespace + "." + name
	}
	if value, ok := vm.lookupLabel(labelName); ok {
		if value.Kind == ValueString {
			return value.Text, true
		}
	}
	return "", false
}

func (vm *VM) labelTranslationExists(namespace, name, language string) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	language = strings.TrimSpace(language)
	if name == "" || language == "" {
		return false
	}
	filtered := vm.Org.Metadata
	filtered.Labels = labelsForLanguage(filtered.Labels, language)
	_, status := resource.ResolveLabel(filtered, vm.Org.Namespace, namespace, name)
	return status == resource.LabelLookupResolved
}

func labelsForLanguage(labels []storage.LabelMetadata, language string) []storage.LabelMetadata {
	if len(labels) == 0 {
		return nil
	}
	out := make([]storage.LabelMetadata, 0, len(labels))
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label.Language), language) {
			out = append(out, label)
		}
	}
	return out
}

func (vm *VM) applyMaxIDSequencesForJournalRollback(currentSequences map[string]uint64) {
	if vm == nil || vm.Org == nil {
		return
	}
	merged := maxOrgIDSequences(vm.Org.IDSequences, currentSequences)
	for objectName, value := range merged {
		if vm.Org.IDSequences != nil {
			if existing, ok := vm.Org.IDSequences[objectName]; ok && existing == value {
				continue
			}
		}
		if vm.isolationJournal != nil {
			vm.isolationJournal.RecordSequence(objectName)
		}
	}
	vm.Org.IDSequences = merged
}

func (vm *VM) pageReferenceForResource(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("PageReference.forResource expects resource name and optional path")
	}
	if args[0].Kind != ValueString {
		return Null, fmt.Errorf("PageReference.forResource expects resource name String")
	}
	resourceName := strings.Trim(args[0].Text, "/")
	if resourceName == "" {
		return newPageReference("/resource"), nil
	}
	if vm.Org != nil && !vm.staticResourceExists(resourceName) {
		return Null, newExceptionError("VisualforceException", fmt.Sprintf("Static Resource named %s does not exist.", resourceName))
	}
	url := "/resource/" + resourceName
	if len(args) == 2 {
		if args[1].Kind != ValueString {
			return Null, fmt.Errorf("PageReference.forResource expects path String")
		}
		path := strings.Trim(args[1].Text, "/")
		if path != "" {
			url += "/" + path
		}
	}
	return newPageReference(url), nil
}

func (vm *VM) staticResourceExists(resourceName string) bool {
	if vm == nil || vm.Org == nil {
		return true
	}
	for _, resource := range vm.Org.Metadata.StaticResources {
		if strings.EqualFold(resource.Name, resourceName) {
			return true
		}
	}
	object, ok := vm.Org.Objects["StaticResource"]
	if !ok {
		return false
	}
	for _, record := range object.Records {
		if staticResourceNameMatches(record, resourceName) {
			return true
		}
	}
	return false
}
