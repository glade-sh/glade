package apextest

import "testing"

func TestMetadataLayoutPresenceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "metadata_layout_presence", "LayoutPresenceProbeTest", "62.0", "67.0")
}
