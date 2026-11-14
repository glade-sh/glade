package vm

import (
	"strings"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/typesys"
)

// Gated platform classes. The standard platform view leaves out the ConnectApi
// and Slack stub types until the process names those namespaces (typesys
// platform_gate.go), so a VM registered from that view holds none of their
// classes. Before the gate, such a VM held every stub class. To keep the same
// answers, a VM that registered platform classes binds a left-out stub class
// from the source below the first time a lookup needs it: on a miss for a
// qualified gated name, before a simple-name search over the class table, and
// before a namespace table of a gated namespace is built. The bound class goes
// through RegisterClass into this VM only; a frozen shared prefix is never
// written.

// GatedPlatformClassSource builds the passive class a VM held for a gated stub
// type before the gate, by its runtime name, ignoring case.
type GatedPlatformClassSource func(runtimeName string) (Class, bool)

var gatedPlatformClassSource atomic.Pointer[GatedPlatformClassSource]

// SetGatedPlatformClassSource installs the process-wide source of gated stub
// classes. The runtime that registers passive platform classes installs it.
func SetGatedPlatformClassSource(source GatedPlatformClassSource) {
	if source == nil {
		gatedPlatformClassSource.Store(nil)
		return
	}
	gatedPlatformClassSource.Store(&source)
}

// notePlatformClassRegistration marks a VM that holds passive platform
// classes; only such a VM held the gated stubs before the gate.
func (vm *VM) notePlatformClassRegistration(class Class) {
	if class.PlatformType {
		vm.gatedPlatformLateBind = true
	}
}

// gatedPlatformClassMiss is lookupClassRaw's miss path: it binds a left-out
// stub class for a qualified gated name and looks the name up again.
func (vm *VM) gatedPlatformClassMiss(typeName string) (Class, bool) {
	if !vm.gatedPlatformLateBind || vm.bindingGatedPlatformClass || !typesys.IsGatedPlatformTypeName(typeName) {
		return Class{}, false
	}
	if !vm.bindGatedPlatformClass(typeName) {
		return Class{}, false
	}
	vm.bindingGatedPlatformClass = true
	defer func() { vm.bindingGatedPlatformClass = false }()
	return vm.lookupClassRaw(typeName)
}

// bindGatedPlatformClass registers the stub class of a gated runtime name and
// adds its namespace's stubs to the view (not the captured ConnectApi shape),
// so every index derived from the view carries the class as before the gate.
func (vm *VM) bindGatedPlatformClass(runtimeName string) bool {
	source := gatedPlatformClassSource.Load()
	if source == nil {
		return false
	}
	class, ok := (*source)(runtimeName)
	if !ok {
		return false
	}
	vm.bindingGatedPlatformClass = true
	defer func() { vm.bindingGatedPlatformClass = false }()
	typesys.IncludeGatedPlatformNamespaceFor(runtimeName)
	// Registration drops the enum suffix memo so the next suffix search
	// rebuilds it. A VM whose memo is still the empty one it was created with
	// never rebuilt it before the gate either (bulk registration keeps it), so
	// its suffix searches found no stub enum; keep that memo as it was.
	suffixMemo, suffixBuilt := vm.enumSuffixLookup, vm.enumSuffixBuilt
	err := vm.RegisterClass(class)
	if suffixMemo != nil && !suffixBuilt {
		vm.enumSuffixLookup = suffixMemo
	}
	// A bind can run inside a lookup whose caller has already prepared these
	// memos and writes to them next; registration drops them, so give them
	// back empty, as a new VM has them.
	if vm.enumLookup == nil {
		vm.enumLookup = make(map[string]enumClassLookup)
	}
	if vm.uniqueNestedTypeCache == nil {
		vm.uniqueNestedTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	if vm.onlyNestedTypeCache == nil {
		vm.onlyNestedTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	if vm.topLevelTypeCache == nil {
		vm.topLevelTypeCache = make(map[string]uniqueNestedTypeLookup)
	}
	return err == nil
}

// bindGatedPlatformClassesNamed binds every gated stub class whose simple name
// is name before a search over the class table by simple name, so the search
// sees the candidates it saw before the gate. Each name binds once per VM.
func (vm *VM) bindGatedPlatformClassesNamed(name string) {
	if !vm.gatedPlatformLateBind || vm.bindingGatedPlatformClass {
		return
	}
	if _, done := vm.gatedPlatformBoundNames[name]; done {
		return
	}
	if vm.gatedPlatformBoundNames == nil {
		vm.gatedPlatformBoundNames = make(map[string]struct{})
	}
	vm.gatedPlatformBoundNames[name] = struct{}{}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.Contains(trimmed, ".") {
		return
	}
	typesys.EachGatedPlatformStubRuntimeName(trimmed, func(runtimeName string) {
		vm.lookupClassRaw(runtimeName)
	})
}

// bindGatedPlatformClassInNamespace binds the stub class namespace.className
// before a namespace table of a gated namespace answers for it.
func (vm *VM) bindGatedPlatformClassInNamespace(namespace, className string) {
	if !vm.gatedPlatformLateBind || vm.bindingGatedPlatformClass {
		return
	}
	namespace = strings.TrimSpace(namespace)
	if !typesys.IsGatedPlatformNamespace(namespace) && !typesys.IsGatedPlatformTypeName(namespace) {
		return
	}
	vm.lookupClassRaw(namespace + "." + shortTypeName(strings.TrimSpace(className)))
}
