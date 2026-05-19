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

func TestValidateOrgShapeIgnoresGeneratedStubProbesForProbeCount(t *testing.T) {
	shape := map[string]interface{}{
		"hasProbeTestObject":  true,
		"hasProbeTestEvent":   true,
		"hasProbeTestMdt":     true,
		"hasProbeTestSetting": true,
		"probeTestObjectRows": float64(3),
		"probeCount":          float64(1),
	}
	probeIDs := []string{
		"stdlib.string.format-null",
		"stub.blob.equals.sig-object",
		"stub.commerceorders-catalogratespreferenceenum.valueof.sig-string",
	}
	if err := validateOrgShape(shape, probeIDs, nil); err != nil {
		t.Fatalf("validateOrgShape returned error: %v", err)
	}
}
