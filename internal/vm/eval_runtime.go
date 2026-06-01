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
			return vm.evalInstanceOf(left, expr.Right.Name), nil
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
			return vm.coerceCast(typeName, value)
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
			if conditionValue {
				return vm.eval(expr.Args[1], result)
			}
			return vm.eval(expr.Args[2], result)
		}
		if expr.Callee == "__coalesce" {
			if len(expr.Args) != 2 {
				return Null, fmt.Errorf("null coalescing expression requires 2 operands")
			}
			left, err := vm.eval(expr.Args[0], result)
			if err != nil {
				return Null, err
			}
			if left.Kind != ValueNull {
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
			if isSafeNavigationNull(receiver) {
				fieldName := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
				if fieldType := vm.fieldPathTargetType(receiver.Type, splitFieldPath(fieldName)); fieldType != "" {
					return safeNavigationNullOfType(fieldType), nil
				}
				return receiver, nil
			}
			if strings.HasPrefix(expr.Callee, "__safe_field:") {
				if receiver.Kind == ValueNull {
					fieldName := strings.TrimPrefix(expr.Callee, "__safe_field:")
					if fieldType := vm.fieldPathTargetType(receiver.Type, splitFieldPath(fieldName)); fieldType != "" {
						return safeNavigationNullOfType(fieldType), nil
					}
					return safeNavigationNull(), nil
				}
				return vm.lookupPath(receiver, []string{strings.TrimPrefix(expr.Callee, "__safe_field:")})
			}
			return vm.lookupPath(receiver, []string{strings.TrimPrefix(expr.Callee, "__field:")})
		}
		var receiver Value
		hasReceiver := expr.Left != nil
		receiverResolved := false
		callee := expr.Callee
		if hasReceiver {
			if receiverName := exprReceiverName(*expr.Left); receiverName != "" {
				member := strings.TrimPrefix(expr.Callee, "__safe_call:")
				if canonical, ok := canonicalBuiltinStaticCall(receiverName + "." + member); ok {
					hasReceiver = false
					callee = canonical
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
		if hasReceiver && !receiverResolved {
			var err error
			if expr.Left.Kind == ir.ExprSOQL {
				receiver, err = vm.executeSOQL(expr.Left.Value, result)
			} else {
				receiver, err = vm.eval(*expr.Left, result)
			}
			if err != nil {
				return Null, err
			}
		}
		if hasReceiver {
			if strings.HasPrefix(callee, "__safe_call:") {
				if receiver.Kind == ValueNull {
					return safeNavigationNull(), nil
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
		if !hasReceiver && strings.EqualFold(callee, "Database.getQueryLocator") && len(expr.Args) == 1 && expr.Args[0].Kind == ir.ExprSOQL {
			return vm.queryLocatorFromSOQL(expr.Args[0].Value, result)
		}
		args := make([]Value, 0, len(expr.Args))
		for _, arg := range expr.Args {
			value, err := vm.eval(arg, result)
			if err != nil {
				return Null, err
			}
			args = append(args, plainNull(value))
		}
		namedArgs := make(map[string]Value, len(expr.NamedArgs))
		for _, arg := range expr.NamedArgs {
			value, err := vm.eval(arg.Expr, result)
			if err != nil {
				return Null, err
			}
			namedArgs[arg.Name] = plainNull(value)
		}
		if hasReceiver {
			receiverName := exprReceiverName(*expr.Left)
			if strings.EqualFold(callee, "values") || strings.EqualFold(callee, "valueOf") {
				if value, handled, err := vm.callEnumStaticMember(receiverName, callee, args); handled || err != nil {
					return value, err
				}
			}
			value, handled, err := vm.callValueMember(receiverName, receiver, callee, args, result)
			if handled || err != nil {
				return value, err
			}
			return Null, unsupportedCallError(expr.Callee)
		}
		return vm.call(callee, args, namedArgs, result)
	case ir.ExprSOQL:
		return vm.executeInlineSOQL(expr.Value, result)
	default:
		return Null, fmt.Errorf("unsupported expression %q", expr.Kind)
	}
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
			leftText, err := vm.displayString(left, result)
			if err != nil {
				return Null, err
			}
			if left.Kind == ValueDecimal {
				leftText = decimalDisplayText(left)
			}
			rightText, err := vm.displayString(right, result)
			if err != nil {
				return Null, err
			}
			if right.Kind == ValueDecimal {
				rightText = decimalDisplayText(right)
			}
			return String(leftText + rightText), nil
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
	if left.Kind == ValueNull || right.Kind == ValueNull {
		return left.Kind == ValueNull && right.Kind == ValueNull, nil
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
			equal, err := vm.apexEquals(left.List[i], right.List[i], result)
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
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
	receiver.List[i] = next
	if err := vm.storeReceiver(receiverName, receiver); err != nil {
		return Null, err
	}
	if strings.HasPrefix(expr.Callee, "__postfix:") {
		return current, nil
	}
	return next, nil
}
