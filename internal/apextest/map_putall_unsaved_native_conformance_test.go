package apextest

import (
	"reflect"
	"strings"
	"testing"
)

func TestMapPutAllUnsavedNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "map_putall_unsaved", "UnsavedMapProbeTest", func(t *testing.T, got, want any) {
		t.Helper()
		project := func(value any) map[string]any {
			observed, ok := value.(map[string]any)
			if !ok {
				t.Fatalf("invalid observation: %T", value)
			}
			out := make(map[string]any, len(observed))
			for key, item := range observed {
				// Key-set display remains outside this JSON serialization regression.
				if strings.HasSuffix(key, "KeysString") {
					continue
				}
				out[key] = item
			}
			return out
		}
		actual, expected := project(got), project(want)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("native=%v; Glade=%v", expected, actual)
		}
	}, "62.0", "67.0")
}
