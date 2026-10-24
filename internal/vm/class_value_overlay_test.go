package vm

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
)

// classValue reads a class entry the way the runtime does: the value the VM
// wrote over a shared class map, else the map's.
func classValue(machine *VM, name string) Class {
	class, _ := machine.classEntry(name)
	return class
}

// Reflection keeps the allocation regression runnable before the overlay exists.
func classValueOverlaySize(machine *VM) int {
	field := reflect.ValueOf(machine).Elem().FieldByName("classValueOverlay")
	if !field.IsValid() {
		return 0
	}
	return field.Len()
}

func staticCount(t *testing.T, machine *VM, alias string) int64 {
	t.Helper()
	field, ok := classValue(machine, alias).StaticFields["Count"]
	if !ok {
		t.Fatalf("%s has no Count static", alias)
	}
	return field.Value.Int
}

// writeStaticCount writes Count through owner, the name a static assignment
// in Apex resolves to.
func writeStaticCount(t *testing.T, machine *VM, owner string, value int64) {
	t.Helper()
	class, ok := machine.lookupClass(owner)
	if !ok {
		t.Fatalf("class %s missing", owner)
	}
	machine.writeStaticFieldValue(owner, "Count", class, class.StaticFields["Count"], Int(value))
}

func TestCloneRuntimeFrozenSharedStaticWritesStayInClone(t *testing.T) {
	base := classMapShareBase(t, 64)
	baseMap := classMapPointer(base.Classes)
	baseSnapshot := snapshotClassMap(base.Classes)
	first := base.CloneRuntimeFrozenShared(nil)
	second := base.CloneRuntimeFrozenShared(nil)

	// Writes through the short alias and through the qualified name reach
	// one class in the writing clone.
	writeStaticCount(t, first, "Registry", 5)
	for _, alias := range []string{"Registry", "pkg.Registry"} {
		if got := staticCount(t, first, alias); got != 5 {
			t.Fatalf("first %s Count = %d after short-alias write, want 5", alias, got)
		}
	}
	writeStaticCount(t, first, "pkg.Registry", 7)
	if !sameMap(classValue(first, "Registry").StaticFields, classValue(first, "pkg.Registry").StaticFields) {
		t.Fatal("clone aliases hold different static maps")
	}
	for _, alias := range []string{"Registry", "pkg.Registry"} {
		if got := staticCount(t, first, alias); got != 7 {
			t.Fatalf("first %s Count = %d after qualified write, want 7", alias, got)
		}
	}
	if value, err := first.CallStatic("Registry.get", nil); err != nil || value.Int != 7 {
		t.Fatalf("first Registry.get = %#v, %v; want 7", value, err)
	}
	snapshot := first.staticFieldSnapshot()
	if snapshot["Registry"]["Count"].Int != 7 || snapshot["pkg.Registry"]["Count"].Int != 7 {
		t.Fatalf("first static snapshot = %v, %v; want 7 under both aliases", snapshot["Registry"]["Count"], snapshot["pkg.Registry"]["Count"])
	}

	// Neither the sibling nor the base sees the clone's writes.
	for label, machine := range map[string]*VM{"second": second, "base": base} {
		for _, alias := range []string{"Registry", "pkg.Registry"} {
			if got := staticCount(t, machine, alias); got != 0 {
				t.Fatalf("%s %s Count = %d, want 0", label, alias, got)
			}
		}
	}
	if value, err := second.CallStatic("Registry.get", nil); err != nil || value.Int != 0 {
		t.Fatalf("second Registry.get = %#v, %v; want 0", value, err)
	}
	writeStaticCount(t, second, "pkg.Registry", 9)
	if got := staticCount(t, first, "Registry"); got != 7 {
		t.Fatalf("first Count = %d after sibling write, want 7", got)
	}
	if got := staticCount(t, second, "Registry"); got != 9 {
		t.Fatalf("second Count = %d, want 9", got)
	}

	// The writes went to overlays: every runtime still holds the base map.
	for label, machine := range map[string]*VM{"first": first, "second": second, "base": base} {
		if got := classMapPointer(machine.Classes); got != baseMap {
			t.Fatalf("%s class map = %x, want shared base map %x", label, got, baseMap)
		}
	}
	if classValueOverlaySize(first) != 2 || classValueOverlaySize(second) != 2 {
		t.Fatalf("overlay sizes = %d, %d; want the two Registry aliases", classValueOverlaySize(first), classValueOverlaySize(second))
	}
	if classValueOverlaySize(base) != 0 {
		t.Fatal("clone writes reached the base overlay")
	}
	assertClassMapUnchanged(t, "base after clone writes", base.Classes, baseSnapshot)
}

// TestCloneRuntimeFrozenSharedOverlayMatchesFreshMachine runs static writes,
// whole-map ranges and a merging re-registration on a frozen-shared clone and
// on a fresh machine; both must observe the same values.
func TestCloneRuntimeFrozenSharedOverlayMatchesFreshMachine(t *testing.T) {
	fresh := New(nil)
	if err := registerClassMapShareClasses(fresh, 16); err != nil {
		t.Fatal(err)
	}
	base := classMapShareBase(t, 16)
	baseSnapshot := snapshotClassMap(base.Classes)
	clone := base.CloneRuntimeFrozenShared(nil)

	run := func(machine *VM) []string {
		var out []string
		note := func(step string) {
			snapshot := machine.staticFieldSnapshot()
			out = append(out, fmt.Sprintf("%s: short=%d qualified=%d snapshot=%d/%d", step,
				staticCount(t, machine, "Registry"), staticCount(t, machine, "pkg.Registry"),
				snapshot["Registry"]["Count"].Int, snapshot["pkg.Registry"]["Count"].Int))
		}
		call := func(name string) {
			value, err := machine.CallStatic(name, nil)
			out = append(out, fmt.Sprintf("%s=%d err=%v", name, value.Int, err))
		}
		call("Registry.bump")
		call("Registry.bump")
		note("bump")
		writeStaticCount(t, machine, "Registry", 10)
		note("short write")
		call("Registry.get")
		if err := machine.ResetStatics(); err != nil {
			out = append(out, "reset: "+err.Error())
		}
		note("reset")
		call("Registry.bump")
		call("Registry.bump")
		call("Registry.bump")
		note("bump after reset")
		// A re-registration merges with the runtime's current class value,
		// so Count keeps this runtime's value and Extra is added.
		if err := machine.RegisterClass(Class{Name: "Registry", Namespace: "pkg", StaticFields: map[string]Field{
			"Extra": {Name: "Extra", Type: "Integer", Static: true, Value: Int(4), InitialValue: Int(4)},
		}}); err != nil {
			out = append(out, "register: "+err.Error())
		}
		note("merge registration")
		for _, alias := range []string{"Registry", "pkg.Registry"} {
			out = append(out, fmt.Sprintf("%s Extra=%d", alias, machine.Classes[alias].StaticFields["Extra"].Value.Int))
		}
		call("Registry.bump")
		note("bump after registration")
		return out
	}

	want := run(fresh)
	got := run(clone)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frozen-shared clone diverged from fresh machine:\n got %v\nwant %v", got, want)
	}
	assertClassMapUnchanged(t, "base after clone run", base.Classes, baseSnapshot)
	if got := staticCount(t, base, "Registry"); got != 0 {
		t.Fatalf("base Count = %d, want 0", got)
	}
	if _, ok := base.Classes["Registry"].StaticFields["Extra"]; ok {
		t.Fatal("clone registration reached the base")
	}
}

// A managed class does not own the short name of a local class; its writes
// must not reach that alias in the overlay.
func TestCloneRuntimeFrozenSharedOverlayKeepsShortNameCollisions(t *testing.T) {
	counter := func(namespace string, dependency bool, start int64) Class {
		return Class{Name: "Worker", Namespace: namespace, Dependency: dependency, StaticFields: map[string]Field{
			"Count": {Name: "Count", Type: "Integer", Static: true, Value: Int(start), InitialValue: Int(start)},
		}}
	}
	base := New(nil)
	for _, class := range []Class{counter("", false, 1), counter("pkg", true, 100)} {
		if err := base.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	clone := base.CloneRuntimeFrozenShared(nil)

	writeStaticCount(t, clone, "pkg.Worker", 150)
	if local := classValue(clone, "Worker"); local.Dependency || local.StaticFields["Count"].Value.Int != 1 {
		t.Fatalf("short alias after managed write = %+v, want the local class with Count 1", local)
	}
	if got := staticCount(t, clone, "pkg.Worker"); got != 150 {
		t.Fatalf("managed Count = %d, want 150", got)
	}
	writeStaticCount(t, clone, "Worker", 2)
	if got := staticCount(t, clone, "Worker"); got != 2 {
		t.Fatalf("local Count = %d, want 2", got)
	}
	if got := staticCount(t, clone, "pkg.Worker"); got != 150 {
		t.Fatalf("managed Count = %d after local write, want 150", got)
	}
	if got := staticCount(t, base, "Worker"); got != 1 {
		t.Fatalf("base local Count = %d, want 1", got)
	}
	if got := staticCount(t, base, "pkg.Worker"); got != 100 {
		t.Fatalf("base managed Count = %d, want 100", got)
	}
	if classValueOverlaySize(clone) != 2 {
		t.Fatalf("clone overlay holds %d entries, want Worker and pkg.Worker", classValueOverlaySize(clone))
	}
}

func TestCloneRuntimeOfOverlayRuntimeCopiesItsValues(t *testing.T) {
	base := classMapShareBase(t, 16)
	parent := base.CloneRuntimeFrozenShared(nil)
	writeStaticCount(t, parent, "pkg.Registry", 3)
	parentOverlay := classValueOverlaySize(parent)

	frozenChild := parent.CloneRuntimeFrozenShared(nil)
	plainChild := parent.CloneRuntime(nil)
	for label, child := range map[string]*VM{"frozen child": frozenChild, "plain child": plainChild} {
		for _, alias := range []string{"Registry", "pkg.Registry"} {
			if got := staticCount(t, child, alias); got != 3 {
				t.Fatalf("%s %s Count = %d, want the parent's 3", label, alias, got)
			}
		}
	}
	// Cloning reads the parent's overlay without changing the parent.
	if classMapPointer(parent.Classes) != classMapPointer(base.Classes) || classValueOverlaySize(parent) != parentOverlay {
		t.Fatal("cloning changed the parent's class map or overlay")
	}

	writeStaticCount(t, frozenChild, "Registry", 4)
	writeStaticCount(t, plainChild, "Registry", 5)
	writeStaticCount(t, parent, "Registry", 6)
	for label, want := range map[string]struct {
		machine *VM
		count   int64
	}{
		"base":         {base, 0},
		"parent":       {parent, 6},
		"frozen child": {frozenChild, 4},
		"plain child":  {plainChild, 5},
	} {
		for _, alias := range []string{"Registry", "pkg.Registry"} {
			if got := staticCount(t, want.machine, alias); got != want.count {
				t.Fatalf("%s %s Count = %d, want %d", label, alias, got, want.count)
			}
		}
	}
}

func TestFreezeClassLookupFoldsClassValueOverlay(t *testing.T) {
	base := classMapShareBase(t, 16)
	clone := base.CloneRuntimeFrozenShared(nil)
	writeStaticCount(t, clone, "pkg.Registry", 3)
	clone.FreezeClassLookup()
	if classValueOverlaySize(clone) != 0 {
		t.Fatal("freeze kept the overlay")
	}
	if classMapPointer(clone.Classes) == classMapPointer(base.Classes) {
		t.Fatal("freeze left the clone on the base map")
	}
	for _, alias := range []string{"Registry", "pkg.Registry"} {
		if got := clone.Classes[alias].StaticFields["Count"].Value.Int; got != 3 {
			t.Fatalf("refrozen %s Count = %d, want 3", alias, got)
		}
	}
	grandchild := clone.CloneRuntimeFrozenShared(nil)
	if classMapPointer(grandchild.Classes) != classMapPointer(clone.Classes) {
		t.Fatal("clone of the refrozen runtime did not share its map")
	}
	if got := staticCount(t, grandchild, "Registry"); got != 3 {
		t.Fatalf("grandchild Count = %d, want 3", got)
	}
	writeStaticCount(t, grandchild, "Registry", 8)
	if got := staticCount(t, clone, "Registry"); got != 3 {
		t.Fatalf("refrozen Count = %d after grandchild write, want 3", got)
	}
	if got := staticCount(t, base, "Registry"); got != 0 {
		t.Fatalf("base Count = %d, want 0", got)
	}
}

// firstStaticWriteBytes returns the bytes one frozen-shared clone of base
// allocates through its first static write, averaged over runs.
func firstStaticWriteBytes(t *testing.T, base *VM) uint64 {
	t.Helper()
	bump := func() {
		clone := base.CloneRuntimeFrozenShared(nil)
		if value, err := clone.CallStatic("Registry.bump", nil); err != nil || value.Int != 1 {
			t.Fatalf("Registry.bump = %#v, %v; want 1", value, err)
		}
	}
	// The first clone of a base builds shared indexes; measure after it.
	bump()
	const runs = 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		bump()
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / runs
}

func TestCloneRuntimeFrozenSharedFirstStaticWriteCostIndependentOfClassCount(t *testing.T) {
	small := classMapShareBase(t, 1)
	large := classMapShareBase(t, 4000)
	if len(large.Classes) < 8000 {
		t.Fatalf("large base has %d class entries, want at least 8000", len(large.Classes))
	}
	smallBytes := firstStaticWriteBytes(t, small)
	largeBytes := firstStaticWriteBytes(t, large)
	t.Logf("clone plus first static write: small=%d B large=%d B", smallBytes, largeBytes)
	const budget = 64 << 10
	if largeBytes > smallBytes+budget {
		t.Fatalf("clone and first static write over %d classes allocate %d bytes, over %d classes %d; class map is being copied",
			len(large.Classes), largeBytes, len(small.Classes), smallBytes)
	}
}

// BenchmarkFrozenClassLookupCacheWithOverlay is BenchmarkFrozenClassLookupCache
// on a clone whose static write left values in its overlay.
func BenchmarkFrozenClassLookupCacheWithOverlay(b *testing.B) {
	base := classMapShareBase(b, 2000)
	machine := base.CloneRuntimeFrozenShared(nil)
	if _, err := machine.CallStatic("Registry.bump", nil); err != nil {
		b.Fatal(err)
	}
	if classValueOverlaySize(machine) == 0 {
		b.Fatal("static write left no overlay values")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := machine.lookupClass("passive01500"); !ok {
			b.Fatal("class lookup miss")
		}
	}
}

func BenchmarkCloneFrozenSharedFirstStaticWrite(b *testing.B) {
	base := classMapShareBase(b, 4000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone := base.CloneRuntimeFrozenShared(nil)
		if _, err := clone.CallStatic("Registry.bump", nil); err != nil {
			b.Fatal(err)
		}
	}
}
