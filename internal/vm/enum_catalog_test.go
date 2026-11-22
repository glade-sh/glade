package vm

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

func enumCatalogScan(machine *VM, suffix string) []string {
	var keys []string
	for key, class := range machine.classEntriesNoLazy() {
		dot := strings.LastIndexByte(class.Name, '.')
		if len(class.EnumValues) > 0 && dot >= 0 && dot+1 < len(class.Name) && class.Name[dot+1:] == suffix {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	seen := make(map[string]bool)
	var out []string
	for _, key := range keys {
		class, ok := machine.classEntry(key)
		if ok && !seen[class.Name] {
			seen[class.Name] = true
			out = append(out, key)
		}
	}
	return out
}

func checkEnumCatalog(t *testing.T, machine *VM, suffixes ...string) {
	t.Helper()
	for _, suffix := range suffixes {
		got := machine.enumSuffixClassCandidates(suffix)
		want := enumCatalogScan(machine, suffix)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("suffix %q: got %v, scan %v", suffix, got, want)
		}
		t.Logf("OUTCOME suffix=%q keys=%q", suffix, got)
	}
}

func TestEnumCatalogOutcomeIdentity(t *testing.T) {
	base := New(nil)
	for _, class := range []Class{
		{Name: "Outer.State", EnumValues: []string{"Ready"}},
		{Name: "Outer.State", Namespace: "pkg", Dependency: true, EnumValues: []string{"Ready"}},
		{Name: "Other.State", EnumValues: []string{"Idle"}},
		{Name: "Outer.Mode", EnumValues: []string{"A"}},
		{Name: "Outer.Tone", EnumValues: []string{"Soft"}},
		{Name: "Plain", StaticFields: map[string]Field{"Count": {Name: "Count", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)}}},
	} {
		if err := base.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	first, sibling := base.CloneRuntimeFrozenShared(nil), base.CloneRuntimeFrozenShared(nil)
	checkEnumCatalog(t, first, "State", "Mode", "Missing", "state")
	if err := first.RegisterClass(Class{Name: "Late.State", EnumValues: []string{"New"}}); err != nil {
		t.Fatal(err)
	}
	checkEnumCatalog(t, first, "State", "Mode", "Missing", "state")
	checkEnumCatalog(t, sibling, "State", "Mode", "Missing", "state")
	if err := first.RegisterClass(Class{Name: "Outer.State", EnumValues: []string{"Changed"}}); err != nil {
		t.Fatal(err)
	}
	checkEnumCatalog(t, first, "State", "Mode", "Missing", "state")
	first.FreezeClassLookup()
	if err := first.RegisterClass(Class{Name: "Late.Mode", EnumValues: []string{"New"}}); err != nil {
		t.Fatal(err)
	}
	checkEnumCatalog(t, first, "State", "Mode", "Missing", "state")
	checkEnumCatalog(t, sibling, "State", "Mode", "Missing", "state")
	// Static writeback changes the cache eligibility without changing the
	// class lookup generation. An unqueried suffix must still be found.
	class, ok := sibling.ensureMutableClass("Plain")
	if !ok {
		t.Fatal("static class missing")
	}
	field := class.StaticFields["Count"]
	field.Value = Int(7)
	class.StaticFields["Count"] = field
	sibling.storeClassValue(class)
	checkEnumCatalog(t, sibling, "State", "Mode", "Tone", "Missing", "state")
	for _, machine := range []*VM{base, first} {
		class, ok := machine.classEntry("Plain")
		if !ok || class.StaticFields["Count"].Value.Int != 0 {
			t.Fatal("static write escaped its clone")
		}
	}
}

func TestEnumCatalogConcurrentClones(t *testing.T) {
	base := New(nil)
	if err := base.RegisterClass(Class{Name: "Root.State", EnumValues: []string{"Ready"}}); err != nil {
		t.Fatal(err)
	}
	base.FreezeClassLookup()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		clone := base.CloneRuntimeFrozenShared(nil)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := clone.RegisterClass(Class{Name: fmt.Sprintf("Late%d.State", i), EnumValues: []string{"New"}}); err != nil {
				t.Error(err)
				return
			}
			checkEnumCatalog(t, clone, "State", "Missing")
		}(i)
	}
	wg.Wait()
	checkEnumCatalog(t, base, "State")
}

func TestEnumCatalogDivergentAliases(t *testing.T) {
	base := New(nil)
	if err := base.RegisterClass(Class{Name: "Outer.State", Namespace: "pkg", EnumValues: []string{"Ready"}}); err != nil {
		t.Fatal(err)
	}
	class := base.Classes["Outer.State"]
	class.EnumValues = nil
	base.Classes["Outer.State"] = class
	base.FreezeClassLookup()
	if base.sharedClassCopyPlan.uniformAliases {
		t.Fatal("test requires divergent aliases")
	}
	clone := base.CloneRuntimeFrozenShared(nil)
	checkEnumCatalog(t, clone, "State", "Missing")
	if err := clone.RegisterClass(Class{Name: "Late.State", EnumValues: []string{"New"}}); err != nil {
		t.Fatal(err)
	}
	checkEnumCatalog(t, clone, "State", "Missing")
}

func TestEnumCatalogUnregisteredCountChange(t *testing.T) {
	base := New(nil)
	if err := base.RegisterClass(Class{Name: "Outer.State", EnumValues: []string{"Ready"}}); err != nil {
		t.Fatal(err)
	}
	base.FreezeClassLookup()
	for _, mode := range []string{"root", "overlay", "layer"} {
		t.Run(mode, func(t *testing.T) {
			clone := base.CloneRuntimeFrozenShared(nil)
			if mode != "root" {
				if err := clone.RegisterClass(Class{Name: "Late.State", EnumValues: []string{"New"}}); err != nil {
					t.Fatal(err)
				}
				if mode == "layer" {
					clone.FreezeClassLookup()
				}
			}
			checkEnumCatalog(t, clone, "State", "Phase")
			clone.prepareClassMapWrite()
			clone.Classes["Extra.Phase"] = Class{Name: "Extra.Phase", EnumValues: []string{"Open"}}
			checkEnumCatalog(t, clone, "State", "Phase")
			checkEnumCatalog(t, base, "State", "Phase")
		})
	}
	t.Run("same count before lookup", func(t *testing.T) {
		clone := base.CloneRuntimeFrozenShared(nil)
		clone.prepareClassMapWrite()
		class := clone.Classes["Outer.State"]
		class.EnumValues = nil
		clone.Classes["Outer.State"] = class
		checkEnumCatalog(t, clone, "State", "Phase")
		checkEnumCatalog(t, base, "State", "Phase")
	})
}

func TestEnumCatalogEmptySuffix(t *testing.T) {
	base := New(nil)
	if err := base.RegisterClass(Class{Name: "Outer.State", EnumValues: []string{"Ready"}}); err != nil {
		t.Fatal(err)
	}
	base.FreezeClassLookup()
	clone := base.CloneRuntimeFrozenShared(nil)
	if err := clone.RegisterClass(Class{Name: "Trailing.", EnumValues: []string{"Value"}}); err != nil {
		t.Fatal(err)
	}
	checkEnumCatalog(t, clone, "", "State")
}

func enumCatalogBenchmarkBase(b testing.TB) *VM {
	b.Helper()
	base := New(nil)
	if err := base.RegisterClass(Class{Name: "Counter", StaticFields: map[string]Field{"Count": {Name: "Count", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)}}}); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 5000; i++ {
		class := Class{Name: fmt.Sprintf("Class%05d", i)}
		if i%25 == 0 {
			class.Name += fmt.Sprintf(".Enum%d", i%7)
			class.EnumValues = []string{"Value"}
		}
		if err := base.RegisterClass(class); err != nil {
			b.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	return base
}

func TestEnumCatalogLargeOutcome(t *testing.T) {
	base := enumCatalogBenchmarkBase(t)
	digest := sha256.New()
	count := 0
	for i := 0; i < 100; i++ {
		clone := base.CloneRuntimeFrozenShared(nil)
		for j := 0; j < 7; j++ {
			if err := clone.RegisterClass(Class{Name: fmt.Sprintf("Late%d.Enum%d", i, j), EnumValues: []string{"Value"}}); err != nil {
				t.Fatal(err)
			}
			for _, key := range clone.enumSuffixClassCandidates(fmt.Sprintf("Enum%d", j)) {
				fmt.Fprintf(digest, "%d/%d/%s\n", i, j, key)
				count++
			}
		}
	}
	if count != 100*207 {
		t.Fatalf("candidate count = %d, want %d", count, 100*207)
	}
	got := fmt.Sprintf("%x", digest.Sum(nil))
	const baseline = "b72e604dec92d7d92498c5dd375d83345f625bcdcadcb22a991da9fb1af2c064"
	if got != baseline {
		t.Fatalf("candidate digest = %s, origin/main = %s", got, baseline)
	}
}

func enumCatalogBenchmarkClones(b *testing.B, base *VM, cloneCount int, staticFirst bool) []*VM {
	b.Helper()
	clones := make([]*VM, 0, cloneCount)
	count := 0
	for i := 0; i < cloneCount; i++ {
		clone := base.CloneRuntimeFrozenShared(nil)
		if staticFirst {
			if _, ok := clone.ensureMutableClass("Counter"); !ok {
				b.Fatal("static class missing")
			}
		}
		for j := 0; j < 7; j++ {
			if err := clone.RegisterClass(Class{Name: fmt.Sprintf("Late%d.Enum%d", i, j), EnumValues: []string{"Value"}}); err != nil {
				b.Fatal(err)
			}
			count += len(clone.enumSuffixClassCandidates(fmt.Sprintf("Enum%d", j)))
		}
		clones = append(clones, clone)
	}
	if count != cloneCount*207 {
		b.Fatalf("candidate count = %d, want %d", count, cloneCount*207)
	}
	return clones
}

func BenchmarkEnumCatalogCloneSuite(b *testing.B) {
	base := enumCatalogBenchmarkBase(b)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = enumCatalogBenchmarkClones(b, base, 100, false)
	}
}

func BenchmarkEnumCatalogStaticFirst(b *testing.B) {
	base := enumCatalogBenchmarkBase(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = enumCatalogBenchmarkClones(b, base, 30, true)
	}
}

func BenchmarkEnumCatalogRetained(b *testing.B) {
	runtime.GC()
	var initial, withBase runtime.MemStats
	runtime.ReadMemStats(&initial)
	base := enumCatalogBenchmarkBase(b)
	runtime.GC()
	runtime.ReadMemStats(&withBase)
	var baseRetained uint64
	if withBase.HeapAlloc > initial.HeapAlloc {
		baseRetained = withBase.HeapAlloc - initial.HeapAlloc
	}
	var total uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		b.StartTimer()
		clones := enumCatalogBenchmarkClones(b, base, 100, false)
		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(clones)
		runtime.KeepAlive(base)
		if after.HeapAlloc > before.HeapAlloc {
			total += after.HeapAlloc - before.HeapAlloc
		}
	}
	b.ReportMetric(float64(total)/float64(b.N), "retained-B/op")
	b.ReportMetric(float64(baseRetained), "base-retained-B")
	b.ReportMetric(float64(baseRetained)+float64(total)/float64(b.N), "suite-retained-B/op")
}
