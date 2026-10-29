package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

func (vm *VM) eval(expr ir.Expr, result *Result) (Value, error) {
	switch expr.Kind {
	case ir.ExprLiteral:
		return parseLiteral(expr.Value)
	case ir.ExprVariable:
		return vm.lookup(expr.Name)
	case ir.ExprUnary:
		if expr.Left == nil {
			return Null, fmt.Errorf("unary expression %q missing operand", expr.Operator)
		}
		value, err := vm.eval(*expr.Left, result)
		if err != nil {
			return Null, err
		}
		return evalUnary(expr.Operator, value)
	case ir.ExprBinary:
		if expr.Left == nil || expr.Right == nil {
			return Null, fmt.Errorf("binary expression %q missing operand", expr.Operator)
		}
		left, err := vm.eval(*expr.Left, result)
		if err != nil {
			return Null, err
		}
		if expr.Operator == "instanceof" {
			return vm.evalInstanceOfChecked(left, expr.Right.Name)
		}
		// Native Boolean operators dereference the left operand before
		// deciding whether to evaluate the right operand.
		if (expr.Operator == "&&" || expr.Operator == "||") && left.Kind == ValueNull {
			return Null, newNullDereferenceError("Boolean operand")
		}
		if expr.Operator == "&&" && left.Kind == ValueBool && !left.Bool {
			return Bool(false), nil
		}
		if expr.Operator == "||" && left.Kind == ValueBool && left.Bool {
			return Bool(true), nil
		}
		right, err := vm.eval(*expr.Right, result)
		if err != nil {
			return Null, err
		}
		return vm.evalBinary(expr.Operator, left, right, result)
	case ir.ExprCall:
		if expr.Left != nil && expr.Left.Kind == ir.ExprCall && strings.HasSuffix(expr.Left.Callee, ".getDescribe") {
			if value, handled, err := vm.evalFieldDescribeScalarChain(&expr, result); handled {
				return value, err
			}
		}
		if value, handled, err := vm.evalLocalListCall(expr, result); handled {
			return value, err
		}
		if strings.HasPrefix(expr.Callee, "__assign:") {
			if len(expr.Args) != 1 {
				return Null, fmt.Errorf("assignment expression requires 1 operand")
			}
			target := strings.TrimPrefix(expr.Callee, "__assign:")
			value, err := vm.evalForAssignment(target, expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if err := vm.assign(target, value); err != nil {
				return Null, err
			}
			return value, nil
		}
		if strings.HasPrefix(expr.Callee, "__assignField:") {
			if expr.Left == nil || len(expr.Args) != 1 {
				return Null, fmt.Errorf("field assignment expression requires receiver and value")
			}
			receiver, err := vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
			field := strings.TrimPrefix(expr.Callee, "__assignField:")
			value, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if err := vm.assignPath(receiver, []string{field}, value); err != nil {
				return Null, err
			}
			return value, nil
		}
		if strings.HasPrefix(expr.Callee, "__prefix:") || strings.HasPrefix(expr.Callee, "__postfix:") {
			return vm.evalIncrementExpression(expr, result)
		}
		if strings.HasPrefix(expr.Callee, "__cast:") {
			if len(expr.Args) != 1 {
				return Null, fmt.Errorf("cast expression requires 1 operand")
			}
			value, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			typeName := strings.TrimPrefix(expr.Callee, "__cast:")
			return vm.coerceCastForOperand(typeName, value, expr.CastQueryOperand)
		}
		if expr.Callee == "__ternary" {
			if len(expr.Args) != 3 {
				return Null, fmt.Errorf("ternary expression requires 3 operands")
			}
			condition, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			conditionValue, err := apexConditionBool(condition, "ternary condition")
			if err != nil {
				return Null, err
			}
			selected, other := expr.Args[2], expr.Args[1]
			if conditionValue {
				selected, other = other, selected
			}
			value, err := vm.eval(selected, result)
			if err != nil {
				return Null, err
			}
			return vm.typedNullTernaryResult(expr, selected, other, value), nil
		}
		if expr.Callee == "__coalesce" {
			if len(expr.Args) != 2 {
				return Null, fmt.Errorf("null coalescing expression requires 2 operands")
			}
			left, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			emptyInlineQuery := expr.Args[0].Kind == ir.ExprSOQL && left.Kind == ValueList && len(left.List) == 0
			if left.Kind != ValueNull && !emptyInlineQuery {
				return left, nil
			}
			return vm.eval(expr.Args[1], result)
		}
		if expr.Callee == "__mapEntry" {
			if len(expr.Args) != 2 {
				return Null, fmt.Errorf("map entry requires key and value")
			}
			key, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			value, err := vm.eval(expr.Args[1], result)
			if err != nil {
				return Null, err
			}
			entry := Object("__mapEntry")
			entry.Fields["__key"] = key
			entry.Fields["__value"] = value
			return entry, nil
		}
		if strings.HasPrefix(expr.Callee, "__newArray:") {
			if len(expr.Args) != 1 {
				return Null, fmt.Errorf("array allocation requires size")
			}
			size, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			typeName := strings.TrimPrefix(expr.Callee, "__newArray:")
			return vm.constructArrayValue(typeName, size)
		}
		if strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:") {
			if expr.Left == nil {
				return Null, fmt.Errorf("field access requires receiver")
			}
			receiver, err := vm.eval(*expr.Left, result)
			if err != nil {
				return Null, err
			}
			fieldName := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
			// Apex's safe navigation operator treats an empty immediate inline
			// SOQL receiver as null, while ordinary singleton reads still throw.
			if strings.HasPrefix(expr.Callee, "__safe_field:") && expr.Left.Kind == ir.ExprSOQL && receiver.Kind == ValueList && len(receiver.List) == 0 {
				elementType, _ := collectionElementType(receiver.Type)
				receiver = typedNull(elementType)
			}
			receiverType := receiver.Static
			if receiverType == "" {
				receiverType = receiver.Type
			}
			// Native provenance R009-R013/R020-R021 and R024-R027 retain
			// the final declared field type for ordinary and safe reads, including null.
			fieldType := vm.fieldPathTargetType(receiverType, splitFieldPath(fieldName))
			if isSafeNavigationNull(receiver) {
				if fieldType != "" {
					return safeNavigationNullOfType(fieldType), nil
				}
				return receiver, nil
			}
			if strings.HasPrefix(expr.Callee, "__safe_field:") {
				// Apex Developer Guide, Safe Navigation Operator (p46): an
				// empty immediate SOQL query followed by ?.field yields null.
				// Stored lists and ordinary singleton reads keep their rules.
				emptyInlineQuery := expr.Left.Kind == ir.ExprSOQL && receiver.Kind == ValueList && len(receiver.List) == 0
				if emptyInlineQuery {
					if elementType, ok := collectionElementType(receiverType); ok {
						fieldType = vm.fieldPathTargetType(elementType, splitFieldPath(fieldName))
					}
				}
				if receiver.Kind == ValueNull || emptyInlineQuery {
					if fieldType != "" {
						return safeNavigationNullOfType(fieldType), nil
					}
					return safeNavigationNull(), nil
				}
			}
			value, err := vm.lookupPath(receiver, splitFieldPath(fieldName))
			if err == nil && fieldType != "" {
				value.Static = fieldType
				if value.Kind == ValueNull {
					value.Type = fieldType
				}
			}
			return value, err
		}
		var receiver Value
		hasReceiver := expr.Left != nil
		receiverResolved := false
		fieldAddError := ""
		fieldAddErrorReceiver := ""
		callee := expr.Callee
		queryLocatorCallee := ""
		if hasReceiver {
			if receiverName := exprReceiverName(*expr.Left); receiverName != "" {
				member := strings.TrimPrefix(expr.Callee, "__safe_call:")
				staticCallee := receiverName + "." + member
				if canonical, ok := canonicalBuiltinStaticCall(staticCallee); ok {
					hasReceiver = false
					callee = canonical
					if strings.EqualFold(canonical, "Database.getQueryLocator") {
						queryLocatorCallee = staticCallee
					}
				} else if vm.staticReceiverName(receiverName, member) && !vm.hasRuntimeReceiver(receiverName) {
					hasReceiver = false
					callee = receiverName + "." + member
				} else if typeName, fieldName, ok := splitDottedTypeMember(receiverName); ok {
					if value, ok := builtinStaticField(typeName, fieldName); ok {
						receiver = value
						receiverResolved = true
					}
				}
			}
		}
		if hasReceiver && !receiverResolved && strings.EqualFold(callee, "addError") && expr.Left.Kind == ir.ExprCall && expr.Left.Left != nil && strings.HasPrefix(expr.Left.Callee, "__field:") {
			var err error
			receiver, err = vm.eval(*expr.Left.Left, result)
			if err != nil {
				return Null, err
			}
			receiverResolved = true
			fieldAddError = strings.TrimPrefix(expr.Left.Callee, "__field:")
			fieldAddErrorReceiver = exprReceiverName(*expr.Left.Left)
		}
		if hasReceiver && !receiverResolved {
			var err error
			if expr.Left.Kind == ir.ExprSOQL {
				receiver, err = vm.executeSOQLQuery(expr.Left.Value, result)
				if err != nil {
					receiver, err = vm.namedApexReferenceStaticQuery(expr.Left.Value, err)
				}
			} else {
				receiver, err = vm.eval(*expr.Left, result)
			}
			if err != nil {
				return Null, err
			}
		}
		if hasReceiver && expr.Operator == "[]" && receiver.Kind == ValueMap {
			receiverType := receiver.Static
			if receiverType == "" {
				receiverType = receiver.Type
			}
			return Null, fmt.Errorf("Expression must be a list type: %s", receiverType)
		}
		if hasReceiver {
			if strings.HasPrefix(callee, "__safe_call:") {
				if receiver.Kind == ValueNull {
					return vm.safeNavigationNullForExpression(expr), nil
				}
				if receiver.Kind == ValueObject && platformScalarObject(receiver.Type) {
					if raw, ok := receiver.Fields["value"]; ok && raw.Kind == ValueNull {
						return safeNavigationNullOfType(receiver.Type), nil
					}
				}
				callee = strings.TrimPrefix(callee, "__safe_call:")
			} else if isSafeNavigationNull(receiver) {
				return receiver, nil
			}
		}
		if queryLocatorCallee == "" && !hasReceiver {
			if canonical, ok := canonicalBuiltinStaticCall(callee); ok && strings.EqualFold(canonical, "Database.getQueryLocator") {
				queryLocatorCallee = callee
				callee = canonical
			}
		}
		inlineQueryLocator := queryLocatorCallee != "" && vm.shouldUseBuiltinStaticPrecedence(queryLocatorCallee, callee) && len(expr.Args) > 0 && expr.Args[0].Kind == ir.ExprSOQL
		args := make([]Value, 0, len(expr.Args))
		for i, arg := range expr.Args {
			value := String(arg.Value)
			var err error
			if !inlineQueryLocator || i != 0 {
				value, err = vm.eval(arg, result)
			}
			if err != nil {
				return Null, err
			}
			args = append(args, plainNull(value))
		}
		var namedArgs map[string]Value
		if len(expr.NamedArgs) != 0 {
			namedArgs = make(map[string]Value, len(expr.NamedArgs))
		}
		namedArgOrder := make([]string, 0, len(expr.NamedArgs))
		for _, arg := range expr.NamedArgs {
			value, err := vm.eval(arg.Expr, result)
			if err != nil {
				return Null, err
			}
			namedArgs[arg.Name] = plainNull(value)
			namedArgOrder = append(namedArgOrder, arg.Name)
		}
		if hasReceiver {
			receiverName := exprReceiverName(*expr.Left)
			if fieldAddError != "" {
				value, handled, err := vm.callSObjectFieldValueAddError(&receiver, fieldAddError, args)
				if handled || err != nil {
					if err == nil && fieldAddErrorReceiver != "" {
						if err := vm.storeReceiver(fieldAddErrorReceiver, receiver); err != nil {
							return Null, err
						}
					}
					return value, err
				}
				value, err = vm.lookupPath(receiver, splitFieldPath(fieldAddError))
				if err != nil {
					return Null, err
				}
				receiver = value
			}
			if strings.EqualFold(callee, "values") || strings.EqualFold(callee, "valueOf") {
				if value, handled, err := vm.callEnumStaticMember(receiverName, callee, args); handled || err != nil {
					return value, err
				}
			}
			value, handled, err := vm.callValueMember(receiverName, receiver, callee, args, result)
			if handled || err != nil {
				// Bracket assignment is an expression. The shared List.set path
				// performs its mutation, but its explicit-method Void result is
				// not the value of the assignment. Operands were evaluated once.
				if handled && err == nil && expr.Operator == "[]=" && receiver.Kind == ValueList && len(args) == 2 {
					return receiver.List[int(args[0].Int)], nil
				}
				return value, err
			}
			return Null, unsupportedCallError(expr.Callee)
		}
		if queryLocatorCallee != "" {
			callee = queryLocatorCallee
		}
		if strings.HasPrefix(callee, "new:") || strings.HasPrefix(callee, "newlit:") {
			return vm.callConstructorWithNamedArgOrder(callee, args, namedArgs, namedArgOrder, result)
		}
		if err := vm.validateSOQLCallAccess(callee, args); err != nil {
			return Null, err
		}
		if len(namedArgs) == 0 && len(vm.currentMethod.triggerLocalMethods) == 0 {
			if value, handled, err := vm.callDirectPrimitiveMember(callee, args, result); handled || err != nil {
				return value, err
			}
		}
		return vm.call(callee, args, namedArgs, result)
	case ir.ExprSOQL:
		return vm.executeInlineSOQL(expr.Value, result)
	default:
		return Null, fmt.Errorf("unsupported expression %q", expr.Kind)
	}
}

// Local list reads/writes with literal or exact local operands cannot execute
// getters or replace their receiver while evaluating arguments. The List
// dispatcher retains argument values, never the slice, so this bounded buffer
// stays on the stack. Other call shapes keep the general evaluator and dispatch.
func (vm *VM) evalLocalListCall(expr ir.Expr, result *Result) (Value, bool, error) {
	if len(expr.Args) == 0 || len(expr.Args) > 2 || len(expr.NamedArgs) != 0 || len(vm.currentMethod.triggerLocalMethods) != 0 {
		return Null, false, nil
	}
	receiverName, method := "", expr.Callee
	if expr.Left != nil {
		if expr.Left.Kind != ir.ExprVariable {
			return Null, false, nil
		}
		receiverName = expr.Left.Name
	} else {
		var qualified bool
		receiverName, method, qualified = strings.Cut(expr.Callee, ".")
		if !qualified {
			return Null, false, nil
		}
	}
	if strings.EqualFold(method, "get") && len(expr.Args) == 1 {
		method = "get"
	} else if strings.EqualFold(method, "set") && len(expr.Args) == 2 {
		method = "set"
	} else {
		return Null, false, nil
	}
	if strings.Contains(receiverName, ".") || strings.EqualFold(receiverName, "this") || strings.EqualFold(receiverName, "super") {
		return Null, false, nil
	}
	receiver, local := vm.Globals[receiverName]
	declared := vm.VarTypes[receiverName]
	if !local || receiver.Kind != ValueList || collectionBase(declared) != "List" {
		return Null, false, nil
	}
	if expr.Left == nil {
		if _, builtin := canonicalBuiltinStaticCall(expr.Callee); builtin || normalizeStaticCallCasing(expr.Callee) != expr.Callee {
			return Null, false, nil
		}
	}
	for _, arg := range expr.Args {
		switch arg.Kind {
		case ir.ExprLiteral:
		case ir.ExprVariable:
			if _, local := vm.Globals[arg.Name]; !local || strings.Contains(arg.Name, ".") {
				return Null, false, nil
			}
		default:
			return Null, false, nil
		}
	}
	if expr.Left != nil {
		// Bracket/local-expression receivers use lookup's declared view,
		// even when alias publication left another view in the stored Value.
		var err error
		receiver, err = vm.lookup(receiverName)
		if err != nil {
			return Null, true, err
		}
	}
	var buffer [2]Value
	args := buffer[:len(expr.Args)]
	for i, arg := range expr.Args {
		value, err := vm.eval(arg, result)
		if err != nil {
			return Null, true, err
		}
		args[i] = plainNull(value)
	}
	if expr.Left == nil {
		vm.markRootCollectionRefsEscaped(args...)
	}
	if receiver.Static == "" {
		receiver.Static = declared
	}
	value, _, err := vm.callListValueMember(receiverName, receiver, method, args, result)
	if err == nil && expr.Operator == "[]=" {
		return receiver.List[int(args[0].Int)], true, nil
	}
	return value, true, err
}

// Direct String and List bindings share the member dispatcher without
// allocating a slice to split the qualified call name.
func (vm *VM) callDirectPrimitiveMember(callee string, args []Value, result *Result) (Value, bool, error) {
	receiverName, member, qualified := strings.Cut(callee, ".")
	if !qualified || strings.Contains(member, ".") || strings.EqualFold(receiverName, "this") || strings.EqualFold(receiverName, "super") {
		return Null, false, nil
	}
	receiver, local := vm.Globals[receiverName]
	if !local || (receiver.Kind != ValueString && receiver.Kind != ValueList) {
		return Null, false, nil
	}
	if _, builtin := canonicalBuiltinStaticCall(callee); builtin || normalizeStaticCallCasing(callee) != callee {
		return Null, false, nil
	}
	vm.markRootCollectionRefsEscaped(args...)
	return vm.callValueMember(receiverName, receiver, member, args, result)
}

func (vm *VM) staticReceiverName(receiverName, member string) bool {
	if receiverName == "" || member == "" {
		return false
	}
	if !apexIdentifierStartsUpper(receiverName) {
		return false
	}
	if _, ok := vm.lookupClass(receiverName); ok {
		return true
	}
	for _, candidate := range vm.registeredMethodCandidates(receiverName + "." + member) {
		if candidate.IsStatic {
			return true
		}
	}
	return false
}

func (vm *VM) hasRuntimeReceiver(name string) bool {
	if name == "" {
		return false
	}
	if _, ok := vm.Globals[name]; ok {
		return true
	}
	if actual, found := vm.lookupGlobalName(name); found {
		if _, ok := vm.Globals[actual]; ok {
			return true
		}
	}
	if _, ok := vm.VarTypes[name]; ok {
		return true
	}
	if !strings.Contains(name, ".") {
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if _, _, ok := objectFieldValue(this, name); ok {
				return true
			}
			if _, _, ok := vm.lookupReceiverField(this.Type, name); ok {
				return true
			}
		}
		if vm.currentClass != "" {
			if _, _, ok := vm.lookupStaticField(vm.currentClass, name); ok {
				return true
			}
		}
	}
	return false
}

func (vm *VM) evalForType(expr ir.Expr, typeName string, result *Result) (Value, error) {
	if expr.Kind == ir.ExprSOQL && typeName != "" {
		return vm.executeSOQLForType(expr.Value, typeName, result)
	}
	return vm.eval(expr, result)
}

func (vm *VM) evalForAssignment(name string, expr ir.Expr, result *Result) (Value, error) {
	if expr.Kind != ir.ExprSOQL {
		return vm.eval(expr, result)
	}
	if typeName := vm.assignmentTargetType(name); typeName != "" {
		return vm.executeSOQLForType(expr.Value, typeName, result)
	}
	return vm.eval(expr, result)
}

func (vm *VM) evalBinary(op string, left, right Value, result *Result) (Value, error) {
	switch op {
	case "+":
		if value, ok, err := platformDateArithmetic("+", left, right); ok || err != nil {
			return value, err
		}
		if isStringConcatOperand(left) || isStringConcatOperand(right) {
			leftText, err := vm.stubNamespaceExpressionText(left, result)
			if err != nil {
				return Null, err
			}
			leftText = concatStringText(left, leftText)
			rightText, err := vm.stubNamespaceExpressionText(right, result)
			if err != nil {
				return Null, err
			}
			rightText = concatStringText(right, rightText)
			return concatApexStrings(leftText, rightText)
		}
		return evalBinary(op, left, right)
	case "-":
		if value, ok, err := platformDateArithmetic("-", left, right); ok || err != nil {
			return value, err
		}
		return evalBinary(op, left, right)
	case "==", "!=":
		if (isImplicitCurrentPageNull(left) && right.Kind == ValueNull) || (left.Kind == ValueNull && isImplicitCurrentPageNull(right)) {
			return Bool(op == "=="), nil
		}
		// String operators preserve length,
		// while Object operands use the generic equality path below.
		if left.Kind == ValueString && right.Kind == ValueString &&
			canonicalApexScalarType(left.Static) != "Object" && canonicalApexScalarType(right.Static) != "Object" {
			equal := strings.EqualFold(left.Text, right.Text)
			if strings.EqualFold(left.Type, "Id") || strings.EqualFold(right.Type, "Id") {
				equal = apexIDTextEqual(left.Text, right.Text)
			}
			if op == "!=" {
				equal = !equal
			}
			return Bool(equal), nil
		}
		equal, err := vm.apexEquals(left, right, result)
		if err != nil {
			return Null, err
		}
		if op == "!=" {
			equal = !equal
		}
		return Bool(equal), nil
	case "===", "!==":
		equal := valueIdentityEqual(left, right)
		if op == "!==" {
			equal = !equal
		}
		return Bool(equal), nil
	default:
		return evalBinary(op, left, right)
	}
}

func (vm *VM) apexEquals(left, right Value, result *Result) (bool, error) {
	var err error
	left, err = vm.refreshMapKeySet(left)
	if err != nil {
		return false, err
	}
	right, err = vm.refreshMapKeySet(right)
	if err != nil {
		return false, err
	}
	if left.Kind == ValueNull || right.Kind == ValueNull {
		return left.Kind == ValueNull && right.Kind == ValueNull, nil
	}
	// Type tokens compare their private identity before scalar text fallback.
	if left.Kind == ValueObject && right.Kind == ValueObject &&
		strings.EqualFold(left.Type, "Type") && strings.EqualFold(right.Type, "Type") {
		return left.Equal(right), nil
	}
	if equal, ok := dateObjectStringEqual(left, right); ok {
		return equal, nil
	}
	if equal, ok := dateObjectStringEqual(right, left); ok {
		return equal, nil
	}
	if left.Kind == ValueString && right.Kind == ValueObject && strings.EqualFold(right.Type, "String") {
		text, err := platformScalarText(right, "String")
		if err == nil {
			return strings.EqualFold(left.Text, text), nil
		}
	}
	if right.Kind == ValueString && left.Kind == ValueObject && strings.EqualFold(left.Type, "String") {
		text, err := platformScalarText(left, "String")
		if err == nil {
			return strings.EqualFold(text, right.Text), nil
		}
	}
	if equal, handled := vm.resolvedEnumValuesEqual(left, right); handled {
		return equal, nil
	}
	if left.Kind == ValueString && right.Kind == ValueString {
		if shouldCompareTextAsID(left.Text, right.Text) {
			return apexIDTextEqual(left.Text, right.Text), nil
		}
		return strings.EqualFold(left.Text, right.Text), nil
	}
	if left.Kind == ValueList && right.Kind == ValueList {
		if len(left.List) != len(right.List) {
			return false, nil
		}
		for i := range left.List {
			if listElementValuesEqual(left.List[i], right.List[i], make(map[[2]uint64]bool)) {
				continue
			}
			if isNumeric(left.List[i]) && isNumeric(right.List[i]) {
				return false, nil
			}
			equal, err := vm.apexCollectionElementEquals(left.List[i], right.List[i], result)
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	}
	if left.Kind == ValueSet && right.Kind == ValueSet {
		return vm.apexSetsEqual(left, right, result)
	}
	if left.Kind == ValueMap && right.Kind == ValueMap {
		return vm.apexMapsEqual(left, right, result, true)
	}

	if left.Kind != ValueObject || platformScalarObject(left.Type) || left.Type == "Type" {
		return left.Equal(right), nil
	}
	if equal, ok, err := vm.apexPlatformValueEquals(left, right, result); ok || err != nil {
		return equal, err
	}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(left.Type, "equals", []Value{right})
	if ambiguous {
		return false, vm.ambiguousOverloadError(left.Type+".equals", []Value{right})
	}
	if !ok || strings.EqualFold(method.ClassName, "Object") {
		return left.Equal(right), nil
	}
	value, err := vm.callMethodWithReceiver(method, left, []Value{right}, result)
	if err != nil {
		return false, err
	}
	if value.Kind != ValueBool {
		return false, fmt.Errorf("%s.equals returned %s, expected Boolean", left.Type, value.Kind)
	}
	return value.Bool, nil
}

func dateObjectStringEqual(dateValue, textValue Value) (bool, bool) {
	if dateValue.Kind != ValueObject || !strings.EqualFold(dateValue.Type, "Date") || textValue.Kind != ValueString {
		return false, false
	}
	leftDate, leftErr := parsePlatformDate(dateValue)
	rightDate, rightErr := parseDateText(textValue.Text)
	if leftErr != nil || rightErr != nil {
		return false, false
	}
	return leftDate.Year() == rightDate.Year() && leftDate.Month() == rightDate.Month() && leftDate.Day() == rightDate.Day(), true
}

func (vm *VM) apexPlatformValueEquals(left, right Value, result *Result) (bool, bool, error) {
	if left.Kind != ValueObject || right.Kind != ValueObject || !strings.EqualFold(left.Type, right.Type) {
		return false, false, nil
	}
	switch left.Type {
	case "Schema.SObjectField", "Schema.SObjectType":
		return left.Equal(right), true, nil
	case "SelectOption":
		for _, field := range []string{"value", "label", "disabled", "escapeItem"} {
			leftValue, leftOK := left.Fields[field]
			rightValue, rightOK := right.Fields[field]
			if leftOK != rightOK {
				return false, true, nil
			}
			if !leftOK {
				continue
			}
			equal, err := vm.apexEquals(leftValue, rightValue, result)
			if err != nil || !equal {
				return equal, true, err
			}
		}
		return true, true, nil
	default:
		return false, false, nil
	}
}

func (vm *VM) evalIncrementExpression(expr ir.Expr, result *Result) (Value, error) {
	if expr.Left != nil && expr.Left.Kind == ir.ExprCall {
		return vm.evalIndexedIncrementExpression(expr, result)
	}
	if expr.Left == nil || expr.Left.Kind != ir.ExprVariable {
		return Null, fmt.Errorf("%s requires assignable variable target", expr.Callee)
	}
	target := expr.Left.Name
	current, err := vm.eval(*expr.Left, result)
	if err != nil {
		return Null, err
	}
	operator := "+"
	if strings.HasSuffix(expr.Callee, "--") {
		operator = "-"
	}
	next, err := evalBinary(operator, current, Int(1))
	if err != nil {
		return Null, err
	}
	if err := vm.assign(target, next); err != nil {
		return Null, err
	}
	if strings.HasPrefix(expr.Callee, "__postfix:") {
		return current, nil
	}
	return next, nil
}

func (vm *VM) evalIndexedIncrementExpression(expr ir.Expr, result *Result) (Value, error) {
	target := expr.Left
	if target != nil && strings.HasPrefix(target.Callee, "__field:") && target.Left != nil {
		receiver, err := vm.eval(*target.Left, result)
		if err != nil {
			return Null, err
		}
		current, err := vm.eval(*target, result)
		if err != nil {
			return Null, err
		}
		operator := "+"
		if strings.HasSuffix(expr.Callee, "--") {
			operator = "-"
		}
		next, err := evalBinary(operator, current, Int(1))
		if err != nil {
			return Null, err
		}
		field := strings.TrimPrefix(target.Callee, "__field:")
		if err := vm.assignPath(receiver, []string{field}, next); err != nil {
			return Null, err
		}
		if strings.HasPrefix(expr.Callee, "__postfix:") {
			return current, nil
		}
		return next, nil
	}
	if target == nil || target.Left == nil || target.Left.Kind != ir.ExprVariable || len(target.Args) != 1 {
		return Null, fmt.Errorf("%s requires assignable variable target", expr.Callee)
	}
	receiverName := target.Left.Name
	receiver, err := vm.lookup(receiverName)
	if err != nil {
		return Null, err
	}
	index, err := vm.eval(target.Args[0], result)
	if err != nil {
		return Null, err
	}
	if receiver.Kind != ValueList || index.Kind != ValueInt {
		return Null, fmt.Errorf("%s requires List integer index target", expr.Callee)
	}
	i := int(index.Int)
	if i < 0 || i >= len(receiver.List) {
		return Null, listIndexException(i)
	}
	current := receiver.List[i]
	operator := "+"
	if strings.HasSuffix(expr.Callee, "--") {
		operator = "-"
	}
	next, err := evalBinary(operator, current, Int(1))
	if err != nil {
		return Null, err
	}
	vm.setListElement(receiver, i, next)
	if err := vm.storeReceiver(receiverName, receiver); err != nil {
		return Null, err
	}
	if strings.HasPrefix(expr.Callee, "__postfix:") {
		return current, nil
	}
	return next, nil
}

// Map numeric values retain Integer/Decimal type identity and Decimal scale.
// Scalar numeric equality and other boxed numeric kinds keep their own rules.
func mapNumericValueIdentityDiffers(left, right Value) bool {
	leftDecimal := left.Kind == ValueDecimal && !isFloatBackedDecimal(left)
	rightDecimal := right.Kind == ValueDecimal && !isFloatBackedDecimal(right)
	if leftDecimal && rightDecimal {
		return decimalScale(left) != decimalScale(right)
	}
	leftInteger := left.Kind == ValueInt && canonicalApexScalarType(valueTypeName(left)) == "Integer"
	rightInteger := right.Kind == ValueInt && canonicalApexScalarType(valueTypeName(right)) == "Integer"
	return leftDecimal && rightInteger || rightDecimal && leftInteger
}

// Whole-Set comparisons of directly declared custom keys use the same
// equality/hash policy as membership. Preserve raw comparison for primitive,
// mixed-type, changed-hash and other unobserved declaration shapes.
func (vm *VM) apexSetsEqual(left, right Value, result *Result) (bool, error) {
	if left.Equal(right) {
		return true, nil
	}
	if len(left.Set) != len(right.Set) || len(left.Set) == 0 {
		return false, nil
	}
	typeName := runtimeObjectType(left.Set[0])
	for _, set := range []Value{left, right} {
		for index, element := range set.Set {
			method, admitted := vm.equalsUserCollectionDeclaration(element, element)
			if !admitted || !strings.EqualFold(method.ReturnType, "Boolean") ||
				!strings.EqualFold(runtimeObjectType(element), typeName) {
				return false, nil
			}
			stored := set.setInsertionHashAt(index)
			if !stored.tracked {
				return false, nil
			}
		}
	}
	for _, set := range []Value{left, right} {
		for index, element := range set.Set {
			stored := set.setInsertionHashAt(index)
			current, err := vm.captureSetInsertionHash(element)
			if err != nil {
				return false, err
			}
			if !current.tracked || current.key != stored.key {
				return false, nil
			}
		}
	}
	for _, element := range left.Set {
		found, err := vm.setContainsValue(right, element, result)
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}

// Map storage slots disambiguate hash collisions; equality compares the actual
// stored keys. String values keep exact collection equality; the operator
// retains its existing fallback for other value kinds.
func (vm *VM) apexMapsEqual(left, right Value, result *Result, operator bool) (bool, error) {
	if right.Kind != ValueMap || len(left.Map) != len(right.Map) {
		return false, nil
	}
	for raw, leftValue := range left.Map {
		key, err := vm.resolvedMapLookupKey(right, mapStoredKey(left, raw))
		if err != nil {
			return false, err
		}
		rightValue, ok := right.Map[key]
		if !ok {
			return false, nil
		}
		if mapNumericValueIdentityDiffers(leftValue, rightValue) {
			return false, nil
		}
		if leftValue.equal(rightValue, make(map[[2]uint64]bool)) {
			continue
		}
		bothStrings := leftValue.Kind == ValueString && rightValue.Kind == ValueString &&
			strings.EqualFold(valueTypeName(leftValue), "String") && strings.EqualFold(valueTypeName(rightValue), "String")
		if !operator || bothStrings {
			return false, nil
		}
		equal, err := vm.apexEquals(leftValue, rightValue, result)
		if err != nil || !equal {
			return equal, err
		}
	}
	return true, nil
}

// evalFieldDescribeScalarChain admits only a direct local token and two built-in
// calls without arguments. No expression is evaluated before eligibility is
// decided, so every fallback retains the normal evaluation and dispatch order.
func (vm *VM) evalFieldDescribeScalarChain(expr *ir.Expr, result *Result) (Value, bool, error) {
	if expr.Left == nil || expr.Operator != "" || len(expr.Args) != 0 || len(expr.NamedArgs) != 0 {
		return Null, false, nil
	}
	switch expr.Callee {
	case "getName", "getLabel", "getInlineHelpText", "getLocalName", "getCompoundFieldName",
		"isNillable", "isExternalId", "isUnique", "isEncrypted", "isCalculated", "getCalculatedFormula",
		"isAutoNumber", "isCaseSensitive", "isNameField", "isCustom", "getLength", "getByteLength",
		"getPrecision", "getScale", "getDigits", "isHtmlFormatted", "isSortable", "isDefaultedOnCreate",
		"getRelationshipName", "getRelationshipOrder", "isAccessible", "isCreateable", "isUpdateable",
		// Enum getters return the stored (or a freshly built) enum value; an
		// eager describe shares that same stored value with its template.
		"getType", "getSoapType":
	default:
		return Null, false, nil
	}
	inner := expr.Left
	if inner.Kind != ir.ExprCall || inner.Left != nil || inner.Operator != "" ||
		len(inner.Args) != 0 || len(inner.NamedArgs) != 0 || len(vm.currentMethod.triggerLocalMethods) != 0 {
		return Null, false, nil
	}
	name, method, ok := strings.Cut(inner.Callee, ".")
	if !ok || method != "getDescribe" || strings.EqualFold(name, "this") || strings.EqualFold(name, "super") {
		return Null, false, nil
	}
	token, local := vm.Globals[name]
	if !local || !isSObjectFieldTokenType(vm.VarTypes[name]) || token.Kind != ValueObject || token.Type != "Schema.SObjectField" ||
		token.Runtime != "" || (token.Static != "" && token.Static != token.Type) || token.classInstance || token.projectClass || isStubProxy(token) {
		return Null, false, nil
	}
	if _, builtin := canonicalBuiltinStaticCall(inner.Callee); builtin || normalizeStaticCallCasing(inner.Callee) != inner.Callee {
		return Null, false, nil
	}
	// Registered methods precede the passive platform implementation and can
	// invoke Apex callbacks. Their initialization/access/dispatch stays eager.
	// A declaration with no member, supertype or initializer (a source class
	// that only shadows the short name) offers dispatch nothing to select.
	for _, name := range []string{"Schema.SObjectField", "SObjectField", "Schema.DescribeFieldResult", "DescribeFieldResult"} {
		if class, registered := vm.lookupClass(name); registered && !memberlessShadowClass(class) {
			return Null, false, nil
		}
	}
	describe, _, _, _, err := vm.callSObjectFieldDescribe(token, nil, result, true)
	if err != nil {
		return Null, true, err
	}
	value, _, _, _, err := vm.callPlatformObjectMember(describe, expr.Callee, nil, result)
	return value, true, err
}

func memberlessShadowClass(class Class) bool {
	return len(class.Methods) == 0 && len(class.Constructors) == 0 &&
		len(class.Fields) == 0 && len(class.StaticFields) == 0 &&
		len(class.StaticInitializers) == 0 && len(class.InstanceInitializers) == 0 &&
		len(class.EnumValues) == 0 && class.SuperClass == "" && len(class.Interfaces) == 0 &&
		!class.IsInterface && !class.PlatformType
}
