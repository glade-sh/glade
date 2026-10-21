package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Native captures cover direct public Integer hashCode() declarations and
// concrete singletons, including executing source helpers at API 52 and 61
// below a modern caller. Keep the SObject, identity and other collection paths.
func (vm *VM) systemCustomHashCode(value Value, result *Result) (Value, error) {
	version := vm.currentMethod.APIVersion
	if version == "" && vm.Org != nil {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 52) || apexversion.AtLeast(version, 69) {
		return Int(int64(valueHashCode(value))), nil
	}
	if method, _, ok := vm.systemDirectSourceHashMethod(value); ok {
		hash, err := vm.callMethodWithReceiver(method, value, nil, result)
		if err != nil {
			return Null, err
		}
		if hash.Kind == ValueInt {
			return hash, nil
		}
		return Int(int64(valueHashCode(value))), nil
	}
	if value.Kind == ValueList && len(value.List) == 1 {
		method, owner, custom := vm.systemDirectSourceHashMethod(value.List[0])
		elementType, typed := collectionElementType(value.Type)
		elementClass, declared := vm.lookupClass(elementType)
		if custom && typed && declared && strings.EqualFold(runtimeClassName(elementClass), runtimeClassName(owner)) {
			hash, err := vm.callMethodWithReceiver(method, value.List[0], nil, result)
			if err != nil {
				return Null, err
			}
			if hash.Kind == ValueInt {
				return Int(int64(int32(31) + int32(hash.Int))), nil
			}
		}
	}
	return Int(int64(valueHashCode(value))), nil
}

func (vm *VM) systemDirectSourceHashMethod(value Value) (Method, Class, bool) {
	if value.Kind != ValueObject || isStubProxy(value) ||
		(sObjectValueType(runtimeObjectType(value)) && !vm.userClassShadowsSObjectType(runtimeObjectType(value))) {
		return Method{}, Class{}, false
	}
	owner, found := vm.lookupClass(runtimeObjectType(value))
	if !found || owner.PlatformType || owner.Dependency || owner.IsInterface || len(owner.EnumValues) != 0 {
		return Method{}, Class{}, false
	}
	method, found, ambiguous := vm.resolveInstanceMethodForArgs(runtimeObjectType(value), "hashCode", nil)
	if !found || ambiguous || method.Dependency || method.IsStatic || len(method.Params) != 0 ||
		!strings.EqualFold(method.Access, "public") || !strings.EqualFold(canonicalApexScalarType(method.ReturnType), "Integer") {
		return Method{}, Class{}, false
	}
	declaring, found := vm.lookupClass(method.ClassName)
	if !found || !strings.EqualFold(runtimeClassName(declaring), runtimeClassName(owner)) {
		return Method{}, Class{}, false
	}
	return method, owner, true
}
