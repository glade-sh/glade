package vm

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"
)

func classMapPointer(classes map[string]Class) uintptr {
	return reflect.ValueOf(classes).Pointer()
}

func snapshotClassMap(classes map[string]Class) map[string]Class {
	out := make(map[string]Class, len(classes))
	for name, class := range classes {
		out[name] = class
	}
	return out
}

func assertClassMapUnchanged(t *testing.T, label string, got, want map[string]Class) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: class map has %d entries, want %d", label, len(got), len(want))
	}
	for name, class := range want {
		current, ok := got[name]
		if !ok {
			t.Fatalf("%s: class %q missing", label, name)
		}
		if !sameClassValue(current, class) {
			t.Fatalf("%s: class %q changed", label, name)
		}
	}
}

// classMapShareBase registers a namespaced Registry class with a static
// counter and n passive classes, then freezes the class lookup.
func classMapShareBase(t testing.TB, n int) *VM {
	t.Helper()
	machine := New(nil)
	if err := registerClassMapShareClasses(machine, n); err != nil {
		t.Fatal(err)
	}
	machine.FreezeClassLookup()
	return machine
}

func registerClassMapShareClasses(machine *VM, n int) error {
	bump, err := CompileAnonymous("Count = Count + 1; return Count;")
	if err != nil {
		return err
	}
	get, err := CompileAnonymous("return Count;")
	if err != nil {
		return err
	}
	if err := machine.RegisterClass(Class{
		Name:      "Registry",
		Namespace: "pkg",
		StaticFields: map[string]Field{
			"Count": {Name: "Count", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"bump": {Name: "Registry.bump", ClassName: "pkg.Registry", ReturnType: "Integer", IsStatic: true, Program: bump},
			"get":  {Name: "Registry.get", ClassName: "pkg.Registry", ReturnType: "Integer", IsStatic: true, Program: get},
		},
	}); err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		if err := machine.RegisterClass(Class{Name: fmt.Sprintf("Passive%05d", i), Namespace: "std"}); err != nil {
			return err
		}
	}
	return nil
}

func TestCloneRuntimeFrozenSharedSharesClassMapUntilWrite(t *testing.T) {
	base := classMapShareBase(t, 64)
	baseMap := classMapPointer(base.Classes)
	baseSnapshot := snapshotClassMap(base.Classes)
	baseStatics := reflect.ValueOf(base.Classes["Registry"].StaticFields).Pointer()

	first := base.CloneRuntimeFrozenShared(nil)
	second := base.CloneRuntimeFrozenShared(nil)
	if got := classMapPointer(first.Classes); got != baseMap {
		t.Fatalf("clone class map = %x, want shared base map %x", got, baseMap)
	}

	value, err := first.CallStatic("Registry.bump", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Int != 1 {
		t.Fatalf("Registry.bump = %#v, want 1", value)
	}
	if got := classMapPointer(first.Classes); got == baseMap {
		t.Fatal("static write did not give the clone its own class map")
	}
	if got := classMapPointer(base.Classes); got != baseMap {
		t.Fatalf("base class map = %x after clone write, want %x", got, baseMap)
	}
	assertClassMapUnchanged(t, "base after clone static write", base.Classes, baseSnapshot)
	if got := reflect.ValueOf(base.Classes["pkg.Registry"].StaticFields).Pointer(); got != baseStatics {
		t.Fatal("clone static write replaced the base static map")
	}
	if got := base.Classes["Registry"].StaticFields["Count"].Value.Int; got != 0 {
		t.Fatalf("base Count = %d, want 0", got)
	}
	if got := classMapPointer(second.Classes); got != baseMap {
		t.Fatal("sibling clone lost the shared base map")
	}
	if value, err := second.CallStatic("Registry.get", nil); err != nil || value.Int != 0 {
		t.Fatalf("sibling Registry.get = %#v, %v; want 0", value, err)
	}

	if err := first.RegisterClass(Class{Name: "Added", Namespace: "pkg"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := first.lookupClass("pkg.Added"); !ok {
		t.Fatal("clone registration missing")
	}
	assertClassMapUnchanged(t, "base after clone registration", base.Classes, baseSnapshot)
	if _, ok := base.lookupClass("pkg.Added"); ok {
		t.Fatal("clone registration reached the base")
	}
	if _, ok := second.lookupClass("pkg.Added"); ok {
		t.Fatal("clone registration reached a sibling")
	}
}

func TestCloneRuntimeFrozenSharedRegistrationLeavesBaseUnchanged(t *testing.T) {
	base := classMapShareBase(t, 64)
	baseMap := classMapPointer(base.Classes)
	baseSnapshot := snapshotClassMap(base.Classes)

	clone := base.CloneRuntimeFrozenShared(nil)
	if err := clone.RegisterClass(Class{Name: "Passive00003", Namespace: "std", IsTest: true}); err != nil {
		t.Fatal(err)
	}
	if !clone.Classes["std.Passive00003"].IsTest {
		t.Fatal("clone replacement not visible in clone")
	}
	if got := classMapPointer(base.Classes); got != baseMap {
		t.Fatal("clone replacement replaced the base map")
	}
	assertClassMapUnchanged(t, "base after clone replacement", base.Classes, baseSnapshot)
}

func TestCloneRuntimeFrozenSharedBaseWriteDoesNotReachClone(t *testing.T) {
	base := classMapShareBase(t, 8)
	clone := base.CloneRuntimeFrozenShared(nil)
	cloneMap := classMapPointer(clone.Classes)
	cloneSnapshot := snapshotClassMap(clone.Classes)

	// The base is written only through class values here: a frozen base
	// shares its static field maps with clones and must not execute.
	if err := base.RegisterClass(Class{Name: "Registry", Namespace: "pkg", IsTest: true}); err != nil {
		t.Fatal(err)
	}
	if !base.Classes["pkg.Registry"].IsTest {
		t.Fatal("base replacement not visible in base")
	}
	if got := classMapPointer(clone.Classes); got != cloneMap {
		t.Fatal("base write replaced the clone map")
	}
	assertClassMapUnchanged(t, "clone after base write", clone.Classes, cloneSnapshot)
	if clone.Classes["Registry"].IsTest {
		t.Fatal("base replacement reached the clone")
	}
	// Registration unfroze the base, so a later clone copies eagerly.
	later := base.CloneRuntimeFrozenShared(nil)
	if classMapPointer(later.Classes) == classMapPointer(base.Classes) {
		t.Fatal("clone of an unfrozen base shared the base map")
	}
}

func TestCloneRuntimeFrozenSharedCopiesWhenAliasesDiverge(t *testing.T) {
	base := New(nil)
	if err := registerClassMapShareClasses(base, 4); err != nil {
		t.Fatal(err)
	}
	short := base.Classes["Registry"]
	short.IsTest = true
	base.Classes["Registry"] = short
	base.FreezeClassLookup()
	if base.sharedClassCopyPlan.uniformAliases {
		t.Fatal("plan reports uniform aliases for a diverged alias")
	}
	clone := base.CloneRuntimeFrozenShared(nil)
	if classMapPointer(clone.Classes) == classMapPointer(base.Classes) {
		t.Fatal("clone shared a base map whose aliases diverge")
	}
	if clone.Classes["Registry"].IsTest != clone.Classes["pkg.Registry"].IsTest {
		t.Fatal("planned copy left clone aliases diverged")
	}
}

func TestCloneRuntimeFrozenSharedMatchesFreshMachine(t *testing.T) {
	fresh := New(nil)
	if err := registerClassMapShareClasses(fresh, 32); err != nil {
		t.Fatal(err)
	}
	base := classMapShareBase(t, 32)
	clone := base.CloneRuntimeFrozenShared(nil)

	run := func(machine *VM) []string {
		var out []string
		for _, call := range []string{"Registry.bump", "Registry.bump", "Registry.get"} {
			value, err := machine.CallStatic(call, nil)
			out = append(out, fmt.Sprintf("%s=%#v err=%v", call, value, err))
		}
		if err := machine.RegisterClass(Class{
			Name:      "Late",
			Namespace: "pkg",
			StaticFields: map[string]Field{
				"Flag": {Name: "Flag", Type: "Boolean", Static: true, Value: Bool(true)},
			},
		}); err != nil {
			out = append(out, "register: "+err.Error())
		}
		for _, name := range []string{"Registry", "pkg.Registry", "PKG.registry", "Late", "pkg.Late", "std.Passive00007", "passive00007", "Missing"} {
			class, ok := machine.lookupClass(name)
			out = append(out, fmt.Sprintf("lookup %s=%v %s.%s", name, ok, class.Namespace, class.Name))
		}
		for _, name := range []string{"Registry", "pkg.Registry", "Late", "pkg.Late"} {
			class := machine.Classes[name]
			var statics []string
			for field, value := range class.StaticFields {
				statics = append(statics, fmt.Sprintf("%s=%#v", field, value.Value))
			}
			sort.Strings(statics)
			out = append(out, fmt.Sprintf("statics %s=%v", name, statics))
		}
		keys := make([]string, 0, len(machine.Classes))
		for name := range machine.Classes {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		out = append(out, fmt.Sprintf("keys=%v", keys))
		return out
	}

	want := run(fresh)
	got := run(clone)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frozen-shared clone diverged from fresh machine:\n got %v\nwant %v", got, want)
	}
	if value, err := base.CallStatic("Registry.get", nil); err != nil || value.Int != 0 {
		t.Fatalf("base Registry.get = %#v, %v; want 0", value, err)
	}
	if _, ok := base.Classes["pkg.Late"]; ok {
		t.Fatal("clone registration reached the base")
	}
}

func TestCloneRuntimeFrozenSharedClassMapCostIndependentOfClassCount(t *testing.T) {
	small := classMapShareBase(t, 1)
	large := classMapShareBase(t, 4000)
	if len(large.Classes) < 8000 {
		t.Fatalf("large base has %d class entries, want at least 8000", len(large.Classes))
	}
	smallAllocs := testing.AllocsPerRun(20, func() { small.CloneRuntimeFrozenShared(nil) })
	largeAllocs := testing.AllocsPerRun(20, func() { large.CloneRuntimeFrozenShared(nil) })
	if largeAllocs > smallAllocs+2 {
		t.Fatalf("clone of %d classes allocates %.0f times, %d-class clone %.0f; class map is being copied",
			len(large.Classes), largeAllocs, len(small.Classes), smallAllocs)
	}

	const runs = 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		large.CloneRuntimeFrozenShared(nil)
	}
	runtime.ReadMemStats(&after)
	perClone := (after.TotalAlloc - before.TotalAlloc) / runs
	const budget = 64 << 10
	if perClone > budget {
		t.Fatalf("clone of %d classes allocates %d bytes, budget %d", len(large.Classes), perClone, budget)
	}
}

// TestSameClassValueDetectsEveryClassField changes one Class field at a time
// so a field added to Class cannot be missed by the alias uniformity check.
func TestSameClassValueDetectsEveryClassField(t *testing.T) {
	base := Class{
		Name:                 "Name",
		Namespace:            "ns",
		APIVersion:           "62.0",
		SuperClass:           "Super",
		Interfaces:           []string{"I"},
		Fields:               map[string]Field{"f": {}},
		StaticFields:         map[string]Field{"s": {}},
		FieldOrder:           []string{"f"},
		StaticFieldOrder:     []string{"s"},
		Methods:              map[string]Method{"m": {}},
		Constructors:         []Method{{}},
		StaticInitializers:   []Method{{}},
		InstanceInitializers: []Method{{}},
		EnumValues:           []string{"A"},
		Access:               "public",
		Modifiers:            []string{"virtual"},
	}
	if !sameClassValue(base, base) {
		t.Fatal("class differs from its own copy")
	}
	classType := reflect.TypeOf(base)
	for i := 0; i < classType.NumField(); i++ {
		changed := base
		field := reflect.ValueOf(&changed).Elem().Field(i)
		switch field.Kind() {
		case reflect.String:
			field.SetString(field.String() + "x")
		case reflect.Bool:
			field.SetBool(!field.Bool())
		case reflect.Slice:
			field.Set(reflect.AppendSlice(reflect.MakeSlice(field.Type(), 0, field.Len()), field))
		case reflect.Map:
			field.Set(reflect.MakeMap(field.Type()))
		default:
			t.Fatalf("Class.%s has kind %s; teach sameClassValue to compare it", classType.Field(i).Name, field.Kind())
		}
		if sameClassValue(base, changed) {
			t.Fatalf("sameClassValue ignores Class.%s", classType.Field(i).Name)
		}
	}
}

func TestCloneRuntimeFrozenSharedConcurrentClonesStayIsolated(t *testing.T) {
	base := classMapShareBase(t, 256)
	baseSnapshot := snapshotClassMap(base.Classes)
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			clone := base.CloneRuntimeFrozenShared(nil)
			for j := 0; j <= i; j++ {
				if _, err := clone.CallStatic("Registry.bump", nil); err != nil {
					errs <- err
					return
				}
			}
			if err := clone.RegisterClass(Class{Name: fmt.Sprintf("Worker%d", i), Namespace: "pkg"}); err != nil {
				errs <- err
				return
			}
			value, err := clone.CallStatic("Registry.get", nil)
			if err != nil {
				errs <- err
				return
			}
			if value.Int != int64(i+1) {
				errs <- fmt.Errorf("worker %d Count = %d, want %d", i, value.Int, i+1)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	assertClassMapUnchanged(t, "base after concurrent clones", base.Classes, baseSnapshot)
	if got := base.Classes["Registry"].StaticFields["Count"].Value.Int; got != 0 {
		t.Fatalf("base Count = %d, want 0", got)
	}
}
