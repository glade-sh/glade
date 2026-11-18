package vm

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// lazyTestClass is one synthetic platform class. build returns a new class on
// every call, as the platform builders do.
type lazyTestClass struct {
	header Class
	build  func() Class
}

func lazyTestMethod(class, name string, static bool, returnType string, params ...string) Method {
	method := Method{Name: class + "." + name, ClassName: class, IsStatic: static, ReturnType: returnType, Access: "global", APIVersion: "67.0"}
	for i, param := range params {
		method.Params = append(method.Params, Param{Name: fmt.Sprintf("arg%d", i), Type: param})
	}
	return method
}

func lazyTestClasses() []lazyTestClass {
	var out []lazyTestClass
	add := func(header Class, build func() Class) {
		header.PlatformType = true
		header.Access = "global"
		out = append(out, lazyTestClass{header: header, build: func() Class {
			class := build()
			class.PlatformType = true
			class.Access = "global"
			class.Name, class.Namespace = header.Name, header.Namespace
			class.SuperClass, class.IsInterface, class.IsAbstract = header.SuperClass, header.IsInterface, header.IsAbstract
			class.Interfaces = append([]string(nil), header.Interfaces...)
			return class
		}})
	}
	add(Class{Name: "Ns.Alpha", Namespace: "Ns", SuperClass: "Object"}, func() Class {
		return Class{
			Fields: map[string]Field{"label": {Name: "label", Type: "String", Property: true, InitialValue: Null}, "size": {Name: "size", Type: "Integer", Property: true, InitialValue: Null}},
			Methods: map[string]Method{
				"run()":        lazyTestMethod("Ns.Alpha", "run", false, "String"),
				"run(String)":  lazyTestMethod("Ns.Alpha", "run", false, "String", "String"),
				"run(Integer)": lazyTestMethod("Ns.Alpha", "run", false, "Integer", "Integer"),
				"build()":      lazyTestMethod("Ns.Alpha", "build", true, "Ns.Alpha"),
			},
			Constructors: []Method{{Name: "Ns.Alpha.<init>", ClassName: "Ns.Alpha", ReturnType: "void", Access: "global"}},
			FieldOrder:   []string{"label", "size"},
		}
	})
	add(Class{Name: "Ns.Beta", Namespace: "Ns", IsInterface: true}, func() Class {
		return Class{Methods: map[string]Method{"call()": lazyTestMethod("Ns.Beta", "call", false, "void")}}
	})
	add(Class{Name: "Ns.Outer.Inner", Namespace: "Ns", Interfaces: []string{"Ns.Beta"}}, func() Class {
		return Class{Methods: map[string]Method{"call()": lazyTestMethod("Ns.Outer.Inner", "call", false, "void")}}
	})
	add(Class{Name: "Other.Gamma", Namespace: "Other", IsAbstract: true}, func() Class {
		return Class{Methods: map[string]Method{"go(Boolean)": lazyTestMethod("Other.Gamma", "go", false, "Boolean", "Boolean")}}
	})
	add(Class{Name: "Plain"}, func() Class {
		return Class{
			Methods:      map[string]Method{"go()": lazyTestMethod("Plain", "go", true, "String")},
			Constructors: []Method{{Name: "Plain.<init>", ClassName: "Plain", ReturnType: "void", Access: "global", Params: []Param{{Name: "a", Type: "String"}}}},
		}
	})
	add(Class{Name: "Ns.Empty", Namespace: "Ns"}, func() Class { return Class{} })
	// Eager: a static member (an enum) and a class whose alias is not its name.
	add(Class{Name: "Ns.Mode", Namespace: "Ns"}, func() Class {
		return Class{
			StaticFields:     map[string]Field{"FAST": {Name: "FAST", Type: "Ns.Mode", Static: true, Value: Value{Kind: ValueObject, Type: "Ns.Mode", Text: "FAST"}}},
			StaticFieldOrder: []string{"FAST"},
			EnumValues:       []string{"FAST"},
		}
	})
	add(Class{Name: "Delta", Namespace: "Ns"}, func() Class {
		return Class{Methods: map[string]Method{"x()": lazyTestMethod("Delta", "x", false, "void")}}
	})
	return out
}

// registerLazyTestClasses registers the classes on machine. With lazy, a class
// that can be a shell is one; every other class, and every class when lazy is
// false, is registered from its full declaration.
func registerLazyTestClasses(t *testing.T, machine *VM, classes []lazyTestClass, set *LazyClassSet) {
	t.Helper()
	for _, entry := range classes {
		class := entry.build()
		if set != nil {
			if len(class.StaticFields) == 0 {
				if shell, ok := set.Add(entry.header, entry.build); ok {
					class = shell
				}
			}
		}
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
}

func lazyTestMethodSummary(method Method) string {
	var params []string
	for _, param := range method.Params {
		params = append(params, param.Type)
	}
	return fmt.Sprintf("%s|%s|static=%v|%s|(%s)|%s", method.Name, method.ClassName, method.IsStatic, method.ReturnType, strings.Join(params, ","), method.Access)
}

func lazyTestMethodSummaries(methods []Method) []string {
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		out = append(out, lazyTestMethodSummary(method))
	}
	sort.Strings(out)
	return out
}

func lazyTestClassSummary(class Class) string {
	var fields, static, methods, ctors []string
	for name, field := range class.Fields {
		fields = append(fields, name+":"+field.Type)
	}
	for name, field := range class.StaticFields {
		static = append(static, name+":"+field.Type)
	}
	for key, method := range class.Methods {
		methods = append(methods, key+"="+lazyTestMethodSummary(method))
	}
	for _, ctor := range class.Constructors {
		ctors = append(ctors, lazyTestMethodSummary(ctor))
	}
	for _, list := range [][]string{fields, static, methods, ctors} {
		sort.Strings(list)
	}
	return fmt.Sprintf("%s|ns=%s|api=%s|super=%s|ifaces=%v|abs=%v|iface=%v|platform=%v|access=%s|order=%v/%v|enum=%v|fields=%v|static=%v|methods=%v|ctors=%v",
		class.Name, class.Namespace, class.APIVersion, class.SuperClass, class.Interfaces, class.IsAbstract, class.IsInterface, class.PlatformType, class.Access,
		class.FieldOrder, class.StaticFieldOrder, class.EnumValues, fields, static, methods, ctors)
}

// lazyTestSpellings lists every name form a lookup is asked with.
var lazyTestSpellings = []string{
	"Ns.Alpha", "ns.alpha", "NS.ALPHA", "Alpha", "alpha", " Ns.Alpha ", "Ns.Ns.Alpha", "ns.ns.alpha",
	"Ns.Beta", "ns.beta", "Beta", "Ns.Outer.Inner", "ns.outer.inner", "Outer.Inner", "Inner", "Ns.Inner",
	"Other.Gamma", "other.gamma", "Gamma", "Plain", "plain", "PLAIN", "Ns.Plain", "Ns.Empty", "empty",
	"Ns.Mode", "ns.mode", "Mode", "Delta", "Ns.Delta", "delta", "Ns.Missing", "Missing", "", "Ns.", ".Alpha",
}

// lazyTestObserve records everything the lookups and method tables answer for
// the machine, in a form independent of map order and registration order among
// overloads.
func lazyTestObserve(machine *VM) map[string]string {
	out := make(map[string]string)
	for _, spelling := range lazyTestSpellings {
		class, ok := machine.lookupClass(spelling)
		if ok {
			out["lookup "+spelling] = lazyTestClassSummary(class)
		} else {
			out["lookup "+spelling] = "missing"
		}
		resolved, ok := machine.resolveClassName(spelling)
		out["resolve "+spelling] = fmt.Sprintf("%s/%v", resolved, ok)
		raw, ok := machine.lookupClassRaw(spelling)
		out["raw "+spelling] = fmt.Sprintf("%s|%s|%s|%v|%v/%v", raw.Name, raw.Namespace, raw.SuperClass, raw.IsInterface, raw.Dependency, ok)
		for _, namespace := range []string{"", "Ns", "ns", "Other"} {
			class, ok := machine.lookupClassInNamespace(namespace, spelling)
			if ok {
				out["ns "+namespace+" "+spelling] = lazyTestClassSummary(class)
			} else {
				out["ns "+namespace+" "+spelling] = "missing"
			}
		}
		entry, ok := machine.classEntry(spelling)
		if ok {
			out["entry "+spelling] = lazyTestClassSummary(entry)
		} else {
			out["entry "+spelling] = "missing"
		}
	}
	var entries []string
	for name, class := range machine.ClassEntries() {
		entries = append(entries, name+" => "+lazyTestClassSummary(class))
	}
	sort.Strings(entries)
	out["entries"] = strings.Join(entries, "\n")
	var names []string
	for name := range machine.classEntriesRaw() {
		names = append(names, name)
	}
	sort.Strings(names)
	out["raw entries"] = strings.Join(names, ",")
	var staticful []string
	for name, class := range machine.classEntriesNoLazy() {
		// Enumerations that read static state must see every class that has
		// some: the shells they skip have none.
		if len(class.StaticFields) > 0 {
			staticful = append(staticful, name)
		}
	}
	sort.Strings(staticful)
	out["static entries"] = strings.Join(staticful, ",")
	var search []string
	for _, entry := range machine.classNameSearchEntries() {
		search = append(search, entry.Name)
	}
	out["search"] = strings.Join(search, ",")
	out["classes"] = fmt.Sprint(len(machine.Classes))
	// Method tables.
	methodNames := []string{"Missing.none", "Ns.Alpha.missing", "x", ""}
	for _, entry := range lazyTestClasses() {
		for _, method := range entry.build().Methods {
			methodNames = append(methodNames, method.Name, strings.ToUpper(method.Name), strings.ToLower(method.Name))
		}
	}
	sort.Strings(methodNames)
	for _, name := range methodNames {
		method, ok := machine.registeredMethod(name)
		if ok {
			out["method "+name] = lazyTestMethodSummary(method)[:strings.Index(lazyTestMethodSummary(method), "|static")]
		} else {
			out["method "+name] = "missing"
		}
		out["overloads "+name] = strings.Join(lazyTestMethodSummaries(machine.registeredOverloads(name)), ";")
		out["folded "+name] = strings.Join(lazyTestMethodSummaries(machine.registeredFolded(strings.ToLower(name))), ";")
	}
	var ranged []string
	machine.RangeRegisteredMethods(func(method Method) { ranged = append(ranged, method.Name) })
	sort.Strings(ranged)
	out["ranged"] = strings.Join(ranged, ",")
	return out
}

func lazyTestCompare(t *testing.T, label string, want, got map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if want[key] != got[key] {
			t.Errorf("%s: %q\n eager: %s\n lazy:  %s", label, key, want[key], got[key])
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("%s: unexpected observation %q", label, key)
		}
	}
}

func lazyTestMachines(t *testing.T, freeze bool) (eager, lazy *VM) {
	t.Helper()
	eager, lazy = New(nil), New(nil)
	registerLazyTestClasses(t, eager, lazyTestClasses(), nil)
	registerLazyTestClasses(t, lazy, lazyTestClasses(), NewLazyClassSet(8))
	if freeze {
		eager.FreezeClassLookup()
		lazy.FreezeClassLookup()
	}
	return eager, lazy
}

func TestLazyClassRegistersShellsWithoutBuilding(t *testing.T) {
	_, lazy := lazyTestMachines(t, true)
	shells := 0
	for _, class := range lazy.classEntriesRaw() {
		if class.IsLazy() {
			shells++
			if len(class.Fields) != 0 || len(class.Methods) != 0 || len(class.Constructors) != 0 {
				t.Fatalf("shell %s carries members", class.Name)
			}
			if class.lazy.built != nil {
				t.Fatalf("shell %s was built by registration", class.Name)
			}
		}
	}
	if shells != 6 {
		t.Fatalf("shells = %d, want 6 (every class without statics whose alias is its name)", shells)
	}
	// Name resolution, existence checks and header reads leave shells unbuilt.
	for _, spelling := range lazyTestSpellings {
		lazy.resolveClassName(spelling)
		lazy.lookupClassRaw(spelling)
		lazy.lookupClassInNamespaceRaw("Ns", spelling)
	}
	lazy.isSubclass("Ns.Outer.Inner", "Ns.Beta")
	for range lazy.classEntriesNoLazy() {
	}
	for _, class := range lazy.classEntriesRaw() {
		if class.IsLazy() && class.lazy.built != nil {
			t.Fatalf("header-only reads built %s", class.Name)
		}
	}
	// One lookup builds one class.
	class, ok := lazy.lookupClass("ns.alpha")
	if !ok || len(class.Methods) != 4 || len(class.Fields) != 2 || len(class.Constructors) != 1 {
		t.Fatalf("lookup built %#v", class.Name)
	}
	built := 0
	for _, class := range lazy.classEntriesRaw() {
		if class.IsLazy() && class.lazy.built != nil {
			built++
		}
	}
	if built != 1 {
		t.Fatalf("built classes after one lookup = %d, want 1", built)
	}
}

func TestLazyClassOutcomeEquality(t *testing.T) {
	for _, freeze := range []bool{true, false} {
		eager, lazy := lazyTestMachines(t, freeze)
		label := fmt.Sprintf("freeze=%v", freeze)
		// Observe the lazy machine first, on a cold set, then again warm.
		cold := lazyTestObserve(lazy)
		want := lazyTestObserve(eager)
		lazyTestCompare(t, label+" cold", want, cold)
		lazyTestCompare(t, label+" warm", want, lazyTestObserve(lazy))
	}
}

func TestLazyClassOutcomeEqualityOnClones(t *testing.T) {
	eager, lazy := lazyTestMachines(t, true)
	want := lazyTestObserve(eager)
	clones := map[string]func(*VM) *VM{
		"frozen shared": func(m *VM) *VM { return m.CloneRuntimeFrozenShared(nil) },
		"clone":         func(m *VM) *VM { return m.CloneRuntime(nil) },
		"clone of clone": func(m *VM) *VM {
			return m.CloneRuntimeFrozenShared(nil).CloneRuntime(nil)
		},
	}
	for name, clone := range clones {
		lazyTestCompare(t, name, want, lazyTestObserve(clone(lazy)))
	}
	// A clone registering a class that overlays the frozen generation, then
	// freezing again, answers as the eager machine does.
	for _, name := range []string{"registered after clone", "registered then frozen"} {
		extraEager, extraLazy := eager.CloneRuntimeFrozenShared(nil), lazy.CloneRuntimeFrozenShared(nil)
		for _, machine := range []*VM{extraEager, extraLazy} {
			if err := machine.RegisterClass(Class{Name: "Project", Methods: map[string]Method{"p()": lazyTestMethod("Project", "p", true, "void")}}); err != nil {
				t.Fatal(err)
			}
			if name == "registered then frozen" {
				machine.FreezeClassLookup()
			}
		}
		lazyTestCompare(t, name, lazyTestObserveWith(extraEager), lazyTestObserveWith(extraLazy))
	}
	// The source machine keeps answering after its clones have been used.
	lazyTestCompare(t, "source after clones", want, lazyTestObserve(lazy))
}

func lazyTestObserveWith(machine *VM) map[string]string {
	out := lazyTestObserve(machine)
	class, ok := machine.lookupClass("Project")
	out["project"] = fmt.Sprintf("%v/%s", ok, lazyTestClassSummary(class))
	method, ok := machine.registeredMethod("Project.p")
	out["project method"] = fmt.Sprintf("%v/%s", ok, lazyTestMethodSummary(method))
	return out
}

// Registering a class under the name of a shell takes the shell's place as it
// would take an eagerly registered class's: the same merge, the same methods.
func TestLazyClassReplacedByRegistrationMatchesEager(t *testing.T) {
	collisions := map[string]Class{
		"same name and namespace": {Name: "Ns.Alpha", Namespace: "Ns", Dependency: true, Methods: map[string]Method{"extra()": lazyTestMethod("Ns.Alpha", "extra", false, "void")}},
		"qualified alias":         {Name: "Alpha", Namespace: "Ns", Methods: map[string]Method{"own()": lazyTestMethod("Alpha", "own", false, "void")}},
		"other class same alias":  {Name: "Plain", Namespace: "Other", Methods: map[string]Method{"o()": lazyTestMethod("Plain", "o", false, "void")}},
	}
	for name, collision := range collisions {
		for _, freeze := range []bool{true, false} {
			eager, lazy := lazyTestMachines(t, freeze)
			for _, machine := range []*VM{eager, lazy} {
				clone := collision
				clone.Methods = map[string]Method{}
				for key, method := range collision.Methods {
					clone.Methods[key] = method
				}
				if err := machine.RegisterClass(clone); err != nil {
					t.Fatal(err)
				}
				if freeze {
					machine.FreezeClassLookup()
				}
			}
			lazyTestCompare(t, fmt.Sprintf("%s freeze=%v", name, freeze), lazyTestObserve(eager), lazyTestObserve(lazy))
		}
	}
}

func TestLazyClassRegistrationFallsBackToEagerWhenNameIsTaken(t *testing.T) {
	set := NewLazyClassSet(4)
	classes := lazyTestClasses()
	machine := New(nil)
	registerLazyTestClasses(t, machine, classes[:1], set)
	registerLazyTestClasses(t, machine, classes[:1], set)
	eager := New(nil)
	registerLazyTestClasses(t, eager, classes[:1], nil)
	registerLazyTestClasses(t, eager, classes[:1], nil)
	lazyTestCompare(t, "duplicate registration", lazyTestObserve(eager), lazyTestObserve(machine))
	// A second set cannot share a VM; its classes register eagerly.
	other := New(nil)
	registerLazyTestClasses(t, other, classes[:2], NewLazyClassSet(4))
	registerLazyTestClasses(t, other, classes[2:3], NewLazyClassSet(4))
	if class, _ := other.classEntryRaw("Ns.Outer.Inner"); class.IsLazy() {
		t.Fatal("a shell from a second set was registered lazily")
	}
	if _, ok := other.lookupClass("Ns.Outer.Inner"); !ok {
		t.Fatal("eager fallback class is missing")
	}
}

func TestLazyClassSetRefusesUnsafeShells(t *testing.T) {
	set := NewLazyClassSet(4)
	if _, ok := set.Add(Class{Name: "Delta", Namespace: "Ns"}, func() Class { return Class{} }); ok {
		t.Fatal("a class whose runtime alias differs from its name became a shell")
	}
	if _, ok := set.Add(Class{Name: "Ns.A", Namespace: "Ns"}, func() Class { return Class{} }); !ok {
		t.Fatal("ordinary class was refused")
	}
	if _, ok := set.Add(Class{Name: "ns.a", Namespace: "ns"}, func() Class { return Class{} }); ok {
		t.Fatal("two shells with one lower-cased name were accepted")
	}
	if _, ok := set.Add(Class{Name: ""}, func() Class { return Class{} }); ok {
		t.Fatal("unnamed class became a shell")
	}
}

// The shared build state is read by concurrent runtimes. Run with -race.
func TestLazyClassConcurrentLookups(t *testing.T) {
	_, lazy := lazyTestMachines(t, true)
	want := lazyTestObserve(lazyTestEagerFor(t))
	var wg sync.WaitGroup
	results := make([]map[string]string, 8)
	for i := range results {
		wg.Add(1)
		clone := lazy.CloneRuntimeFrozenShared(nil)
		go func() {
			defer wg.Done()
			results[i] = lazyTestObserve(clone)
		}()
	}
	wg.Wait()
	for i, got := range results {
		lazyTestCompare(t, fmt.Sprintf("goroutine %d", i), want, got)
	}
}

func lazyTestEagerFor(t *testing.T) *VM {
	t.Helper()
	eager, _ := lazyTestMachines(t, true)
	return eager
}

// Writing a static value into a class does not reach the shared build state or
// other runtimes: only classes with statics are written, and they are eager.
func TestLazyClassStaticWritesStayInTheRuntime(t *testing.T) {
	_, lazy := lazyTestMachines(t, true)
	first, second := lazy.CloneRuntimeFrozenShared(nil), lazy.CloneRuntimeFrozenShared(nil)
	class, ok := first.ensureMutableClass("Ns.Mode")
	if !ok {
		t.Fatal("Ns.Mode is missing")
	}
	field := class.StaticFields["FAST"]
	field.Value = Value{Kind: ValueString, Text: "changed"}
	class.StaticFields["FAST"] = field
	first.storeMutableClassAtAlias("Ns.Mode", class)
	if got, _ := first.classEntry("Ns.Mode"); got.StaticFields["FAST"].Value.Text != "changed" {
		t.Fatal("first runtime lost its static write")
	}
	if got, _ := second.classEntry("Ns.Mode"); got.StaticFields["FAST"].Value.Text != "FAST" {
		t.Fatal("static write reached another runtime")
	}
	if err := first.ResetStatics(); err != nil {
		t.Fatal(err)
	}
	if got, _ := first.classEntry("Ns.Alpha"); !reflect.DeepEqual(got.FieldOrder, []string{"label", "size"}) {
		t.Fatalf("Ns.Alpha after ResetStatics = %v", got.FieldOrder)
	}
}
