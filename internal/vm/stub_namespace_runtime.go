package vm

import "strings"

// Explicit valueOf/toString conversion of a fieldless namespaced top-level stub
// with a captured same-namespace provider displays its local class name there.
// Keep its qualified runtime type for casts and reflection.
// Named inner stubs remain rejected.
// External-namespace stubs and uncaptured provider shapes retain their text.
func (vm *VM) stubNamespaceFieldlessProxyName(value Value) (string, bool) {
	typeName, proxy := stubProxyTypeName(value)
	if !proxy {
		return "", false
	}
	for name := range value.Fields {
		if !strings.HasPrefix(name, "__glade") {
			return "", false
		}
	}
	class, ok := vm.lookupClass(typeName)
	if !ok || class.Namespace == "" || !strings.EqualFold(class.Namespace, vm.currentCallerNamespace()) || classIsInner(class) {
		return "", false
	}
	// Captures cover an empty provider and a field-bearing recorder provider.
	provider := vm.currentStubProvider(value)
	if provider.Kind != ValueObject {
		return "", false
	}
	providerClass, providerOK := vm.lookupClass(provider.Type)
	if !providerOK || !strings.EqualFold(class.Namespace, providerClass.Namespace) || classIsInner(providerClass) || providerClass.SuperClass != "" {
		return "", false
	}
	if len(provider.Fields) != 0 || len(providerClass.Fields) != 0 {
		if vm.Org == nil || !strings.EqualFold(vm.Org.Namespace, class.Namespace) || class.Dependency || class.PlatformType || class.IsInterface || class.SuperClass != "" || providerClass.Dependency || providerClass.PlatformType || !vm.stubNamespaceRecorderProvider(provider, providerClass) {
			return "", false
		}
	}
	return stubNamespaceLocalClassName(class), true
}

func stubNamespaceLocalClassName(class Class) string {
	name := class.Name
	namespace := strings.TrimSpace(class.Namespace)
	if namespace != "" && hasPrefixFold(name, namespace+".") {
		name = name[len(namespace)+1:]
	}
	return name
}

// The provider display head is local in the captured namespace. Nested Type
// values retain their qualified spelling; no text-wide replacement is applied.
func (vm *VM) stubNamespaceProviderText(proxy, provider Value, seen map[uint64]bool) string {
	_, namespaced := vm.stubNamespaceFieldlessProxyName(proxy)
	_, plain := vm.stubRecorderPlainProxyName(proxy)
	if !namespaced && !plain {
		return vm.nestedDisplayString(provider, seen)
	}
	providerClass, ok := vm.lookupClass(provider.Type)
	if !ok {
		return vm.nestedDisplayString(provider, seen)
	}
	var text string
	if vm.stubNamespaceRecorderProvider(provider, providerClass) {
		text = vm.stubRecorderProviderDisplayString(provider, seen)
	} else {
		text = vm.nestedDisplayString(provider, seen)
	}
	if namespaced && strings.HasPrefix(text, provider.Type+":") {
		return stubNamespaceLocalClassName(providerClass) + strings.TrimPrefix(text, provider.Type)
	}
	return text
}

// Direct Object.toString calls opt into captured namespaced stub and empty
// ordinary-class text. Captured plain recorder proxies use their live state.
func (vm *VM) stubNamespaceObjectText(receiver Value, method string, args []Value) (string, bool) {
	if receiver.Kind != ValueObject || canonicalObjectMemberMethod(method) != "toString" || len(args) != 0 {
		return "", false
	}
	if _, captured := vm.stubNamespaceFieldlessProxyName(receiver); captured {
		return vm.stubProxyDisplayStringForOperation(receiver, make(map[uint64]bool), true), true
	}
	if _, captured := vm.stubRecorderPlainProxyName(receiver); captured {
		return vm.stubProxyDisplayStringForOperation(receiver, make(map[uint64]bool), true), true
	}
	class, ok := vm.lookupClass(runtimeObjectType(receiver))
	if !ok || class.Namespace == "" || !strings.EqualFold(class.Namespace, vm.currentCallerNamespace()) || class.PlatformType || class.Dependency || vm.isSubclass(receiver.Type, "Exception") || len(class.EnumValues) != 0 || len(receiver.Fields) != 0 || isStubProxy(receiver) {
		return "", false
	}
	return vm.defaultObjectDisplayString(receiver), true
}

// JSON.serialize of the captured fieldless proxies exposes
// only stubProvider, whose captured empty state serializes as {}. Fieldful
// proxies/providers retain the existing serializer pending native coverage.
func (vm *VM) stubNamespaceJSON(value Value) (any, bool) {
	if _, captured := vm.stubNamespaceFieldlessProxyName(value); !captured {
		return nil, false
	}
	provider := vm.currentStubProvider(value)
	if provider.Kind != ValueObject || len(provider.Fields) != 0 {
		return nil, false
	}
	stubClass, stubOK := vm.lookupClass(value.Type)
	providerClass, providerOK := vm.lookupClass(provider.Type)
	if !stubOK || !providerOK || classIsInner(providerClass) || !strings.EqualFold(stubClass.Namespace, providerClass.Namespace) {
		return nil, false
	}
	return orderedJSONObject{{name: "stubProvider", value: orderedJSONObject{}}}, true
}

// String.valueOf is a distinct operation from implicit concatenation, debug,
// assertion display and formatting an object argument. Only this explicit API
// and direct Object.toString calls use the empty-provider local proxy spelling.
// The captured stateful provider also has local concatenation and format text.
func (vm *VM) stubNamespaceValueOfText(value Value) (string, bool) {
	if _, captured := vm.stubNamespaceFieldlessProxyName(value); captured {
		return vm.stubProxyDisplayStringForOperation(value, make(map[uint64]bool), true), true
	}
	return vm.stubNamespaceRecordedContainerText(value, make(map[uint64]bool))
}
