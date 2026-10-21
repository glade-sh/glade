package vm

import "strings"

// Recording-mode checks must not materialize missing members on captured
// ordinary providers. Their state comes solely from declared source fields.
func (vm *VM) stubRecorderProviderHasNoModeState(provider Value) bool {
	if provider.Kind != ValueObject || vm.Org == nil {
		return false
	}
	class, ok := vm.lookupClass(provider.Type)
	if !ok || class.PlatformType || class.Dependency || classIsInner(class) || class.SuperClass != "" || !strings.EqualFold(class.Namespace, vm.Org.Namespace) || !strings.EqualFold(class.Namespace, vm.currentCallerNamespace()) {
		return false
	}
	if len(class.Fields) == 0 && len(provider.Fields) == 0 && len(class.Methods) == 1 {
		return true
	}
	return vm.stubNamespaceRecorderProvider(provider, class)
}
