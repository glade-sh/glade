package apextest

import "testing"

func TestMetadataLanguagePopulationNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "metadata_language_population", "MetadataFieldProbeTest", 5)
}
