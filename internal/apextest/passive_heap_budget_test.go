package apextest

import (
	"runtime"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Registering the passive platform classes through the runner's path retains
// far less heap than registering their full declarations. The platform view and
// the templates are built before the first measurement, so only registration is
// counted.
func TestPassiveStandardRegistrationRetainsLessHeap(t *testing.T) {
	storage.EnableStandardCatalogRelease()
	passiveStandardRuntimeClasses(nil, nil)
	retained := func(register func() (*vm.VM, []vm.Class)) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		machine, classes := register()
		machine.FreezeClassLookup()
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(machine)
		runtime.KeepAlive(classes)
		if after.HeapAlloc < before.HeapAlloc {
			return 0
		}
		return after.HeapAlloc - before.HeapAlloc
	}
	register := func(build func() []vm.Class) func() (*vm.VM, []vm.Class) {
		return func() (*vm.VM, []vm.Class) {
			classes := build()
			machine := vm.New(nil)
			if err := registerBaseRuntime(machine, nil, classes, nil); err != nil {
				t.Fatal(err)
			}
			return machine, classes
		}
	}
	full := retained(register(func() []vm.Class { return freshPassiveStandardRuntimeClasses(nil, nil) }))
	runner := retained(register(func() []vm.Class { return passiveStandardRuntimeClasses(nil, nil) }))
	t.Logf("retained heap: full declarations %.1f MiB, runner registration %.1f MiB", float64(full)/(1<<20), float64(runner)/(1<<20))
	if full < 20<<20 {
		t.Fatalf("full registration retains %d bytes; the measurement is not seeing the passive classes", full)
	}
	if runner > full/3 {
		t.Fatalf("runner registration retains %d bytes, more than a third of full registration (%d)", runner, full)
	}
}
