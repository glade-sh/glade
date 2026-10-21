package apextest

import "testing"

func TestTabObjectNamesOrgConformance(t *testing.T) {
	runMetadataObservationConformance(t, "tab_object_names.json", 48, 0, 18, 0)
}
