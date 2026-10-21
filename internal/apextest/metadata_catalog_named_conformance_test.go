package apextest

import "testing"

func TestMetadataCatalogNamedConformance(t *testing.T) {
	runMetadataObservationConformance(t, "metadata_catalog_named.json", 30, 0, 30, 0)
}
