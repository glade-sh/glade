package apextest

import "testing"

func TestMetadataChildQueryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "metadata_child_query", "MetadataChildrenTest", []string{"62.0", "67.0"}, 3)
}
