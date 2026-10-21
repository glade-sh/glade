package apextest

import "testing"

func TestProfileIdentityVisibilityNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "profile_identity_visibility", "ProfileVisibilityProbeTest", "62.0", "67.0")
}
