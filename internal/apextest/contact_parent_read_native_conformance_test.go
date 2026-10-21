package apextest

import "testing"

func TestContactParentReadNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "contact_parent_read", "ContactReadProbeTest")
}
