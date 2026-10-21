package apextest

import "testing"

func TestIndexedListInsertionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "indexed_list_insertion", "IndexedInsertionProbeTest", "62.0", "67.0")
}
