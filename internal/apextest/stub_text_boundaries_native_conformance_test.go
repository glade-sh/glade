package apextest

import "testing"

func TestStubTextBoundariesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "stub_text_boundaries", "StubTextBoundaryProbeTest", "62.0", "67.0")
}

func TestQueryPageBoundariesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "query_page_boundaries", "QueryPageBoundaryProbeTest", "62.0", "67.0")
}
