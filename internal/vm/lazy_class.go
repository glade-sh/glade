package vm

import (
	"iter"
	"sort"
	"strings"
	"sync"
)

// LazyClassSet indexes passive platform classes whose members are built on first
// use. A lazy class is registered as a header-only shell: its name, namespace
// and flags, with no fields, methods or constructors. Every runtime that
// registers the shell shares one handle, so the full class is built once per
// process, on the first lookup that needs its members, and is then shared like
// any other registered class value.
//
// A lazy class has no static fields and no initializers. Nothing writes a
// static value into it, so sharing the built class between runtimes is safe,
// and enumerations that visit static state skip the shells.
type LazyClassSet struct {
	byLower map[string]*lazyClass
	all     []*lazyClass
}

type lazyClass struct {
	set   *LazyClassSet
	key   string
	build func() Class

	once  sync.Once
	built *builtLazyClass
}

// builtLazyClass is a lazy class once its members exist.
type builtLazyClass struct {
	class Class
	// registrations are the method registrations an eager RegisterClass would
	// make for the class, in order; index keys them for lookup.
	registrations []Method
	index         lazyMethodIndex
}

type lazyMethodIndex struct {
	last     map[string]Method
	overload map[string][]Method
	folded   map[string][]Method
}

// NewLazyClassSet returns an empty set sized for capacity classes.
func NewLazyClassSet(capacity int) *LazyClassSet {
	return &LazyClassSet{
		byLower: make(map[string]*lazyClass, capacity),
		all:     make([]*lazyClass, 0, capacity),
	}
}

// Add returns the shell for a class built by build, which must return the class
// exactly as RegisterClass would receive it. header supplies the shell's scalar
// fields; its members are ignored. Add reports false, and the caller registers
// the class eagerly, when the class cannot be lazy: it needs a runtime alias
// other than its name, or another class in the set has the same name once
// lower-cased.
func (s *LazyClassSet) Add(header Class, build func() Class) (Class, bool) {
	name := header.Name
	if name == "" || build == nil || runtimeClassName(header) != name {
		return Class{}, false
	}
	lower := canonicalClassLookupKey(name)
	if _, exists := s.byLower[lower]; exists || name != strings.TrimSpace(name) {
		return Class{}, false
	}
	lc := &lazyClass{set: s, key: name, build: build}
	s.byLower[lower] = lc
	s.all = append(s.all, lc)
	return Class{
		SourceUnit:   header.SourceUnit,
		PlatformType: header.PlatformType,
		Name:         header.Name,
		Namespace:    header.Namespace,
		APIVersion:   header.APIVersion,
		SuperClass:   header.SuperClass,
		Interfaces:   header.Interfaces,
		Access:       header.Access,
		Modifiers:    header.Modifiers,
		IsAbstract:   header.IsAbstract,
		IsInterface:  header.IsInterface,
		IsTest:       header.IsTest,
		Nested:       header.Nested,
		Dependency:   header.Dependency,
		lazy:         lc,
	}, true
}

// IsLazy reports whether the class is a shell whose members are not built yet
// or are built on demand.
func (c Class) IsLazy() bool { return c.lazy != nil }

// BuiltClass returns the class with its members built. A class that is not
// lazy is returned as is. The result of a lazy class is shared; callers must
// not write its maps or slices.
func (c Class) BuiltClass() Class {
	if c.lazy == nil {
		return c
	}
	return c.lazy.materialized()
}

// FreshClass returns a private copy of the class as RegisterClass would
// receive it, with no shared maps or slices. A class that is not lazy is
// returned as is.
func (c Class) FreshClass() Class {
	if c.lazy == nil {
		return c
	}
	class := c.lazy.build()
	class.lazy = nil
	return class
}

func (lc *lazyClass) materialized() Class {
	return lc.builtClass().class
}

func (lc *lazyClass) builtClass() *builtLazyClass {
	lc.once.Do(func() {
		class := lc.build()
		class.lazy = nil
		initializeReportsRuntimeClass(&class)
		prepareRegisteredClass(&class)
		ownerName := runtimeClassName(class)
		registrations := stampRegisteredMethods(&class, ownerName, true)
		lc.built = &builtLazyClass{class: class, registrations: registrations, index: newLazyMethodIndex(registrations)}
	})
	return lc.built
}

func newLazyMethodIndex(registrations []Method) lazyMethodIndex {
	index := lazyMethodIndex{
		last:     make(map[string]Method, len(registrations)),
		overload: make(map[string][]Method, len(registrations)),
		folded:   make(map[string][]Method, len(registrations)),
	}
	for _, method := range registrations {
		index.last[method.Name] = method
		index.overload[method.Name] = append(index.overload[method.Name], method)
		folded := strings.ToLower(method.Name)
		index.folded[folded] = append(index.folded[folded], method)
	}
	return index
}

// resolveLazy returns the built class for a shell and any other class as is.
func resolveLazy(class Class) Class {
	if class.lazy != nil {
		return class.lazy.materialized()
	}
	return class
}

// lazyClassFor returns the lazy class that registers methods named name's
// class part, when this VM still holds its shell. exact requires the class part
// to match the registered spelling.
func (vm *VM) lazyClassFor(name string, exact bool) *lazyClass {
	set := vm.lazySet
	if set == nil {
		return nil
	}
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 {
		return nil
	}
	classPart := name[:dot]
	lc := foldLookupLazyClass(set.byLower, classPart)
	if lc == nil || (exact && lc.key != classPart) {
		return nil
	}
	if raw, ok := vm.classEntryRaw(lc.key); !ok || raw.lazy != lc {
		return nil
	}
	return lc
}

func (vm *VM) lazyRegisteredMethod(name string) (Method, bool) {
	if lc := vm.lazyClassFor(name, true); lc != nil {
		method, ok := lc.builtClass().index.last[name]
		return method, ok
	}
	return Method{}, false
}

func (vm *VM) lazyRegisteredOverloads(name string) []Method {
	if lc := vm.lazyClassFor(name, true); lc != nil {
		return lc.builtClass().index.overload[name]
	}
	return nil
}

// lazyRegisteredFolded reads the folded table; foldedName is lower case.
func (vm *VM) lazyRegisteredFolded(foldedName string) []Method {
	if lc := vm.lazyClassFor(foldedName, false); lc != nil {
		return lc.builtClass().index.folded[foldedName]
	}
	return nil
}

// rangeLazyMethods visits the method each lazy class registers per name, as the
// Methods table holds it, for every shell this VM still holds.
func (vm *VM) rangeLazyMethods(visit func(Method)) {
	set := vm.lazySet
	if set == nil {
		return
	}
	for _, lc := range set.all {
		if raw, ok := vm.classEntryRaw(lc.key); !ok || raw.lazy != lc {
			continue
		}
		last := lc.builtClass().index.last
		names := make([]string, 0, len(last))
		for name := range last {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			visit(last[name])
		}
	}
}

// canRegisterLazy reports whether the shell can be registered without building
// it: no class holds its name yet, and this VM has not taken shells from another
// set.
func (vm *VM) canRegisterLazy(class Class) bool {
	if vm.lazySet != nil && vm.lazySet != class.lazy.set {
		return false
	}
	_, exists := vm.classEntryRaw(class.Name)
	return !exists
}

// registerLazyShell is RegisterClass for a shell: the class is built when
// first used and its methods answer through the lazy method lookups.
func (vm *VM) registerLazyShell(class Class) {
	vm.lazySet = class.lazy.set
	if vm.Classes == nil {
		vm.Classes = make(map[string]Class)
	}
	vm.methodResolveCache = nil
	vm.uniqueNestedTypeCache = nil
	vm.onlyNestedTypeCache = nil
	vm.topLevelTypeCache = nil
	vm.classNameSearchCache = nil
	vm.storeClassAliases(class)
	vm.markStaticInitializationUninitialized(class)
}

// realizeLazyClassAt replaces the shell held at key with its built class, as if
// the class had been registered eagerly: the write registers the shell's
// methods in the method tables (see releaseReplacedLazy). A key that holds no
// shell is left alone.
func (vm *VM) realizeLazyClassAt(key string) {
	if vm.lazySet == nil {
		return
	}
	raw, ok := vm.classEntryRaw(key)
	if !ok || raw.lazy == nil {
		return
	}
	vm.storeClassAliases(raw.lazy.materialized())
}

// releaseReplacedLazy registers the methods of a shell that a write is about to
// replace with another class, so the methods stay visible as they were when the
// class was registered eagerly.
func (vm *VM) releaseReplacedLazy(previous Class, existed bool, next Class) {
	if !existed || previous.lazy == nil || previous.lazy == next.lazy {
		return
	}
	vm.methodResolveCache = nil
	vm.methodCandidates = nil
	for _, method := range previous.lazy.builtClass().registrations {
		_ = vm.RegisterMethod(method)
	}
}

// classEntriesRaw ranges over the class entries, with shells left unbuilt. Use
// it only where a loop reads names, namespaces and flags.
func (vm *VM) classEntriesRaw() iter.Seq2[string, Class] {
	return vm.rangeClassEntries(false, false)
}

// classEntriesNoLazy ranges over the class entries that are not shells. Lazy
// classes carry no static fields or initializers, so a loop that reads only
// static state or enum values sees the same result without them.
func (vm *VM) classEntriesNoLazy() iter.Seq2[string, Class] {
	return vm.rangeClassEntries(false, true)
}

func (vm *VM) rangeClassEntries(build, skipLazy bool) iter.Seq2[string, Class] {
	return func(yield func(string, Class) bool) {
		classes := vm.Classes
		if vm.classValueOverlay == nil {
			for name, class := range classes {
				if class.lazy != nil {
					if skipLazy {
						continue
					}
					if build {
						class = class.lazy.materialized()
					}
				}
				if !yield(name, class) {
					return
				}
			}
			return
		}
		for name := range classes {
			class, ok := vm.classEntryRaw(name)
			if !ok {
				continue
			}
			if class.lazy != nil {
				if skipLazy {
					continue
				}
				if build {
					class = class.lazy.materialized()
				}
			}
			if !yield(name, class) {
				return
			}
		}
	}
}

// foldLookupLazyClass probes m with the case-folded form of name without
// allocating, as foldLookupStringMap does.
func foldLookupLazyClass(m map[string]*lazyClass, name string) *lazyClass {
	trimmed := strings.TrimSpace(name)
	needsFold := false
	for i := 0; i < len(trimmed); i++ {
		if c := trimmed[i]; c >= 'A' && c <= 'Z' {
			needsFold = true
			break
		}
	}
	if !needsFold {
		return m[trimmed]
	}
	if len(trimmed) <= foldClassKeyBuf {
		var buf [foldClassKeyBuf]byte
		for i := 0; i < len(trimmed); i++ {
			c := trimmed[i]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			buf[i] = c
		}
		return m[string(buf[:len(trimmed)])]
	}
	return m[canonicalClassLookupKey(name)]
}
