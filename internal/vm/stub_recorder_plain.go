package vm

import "strings"

// Plain named execution captures the same fieldless target and recorder state.
// Other provider shapes and calls from a namespace retain their existing text.
func (vm *VM) stubRecorderPlainProxyName(value Value) (string, bool) {
	typeName, proxy := stubProxyTypeName(value)
	if !proxy || vm.Org == nil || vm.Org.Namespace != "" || vm.currentCallerNamespace() != "" {
		return "", false
	}
	for name := range value.Fields {
		if !strings.HasPrefix(name, "__glade") {
			return "", false
		}
	}
	class, ok := vm.lookupClass(typeName)
	if !ok || class.Namespace != "" || classIsInner(class) || class.Dependency || class.PlatformType || class.IsInterface || class.SuperClass != "" {
		return "", false
	}
	provider := vm.currentStubProvider(value)
	if provider.Kind != ValueObject {
		return "", false
	}
	providerClass, ok := vm.lookupClass(provider.Type)
	if !ok || providerClass.Namespace != "" || classIsInner(providerClass) || providerClass.Dependency || providerClass.PlatformType || !vm.stubNamespaceRecorderProvider(provider, providerClass) {
		return "", false
	}
	return class.Name, true
}
