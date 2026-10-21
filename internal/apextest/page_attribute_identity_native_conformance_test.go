package apextest

import "testing"

func TestPageAttributeIdentityNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "page_attribute_identity", "PageAttributeProbeTest", "59.0", "67.0")
}
