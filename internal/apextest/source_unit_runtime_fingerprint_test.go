package apextest

import (
	"github.com/glade-sh/glade/internal/vm"
	"testing"
)

func TestRuntimePatchFingerprintBindsSourceUnit(t *testing.T) {
	for _, kind := range []string{"method", "class", "field"} {
		t.Run(kind, func(t *testing.T) {
			entry := func(unit string) runtimeCacheEntry {
				switch kind {
				case "method":
					return runtimeCacheEntry{Methods: map[string]vm.Method{"Unit.read": {Name: "Unit.read", SourceUnit: unit}}}
				case "class":
					return runtimeCacheEntry{Classes: []vm.Class{{Name: "Unit", SourceUnit: unit}}}
				default:
					return runtimeCacheEntry{Classes: []vm.Class{{Name: "Unit", StaticFields: map[string]vm.Field{"seed": {Name: "seed", SourceUnit: unit}}}}}
				}
			}
			requireDifferentRuntimePayloadFingerprints(t, entry("first"), entry("second"))
		})
	}
}
