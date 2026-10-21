package apextest

import "testing"

func TestMetadataRelationshipValuesNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "metadata_relationship_values", "MetadataValueTest", []string{"62.0", "67.0"}, 6)
}
