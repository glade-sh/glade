package vm

import "strings"

// Short aliases of unrelated inner declarations are not visible platform-root
// receivers. Qualified names and lexical inner declarations keep their identity.
func (vm *VM) cacheUnrelatedPlatformRootAlias(name string, class Class) bool {
	if !vm.cacheCapturedAPI() || vm.currentExecutionNamespace() != "" || strings.Contains(name, ".") ||
		class.PlatformType || class.Dependency || class.Namespace != "" || !strings.Contains(class.Name, ".") {
		return false
	}
	switch strings.ToLower(name) {
	case "request", "limits", "cache", "platformcache":
	default:
		return false
	}
	if vm.currentClass != "" {
		if resolved, visible := vm.resolveNestedTypeInClassHierarchy(vm.currentClass, name); visible && strings.EqualFold(resolved, class.Name) {
			return false
		}
	}
	return true
}

// An empty inner declaration cannot turn a platform namespace into a source
// receiver in code outside that declaration. Value bindings still win.
func (vm *VM) cacheInnerNamespacePrecedence(original, canonical string) bool {
	if !vm.cacheCapturedAPI() || vm.currentExecutionNamespace() != "" || vm.calleeStartsWithRuntimeReceiver(original, nil) {
		return false
	}
	root, _, _ := strings.Cut(original, ".")
	if !strings.EqualFold(root, "Cache") && !strings.EqualFold(root, "PlatformCache") {
		return false
	}
	if !strings.HasPrefix(canonical, "Cache.Org.") && !strings.HasPrefix(canonical, "Cache.Session.") && !strings.HasPrefix(canonical, "PlatformCache.") {
		return false
	}
	class, found := vm.lookupClass(root)
	if !found || class.PlatformType || class.Dependency || class.Namespace != "" || !strings.Contains(class.Name, ".") ||
		strings.EqualFold(vm.currentClass, class.Name) || class.SuperClass != "" || len(class.Interfaces) != 0 ||
		len(class.Fields) != 0 || len(class.StaticFields) != 0 || len(class.Methods) != 0 ||
		len(class.StaticInitializers) != 0 || len(class.InstanceInitializers) != 0 {
		return false
	}
	return true
}
