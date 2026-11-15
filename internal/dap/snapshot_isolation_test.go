package dap

import (
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPauseSnapshotIsolatedFromStubProvider(t *testing.T) {
	provider := vm.Object("Provider")
	provider.Fields["__gladeStubProvider"] = vm.Bool(true)
	provider.Fields["count"] = vm.Int(0)
	h := NewHandler(Snapshot{})
	h.ApplyPause(vm.DebugPause{Vars: map[string]vm.Value{"stub": provider}})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			provider.Fields["count"] = vm.Int(int64(i))
		}
	}()
	for i := 0; i < 1000; i++ {
		h.handleVariables(request(1, CommandVariables, map[string]any{"variablesReference": localsReference}))
	}
	wg.Wait()
	if got := h.snapshot.Vars["stub"].Fields["count"].String(); got != "0" {
		t.Fatalf("snapshot changed: %s", got)
	}
}
