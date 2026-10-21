package vm

import (
	"fmt"
	"strings"
)

// Only these existing local signatures have a passive catalog entry bridge.
// Other Cache methods keep their existing dispatch and unsupported boundaries.
func cacheScopeStaticDeclaration(method Method) bool {
	if !method.IsStatic || !passiveGeneratedMethod(method) {
		return false
	}
	callee, ok := canonicalBuiltinStaticCall(method.ClassName + "." + apexMethodMemberName(method.Name))
	if !ok {
		return false
	}
	switch callee {
	case "Cache.Org.getPartition", "Cache.Session.getPartition":
		return len(method.Params) == 1 && (strings.EqualFold(method.Params[0].Type, "String") || strings.EqualFold(method.Params[0].Type, "System.String"))
	case "Cache.Session.isAvailable":
		return len(method.Params) == 0
	}
	return false
}

func (vm *VM) callCacheScopeStaticDefault(callee string, args []Value) (Value, error) {
	callee = normalizeStaticCallCasing(callee)
	switch callee {
	case "Cache.Org.getPartition", "Cache.Session.getPartition":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s expects String partition name", callee)
		}
		return vm.cachePartition(callee, args[0])
	case "Cache.Session.isAvailable":
		if len(args) != 0 {
			return Null, fmt.Errorf("%s expects 0 arguments", callee)
		}
		if err := vm.cacheRequireCapturedDefault(callee); err != nil {
			return Null, err
		}
		return Bool(vm.cacheCapturedSessionAvailable("Cache.Session", vm.cacheDefaultName())), nil
	}
	return Null, unsupportedCallError(callee)
}
