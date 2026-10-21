package apextest

import "testing"

func TestStandardExtensionLookupOrgConformance(t *testing.T) {
	runStandardExtensionNative(t, "standard_extension_lookup", 20)
}
