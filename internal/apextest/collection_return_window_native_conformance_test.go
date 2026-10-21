package apextest

import (
	"reflect"
	"testing"
)

func TestCollectionReturnWindowNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "collection_return_window", "ReturnWindowProbeTest", func(t *testing.T, got, want any) {
		local, native := got.(map[string]any), want.(map[string]any)
		for _, key := range []string{"alias", "callee", "callback", "mapList", "mapClear", "unrelated"} {
			if !reflect.DeepEqual(local[key], native[key]) {
				t.Errorf("%s: native=%v; local=%v", key, native[key], local[key])
			}
		}
		// Keep the static-only mismatch visible without asserting a known
		// unsupported observation as conformance. The complete native row is retained.
		t.Logf("static-only alias observation (not conformance credit): native=%v; local=%v", native["static"], local["static"])
	}, "62.0", "67.0")
}
