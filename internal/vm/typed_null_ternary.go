package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

// ExpressionTypeResolver supplies checked expression types without executing
// expressions. Project runtimes bind it to their semantic source generation.
type ExpressionTypeResolver func(ir.Expr, map[string]string, string) string

func (vm *VM) SetExpressionTypeResolver(resolve ExpressionTypeResolver) {
	vm.expressionTypeResolver = resolve
}

// Additional registrations (such as anonymous helper classes) can have their
// own source index without replacing the project's existing type resolver.
func (vm *VM) AddExpressionTypeResolver(resolve ExpressionTypeResolver) {
	if resolve == nil {
		return
	}
	previous := vm.expressionTypeResolver
	if previous == nil {
		vm.expressionTypeResolver = resolve
		return
	}
	vm.expressionTypeResolver = func(expr ir.Expr, variables map[string]string, owner string) string {
		if typeName := resolve(expr, variables, owner); typeName != "" {
			return typeName
		}
		return previous(expr, variables, owner)
	}
}

// The captured rows: a selected null literal has the
// declared type of the other branch. Resolve declarations only; executing the
// unselected branch would change Apex's lazy execution contract. Bare null/null
// branches and already typed values keep their existing views.
func (vm *VM) typedNullTernaryResult(expression, selected, other ir.Expr, value Value) Value {
	if selected.Kind != ir.ExprLiteral || !strings.EqualFold(selected.Value, "null") ||
		value.Kind != ValueNull || value.Type != "" || value.Static != "" {
		return value
	}
	typeName := vm.resolvedExpressionType(expression)
	if typeName == "" {
		typeName = vm.declaredExpressionType(other)
	}
	if typeName != "" && !strings.EqualFold(typeName, "null") && !strings.EqualFold(typeName, "void") {
		value.Type, value.Static = typeName, typeName
	}
	return value
}

func (vm *VM) resolvedExpressionType(expr ir.Expr) string {
	if vm.expressionTypeResolver != nil {
		if typeName := vm.expressionTypeResolver(expr, vm.VarTypes, vm.currentClass); typeName != "" {
			return typeName
		}
	}
	return vm.declaredExpressionType(expr)
}

// Safe navigation changes the value, not the expression's declared type.
// Resolve the skipped method from declarations without executing its arguments.
func (vm *VM) safeNavigationNullForExpression(expr ir.Expr) Value {
	value := safeNavigationNull()
	if typeName := vm.resolvedExpressionType(expr); typeName != "" && !strings.EqualFold(typeName, "void") && !strings.EqualFold(typeName, "null") {
		value.Type, value.Static = typeName, typeName
	}
	return value
}

// declaredExpressionType resolves the static view needed by a null branch.
// Unknown expressions stay untyped rather than running user code to infer a
// type. Calls use the same declaration/overload registries as normal dispatch.
func (vm *VM) declaredExpressionType(expr ir.Expr) string {
	switch expr.Kind {
	case ir.ExprLiteral:
		value, err := parseLiteral(expr.Value)
		if err == nil {
			return valueTypeName(value)
		}
	case ir.ExprVariable:
		// A static field's nested types belong to its declaring owner, which
		// can differ from the current caller's type-resolution context.
		if owner, member, ok := vm.splitClassMember(expr.Name); ok && !vm.calleeStartsWithRuntimeReceiver(expr.Name, nil) {
			preferDependency := vm.classMemberReferenceUsesExplicitNamespace(expr.Name, owner)
			if field, declaring, ok := vm.lookupStaticFieldForReceiver(owner, member, preferDependency); ok {
				return vm.resolveTypeNameInClass(declaring, field.Type)
			}
		}
		if typeName := vm.declaredReceiverType(expr.Name); typeName != "" {
			return vm.resolveTypeNameInCurrentExecutionContext(typeName)
		}
		if root, path, ok := strings.Cut(expr.Name, "."); ok {
			return vm.fieldPathTargetType(vm.declaredReceiverType(root), strings.Split(path, "."))
		}
	case ir.ExprCall:
		expr.Callee = strings.TrimPrefix(expr.Callee, "__safe_call:")
		for _, prefix := range []string{"new:", "newlit:", "__newArray:"} {
			if strings.HasPrefix(expr.Callee, prefix) {
				return vm.resolveTypeNameInCurrentExecutionContext(strings.TrimPrefix(expr.Callee, prefix))
			}
		}
		if strings.HasPrefix(expr.Callee, "__cast:") {
			return vm.resolveTypeNameInCurrentExecutionContext(strings.TrimPrefix(expr.Callee, "__cast:"))
		}
		if expr.Callee == "__ternary" && len(expr.Args) == 3 {
			left := vm.declaredExpressionType(expr.Args[1])
			right := vm.declaredExpressionType(expr.Args[2])
			if strings.EqualFold(left, "null") {
				return right
			}
			if strings.EqualFold(right, "null") || strings.EqualFold(left, right) {
				return left
			}
			return ""
		}
		if (strings.HasPrefix(expr.Callee, "__field:") || strings.HasPrefix(expr.Callee, "__safe_field:")) && expr.Left != nil {
			receiverType := vm.declaredExpressionType(*expr.Left)
			field := strings.TrimPrefix(strings.TrimPrefix(expr.Callee, "__safe_field:"), "__field:")
			return vm.fieldPathTargetType(receiverType, []string{field})
		}
		args := make([]Value, len(expr.Args))
		for i, arg := range expr.Args {
			typeName := vm.declaredExpressionType(arg)
			if typeName == "" {
				return ""
			}
			args[i] = Null
			if !strings.EqualFold(typeName, "null") {
				args[i].Type, args[i].Static = typeName, typeName
			}
		}
		callee := expr.Callee
		if expr.Left != nil {
			callee = exprReceiverName(*expr.Left) + "." + callee
		}
		if canonical, known := canonicalBuiltinStaticCall(callee); known && vm.shouldUseBuiltinStaticPrecedence(callee, canonical) {
			owner, method, _ := strings.Cut(canonical, ".")
			if target, ok := vm.generatedPlatformMethodForArgs(owner, method, args, true); ok {
				return vm.resolveTypeNameInClass(target.ClassName, target.ReturnType)
			}
			return ""
		}
		var target Method
		var found, ambiguous bool
		if expr.Left != nil {
			if receiverType := vm.declaredExpressionType(*expr.Left); receiverType != "" {
				target, found, ambiguous = vm.resolveInstanceMethodForArgs(receiverType, expr.Callee, args)
				if !found && !ambiguous {
					platformType := receiverType
					if class, known := vm.lookupClass(receiverType); known && !class.PlatformType {
						return ""
					}
					if vm.isSObjectLikeType(receiverType) {
						platformType = "SObject"
					}
					target, found = vm.generatedPlatformMethodForArgs(platformType, expr.Callee, args, false)
				}
			} else {
				target, found, ambiguous = vm.resolveStaticMethodForArgs(exprReceiverName(*expr.Left), expr.Callee, args)
			}
		} else if dot := strings.LastIndexByte(callee, '.'); dot >= 0 {
			receiver, method := callee[:dot], callee[dot+1:]
			if receiverType := vm.declaredExpressionType(ir.Expr{Kind: ir.ExprVariable, Name: receiver}); receiverType != "" {
				target, found, ambiguous = vm.resolveInstanceMethodForArgs(receiverType, method, args)
			} else if owner, method, ok := vm.splitClassMember(callee); ok {
				target, found, ambiguous = vm.resolveStaticMethodForArgs(owner, method, args)
			}
		} else {
			target, found, ambiguous = vm.resolveInstanceMethodForArgs(vm.currentClass, callee, args)
			if !found && !ambiguous {
				target, found, ambiguous = vm.resolveStaticMethodForArgs(vm.currentClass, callee, args)
			}
		}
		if found && !ambiguous {
			return vm.resolveTypeNameInClass(vm.methodTypeResolutionClass(target), target.ReturnType)
		}
	}
	return ""
}
