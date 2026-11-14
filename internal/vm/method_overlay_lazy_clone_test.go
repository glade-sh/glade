package vm

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// overlayCloneSource returns a clone of a template with n registered methods in
// its overlay, so cloning it again copies an overlay of that size.
func overlayCloneSource(t testing.TB, n int) *VM {
	t.Helper()
	template := New(nil)
	if err := template.RegisterMethod(Method{Name: "Base.run", ClassName: "Base", IsStatic: true}); err != nil {
		t.Fatal(err)
	}
	source := template.CloneRuntime(nil)
	for i := 0; i < n; i++ {
		if err := source.RegisterMethod(Method{Name: fmt.Sprintf("Case%d.run", i), ClassName: fmt.Sprintf("Case%d", i), IsStatic: true}); err != nil {
			t.Fatal(err)
		}
	}
	if source.methodOverlay == nil {
		t.Fatal("source clone has no method overlay")
	}
	return source
}

func overlayNames(n int) []string {
	names := []string{"Base.run", "Late.run", "Extra.run"}
	for i := 0; i < n; i++ {
		names = append(names, fmt.Sprintf("Case%d.run", i))
	}
	return names
}

func TestMethodOverlayCloneCostIndependentOfOverlaySize(t *testing.T) {
	small := overlayCloneSource(t, 4)
	large := overlayCloneSource(t, 400)
	measure := func(source *VM) float64 {
		return testing.AllocsPerRun(50, func() { _ = source.methodOverlay.clone() })
	}
	smallAllocs, largeAllocs := measure(small), measure(large)
	if largeAllocs > 2 || largeAllocs > smallAllocs {
		t.Fatalf("overlay clone allocs = %.0f (400 methods), %.0f (4 methods); want at most 2 and no growth", largeAllocs, smallAllocs)
	}
}

func TestMethodOverlayCloneRegistrationIsolation(t *testing.T) {
	const n = 20
	names := overlayNames(n)
	source := overlayCloneSource(t, n)
	sourceBefore := snapshotEffective(source, names)

	first := source.CloneRuntime(nil)
	second := source.CloneRuntime(nil)
	if !reflect.DeepEqual(snapshotEffective(first, names), sourceBefore) {
		t.Fatal("clone does not see the source overlay")
	}

	// A clone registration stays out of the source and its sibling.
	late := Method{Name: "Late.run", ClassName: "Late", IsStatic: true}
	if err := first.RegisterMethod(late); err != nil {
		t.Fatal(err)
	}
	shadow := Method{Name: "Case3.run", ClassName: "Case3", IsStatic: true, Params: []Param{{Name: "a", Type: "String"}}}
	if err := first.RegisterMethod(shadow); err != nil {
		t.Fatal(err)
	}
	if _, ok := first.registeredMethod("Late.run"); !ok {
		t.Fatal("clone lost its own registration")
	}
	if got := len(first.registeredOverloads("Case3.run")); got != 2 {
		t.Fatalf("clone Case3.run overloads = %d, want 2", got)
	}
	for label, vm := range map[string]*VM{"source": source, "sibling": second} {
		if !reflect.DeepEqual(snapshotEffective(vm, names), sourceBefore) {
			t.Fatalf("clone registration reached the %s", label)
		}
	}

	// A source registration after cloning stays out of existing clones.
	extra := Method{Name: "Extra.run", ClassName: "Extra", IsStatic: true}
	if err := source.RegisterMethod(extra); err != nil {
		t.Fatal(err)
	}
	for label, vm := range map[string]*VM{"sibling": second} {
		if _, ok := vm.registeredMethod("Extra.run"); ok {
			t.Fatalf("source registration reached the %s", label)
		}
	}
	if _, ok := first.registeredMethod("Extra.run"); ok {
		t.Fatal("source registration reached the first clone")
	}
	if _, ok := source.registeredMethod("Extra.run"); !ok {
		t.Fatal("source lost its own registration")
	}
}

func snapshotEffective(vm *VM, names []string) map[string][]string {
	out := map[string][]string{}
	for _, name := range names {
		var keys []string
		if method, ok := vm.registeredMethod(name); ok {
			keys = append(keys, "m:"+method.Name)
		}
		for _, method := range vm.registeredOverloads(name) {
			keys = append(keys, fmt.Sprintf("o:%s/%d", method.Name, len(method.Params)))
		}
		out[name] = keys
	}
	return out
}

// Removing the copy before write in registerSharedMethod lets a clone write
// into maps its siblings read; this test then fails or trips the race detector.
func TestMethodOverlayCloneConcurrentRegistration(t *testing.T) {
	const n = 30
	source := overlayCloneSource(t, n)
	names := overlayNames(n)
	want := snapshotEffective(source, names)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			clone := source.CloneRuntime(nil)
			for i := 0; i < 20; i++ {
				method := Method{Name: fmt.Sprintf("Worker%d.m%d", worker, i), ClassName: "W", IsStatic: true}
				if err := clone.RegisterMethod(method); err != nil {
					t.Error(err)
					return
				}
				shadow := Method{Name: "Case1.run", ClassName: "Case1", IsStatic: true, Params: []Param{{Name: "p", Type: "Integer"}}}
				if i == 0 {
					if err := clone.RegisterMethod(shadow); err != nil {
						t.Error(err)
						return
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	if !reflect.DeepEqual(snapshotEffective(source, names), want) {
		t.Fatal("concurrent clone registrations changed the source overlay")
	}
}
