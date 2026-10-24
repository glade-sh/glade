package apextest

import "testing"

func TestAsyncHierarchyNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "async_hierarchy", "HierarchyProbeTest", "62.0", "67.0")
}

func TestRelatedFieldRequeryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "related_field_requery", "ParentFieldProbeTest", "62.0", "67.0")
}

func TestConfiguredParentDateNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "configured_parent_date", "DatedValueProbeTest", "62.0", "67.0")
}

func TestCachedParentBatchesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "cached_parent_batches", "BatchSummaryProbeTest", "62.0", "67.0")
}
