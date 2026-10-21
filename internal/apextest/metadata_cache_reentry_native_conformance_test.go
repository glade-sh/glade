package apextest

import "testing"

func TestMetadataCacheReentryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "metadata_cache_reentry", "MetadataCacheReentryTest", []string{"62.0", "67.0"}, 2)
}
