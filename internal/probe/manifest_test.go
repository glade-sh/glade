package probe

import "testing"

func TestProbeIDsForTierIncludesCore(t *testing.T) {
	core := ProbeIDsForTier("core")
	if len(core) == 0 {
		t.Fatalf("expected core probe IDs")
	}
	foundBuiltin := false
	for _, id := range core {
		if id == "stdlib.string.format-null" {
			foundBuiltin = true
		}
	}
	if !foundBuiltin {
		t.Fatalf("expected built-in core/smoke probe id in core tier")
	}
}
