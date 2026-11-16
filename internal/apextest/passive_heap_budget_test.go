package apextest

import (
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Registering the passive platform classes through the runner's path retains
// far less heap than registering their full declarations. The platform view and
// the templates are built before the first measurement, so only registration is
// counted.
//
// Heap readings depend on process-wide caches that earlier tests warm, which
// shrinks the full-registration delta. The measurement therefore runs in a
// fresh subprocess: the test binary re-executes itself with an env flag.
// A fresh process measures about 21 MiB; the floor leaves margin for noise.
const passiveHeapFullFloor = 16 << 20

const passiveHeapChildEnv = "GLADE_PASSIVE_HEAP_CHILD"

func TestPassiveStandardRegistrationRetainsLessHeap(t *testing.T) {
	if os.Getenv(passiveHeapChildEnv) == "" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestPassiveStandardRegistrationRetainsLessHeap$", "-test.v")
		cmd.Env = append(os.Environ(), passiveHeapChildEnv+"=1")
		out, err := cmd.CombinedOutput()
		t.Logf("%s", out)
		if err != nil {
			t.Fatalf("fresh-process measurement failed: %v", err)
		}
		return
	}
	measurePassiveStandardHeap(t)
}

func measurePassiveStandardHeap(t *testing.T) {
	storage.EnableStandardCatalogRelease()
	passiveStandardRuntimeClasses(nil, nil)
	retained := func(register func() (*vm.VM, []vm.Class)) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		debug.FreeOSMemory()
		runtime.ReadMemStats(&before)
		machine, classes := register()
		machine.FreezeClassLookup()
		runtime.GC()
		debug.FreeOSMemory()
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
	if full < passiveHeapFullFloor {
		t.Fatalf("full registration retains %d bytes; the measurement is not seeing the passive classes", full)
	}
	if runner > full/3 {
		t.Fatalf("runner registration retains %d bytes, more than a third of full registration (%d)", runner, full)
	}
}
