package apextest

import "testing"

func TestPageReferenceQueryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "page_reference_query", "PageUrlSliceTest", []string{"62.0", "67.0"}, 2)
}
