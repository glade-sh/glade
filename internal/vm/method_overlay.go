package vm

import "strings"

// methodTableOverlay holds methods registered on a clone whose Methods,
// MethodOverloads and MethodFolded maps are still shared with the source VM.
// A key present in the overlay replaces the shared entry for that key; the
// overlay slices already contain the shared entries followed by the new
// registrations, in registration order. The overlay only grows: unregister and
// trigger registration materialize private maps through
// ensureRuntimeArtifactsOwned first.
type methodTableOverlay struct {
	methods   map[string]Method
	overloads map[string][]Method
	folded    map[string][]Method
}

func (o *methodTableOverlay) clone() *methodTableOverlay {
	if o == nil {
		return nil
	}
	// Overlay slices are never appended in place, so sharing them is safe.
	out := &methodTableOverlay{
		methods:   make(map[string]Method, len(o.methods)),
		overloads: make(map[string][]Method, len(o.overloads)),
		folded:    make(map[string][]Method, len(o.folded)),
	}
	for name, method := range o.methods {
		out.methods[name] = method
	}
	for name, methods := range o.overloads {
		out.overloads[name] = methods
	}
	for name, methods := range o.folded {
		out.folded[name] = methods
	}
	return out
}

// registerSharedMethod is RegisterMethod for a VM whose method maps are shared.
// It records the registration in the overlay instead of copying every shared
// map, and leaves the same effective tables as the copy-then-append path.
func (vm *VM) registerSharedMethod(method Method) {
	if vm.methodOverlay == nil {
		vm.methodOverlay = &methodTableOverlay{
			methods:   make(map[string]Method),
			overloads: make(map[string][]Method),
			folded:    make(map[string][]Method),
		}
	}
	overlay := vm.methodOverlay
	overlay.methods[method.Name] = method
	overlay.overloads[method.Name] = appendMethodCopy(vm.registeredOverloads(method.Name), method)
	foldedName := strings.ToLower(method.Name)
	overlay.folded[foldedName] = appendMethodCopy(vm.registeredFolded(foldedName), method)
}

func appendMethodCopy(methods []Method, method Method) []Method {
	out := make([]Method, 0, len(methods)+1)
	out = append(out, methods...)
	return append(out, method)
}

// applyTo writes the overlay into private method maps.
func (o *methodTableOverlay) applyTo(methods map[string]Method, overloads, folded map[string][]Method) {
	if o == nil {
		return
	}
	for name, method := range o.methods {
		methods[name] = method
	}
	for name, list := range o.overloads {
		overloads[name] = append([]Method(nil), list...)
	}
	for name, list := range o.folded {
		folded[name] = append([]Method(nil), list...)
	}
}

// registeredMethod reads vm.Methods through the clone overlay.
func (vm *VM) registeredMethod(name string) (Method, bool) {
	if vm.methodOverlay != nil {
		if method, ok := vm.methodOverlay.methods[name]; ok {
			return method, true
		}
	}
	method, ok := vm.Methods[name]
	return method, ok
}

// registeredOverloads reads vm.MethodOverloads through the clone overlay.
func (vm *VM) registeredOverloads(name string) []Method {
	if vm.methodOverlay != nil {
		if methods, ok := vm.methodOverlay.overloads[name]; ok {
			return methods
		}
	}
	return vm.MethodOverloads[name]
}

// registeredFolded reads vm.MethodFolded through the clone overlay. The name
// must already be lower case.
func (vm *VM) registeredFolded(foldedName string) []Method {
	if vm.methodOverlay != nil {
		if methods, ok := vm.methodOverlay.folded[foldedName]; ok {
			return methods
		}
	}
	return vm.MethodFolded[foldedName]
}
