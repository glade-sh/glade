package vm

import (
	"fmt"
	"strings"
)

// Captured T05_VALUE returns actual null from an ordinary declared override.
// Keep this value-returning route separate from concatenation/format display,
// whose null-override behavior has not been captured.
func (vm *VM) stringValueOfOverride(value Value, result *Result) (Value, bool, error) {
	if text, ok := originalIDDisplayText(value); ok {
		return String(text), true, nil
	}
	if value.Kind != ValueObject || isStubProxy(value) || vm.isSubclass(value.Type, "Exception") {
		return Null, false, nil
	}
	class, ok := vm.lookupClass(runtimeObjectType(value))
	if !ok || class.PlatformType || class.Dependency || class.IsInterface || len(class.EnumValues) != 0 {
		return Null, false, nil
	}
	if vm.isSObjectLikeType(value.Type) && !vm.userClassShadowsSObjectType(value.Type) {
		return Null, false, nil
	}
	method, found, ambiguous := vm.resolveInstanceMethodForArgs(value.Type, "toString", nil)
	if !found || ambiguous || method.Dependency || method.IsStatic || len(method.Params) != 0 || !strings.EqualFold(method.ReturnType, "String") {
		return Null, false, nil
	}
	owner, exists := vm.lookupClass(method.ClassName)
	if !exists || owner.PlatformType || owner.Dependency {
		return Null, false, nil
	}
	out, err := vm.callMethodWithReceiver(method, value, nil, result)
	if err != nil {
		return Null, true, err
	}
	if out.Kind == ValueNull {
		return Value{Kind: ValueNull, Type: "String"}, true, nil
	}
	if out.Kind != ValueString {
		return Null, true, fmt.Errorf("%s returned %s, want String", method.Name, out.Kind)
	}
	return String(vm.resolveLabelMergeExpressions(out.Text)), true, nil
}
