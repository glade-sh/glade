package apextest

import "testing"

func TestInheritedFinalAssignmentNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "inherited_final_assignment", "InheritedValueChildTest", "62.0", "67.0")
}

func TestStandardLookupPathsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "standard_lookup_paths", "TeamPathProbeTest", "62.0", "67.0")
}

func TestQualifiedPageReferenceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "qualified_page_reference", "QualifiedPageProbeTest", "62.0", "67.0")
}
