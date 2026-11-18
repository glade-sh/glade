package visualforce

import (
	"encoding/json"
	"sort"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/semanticcache"
)

// The controller gate is a pure function of the project generation key and the
// binary, so its outcome can outlive the process. Persistence is opt-in: the
// commands that honor --no-cache enable it, and every other caller (library
// users, tests) keeps the in-process cache only.
var persistGate atomic.Bool

// SetPersistentGateCache enables or disables reading and writing the on-disk
// controller gate outcome for this process.
func SetPersistentGateCache(enabled bool) { persistGate.Store(enabled) }

type persistedGate struct {
	Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
	Rejected    []string                `json:"rejected"`
}

func loadPersistedControllerGate(key string) (controllerGate, bool) {
	if !persistGate.Load() {
		return controllerGate{}, false
	}
	data, ok := semanticcache.LoadPersistedGate(key)
	if !ok {
		return controllerGate{}, false
	}
	var rec persistedGate
	if err := json.Unmarshal(data, &rec); err != nil {
		return controllerGate{}, false
	}
	rejected := make(map[string]bool, len(rec.Rejected))
	for _, name := range rec.Rejected {
		rejected[name] = true
	}
	return controllerGate{diagnostics: rec.Diagnostics, rejected: rejected}, true
}

func storePersistedControllerGate(key string, gate controllerGate) {
	// A gate that carries an error is never persisted; errors are recomputed.
	if !persistGate.Load() || gate.rejectedErr != nil {
		return
	}
	rec := persistedGate{Diagnostics: gate.diagnostics, Rejected: make([]string, 0, len(gate.rejected))}
	for name, rejected := range gate.rejected {
		if rejected {
			rec.Rejected = append(rec.Rejected, name)
		}
	}
	sort.Strings(rec.Rejected)
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	semanticcache.StorePersistedGate(key, data)
}
