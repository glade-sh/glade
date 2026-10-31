package apextest

import (
	"reflect"
	"testing"
)

func TestAdjacentEmptySObjectHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "adjacent_empty_sobject_hash", "HashAdjacent62Test", func(t *testing.T, got, want any) {
		t.Helper()
		gm, gok := got.(map[string]any)
		wm, wok := want.(map[string]any)
		if !gok || !wok {
			t.Fatal("invalid observation")
		}
		for _, key := range []string{"H01", "H03", "H04"} {
			if !reflect.DeepEqual(gm[key], wm[key]) {
				t.Errorf("%s = %v; native = %v", key, gm[key], wm[key])
			}
		}
	}, "62.0", "67.0")
}
