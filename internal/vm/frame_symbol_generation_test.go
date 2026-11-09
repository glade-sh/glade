package vm

import (
	"sync"
	"testing"
)

func TestFrameSymbolFirstEntryAndReset(t *testing.T) {
	machine := New(nil)
	first := Method{Name: "First.run"}
	second := Method{Name: "Second.run"}
	for _, method := range []Method{first, first, second, first, second} {
		if got, want := machine.cachedApexMethodFrameSymbol(method), machine.apexMethodFrameSymbol(method); got != want {
			t.Fatalf("symbol for %s = %q, want %q", method.Name, got, want)
		}
	}
	if !machine.frameSymbolFirstValid || len(machine.frameSymbolCache) != 1 {
		t.Fatalf("expected one inline and one additional entry")
	}
	machine.resetClassAccessCaches()
	if machine.frameSymbolFirstValid || machine.frameSymbolFirstKey != (frameSymbolKey{}) || machine.frameSymbolFirstValue != "" || machine.frameSymbolCache != nil {
		t.Fatal("reset did not clear the frame symbol cache")
	}
	if got, want := machine.cachedApexMethodFrameSymbol(second), machine.apexMethodFrameSymbol(second); got != want {
		t.Fatalf("symbol after reset = %q, want %q", got, want)
	}
}

// A freeze re-ranks short-name collisions without resetting the access caches,
// so the frame-symbol memo must answer as the uncached form after it. The
// incremental index lets the last registration of a short name win; the frozen
// index prefers the local class over the dependency. The lowercase class name
// misses the exact Classes key, so it reaches those indexes.
func TestFrameSymbolMemoFollowsClassLookupGeneration(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Foo"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Foo", Namespace: "dep", Dependency: true}); err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "foo.run", ClassName: "foo"}
	before := machine.apexMethodFrameSymbol(method)
	if got := machine.cachedApexMethodFrameSymbol(method); got != before {
		t.Fatalf("memo before freeze = %q, uncached = %q", got, before)
	}
	machine.FreezeClassLookup()
	after := machine.apexMethodFrameSymbol(method)
	if before == after {
		t.Fatalf("fixture must change the symbol across the freeze, got %q on both sides", before)
	}
	if got := machine.cachedApexMethodFrameSymbol(method); got != after {
		t.Fatalf("memo after freeze = %q, uncached = %q", got, after)
	}
}

// The memos live on each VM. Clones of one frozen base start empty and fill
// their own maps, so parallel test methods do not write a shared map (run with
// -race).
func TestClassMemosAreOwnedPerClone(t *testing.T) {
	base := New(nil)
	for _, class := range []Class{{Name: "Foo", Namespace: "dep", Dependency: true}, {Name: "Bar"}} {
		if err := base.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	method := Method{Name: "Foo.run", ClassName: "Foo"}
	base.sameAccessScope("Foo", "Bar")
	base.cachedApexMethodFrameSymbol(method)
	base.lookupClassInNamespace("dep", "Foo")
	base.sourceLineColumnCached("a\nb\n", 3)
	want := base.apexMethodFrameSymbol(method)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		clone := base.CloneRuntime(nil)
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 200; i++ {
				clone.currentClass = "Bar"
				if clone.sameAccessScope("Foo", "Bar") {
					t.Error("Foo and Bar share an access scope")
				}
				if got := clone.cachedApexMethodFrameSymbol(method); got != want {
					t.Errorf("clone symbol = %q, want %q", got, want)
				}
				if _, ok := clone.lookupClassInNamespace("DEP", " foo "); !ok {
					t.Error("clone lost dep.Foo")
				}
				if line, _ := clone.sourceLineColumnCached("a\nb\n", 3); line != 2 {
					t.Errorf("clone line = %d, want 2", line)
				}
			}
		}()
	}
	workers.Wait()
}
